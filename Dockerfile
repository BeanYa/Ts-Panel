# 多阶段构建：Go 编译 → Debian 运行
FROM golang:1.23 AS builder

ARG GOPROXY
ENV GOPROXY=${GOPROXY}

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY src/ ./src/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o ts-panel ./src/

# === 运行镜像 (Alpine + 阿里云源) ===
FROM alpine:latest
RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories && \
    apk add --no-cache ca-certificates docker-cli tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app
COPY --from=builder /build/ts-panel /app/ts-panel

EXPOSE 8080
CMD ["/app/ts-panel"]
