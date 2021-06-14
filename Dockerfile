FROM golang:alpine as builder

ARG IMG_PATH=/opt/pics
ARG EXHAUST_PATH=/opt/exhaust
RUN apk update && apk add alpine-sdk && mkdir /build
COPY go.mod /build
RUN cd /build && go mod download

COPY . /build
RUN cd /build && make docker



FROM alpine

COPY --from=builder /build/webp-server  /usr/bin/webp-server
COPY --from=builder /build/config.json /etc/config.json

WORKDIR /opt
VOLUME /opt/exhaust
EXPOSE 3000
CMD ["/usr/bin/webp-server", "--config", "/etc/config.json"]
