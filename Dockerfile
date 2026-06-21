# syntax=docker/dockerfile:1.7
#
# 多阶段构建 + BuildKit 缓存挂载,大幅加速 CI 重复构建:
#   - 阶段 1 (mod):  仅依赖 go.mod/go.sum,命中缓存的概率最高
#   - 阶段 2 (build): 通过 --mount=type=cache 持久化
#                     /root/.cache/go-build 和 /go/pkg/mod,
#                     后续构建只重编改了的部分
#   - 阶段 3 (runtime): distroless 静态镜像,~2MB,无 shell 更安全
#
# 配合 buildx 使用 platforms=linux/amd64,linux/arm64
ARG GO_VERSION=1.26.0

# ---- 阶段 1: 下载依赖 ----
FROM golang:${GO_VERSION}-alpine AS mod
WORKDIR /src
COPY go.mod go.sum ./
# --mount=type=cache 把 /go/pkg/mod 挂到 BuildKit 缓存卷,
# 跨构建复用已下载的模块,go.sum 不变时此层完全命中
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download -x

# ---- 阶段 2: 编译 ----
FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
# 复用阶段 1 拉好的 module(已带缓存)
COPY --from=mod /go/pkg/mod /go/pkg/mod
COPY . .
# TARGETOS / TARGETARCH 由 buildx 在多架构构建时自动注入
ARG TARGETOS TARGETARCH
# 静态二进制,避免 CGO 跨架构链接
ENV CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH}
# --mount=type=cache 持久化 Go 编译缓存,增量构建时
# 只重编改过的包;首次构建建立缓存,后续构建 < 30s
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -trimpath -ldflags='-s -w -buildid=' -o /out/aurora .

# ---- 阶段 3: 运行镜像(distroless,~2MB)----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/aurora /aurora
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/aurora"]
