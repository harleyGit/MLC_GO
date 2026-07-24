# ======================
# 第一阶段：构建阶段
# ======================
# ← 不要加 registry 前缀！，比如： FROM registry.cn-hangzhou.aliyuncs.com/library/golang:1.23 AS builder
# go version 查到go版本
# 定义一个构建阶段名字叫 builder
FROM golang:1.23 AS builder   

# Go 模块代理（国内可用）
ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /app

# 先拷贝依赖文件，利用 Docker 缓存
COPY go.mod go.sum ./
RUN go mod download

# 再拷贝源码
COPY . .

# 显式指定目标平台，禁用 CGO，生成静态二进制
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app .

# ======================
# 第二阶段：运行阶段
# ======================
# ← 不要用 latest，也不要加 registry 前缀, 比如： registry.cn-hangzhou.aliyuncs.com/library/alpine:latest
FROM alpine:3.20   

# 安装 CA 证书（HTTPS 必须）
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# 从builder这个阶段里面复制文件【拷贝编译好的二进制】
COPY --from=builder /app/app .

# 运行时只复制不含真实密钥的模块化 YAML；数据库和 Redis 密钥由部署环境注入。
COPY --from=builder /app/config/base ./config/base
COPY --from=builder /app/config/debug ./config/debug
COPY --from=builder /app/config/pre ./config/pre
COPY --from=builder /app/config/prod ./config/prod

# 启动应用
CMD ["./app"]
