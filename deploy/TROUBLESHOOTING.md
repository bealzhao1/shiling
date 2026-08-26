# Shiling 部署问题排查与最终方案

> 记录 `bealzhao.cn` 部署 K3s + Traefik + nginx 反代过程中遇到的关键问题、根因分析和最终解法。

---

## 1. 项目背景

- **域名**：`bealzhao.cn`
- **服务器**：腾讯云 CVM（单节点 K3s）
- **入口**：K3s 自带 Traefik 作为 Ingress Controller
- **后端**：`shiling`（飞花令 Agent），运行在 `shiling` namespace
- **目标**：`https://bealzhao.cn` 正常访问，80 自动跳转 HTTPS，SSE 流式输出正常

---

## 2. 问题总览

| 阶段 | 现象 | 根因 |
|---|---|---|
| GitHub Actions 部署 | `kubectl` 连不上集群，报错 `localhost:8080 connection refused` | `KUBECONFIG` Secret 值需 base64 / 名称不一致 |
| 域名访问 | 访问 `bealzhao.cn` 显示 nginx 欢迎页 | Ingress `nginx-ingress` 将流量路由到了 `default/nginx` |
| 端口排查 | 宿主机 `ss -tlnp` 看不到 80/443 监听 | K3s `svclb-traefik` 通过 `hostPort` + iptables DNAT 暴露端口 |
| 应用访问 | 前端加载后提示"未设置 API Key" | `shiling-secrets` 中 `HY_API_KEY` 未注入真实值 |

---

## 3. GitHub Actions 连接 K8s 失败

### 现象

```text
failed to download openapi: Get "http://localhost:8080/openapi/v2?timeout=32s":
dial tcp [::1]:8080: connect: connection refused
```

### 根因

`kubectl` 没有读取到有效的 kubeconfig，回退到默认地址 `localhost:8080`。

### 解法

参考仓库 `web-manager` 的 `ci.yaml` 做法，kubeconfig **原文直接写入**，不再 base64 编码。

GitHub Secrets 设置：

| Secret | 说明 |
|---|---|
| `KUBE_CONFIG` | 本地 `cat ~/.kube/config` 的原文 |
| `HY_API_KEY` | 腾讯混元 API Key（默认模型，必填） |
| `DEEPSEEK_API_KEY` / `OPENAI_API_KEY` / ... | 可选 |

对应 workflow 片段：

```yaml
- name: Prepare kubeconfig
  run: |
    mkdir -p "$HOME/.kube"
    echo "${{ secrets.KUBE_CONFIG }}" > "$HOME/.kube/config"
    chmod 600 "$HOME/.kube/config"
```

---

## 4. 访问域名显示 nginx 欢迎页

### 现象

- `bealzhao.cn` 能通，但返回 `Welcome to nginx!`
- 宿主机执行 `ss -tlnp | grep ':80\s'` 没有输出

### 根因分析

链路如下：

```text
用户 → https://bealzhao.cn:443
  → 公网 IP 1.13.190.181
  → iptables DNAT
  → svclb-traefik Pod (10.42.0.7)
  → Traefik Pod (10.42.0.8)
  → Ingress nginx-ingress (host=bealzhao.cn)
  → nginx Service (default/nginx)
  → nginx Pod (nginx:alpine，默认配置)
  → 返回 nginx 欢迎页
```

K3s 中 Traefik 的 LoadBalancer 通过 `svclb-traefik`（klipper-lb）实现，它使用 `hostPort` 方式暴露 80/443，所以 `ss -tlnp` 看不到监听，但 iptables 中存在 DNAT 规则：

```text
DNAT tcp -- 0.0.0.0/0 0.0.0.0/0 tcp dpt:80 to:10.42.0.7:80
DNAT tcp -- 0.0.0.0/0 0.0.0.0/0 tcp dpt:443 to:10.42.0.7:443
```

`nginx-ingress` 的 backend 指向 `default/nginx`，所以流量最终到了裸 Pod `nginx`，该 Pod 没有配置反代，直接返回欢迎页。

### 排查命令

```bash
# 查看所有 Pod，确认 svclb-traefik / traefik / nginx 的位置
kubectl get pods -A -o wide

# 查看 iptables 中的 DNAT 规则
iptables -t nat -L -n | grep -E ':80|:443'

# 查看 Ingress 路由
kubectl get ingress -A
```

---

## 5. 最终方案：nginx 作为反向代理

### 设计思路

保留现有入口：

- `bealzhao.cn` 的 DNS 指向 `1.13.190.181`
- Traefik 继续处理 TLS 终止和 HTTP → HTTPS 跳转
- `nginx-ingress` 继续指向 `default/nginx`
- 在 `default/nginx` Pod 内部配置反代到 `shiling` Service

链路变为：

```text
用户 → https://bealzhao.cn
  → Traefik (TLS 终止、跳转)
  → nginx Pod
  → proxy_pass → shiling.shiling.svc.cluster.local:80
```

### 为什么不直接改 Ingress 指向 shiling？

直接改 Ingress 更简洁（`Traefik → shiling`），但当前环境已经存在一个完整工作的 nginx 入口（含默认欢迎页）。保留 nginx 可以在未来方便地：

- 添加多个路径规则
- 做静态资源缓存
- 配置自定义错误页

如果不需要这些，也可以直接删除 `nginx-ingress` 并创建指向 `shiling` Service 的 Ingress。

### nginx 配置（ConfigMap）

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: nginx-conf
  namespace: default
data:
  default.conf: |
    server {
        listen 80;
        server_name _;

        location / {
            proxy_pass http://shiling.shiling.svc.cluster.local:80;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;

            # SSE 流式输出必需
            proxy_http_version 1.1;
            proxy_buffering off;
            proxy_cache off;
            proxy_read_timeout 3600s;
            proxy_send_timeout 3600s;
        }
    }
```

### 使用 Deployment 替换裸 Pod

原来的 `nginx` 是裸 Pod（`kubectl run` 创建），配置不持久、挂了不自动重启。改用 Deployment + ConfigMap 挂载：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: default
  labels:
    run: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      run: nginx
  template:
    metadata:
      labels:
        run: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        ports:
        - containerPort: 80
        volumeMounts:
        - name: conf
          mountPath: /etc/nginx/conf.d
      volumes:
      - name: conf
        configMap:
          name: nginx-conf
```

关键点：

- `label: run: nginx` 必须保持与 `default/nginx` Service 的 `selector` 一致
- 删除旧裸 Pod 后再创建 Deployment，避免 label 冲突

### 一键部署脚本

```bash
# 删除裸 Pod
kubectl delete pod nginx -n default --grace-period=0 --force

# 应用 ConfigMap + Deployment
kubectl apply -f nginx-conf.yaml
kubectl apply -f nginx-deployment.yaml

# 验证
kubectl get pods -n default -o wide | grep nginx
curl -i https://bealzhao.cn
```

---

## 6. "未设置 API Key" 问题

### 现象

页面能加载，但发送消息后提示：

```text
未设置 API Key（请检查对应模型配置的 api_key / api_key_env，或本地服务设置 no_auth: true）
```

### 根因

`shiling` 默认模型是 `hy`（腾讯混元），需要环境变量 `HY_API_KEY`。Deployment 通过 `envFrom.secretRef` 从 `shiling-secrets` 读取：

```yaml
envFrom:
  - secretRef:
      name: shiling-secrets
```

GitHub Actions 会创建该 Secret，但如果：

- Actions 尚未成功运行
- `HY_API_KEY` Secret 未在 GitHub 设置
- 手动部署时未替换 `REPLACE_ME`

都会导致该问题。

### 解法

**临时方案（服务器上手动注入）**：

```bash
kubectl -n shiling create secret generic shiling-secrets \
  --from-literal=HY_API_KEY="你的真实腾讯混元Key" \
  --from-literal=DEEPSEEK_API_KEY="" \
  --from-literal=OPENAI_API_KEY="" \
  --from-literal=ANTHROPIC_API_KEY="" \
  --from-literal=MOONSHOT_API_KEY="" \
  --from-literal=DASHSCOPE_API_KEY="" \
  --dry-run=client -o yaml | kubectl apply -f -

# 重启 shiling 使环境变量生效
kubectl -n shiling rollout restart deployment/shiling
```

**长期方案（GitHub Actions）**：

在仓库 `Settings → Secrets and variables → Actions → Secrets` 中添加 `HY_API_KEY`，然后 push 触发 deploy。

---

## 7. 当前最终架构

```text
[用户浏览器]
    │
    │ https://bealzhao.cn
    ▼
[公网 IP 1.13.190.181]
    │
    │ iptables DNAT (hostPort)
    ▼
[svclb-traefik Pod]
    │
    ▼
[Traefik Pod]
    │
    │ Ingress: nginx-ingress (host=bealzhao.cn, TLS=demo-tls-secret)
    │ Middleware: redirect-https (80 → 443)
    ▼
[nginx Service (default/nginx)]
    │
    ▼
[nginx Pod (Deployment 管理)]
    │
    │ proxy_pass
    ▼
[shiling Service (shiling/shiling)]
    │
    ▼
[shiling Pod]  ← envFrom: shiling-secrets (HY_API_KEY)
```

---

## 8. 后续维护建议

1. **不要用裸 Pod**：入口组件改用 Deployment 管理，保证高可用和可复现
2. **Secret 不要提交到仓库**：`deploy/k8s/secret.yaml` 仅作占位模板，真实值通过 GitHub Actions 注入或手动创建
3. **保留 `nginx-ingress` 或迁移到直接 Ingress**：
   - 当前保留 nginx 反代，便于未来扩展
   - 如果不需要 nginx 层，可直接删除 `nginx-ingress`，改 Ingress backend 到 `shiling` Service
4. **HTTPS 跳转**：已通过 `default/redirect-https` Middleware 实现，无需 nginx 处理
5. **SSE 配置**：nginx 反代务必关闭 `proxy_buffering`，否则流式输出会卡住

---

## 9. 常用调试命令

```bash
# 查看 Pod 分布和 IP
kubectl get pods -A -o wide

# 查看 Ingress 规则
kubectl get ingress -A

# 查看服务端口映射
kubectl get svc -A

# 查看 iptables DNAT（80/443 怎么进的容器）
iptables -t nat -L -n | grep -E ':80|:443'

# 查看 Secret 是否存在（不暴露值）
kubectl -n shiling get secret shiling-secrets

# 查看 Pod 环境变量
kubectl -n shiling exec -it deployment/shiling -- env | grep API_KEY

# 测试 shiling 内部是否可达
kubectl -n default exec -it deployment/nginx -- wget -qO- http://shiling.shiling.svc.cluster.local/healthz
```
