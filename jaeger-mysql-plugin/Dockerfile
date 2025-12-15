FROM golang:1.21-alpine AS builder

WORKDIR /build

# 安装依赖
RUN apk add --no-cache git

# 复制 go mod 文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY *.go ./

# 构建
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o jaeger-mysql-plugin .

# 最终镜像
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /build/jaeger-mysql-plugin .

EXPOSE 17271

ENTRYPOINT ["./jaeger-mysql-plugin"]



