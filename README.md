# rcp-api-gateway · 平台唯一对外入口

| 项 | 值 |
|---|---|
| 服务名 | `api-gateway` |
| 端口 | `8080` |
| 实现 | Go 1.22 + Gin 1.10（版本真源：`rcp-ontology/ontology/abox/platform.ttl` 的 `rcp:stack`） |
| 契约 | `rcp-contracts/contracts/openapi/api-gateway.yaml` |
| 依赖 | 唯一 IAM `rcp-iam`（Go/Gin） |

## 职责

1. **唯一入口**：边缘只暴露本网关（红线 RL-002），前端与外部系统都只经 `/api/v1/*`。
2. **鉴权**：受保护请求必须带 `Authorization: Bearer <token>`；令牌由 `rcp-iam` 签发，
   网关逐请求调 `/api/v1/iam/introspect` 判定有效性并取 `permissions`。
   令牌无效 → `401`；缺权限 → `403`；IAM 不可达 → `503`（**fail closed**，绝不降级放行）。
3. **透传**：把 `/api/v1/iam/**` 原样转发给 `rcp-iam`，不改写状态码与响应体。
   地址来自部署侧注入的服务名（`IAM_HOST` / `IAM_PORT`），按 k8s/compose DNS 解析；
   多实例负载由 Service 承担，网关不做轮转。

## 接口

```
*    /api/v1/iam/**            # 透传到 rcp-iam（登录、内省、用户管理由 IAM 自行校验）
GET  /actuator/health          # 探活（非契约路由）
```

`/api/v1/iam/**` 在网关侧**不做鉴权**：`rcp-iam` 是平台唯一的身份与权限判定点，
网关只搬运请求与响应，不改写 401/403/409 语义。

## 配置（只认环境变量）

| 变量 | 默认 | 说明 |
|---|---|---|
| `SERVER_PORT` | `8080` | 监听端口 |
| `IAM_AUTH_ENABLED` | `true` | 关闭后不鉴权，仅本地联调；生产必须为 `true` |
| `IAM_PUBLIC_PATHS` | `/actuator/**` | 免令牌路径，逗号分隔，支持尾部 `/**` |
| `GATEWAY_WEB_ALLOWED_ORIGINS` | `http://localhost:5173,http://127.0.0.1:5173` | CORS 白名单，不支持 `*` |
| `IAM_HOST` / `IAM_PORT` | `localhost` / `8085` | IAM 地址（部署侧传 Service 名 `iam`） |

清单里只出现服务名与端口，不出现 `http://<内部服务>:<端口>`（RL-007）。

## 构建与验证

```bash
go mod verify && go vet ./... && go test ./... && go build ./...

python ../rcp-contracts/tools/contract_check.py \
  --repo . --contract ../rcp-contracts/contracts/openapi/api-gateway.yaml
```

## 全链路冒烟（可选）

```bash
go run . &                                          # 网关 :8080
(cd ../rcp-iam && go run .) &                       # IAM :8085

# 取令牌（演示账号见 rcp-iam/README.md）
TOKEN=$(curl -s --noproxy '*' -X POST localhost:8080/api/v1/iam/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"operator","password":"Operator@12345"}' | sed -n 's/.*"accessToken":"\([^"]*\)".*/\1/p')

# 带令牌调内省
curl --noproxy '*' -X POST localhost:8080/api/v1/iam/introspect \
  -H 'Content-Type: application/json' -H "Authorization: Bearer $TOKEN" \
  -d "{\"token\":\"$TOKEN\"}"
```

## 相对 Java 版的行为差异（破坏性，已获准）

| 项 | Java 版 | 现在 |
|---|---|---|
| 实现栈 | Spring Boot 3.3 + JDK 17 | Go 1.22 + Gin 1.10 |
| 共享基座 | 依赖共享库 `rcp-core`（**该仓已下线**） | 无该依赖 |
| 服务发现 | Nacos 客户端按注册名解析 + `endpoints` 兜底 | 不接注册中心；按部署侧 Service 名 DNS 解析 |
| 多实例 | 网关侧轮转各实例 | 由 k8s Service / compose DNS 分发 |
| 探针 | `/actuator/health` + readiness/liveness | 仅 `/actuator/health` |
