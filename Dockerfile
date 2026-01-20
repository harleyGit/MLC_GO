# ======================
# 构建阶段
# ======================
# ← 不要加 registry 前缀！，比如： FROM registry.cn-hangzhou.aliyuncs.com/library/golang:1.23 AS builder
# go version 查到go版本
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
# 运行阶段
# ======================
# ← 不要用 latest，也不要加 registry 前缀, 比如： registry.cn-hangzhou.aliyuncs.com/library/alpine:latest
FROM alpine:3.20   

# 安装 CA 证书（HTTPS 必须）
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# 拷贝编译好的二进制
COPY --from=builder /app/app .

# 启动应用
CMD ["./app"]
