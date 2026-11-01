# rcp-api-gateway · 平台唯一对外入口

| 项 | 值 |
|---|---|
| 服务名 | `api-gateway` |
| 端口 | `8080` |
| 契约 | `rcp-contracts/contracts/openapi/api-gateway.yaml` |
| 依赖 | `rcp-core` 钉版 `v1.0.0`；下游四条巡查线 |

## 职责

1. **唯一入口**：边缘只暴露本网关（红线 RL-002），前端与外部系统都只经 `/api/v1/patrol/*`。
2. **消费幂等**：`event_id + version` 作为幂等键；同键新版本才生效，旧版本回放直接拒绝（`STALE_VERSION`）。
3. **状态机单调前进**：决策按 `Decision.ordinal()` 只升不降，回跳被拦截并置 `stateRollbackProtected=true`。
4. **分级处置**：按决策映射处置动作；**巡查线不可用时不放过**，降级转加审（`EXTRA_REVIEW`，`reason=PATROL_SERVICE_DEGRADED`）。
5. **路由**：`domain` → 巡查线服务地址，由 `patrol.routes.endpoints` 配置（K8s 中由环境变量覆盖为 Service DNS）。

## 接口

```
POST /api/v1/patrol/dispatch   # 统一巡查入口
GET  /api/v1/patrol/domains    # 平台自描述（巡查线清单）
```

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
java -jar ../rcp-content-patrol/target/content-patrol-1.0.0.jar --server.port=8081 &
curl --noproxy '*' -X POST localhost:8080/api/v1/patrol/dispatch \
  -H 'Content-Type: application/json' \
  -d '{"eventId":"job:1","domain":"content","version":1,"payload":{"jobId":1,"city":"杭州","trade":"装修","title":"招瓦工","body":"招瓦工日结300"}}'
```

注意：本机环境变量 `SERVER__PORT` 会干扰 Spring Boot 端口，需显式 `--server.port=`；curl 本机请求需 `--noproxy '*'`。
