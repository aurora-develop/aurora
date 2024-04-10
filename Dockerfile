# 使用 Go 1.21 官方镜像作为构建环境
FROM golang:1.21 AS builder

# 禁用 CGO
ENV CGO_ENABLED=0

# 设置工作目录
WORKDIR /app

# 复制 go.mod 和 go.sum 并下载依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码并构建应用
COPY . .
RUN go build -ldflags "-s -w" -o /app/rgzn .

# 使用 Alpine Linux 作为最终镜像
FROM alpine:3.18

# 设置工作目录
WORKDIR /app

# 从构建阶段复制编译好的应用和资源
COPY --from=builder /app/rgzn /app/rgzn
COPY harPool /app/harPool

# 暴露端口
EXPOSE 8000

CMD ["/app/rgzn"]
