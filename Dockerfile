# syntax=docker/dockerfile:1.7
#
# 多阶段构建 + BuildKit 缓存挂载,加速 CI 重复构建:
#   - mod + build 合并到一个阶段,避免 --mount=type=cache 路径
#     无法被 COPY --from 引用的坑(cache mount 不写入镜像层)
#   - /root/.cache/go-build 和 /go/pkg/mod 通过 --mount=type=cache
#     持久化到 BuildKit 缓存卷;go.sum 不变时完全命中
#   - 配套 workflow 平台:linux/amd64,linux/arm64(其余架构编译慢,
#     对本服务无收益)
#
# 注意:必须先 COPY go.mod/go.sum 再 go mod download,
# 这样 buildx 才能在 go.sum 变化时失效此层缓存,触发重新下载。
ARG GO_VERSION=1.26.0

# ---- 阶段 1: 编译 ----
FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

# 1) 先拷贝 module 清单(几乎不变,缓存命中率最高)
COPY go.mod go.sum ./
# 2) 用 cache mount 拉 module,持久化到 /go/pkg/mod
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download -x

# 3) 拷贝其余源码(改动频繁,缓存粒度细)
COPY . .

# buildx 多架构构建时自动注入 TARGETOS / TARGETARCH
ARG TARGETOS TARGETARCH
# 静态二进制,避免 CGO 跨架构链接;同时让 go build 启用
# 完全确定性的输出(trimpath 剥路径前缀)
ENV CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH}

# 4) 编译:cache mount 让 go build 复用上次编译结果
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -trimpath -ldflags='-s -w -buildid=' -o /out/aurora .

# ---- 阶段 2: 运行镜像(distroless,~2MB,无 shell 更安全)----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/aurora /aurora
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/aurora"]
