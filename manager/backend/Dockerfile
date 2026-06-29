FROM golang:1.24-alpine AS builder

WORKDIR /app

# 复制go mod文件
COPY go.mod go.sum ./
ENV GOPROXY=https://goproxy.cn,https://goproxy.io,direct
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# 运行阶段
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

# 从构建阶段复制二进制文件和配置文件
COPY --from=builder /app/main .
COPY --from=builder /app/config/config.json ./config/config.json

# 设置时区
ENV TZ=Asia/Shanghai

EXPOSE 8080

CMD ["./main"]