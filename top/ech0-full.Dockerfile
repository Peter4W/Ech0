# SPDX-License-Identifier: AGPL-3.0-or-later
# 一体镜像：同时包含 Ech0 本体与 top 置顶边车，单容器运行。
# 构建上下文为仓库根目录：docker build -t ech0:full -f top/ech0-full.Dockerfile .

# ---- 阶段 1：构建 Ech0 前端 ----
FROM node:26-alpine AS frontend-builder
WORKDIR /app
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN npm install -g pnpm@11.9.0 \
    && pnpm install --frozen-lockfile
COPY web/ .
RUN pnpm build

# ---- 阶段 2：构建 Ech0 后端二进制 ----
FROM golang:1.27.3-alpine AS backend-builder
RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories \
    && apk add --no-cache git ca-certificates gcc musl-dev sqlite-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /template/dist /app/template/dist
RUN CGO_ENABLED=1 CGO_CFLAGS="-Du_int8_t=uint8_t -Du_int16_t=uint16_t -Du_int64_t=uint64_t" \
    go build -tags netgo \
    -ldflags="-linkmode external -extldflags '-static'" \
    -o /out/ech0 ./cmd/ech0/main.go

# ---- 阶段 3：构建 top 边车二进制（纯标准库，静态）----
FROM golang:1.27.3-alpine AS top-builder
WORKDIR /src
COPY top/ .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ech0-top .

# ---- 阶段 4：运行镜像 ----
FROM alpine:latest
RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories \
    && apk add --no-cache ca-certificates tzdata \
    && mkdir -p /app/data /app/top-data

WORKDIR /app
COPY --from=backend-builder /out/ech0 /app/ech0
COPY --from=top-builder /out/ech0-top /app/ech0-top

# top 边车：对外监听 6278，回源到容器内部 6277 的 Ech0。
ENV TOP_ADDR=":6278" \
    TOP_UPSTREAM="http://127.0.0.1:6277" \
    TOP_DATA="/app/top-data"

EXPOSE 6278

# 入口：后台启动 Ech0（内部 6277），前台运行 top 边车（对外 6278）。
# JWT_SECRET 通过环境变量注入即可（ech0 自动读取）。
CMD ["sh", "-c", "/app/ech0 serve & exec /app/ech0-top"]
