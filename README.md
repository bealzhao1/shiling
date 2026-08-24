# 诗词飞花令 Agent 🌸

基于 **可切换大模型 + SKILL.md 技能注入 + function calling + SSE 流式输出** 的诗词对话 Agent，
提供 **网页版对话框**（类 ChatGPT / DeepSeek）与命令行调试两种模式。

## 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│  前端  web/index.html                                        │
│  ChatGPT 风格对话框 · 流式打字 · 工具调用提示 · 模型切换       │
└──────────────────────────┬──────────────────────────────────┘
                           │  POST /api/chat   （SSE 流式响应）
                           ▼
┌─────────────────────────────────────────────────────────────┐
│  网关层  internal/gateway                                    │
│  · HTTP 路由（/api/chat /api/models /api/reset）             │
│  · 会话管理（session 绑定 Agent + TTL 过期清理）              │
│  · SSE 封装（event 帧 + flush）                              │
│  · 前端静态文件托管                                           │
└──────────────────────────┬──────────────────────────────────┘
                           │  调用 StreamChat(emit)
                           ▼
┌─────────────────────────────────────────────────────────────┐
│  Agent 层  internal/agent                                    │
│  · 加载 skill → 注入 system 提示                             │
│  · 维护对话历史（含并发保护）                                 │
│  · function calling 多轮编排                                 │
│  · 流式回调（delta / tool_call / done / error）              │
└──────────────────────────┬──────────────────────────────────┘
                           │  调用 ChatStream
                           ▼
┌─────────────────────────────────────────────────────────────┐
│  LLM 客户端  internal/llm                                    │
│  OpenAI 兼容协议 · 流式 SSE 解析 · tool_calls 增量累积        │
└─────────────────────────────────────────────────────────────┘
```

## 项目结构

```
shiling/
├── main.go                    # 入口：Web 网关 / CLI 双模式
├── config.json                # ★ 模型配置文件：多模型清单 + 默认模型
├── Dockerfile                 # 多阶段构建（golang → alpine 非 root）
├── .dockerignore              # 构建排除清单
├── .github/workflows/deploy.yml # CI/CD 流水线（build → deploy）
├── web/index.html             # ★ 前端对话框（纯 HTML/CSS/JS，零依赖）
├── skills/shiling/SKILL.md     # ★ 飞花令技能手册（Agent 行为依据）
├── deploy/k8s/                # ★ Kubernetes 部署清单
│   ├── namespace.yaml         # 命名空间
│   ├── configmap.yaml         # 模型配置（不含密钥，挂载覆盖镜像默认值）
│   ├── secret.yaml            # 密钥模板（占位符，生产用 CI 变量/外部密钥）
│   ├── deployment.yaml        # 副本 + 探针 + 资源 + 安全上下文
│   ├── service.yaml           # ClusterIP
│   └── ingress.yaml           # 对外入口（含 SSE 优化注解）
└── internal/
    ├── gateway/gateway.go     # 网关层：路由 + 会话 + SSE + 静态文件 + /healthz
    ├── agent/agent.go         # Agent 层：编排 + 工具循环 + 流式回调
    ├── llm/llm.go             # LLM 客户端：流式/非流式 + function calling
    ├── config/config.go       # 配置加载器（JSON）
    ├── skills/skills.go       # SKILL.md 解析器
    ├── poems/poems.go         # 内置诗词库（按令字检索）
    └── tools/tools.go         # 工具注册表（search_poems）
```

## 快速开始

```bash
# 1. 设置默认模型对应的 API Key（默认混元 hy）
export HY_API_KEY=sk-你的key

# 2. 启动 Web 网关（默认 :8080）
go run .

# 3. 浏览器打开
#    http://localhost:8080
```

```bash
# 指定监听地址 / 配置文件 / 前端目录
go run . -addr :9090 -config ./my-config.json -web ./web

# 命令行调试模式（不启动 Web）
go run . -cli
```

## 服务端接口

| 接口 | 方法 | 说明 |
|---|---|---|
| `/api/chat` | POST | 对话核心接口，返回 SSE 流。请求体 `{"message":"...","session_id":"...","model":"..."}` |
| `/api/models` | GET | 返回模型清单：`{"default":"hy","models":{...}}` |
| `/api/reset` | POST | 重置指定会话历史：`{"session_id":"..."}` |
| `/healthz` | GET | 健康检查（K8s liveness/readiness 探针） |
| `/` | GET | 前端页面 |

### SSE 事件协议

服务端按 `event: <type>` + `data: <json>` 成帧推送，前端据此渲染：

| event | data 字段 | 说明 |
|---|---|---|
| `meta` | `session_id`, `model`, `upstream` | 首帧：会话元信息 |
| `delta` | `content` | 文本增量（前端流式追加，打字机效果） |
| `tool_call` | `name`, `arguments` | 模型发起的工具调用（前端显示 🔧 卡片） |
| `done` | - | 本轮完成 |
| `error` | `message` | 出错（前端显示红色提示） |
| `close` | - | 连接关闭 |

### 会话机制

- 前端首次请求不带 `session_id`，服务端生成并随 `meta` 帧返回
- 前端将 `session_id` 存入 `localStorage`，后续请求带上以保持上下文
- 服务端 30 分钟未活跃的会话会被后台 goroutine 自动清理

## 模型配置（config.json）

```json
{
  "default_model": "hy",
  "models": {
    "hy": {
      "provider": "tencent",
      "base_url": "https://tokenhub.tencentmaas.com/v1",
      "model": "hy",
      "api_key_env": "HY_API_KEY",
      "description": "腾讯混元大模型（默认）"
    }
  }
}
```

| 字段 | 必填 | 说明 |
|---|---|---|
| `default_model` | 否 | 启动时使用的模型 key |
| `models.<key>` | 是 | 模型标识，运行时用 `/model <key>` 或前端下拉切换 |
| `provider` | 是 | 厂商标识（日志用，不发送上游） |
| `base_url` | 是 | API 网关地址，**不含** `/chat/completions` |
| `model` | 是 | 上游真实模型名 |
| `api_key` | 否 | 直接写死 key（不推荐，仅测试） |
| `api_key_env` | 否 | 从此环境变量读取 key |
| `description` | 否 | 前端下拉框展示的备注 |

`config.json` 已预置 8 个模型：混元 hy、DeepSeek ×2、GPT-4o ×2、Claude、Kimi、通义千问、本地 Ollama。
前端顶部下拉框可**实时切换模型**，切换后下一轮请求即走新模型（上下文不丢失）。

## 支持的能力

| 输入 | 触发行为 |
|---|---|
| `以"月"行令` | 飞花令：输出含"月"的诗句（查本地库 + 知识补充） |
| `接龙"床前明月光"` | 提取末字"光"，接出含"光"的诗句 |
| `这句诗什么意思` | 三段式解读：释义 / 出处 / 赏析 |
| `推荐写思乡的诗` | 按主题推荐 2~3 首 |
| `考考你，含"花"的边塞诗` | 互动对诗 |

## 工作原理（Agent 核心）

```
1. 前端 POST /api/chat → 网关解析 → 获取/新建 session → 交给 agent.StreamChat
2. agent 把用户消息入历史 → 调 llm.ChatStream（流式）
3. 模型流式返回文本 → agent 逐 token emit "delta" → 网关转 SSE → 前端打字机渲染
4. 若模型请求工具 → emit "tool_call" → 执行 search_poems → 结果回填历史 → 回到步骤 2
5. 直到模型给出纯文本 → emit "done" → 前端本轮结束
```

## 扩展指南

- **加诗**：编辑 `internal/poems/poems.go` 的 `all` 切片
- **改行为**：编辑 `skills/shiling/SKILL.md`
- **加工具**：在 `internal/tools/tools.go` 的 `Defs` 注册 + `Execute` 分发
- **加模型**：往 `config.json` 的 `models` 加一条
- **接入任意 OpenAI 兼容服务**：只要支持 `/chat/completions` + Bearer + `stream` + `tools` 即可直接接入

## 容器化部署（Docker）

### 构建镜像

```bash
docker build -t shiling:latest .
```

镜像特点：

- **多阶段构建**：`golang:1.21-alpine` 编译 → `alpine:3.20` 运行，最终镜像仅含静态二进制 + `web/` + `skills/` + 默认 `config.json`
- **非 root 运行**：以 uid=1000 的 `app` 用户启动
- **零外部依赖**：纯标准库，静态编译（`CGO_ENABLED=0`），无运行时依赖

### 本地运行容器

```bash
docker run --rm -p 8080:8080 \
  -e HY_API_KEY=sk-你的key \
  shiling:latest
# 浏览器打开 http://localhost:8080
```

## Kubernetes 流水线部署

### 部署架构

```
Git 推送 (main/master)
   │
   ▼
GitHub Actions 流水线（.github/workflows/deploy.yml）
   ├── build  阶段：docker build → push 到 GHCR（tag=提交 SHA）
   └── deploy 阶段：从 Secrets 创建 Secret → sed 替换镜像 tag → kubectl apply → 滚动更新
                          │
                          ▼
┌──────────────────────────────────────────────┐
│  Ingress（nginx，SSE 关闭缓冲）                │
│        │                                      │
│        ▼                                      │
│  Service (ClusterIP :80 → :8080)              │
│        │                                      │
│        ▼                                      │
│  Deployment（replicas=2）                      │
│     · /healthz 探针                            │
│     · ConfigMap 挂载 /app/config.json         │
│     · Secret 注入 API Key 环境变量             │
└──────────────────────────────────────────────┘
```

### 手动部署（本地 kubectl）

```bash
# 1) 创建命名空间 + 模型配置（ConfigMap，不含密钥）
kubectl apply -f deploy/k8s/namespace.yaml
kubectl apply -f deploy/k8s/configmap.yaml

# 2) 注入真实密钥（勿提交明文到 Git）
kubectl -n shiling create secret generic shiling-secrets \
  --from-literal=HY_API_KEY='sk-xxx' \
  --from-literal=DEEPSEEK_API_KEY='sk-xxx'

# 3) 构建并推送镜像到你的仓库，替换 deployment.yaml 中的 image 后部署
docker build -t <registry>/shiling:latest .
docker push <registry>/shiling:latest
# 编辑 deploy/k8s/deployment.yaml 的 image 为 <registry>/shiling:latest
kubectl apply -f deploy/k8s/deployment.yaml
kubectl apply -f deploy/k8s/service.yaml
kubectl apply -f deploy/k8s/ingress.yaml   # 记得改 ingress 里的域名

# 4) 查看状态
kubectl -n shiling rollout status deployment/shiling
kubectl -n shiling get pods
```

### 流水线前置配置（GitHub Actions）

| 配置项 | 位置 | 说明 |
|---|---|---|
| `HY_API_KEY` 等 6 个密钥 | Settings → Secrets and variables → Actions → Secrets | 真实密钥，由流水线注入 Secret |
| `KUBECONFIG` | 同上（base64 编码后存入） | 目标 K8s 集群凭证 |
| `GITHUB_TOKEN` | 内置 | 推送镜像到 GHCR 自动可用 |
| 域名 | `deploy/k8s/ingress.yaml` + `.github/workflows/deploy.yml` | 改成真实域名 |

推送到 `main`/`master` 即触发「构建 → 部署」，`rollout status` 会等待滚动更新完成并失败即中断；也可在 Actions 页面手动触发（`workflow_dispatch`）。

### 安全要点

- **密钥不入库**：`secret.yaml` 仅是占位模板，真实 Key 走 CI 变量 / 外部密钥管理（SealedSecrets、External Secrets）
- **ConfigMap 只放配置**：`base_url` / `model` / `api_key_env` 引用，不写明文 Key
- **最小权限容器**：非 root + `allowPrivilegeEscalation: false` + 丢弃所有 capabilities
- **资源限制**：CPU/内存 request/limit，避免单 Pod 打爆节点

## 测试

```bash
go build ./...
go vet ./...
go test ./...
```
