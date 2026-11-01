package com.risk.platform.gateway.dispatch;

import com.risk.platform.gateway.api.PatrolEventRequest;
import com.risk.platform.gateway.api.PatrolEventResponse;
import com.risk.platform.model.Decision;
import com.risk.platform.model.PatrolVerdict;
import org.springframework.stereotype.Service;

import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

/**
 * 平台巡查编排（共同基建的进程内实现，生产形态见 deploy/sql/common-schema.sql）：
 *
 *   1. 消费幂等：event_id + version 唯一键，重复消费拒绝旧版本（Kafka 重复投递是常态）；
 *   2. 状态机：处置状态单调前进（PASS -> WARN -> LIMIT -> EXTRA_REVIEW -> INTERCEPT -> BAN），
 *      防乱序/重放消息导致状态回跳；
 *   3. 分级处置：同一决策按对象落地不同动作；
 *   4. 巡查线降级：巡查线不可用时转人工加审（宁可人审，不可漏审）。
 */
@Service
public class PatrolDispatchService {

    /** 幂等表：event_id -> 已处理的最大 version（生产为 MySQL 唯一键）。 */
    private final Map<String, Long> idempotentTable = new ConcurrentHashMap<>();

    /** 状态机：event_id -> 已落地的处置状态序（生产为 MySQL 状态机表）。 */
    private final Map<String, Integer> stateMachine = new ConcurrentHashMap<>();

    private final PatrolClient patrolClient;

    public PatrolDispatchService(PatrolClient patrolClient) {
        this.patrolClient = patrolClient;
    }

    public PatrolEventResponse dispatch(PatrolEventRequest req) {
        // ---- 1. 消费幂等：event_id + version ----
        Long seen = idempotentTable.putIfAbsent(req.eventId(), req.version());
        if (seen != null) {
            if (req.version() <= seen) {
                return new PatrolEventResponse(false, "STALE_VERSION_REPLAY",
                        null, null, false);
            }
            idempotentTable.put(req.eventId(), req.version());
        }

        // ---- 2. 路由巡查线检测（不可用则降级加审） ----
        PatrolVerdict verdict;
        try {
            Map<String, Object> payload = new HashMap<>(req.payload() == null ? Map.of() : req.payload());
            payload.putIfAbsent("eventId", req.eventId());
            verdict = patrolClient.check(req.domain(), payload);
        } catch (Exception e) {
            verdict = new PatrolVerdict(req.eventId(), Decision.EXTRA_REVIEW, 60,
                    "PATROL_SERVICE_DEGRADED", Map.of("domain", req.domain()));
        }

        // ---- 3. 状态机单调前进 ----
        int rank = verdict.decision().ordinal();
        boolean rollbackProtected = false;
        Integer prev = stateMachine.put(req.eventId(), rank);
        if (prev != null && rank < prev) {
            stateMachine.put(req.eventId(), prev); // 拒绝回跳
            rollbackProtected = true;
        }

        // ---- 4. 分级处置 ----
        Map<String, Object> disposition = dispositionOf(verdict.decision());
        return new PatrolEventResponse(true, null,
                verdictMap(verdict), disposition, rollbackProtected);
    }

    /** 分级处置：同一决策落地统一动作语义。 */
    private static Map<String, Object> dispositionOf(Decision decision) {
        Map<String, Object> m = new LinkedHashMap<>();
        switch (decision) {
            case PASS -> m.put("action", "RELEASE");
            case WARN -> {
                m.put("action", "NOTIFY");
                m.put("note", "仅提示不拦截");
            }
            case LIMIT -> {
                m.put("action", "THROTTLE");
                m.put("note", "当日触达/申请次数受限");
            }
            case EXTRA_REVIEW -> {
                m.put("action", "MANUAL_REVIEW");
                m.put("note", "进入人工复核队列");
            }
            case INTERCEPT -> {
                m.put("action", "BLOCK");
                m.put("note", "拦截并进入复核队列");
            }
            case BAN -> {
                m.put("action", "BAN_ACCOUNT");
                m.put("note", "账号封禁，可申诉");
            }
        }
        return m;
    }

    private static Map<String, Object> verdictMap(PatrolVerdict v) {
        Map<String, Object> m = new LinkedHashMap<>();
        m.put("eventId", v.eventId());
        m.put("decision", v.decision().name());
        m.put("score", v.score());
        m.put("reason", v.reason());
        m.put("detail", v.detail());
        return m;
    }
}
