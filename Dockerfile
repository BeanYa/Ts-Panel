# 多阶段构建：Go 编译 → Alpine 运行
FROM golang:alpine AS builder

ARG GOPROXY
ENV GOPROXY=${GOPROXY}

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories && \
    apk add --no-cache gcc musl-dev

WORKDIR /build
COPY go.mod ./
COPY src/ ./src/
RUN go mod tidy
RUN go mod download
RUN CGO_ENABLED=1 GOOS=linux go build -o ts-panel ./src/

# === 运行镜像 ===
FROM alpine
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories && \
    apk add --no-cache ca-certificates docker-cli tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app
COPY --from=builder /build/ts-panel /app/ts-panel

EXPOSE 8080
CMD ["/app/ts-panel"]
