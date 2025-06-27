# syntax=docker/dockerfile:1.6
ARG TARGETARCH                   # 只需这一个
FROM golang:1.22-alpine AS builder   # ← 去掉 --platform

# <<< MOD ─ 安装 libwebp-dev（C 头文件 & 静态库）并留出 build 缓存
RUN apk add --no-cache alpine-sdk libwebp-dev

WORKDIR /build

# 先拉依赖
COPY go.mod ./
RUN go mod download

# 拷贝源码
COPY . .

# <<< MOD ─ 默认 CGO_ENABLED=1；不再人为改 GOOS/GOARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    make docker   # 你的 Makefile “docker” 目标内部执行 `go build ...`

###############################################################################
# 2) runtime 阶段                                                             #
###############################################################################
FROM alpine

# 运行期只需动态 libwebp（APK 自动带上）
RUN apk add --no-cache libwebp

# 拷贝二进制和配置
COPY --from=builder /build/webp-server /usr/bin/webp-server
COPY --from=builder /build/config.json /etc/config.json

WORKDIR /opt
VOLUME /opt/exhaust
EXPOSE 3000
CMD ["/usr/bin/webp-server", "--config", "/etc/config.json"]
