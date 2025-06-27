###############################################################################
# syntax 行启用 BuildKit v1.6 语法，用于 buildx 注入 TARGETARCH/TARGETPLATFORM
###############################################################################
# syntax=docker/dockerfile:1.6

###############################################################################
# 1. builder 阶段：使用 Go 官方多架构镜像
###############################################################################
ARG TARGETPLATFORM   # buildx 注入（linux/amd64、linux/arm64…）
ARG TARGETARCH       # buildx 注入（amd64、arm64）

FROM --platform=$TARGETPLATFORM golang:1.22-alpine AS builder

# ----- 系统依赖保持不变 -----
ARG IMG_PATH=/opt/pics
ARG EXHAUST_PATH=/opt/exhaust
RUN apk update && apk add --no-cache alpine-sdk

# ----- 复制 go.mod 先拉依赖 -----
WORKDIR /build
COPY go.mod ./
RUN go mod download

# ----- 复制全部源码 -----
COPY . .

# <<< MOD ─ 如果 Makefile 没做 GOOS/GOARCH 交叉编译，这里注入
#            否则可删掉这行
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH make docker

###############################################################################
# 2. 运行阶段：极简 alpine (或 scratch)
###############################################################################
FROM alpine AS runtime

# ----- 拷贝可执行文件和配置 -----
COPY --from=builder /build/webp-server  /usr/bin/webp-server
COPY --from=builder /build/config.json /etc/config.json

# ----- 运行目录、卷、端口 -----
WORKDIR /opt
VOLUME /opt/exhaust
EXPOSE 3000

CMD ["/usr/bin/webp-server", "--config", "/etc/config.json"]
