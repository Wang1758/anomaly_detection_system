FROM golang:1.23-alpine AS builder

WORKDIR /app

# 安装构建依赖（包括 protoc 和 CGO 所需的库）
RUN apk add --no-cache gcc musl-dev protobuf protobuf-dev sqlite-dev

# 安装 Go 的 protoc 插件（固定版本以兼容 Go 1.23）
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.0 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

# 设置 PATH 确保 protoc 可以找到 Go 插件
ENV PATH="$PATH:/root/go/bin"

# 复制 go.mod 和 go.sum（如果存在）
COPY backend/go.mod backend/go.sum* ./

# 下载依赖（忽略错误，后续会 tidy）
RUN go mod download || true

# 复制 proto 文件
COPY proto/ proto/

# 生成 gRPC 代码：使用 module 模式将代码生成到 pb 目录
RUN mkdir -p pb && \
    protoc --proto_path=proto \
           --go_out=pb --go_opt=module=anomaly_detection_system/backend/pb \
           --go-grpc_out=pb --go-grpc_opt=module=anomaly_detection_system/backend/pb \
           detection.proto

# 验证生成的文件
RUN echo "=== Generated pb files ===" && ls -la pb/

# 复制所有 backend 源代码
COPY backend/ .

# 更新依赖
RUN go mod tidy

# 构建（启用 CGO 支持 SQLite，使用 libsqlite3 标签避免 musl libc 兼容性问题）
RUN CGO_ENABLED=1 GOOS=linux go build -tags "libsqlite3" -o server ./cmd/server

# 运行镜像
FROM alpine:latest

WORKDIR /app

# 安装运行时依赖
RUN apk add --no-cache ca-certificates sqlite sqlite-libs ffmpeg

# 复制二进制文件
COPY --from=builder /app/server .

# 创建数据目录
RUN mkdir -p data

# 暴露端口
EXPOSE 8080

# 启动服务
CMD ["./server"]
