package com.risk.platform.gateway.api;

import com.risk.platform.gateway.api.PatrolEventRequest;
import com.risk.platform.gateway.api.PatrolEventResponse;
import com.risk.platform.gateway.dispatch.PatrolDispatchService;
import jakarta.validation.Valid;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.List;
import java.util.Map;

/**
 * 平台统一接入入口：POST /api/v1/patrol/dispatch
 */
@RestController
@RequestMapping("/api/v1/patrol")
public class GatewayController {

    public static final List<String> DOMAINS = List.of("content", "message", "account", "fund");

    private final PatrolDispatchService dispatchService;

    public GatewayController(PatrolDispatchService dispatchService) {
        this.dispatchService = dispatchService;
    }

    @PostMapping("/dispatch")
    public ResponseEntity<PatrolEventResponse> dispatch(@Valid @RequestBody PatrolEventRequest req) {
        if (!DOMAINS.contains(req.domain())) {
            return ResponseEntity.badRequest().build();
        }
        return ResponseEntity.ok(dispatchService.dispatch(req));
    }

    /** 平台自描述（巡查线清单）。 */
    @GetMapping("/domains")
    public Map<String, Object> domains() {
        return Map.of("platform", "risk-control-platform", "patrolLines", DOMAINS);
    }
}
