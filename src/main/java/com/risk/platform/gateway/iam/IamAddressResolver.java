package com.risk.platform.gateway.iam;

import com.risk.platform.gateway.config.IamProperties;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.cloud.client.ServiceInstance;
import org.springframework.cloud.client.discovery.DiscoveryClient;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.concurrent.atomic.AtomicInteger;

/**
 * IAM 地址解析：服务发现优先，配置地址兜底。
 *
 * 网关的鉴权与透传都必须走同一处解析，否则会出现"内省打 A 实例、透传打 B 地址"
 * 这种只在故障时才暴露的错位。
 */
@Component
public class IamAddressResolver {

    private final IamProperties properties;
    private final ObjectProvider<DiscoveryClient> discovery;
    private final AtomicInteger cursor = new AtomicInteger();

    public IamAddressResolver(IamProperties properties, ObjectProvider<DiscoveryClient> discovery) {
        this.properties = properties;
        this.discovery = discovery;
    }

    /** 返回 IAM 基础地址（不带尾斜杠）；无可用地址时返回配置兜底值。 */
    public String resolve() {
        return resolve(properties.getRoutes().getService(), properties.getRoutes().getEndpoint());
    }

    private String resolve(String service, String endpoint) {
        if (service != null && !service.isBlank()) {
            DiscoveryClient client = discovery.getIfAvailable();
            if (client != null) {
                try {
                    List<ServiceInstance> instances = client.getInstances(service);
                    if (!instances.isEmpty()) {
                        ServiceInstance instance =
                                instances.get(Math.floorMod(cursor.getAndIncrement(), instances.size()));
                        return "http://" + instance.getHost() + ":" + instance.getPort();
                    }
                } catch (Exception e) {
                    // 注册中心不可达：回落配置地址，由调用方按 fail closed 处理
                }
            }
        }
        return endpoint;
    }
}
