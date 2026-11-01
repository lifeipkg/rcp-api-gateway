# 平台唯一对外入口（api-gateway）
FROM maven:3.9-eclipse-temurin-17 AS build
ARG CORE_REF=v1.0.0
RUN apt-get update && apt-get install -y --no-install-recommends git \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /build
RUN git clone --depth 1 --branch ${CORE_REF} https://github.com/lifeipkg/rcp-core.git /build/rcp-core \
 && mvn -B -q -f /build/rcp-core/pom.xml install -DskipTests
COPY pom.xml .
RUN mvn -B -q dependency:go-offline || true
COPY src ./src
RUN mvn -B -q package -DskipTests

FROM eclipse-temurin:17-jre
WORKDIR /app
RUN groupadd -r risk && useradd -r -g risk risk
COPY --from=build /build/target/api-gateway-1.0.0.jar app.jar
USER risk
EXPOSE 8080
ENTRYPOINT ["java", "-XX:MaxRAMPercentage=75.0", "-jar", "app.jar"]
