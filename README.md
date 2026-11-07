# rcp-api-gateway · 平台唯一对外入口

| 项 | 值 |
|---|---|
| 服务名 | `api-gateway` |
| 端口 | `8080` |
| 契约 | `rcp-contracts/contracts/openapi/api-gateway.yaml` |
| 依赖 | `rcp-core` 钉版 `v1.0.0`；下游四条巡查线与唯一 IAM `rcp-iam`（Go/Gin） |

## 职责

1. **唯一入口**：边缘只暴露本网关（红线 RL-002），前端与外部系统都只经 `/api/v1/*`。
2. **鉴权**：受保护请求必须带 `Authorization: Bearer <token>`；令牌由 `rcp-iam` 签发，
   网关逐请求调 `/api/v1/iam/introspect` 判定有效性并取 `permissions`。
   令牌无效 → `401`；缺权限 → `403`；IAM 不可达 → `503`（**fail closed**，绝不降级放行）。
3. **消费幂等**：`event_id + version` 作为幂等键；同键新版本才生效，旧版本回放直接拒绝（`STALE_VERSION`）。
4. **状态机单调前进**：决策按 `Decision.ordinal()` 只升不降，回跳被拦截并置 `stateRollbackProtected=true`。
5. **分级处置**：按决策映射处置动作；**巡查线不可用时不放过**，降级转加审（`EXTRA_REVIEW`，`reason=PATROL_SERVICE_DEGRADED`）。
6. **路由**：`domain` → 巡查线，优先经 Nacos 按注册名解析（`patrol.routes.services`），
   注册中心不可用或无实例时回落 `patrol.routes.endpoints`（本地默认值）。

## 接口

```
POST /api/v1/patrol/dispatch   # 统一巡查入口（需 patrol:dispatch 权限）
GET  /api/v1/patrol/domains    # 平台自描述（公开，不需令牌）
*    /api/v1/iam/**            # 透传到 rcp-iam（Go/Gin，登录、内省、用户管理由 IAM 自行校验）
```

`/api/v1/iam/**` 在网关侧**不做鉴权**：`rcp-iam`（Go + Gin）是平台唯一的身份与权限判定点，
网关只搬运请求与响应，不改写 401/403/409 语义。

**鉴权链**：网关对受保护请求调 `rcp-iam` 的 `/api/v1/iam/introspect` 逐请求内省
（`AuthFilter` + `IamClient`）；IAM 不可达一律 503 fail closed，不降级放行。

## 构建与验证

```bash
mvn -B install -f ../rcp-core/pom.xml -DskipTests   # 首次
mvn -B verify
python ../rcp-contracts/tools/contract_check.py \
  --repo . --contract ../rcp-contracts/contracts/openapi/api-gateway.yaml
```

## 全链路冒烟（可选）

```bash
java -jar target/api-gateway-1.0.0.jar --server.port=8080 &
java -jar ../rcp-iam/target/iam-1.0.0.jar --server.port=8085 &
java -jar ../rcp-content-patrol/target/content-patrol-1.0.0.jar --server.port=8081 &

# 1) 取令牌（演示账号见 rcp-iam/README.md）
TOKEN=$(curl -s --noproxy '*' -X POST localhost:8080/api/v1/iam/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"operator","password":"Operator@12345"}' | sed -n 's/.*"accessToken":"\([^"]*\)".*/\1/p')

# 2) 带令牌提交
curl --noproxy '*' -X POST localhost:8080/api/v1/patrol/dispatch \
  -H 'Content-Type: application/json' -H "Authorization: Bearer $TOKEN" \
  -d '{"eventId":"job:1","domain":"content","version":1,"payload":{"jobId":1,"city":"杭州","trade":"装修","title":"招瓦工","body":"招瓦工日结300"}}'
```

注意：本机环境变量 `SERVER__PORT` 会干扰 Spring Boot 端口，需显式 `--server.port=`；curl 本机请求需 `--noproxy '*'`。
