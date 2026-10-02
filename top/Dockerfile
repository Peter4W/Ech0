# SPDX-License-Identifier: AGPL-3.0-or-later
# top 模块镜像：独立构建（构建上下文为 top/ 目录），零外部依赖。

# ---- 构建阶段 ----
FROM golang:1.27.1-alpine AS builder

WORKDIR /src
COPY . .

# 纯标准库 + go:embed，CGO 关闭即可得到静态二进制。
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/ech0-top .

# ---- 运行阶段 ----
FROM alpine:latest

RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories \
    && apk add --no-cache ca-certificates tzdata \
    && mkdir -p /app/data

WORKDIR /app
COPY --from=builder /out/ech0-top /app/ech0-top

# 边车默认监听 6278，通过容器网络访问 ech0:6277。
ENV TOP_ADDR=":6278" \
    TOP_UPSTREAM="http://ech0:6277" \
    TOP_DATA="/app/data"

EXPOSE 6278

ENTRYPOINT ["/app/ech0-top"]
