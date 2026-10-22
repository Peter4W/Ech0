
FROM node:26-alpine AS frontend-builder

WORKDIR /web

COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./

RUN npm install -g pnpm@11.9.0

RUN pnpm install --frozen-lockfile

COPY web/ .

RUN pnpm run build --mode production

FROM golang:1.27.3-alpine AS backend-builder

RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories \
    && apk add --no-cache git ca-certificates gcc musl-dev sqlite-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend-builder /template/dist /app/template/dist

ARG TARGETOS
ARG TARGETARCH
ARG GIT_COMMIT
ARG BUILD_TIME

RUN COMMIT="${GIT_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}" \
    && BUILD="${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}" \
    && CGO_ENABLED=1 CGO_CFLAGS="-Du_int8_t=uint8_t -Du_int16_t=uint16_t -Du_int64_t=uint64_t" GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -tags netgo \
    -ldflags="-linkmode external -extldflags '-static' -X github.com/lin-snow/ech0/internal/version.Commit=${COMMIT} -X github.com/lin-snow/ech0/internal/version.BuildTime=${BUILD}" \
    -o ech0 ./cmd/ech0/main.go

FROM alpine:latest

WORKDIR /app

RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories \
    && apk add --no-cache tzdata \
    && mkdir -p /app/data

COPY --from=backend-builder /app/ech0 /app/ech0

RUN chmod +x /app/ech0

EXPOSE 6277

ENTRYPOINT ["/app/ech0"]
CMD ["serve"]
