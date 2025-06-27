# syntax=docker/dockerfile:1.6

###############################################################################
# builder 阶段                                                                #
###############################################################################
ARG TARGETARCH                     # buildx 注入；本地 docker build 时为空
FROM golang:1.22-alpine AS builder

# 必备工具 & libwebp 头文件
RUN apk add --no-cache alpine-sdk libwebp-dev

WORKDIR /build

COPY go.mod ./
RUN go mod download

COPY . .

# 若 Makefile 内已有交叉编译逻辑，可删掉 CGO_ENABLED 这一行
RUN --mount=type=cache,target=/root/.cache/go-build \
    make docker

###############################################################################
# runtime 阶段                                                                #
###############################################################################
FROM alpine

RUN apk add --no-cache libwebp        # 运行期动态库

COPY --from=builder /build/webp-server /usr/bin/webp-server
COPY --from=builder /build/config.json /etc/config.json

WORKDIR /opt
VOLUME /opt/exhaust
EXPOSE 3000

CMD ["/usr/bin/webp-server", "--config", "/etc/config.json"]