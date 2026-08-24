# syntax=docker/dockerfile:1

# ===================== 构建阶段 =====================
FROM golang:1.21-alpine AS builder

WORKDIR /src

# 复制全部源码（本项目零外部依赖，无需 go mod download）
COPY . .

# 静态编译，去掉调试信息并裁剪路径，减小体积
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/shiling .

# ===================== 运行阶段 =====================
FROM alpine:3.20

# CA 证书（访问 HTTPS 上游必需）+ 时区 + 非 root 用户（uid=1000）
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S app && adduser -S -u 1000 -G app app

WORKDIR /app

# 二进制
COPY --from=builder /out/shiling /app/shiling

# 静态资源：前端页面 + 技能文件
COPY --from=builder /src/web /app/web
COPY --from=builder /src/skills /app/skills

# 默认配置（K8s 下会被 ConfigMap 挂载覆盖；本地 docker run 可直接用）
COPY --from=builder /src/config.json /app/config.json

USER app

EXPOSE 8080

ENTRYPOINT ["/app/shiling"]
