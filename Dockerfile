# 平台唯一对外入口（api-gateway）
FROM golang:1.22-alpine AS build
WORKDIR /src
# go.sum 已入库：先只拷清单取依赖，源码改动不触发重新下载模块图
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/api-gateway .

FROM alpine:3.20
RUN adduser -D -u 10001 app
COPY --from=build /out/api-gateway /app/api-gateway
USER app
EXPOSE 8080
ENV SERVER_PORT=8080
ENTRYPOINT ["/app/api-gateway"]
