# rcp-api-gateway · 平台唯一对外入口

| 项 | 值 |
|---|---|
| 服务名 | `api-gateway` |
| 端口 | `8080` |
| 实现 | Go 1.22 + Gin 1.10（版本真源：`rcp-ontology/ontology/abox/platform.ttl` 的 `rcp:stack`） |
| 契约 | `rcp-contracts/contracts/openapi/api-gateway.yaml` |
| 依赖 | 下游四条巡查线与唯一 IAM `rcp-iam`（Go/Gin）；**不再依赖 `rcp-core`**（Java 库） |

## 职责

1. **唯一入口**：边缘只暴露本网关（红线 RL-002），前端与外部系统都只经 `/api/v1/*`。
2. **鉴权**：受保护请求必须带 `Authorization: Bearer <token>`；令牌由 `rcp-iam` 签发，
   网关逐请求调 `/api/v1/iam/introspect` 判定有效性并取 `permissions`。
   令牌无效 → `401`；缺权限 → `403`；IAM 不可达 → `503`（**fail closed**，绝不降级放行）。
3. **消费幂等**：`event_id + version` 作为幂等键；同键新版本才生效，旧版本回放直接拒绝（`STALE_VERSION_REPLAY`）。
4. **状态机单调前进**：决策按秩只升不降，回跳被拦截并置 `stateRollbackProtected=true`。
5. **分级处置**：按决策映射处置动作；**巡查线不可用时不放过**，降级转加审（`EXTRA_REVIEW`，`reason=PATROL_SERVICE_DEGRADED`）。
6. **路由**：`domain` → 巡查线，地址来自部署侧注入的服务名（`PATROL_<DOMAIN>_HOST/_PORT`），
   按 k8s/compose DNS 解析；多实例负载由 Service 承担，网关不再自己做轮转。

## 接口

```
POST /api/v1/patrol/dispatch   # 统一巡查入口（需 patrol:dispatch 权限）
GET  /api/v1/patrol/domains    # 平台自描述（公开，不需令牌）
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
| `IAM_PUBLIC_PATHS` | `/actuator/**,/api/v1/patrol/domains` | 免令牌路径，逗号分隔，支持尾部 `/**` |
| `PATROL_WEB_ALLOWED_ORIGINS` | `http://localhost:5173,http://127.0.0.1:5173` | CORS 白名单，不支持 `*` |
| `IAM_HOST` / `IAM_PORT` | `localhost` / `8085` | IAM 地址（部署侧传 Service 名 `iam`） |
| `PATROL_CONTENT_HOST` / `_PORT` | `localhost` / `8081` | 内容巡查线 |
| `PATROL_MESSAGE_HOST` / `_PORT` | `localhost` / `8082` | 消息巡查线 |
| `PATROL_ACCOUNT_HOST` / `_PORT` | `localhost` / `8083` | 账号巡查线 |
| `PATROL_FUND_HOST` / `_PORT` | `localhost` / `8084` | 资金巡查线 |

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
(cd ../rcp-content-patrol && mvn -B -q spring-boot:run) &   # 内容巡查线 :8081

# 1) 取令牌（演示账号见 rcp-iam/README.md）
TOKEN=$(curl -s --noproxy '*' -X POST localhost:8080/api/v1/iam/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"operator","password":"Operator@12345"}' | sed -n 's/.*"accessToken":"\([^"]*\)".*/\1/p')

# 2) 带令牌提交
curl --noproxy '*' -X POST localhost:8080/api/v1/patrol/dispatch \
  -H 'Content-Type: application/json' -H "Authorization: Bearer $TOKEN" \
  -d '{"eventId":"job:1","domain":"content","version":1,"payload":{"jobId":1,"city":"杭州","trade":"装修","title":"招瓦工","body":"招瓦工日结300"}}'
```

## 相对 Java 版的行为差异（破坏性，已获准）

| 项 | Java 版 | 现在 |
|---|---|---|
| 实现栈 | Spring Boot 3.3 + JDK 17 | Go 1.22 + Gin 1.10 |
| 共享基座 | 依赖 `rcp-core` 钉版 `v1.0.0`（`Decision`/`PatrolVerdict`） | 无该依赖；决策序与处置映射在本仓 `internal/model`，语义真源仍是 `decisions.ttl` |
| 服务发现 | Nacos 客户端按注册名解析 + `endpoints` 兜底 | 不接注册中心；按部署侧 Service 名 DNS 解析 |
| 多实例 | 网关侧轮转各实例 | 由 k8s Service / compose DNS 分发 |
| 探针 | `/actuator/health` + readiness/liveness | 仅 `/actuator/health` |
| 请求校验 | `version` 仅校验非空 | `version` 必须是 ≥ 1 的整数（对齐契约 `minimum: 1`），否则 `400` |
| 幂等/状态机存储 | 进程内 `ConcurrentHashMap` | 进程内 map（加锁，判读写在同一次临界区）。**同样重启失效、无 TTL** |
