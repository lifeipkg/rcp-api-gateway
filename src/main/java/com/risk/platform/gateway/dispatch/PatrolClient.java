package com.risk.platform.gateway.dispatch;

import com.risk.platform.model.PatrolVerdict;

/**
 * 巡查线调用客户端抽象（便于测试替换与未来 gRPC/Kafka 形态替换）。
 */
public interface PatrolClient {

    /**
     * 调用巡查线检测接口。
     *
     * @param domain  巡查线
     * @param payload 业务 payload（含 eventId）
     * @return 统一裁决
     * @throws Exception 网络/服务异常（由调用方降级处理）
     */
    PatrolVerdict check(String domain, Object payload) throws Exception;
}
