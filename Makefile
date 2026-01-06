.PHONY: all proto proto-go proto-py backend frontend ai-service clean install

# 项目根目录
ROOT_DIR := $(shell pwd)
PROTO_DIR := $(ROOT_DIR)/proto
BACKEND_DIR := $(ROOT_DIR)/backend
AI_SERVICE_DIR := $(ROOT_DIR)/ai_service
FRONTEND_DIR := $(ROOT_DIR)/frontend

# Proto 文件
PROTO_FILES := $(wildcard $(PROTO_DIR)/*.proto)

# ==================== 初始化 ====================

all: install proto

# 安装所有依赖
install: install-backend install-ai install-frontend

install-backend:
	@echo "==> 安装 Go 后端依赖..."
	cd $(BACKEND_DIR) && go mod tidy

install-ai:
	@echo "==> 安装 Python AI 服务依赖..."
	cd $(AI_SERVICE_DIR) && pip install -r requirements.txt

install-frontend:
	@echo "==> 安装前端依赖..."
	cd $(FRONTEND_DIR) && npm install

# ==================== Proto 编译 ====================

proto: proto-go proto-py
	@echo "==> Proto 编译完成"

# 编译 Go 版本的 proto
proto-go:
	@echo "==> 编译 Go proto 文件..."
	@mkdir -p $(BACKEND_DIR)/pb
	protoc --proto_path=$(PROTO_DIR) \
		--go_out=$(BACKEND_DIR)/pb --go_opt=paths=source_relative \
		--go-grpc_out=$(BACKEND_DIR)/pb --go-grpc_opt=paths=source_relative \
		$(PROTO_FILES)

# 编译 Python 版本的 proto
proto-py:
	@echo "==> 编译 Python proto 文件..."
	@mkdir -p $(AI_SERVICE_DIR)/pb
	python -m grpc_tools.protoc --proto_path=$(PROTO_DIR) \
		--python_out=$(AI_SERVICE_DIR)/pb \
		--grpc_python_out=$(AI_SERVICE_DIR)/pb \
		$(PROTO_FILES)
	@touch $(AI_SERVICE_DIR)/pb/__init__.py

# ==================== 服务启动 ====================

# 启动 Go 后端
backend:
	@echo "==> 启动 Go 后端服务..."
	cd $(BACKEND_DIR) && go run ./cmd/server

# 启动 Python AI 服务
ai-service:
	@echo "==> 启动 Python AI 服务..."
	cd $(AI_SERVICE_DIR) && python server.py

# 启动前端开发服务器
frontend:
	@echo "==> 启动前端开发服务器..."
	cd $(FRONTEND_DIR) && npm run dev

# 启动所有服务 (需要多个终端或使用 tmux/screen)
run-all:
	@echo "请在不同终端分别运行:"
	@echo "  make ai-service  # 终端1: Python AI 服务"
	@echo "  make backend     # 终端2: Go 后端"
	@echo "  make frontend    # 终端3: 前端开发服务器"

# ==================== 训练 ====================

# 触发增量训练
train:
	@echo "==> 启动增量训练..."
	cd $(AI_SERVICE_DIR) && python train.py

# ==================== 清理 ====================

clean:
	@echo "==> 清理生成文件..."
	rm -rf $(BACKEND_DIR)/pb/*.pb.go
	rm -rf $(AI_SERVICE_DIR)/pb/*.py
	rm -rf $(FRONTEND_DIR)/dist

# ==================== Docker ====================

# 自动检测 docker compose 命令 (V2 使用 'docker compose', V1 使用 'docker-compose')
DOCKER_COMPOSE := $(shell if sudo docker compose version > /dev/null 2>&1; then echo 'sudo docker compose'; else echo 'sudo docker-compose'; fi)

docker-build:
	@echo "==> 构建 Docker 镜像..."
	$(DOCKER_COMPOSE) build

docker-up:
	@echo "==> 启动 Docker 服务..."
	$(DOCKER_COMPOSE) up -d

docker-up-gpu:
	@echo "==> 启动 Docker 服务 (GPU 模式)..."
	$(DOCKER_COMPOSE) -f docker-compose.yml -f docker-compose.gpu.yml up -d

docker-down:
	@echo "==> 停止 Docker 服务..."
	$(DOCKER_COMPOSE) down

docker-logs:
	@echo "==> 查看 Docker 日志..."
	$(DOCKER_COMPOSE) logs -f

docker-ps:
	@echo "==> 查看 Docker 服务状态..."
	$(DOCKER_COMPOSE) ps

docker-restart:
	@echo "==> 重启 Docker 服务..."
	$(DOCKER_COMPOSE) restart

docker-clean:
	@echo "==> 清理 Docker 资源..."
	$(DOCKER_COMPOSE) down -v --rmi local

# ==================== 帮助 ====================

help:
	@echo "智慧养殖场监控系统 - Makefile 命令"
	@echo ""
	@echo "初始化:"
	@echo "  make install        安装所有依赖"
	@echo "  make proto          编译 Proto 文件"
	@echo ""
	@echo "运行服务:"
	@echo "  make ai-service     启动 Python AI 服务"
	@echo "  make backend        启动 Go 后端"
	@echo "  make frontend       启动前端开发服务器"
	@echo ""
	@echo "Docker 部署:"
	@echo "  make docker-build   构建 Docker 镜像"
	@echo "  make docker-up      启动服务 (CPU 模式)"
	@echo "  make docker-up-gpu  启动服务 (GPU 模式)"
	@echo "  make docker-down    停止服务"
	@echo "  make docker-logs    查看日志"
	@echo "  make docker-ps      查看服务状态"
	@echo ""
	@echo "训练:"
	@echo "  make train          启动增量训练"
	@echo ""
	@echo "其他:"
	@echo "  make clean          清理生成文件"
	@echo "  make help           显示帮助信息"
