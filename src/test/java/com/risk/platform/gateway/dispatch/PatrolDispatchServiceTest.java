package com.risk.platform.gateway.dispatch;

import com.risk.platform.gateway.api.PatrolEventRequest;
import com.risk.platform.gateway.api.PatrolEventResponse;
import com.risk.platform.model.Decision;
import com.risk.platform.model.PatrolVerdict;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.HashMap;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

class PatrolDispatchServiceTest {

    /** 可编程假巡查线客户端。 */
    static class FakePatrolClient implements PatrolClient {
        Decision decision = Decision.PASS;
        boolean down = false;

        @Override
        public PatrolVerdict check(String domain, Object payload) {
            if (down) {
                throw new IllegalStateException("service down");
            }
            Map<String, Object> p = (Map<String, Object>) payload;
            return new PatrolVerdict(String.valueOf(p.get("eventId")), decision,
                    decision.baseScore(), "FAKE", Map.of("domain", domain));
        }
    }

    private final FakePatrolClient fake = new FakePatrolClient();
    private final PatrolDispatchService service = new PatrolDispatchService(fake);

    private PatrolEventRequest event(String id, long version, Decision forced) {
        fake.decision = forced;
        Map<String, Object> payload = new HashMap<>();
        payload.put("foo", "bar");
        return new PatrolEventRequest(id, "content", version, payload);
    }

    @Test
    @DisplayName("幂等：同版本重放被拒绝，更高版本被受理")
    void idempotentReplayRejected() {
        PatrolEventResponse first = service.dispatch(event("job:1", 1, Decision.PASS));
        assertTrue(first.accepted());
        assertNull(first.rejectReason());

        PatrolEventResponse replay = service.dispatch(event("job:1", 1, Decision.INTERCEPT));
        assertFalse(replay.accepted());
        assertEquals("STALE_VERSION_REPLAY", replay.rejectReason());

        // 更高版本（业务更正）应受理
        PatrolEventResponse v2 = service.dispatch(event("job:1", 2, Decision.PASS));
        assertTrue(v2.accepted());
        assertEquals(Decision.PASS, Decision.valueOf((String) v2.verdict().get("decision")));
    }

    @Test
    @DisplayName("状态机：状态只允许单调前进，回跳被拦截")
    void stateMachineMonotonic() {
        service.dispatch(event("msg:1", 1, Decision.PASS));
        PatrolEventResponse escalate = service.dispatch(event("msg:1", 2, Decision.BAN));
        assertTrue(escalate.accepted());
        assertFalse(escalate.stateRollbackProtected());

        // 后续事件给出更低风险决策 -> 回跳保护
        PatrolEventResponse rollback = service.dispatch(event("msg:1", 3, Decision.PASS));
        assertTrue(rollback.accepted());
        assertTrue(rollback.stateRollbackProtected());
    }

    @Test
    @DisplayName("降级：巡查线不可用 -> 加审兜底，绝不漏审")
    void degradeToExtraReviewWhenPatrolDown() {
        fake.down = true;
        PatrolEventResponse r = service.dispatch(event("job:2", 1, Decision.PASS));
        assertTrue(r.accepted());
        assertEquals(Decision.EXTRA_REVIEW, Decision.valueOf((String) r.verdict().get("decision")));
        assertEquals("PATROL_SERVICE_DEGRADED", r.verdict().get("reason"));
        assertEquals("MANUAL_REVIEW", r.disposition().get("action"));
    }

    @Test
    @DisplayName("分级处置：不同决策落地不同动作")
    void dispositionByDecision() {
        assertEquals("RELEASE", service.dispatch(event("f:1", 1, Decision.PASS)).disposition().get("action"));
        assertEquals("NOTIFY", service.dispatch(event("f:2", 1, Decision.WARN)).disposition().get("action"));
        assertEquals("THROTTLE", service.dispatch(event("f:3", 1, Decision.LIMIT)).disposition().get("action"));
        assertEquals("MANUAL_REVIEW", service.dispatch(event("f:4", 1, Decision.EXTRA_REVIEW)).disposition().get("action"));
        assertEquals("BLOCK", service.dispatch(event("f:5", 1, Decision.INTERCEPT)).disposition().get("action"));
        assertEquals("BAN_ACCOUNT", service.dispatch(event("f:6", 1, Decision.BAN)).disposition().get("action"));
    }
}
