FROM golang:1.21-alpine AS builder

WORKDIR /app

# 复制项目文件
COPY . .

# 下载依赖并构建应用
RUN go mod download
RUN go build -o hotupdate .

# 第二阶段：创建最终镜像
FROM alpine:latest

WORKDIR /app

# 安装运行时依赖
RUN apk --no-cache add ca-certificates tzdata

# 创建必要的目录
RUN mkdir -p /app/uploads /app/logs

# 从builder阶段复制编译好的程序
COPY --from=builder /app/hotupdate /app/
# 复制静态文件和模板
COPY --from=builder /app/app/views /app/app/views
COPY --from=builder /app/app/static /app/app/static
# 复制配置文件
COPY --from=builder /app/config.json /app/

# 设置工作目录的权限
RUN chmod -R 755 /app

# 暴露端口
EXPOSE 9090

# 设置卷，用于持久化数据
VOLUME ["/app/uploads", "/app/logs"]

# 设置容器启动命令
CMD ["/app/hotupdate"] 