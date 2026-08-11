# ======================
# 第一阶段：构建阶段
# ======================
# ← 不要加 registry 前缀！，比如： FROM registry.cn-hangzhou.aliyuncs.com/library/golang:1.23 AS builder
# go version 查到go版本
# 定义一个构建阶段名字叫 builder
# 必须与 go.mod 的 go 指令一致，否则 GOTOOLCHAIN=local 会在下载依赖前直接失败。
FROM golang:1.26.4 AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64

# Go 模块代理（国内可用）
ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /app

# 先拷贝依赖文件，利用 Docker 缓存
COPY go.mod go.sum ./
RUN go mod download

# 再拷贝源码
COPY . .

# production tag 排除根目录练习入口及 TestNotes 依赖，只编译实际 HTTP/Kafka/弹幕应用。
# TARGETOS/TARGETARCH 由 BuildKit 注入，同一 Dockerfile 可生成 amd64 或 arm64 镜像。
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -tags production -o app .

# ======================
# 第二阶段：运行阶段
# ======================
# ← 不要用 latest，也不要加 registry 前缀, 比如： registry.cn-hangzhou.aliyuncs.com/library/alpine:latest
FROM alpine:3.20   

# 安装 CA 证书（HTTPS 必须）
RUN apk --no-cache add ca-certificates

# 生产容器不使用 root。固定 UID/GID 便于 Kubernetes securityContext 与镜像内身份一致。
RUN addgroup -S -g 10001 mlc && adduser -S -D -H -u 10001 -G mlc mlc

WORKDIR /app

# 兼容仍使用本地 uploads 的接口；生产持久文件应挂载 PVC 或改走对象存储。
RUN mkdir -p /app/uploads && chown 10001:10001 /app/uploads

# 从builder这个阶段里面复制文件【拷贝编译好的二进制】
COPY --from=builder /app/app .

# 运行时只复制不含真实密钥的模块化 YAML；数据库和 Redis 密钥由部署环境注入。
COPY --from=builder /app/config/base ./config/base
COPY --from=builder /app/config/debug ./config/debug
COPY --from=builder /app/config/pre ./config/pre
COPY --from=builder /app/config/prod ./config/prod

# 业务 HTTP、弹幕 gnet 和管理探活/指标分别使用独立端口；Service 再映射公网端口。
EXPOSE 8080 8081 9091

USER 10001:10001

# 启动应用
CMD ["./app"]
