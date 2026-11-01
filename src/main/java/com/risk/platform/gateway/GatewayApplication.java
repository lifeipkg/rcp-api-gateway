package com.risk.platform.gateway;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

/**
 * 风控平台统一接入网关。
 *
 * 四条巡查线（内容/消息/账号/资金）的唯一入口：
 *   事件路由 -> 消费幂等(event_id+version) -> 巡查线检测 -> 状态机单调前进 -> 分级处置。
 */
@SpringBootApplication
public class GatewayApplication {

    public static void main(String[] args) {
        SpringApplication.run(GatewayApplication.class, args);
    }
}
