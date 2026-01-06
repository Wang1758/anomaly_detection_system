# 🐔 智慧养殖场监控系统

<p align="center">
  <b>基于主动学习的实时计算机视觉闭环系统</b>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Python-3.9+-blue.svg" alt="Python">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8.svg" alt="Go">
  <img src="https://img.shields.io/badge/Vue-3.0+-4FC08D.svg" alt="Vue">
  <img src="https://img.shields.io/badge/YOLOv11-ultralytics-FF6F00.svg" alt="YOLO">
</p>

---

## 📖 系统简介

智慧养殖场监控系统是一个**端到端的实时计算机视觉解决方案**，专为养殖场景设计。系统能够实时检测视频流中的目标，并通过**主动学习机制**识别模型不确定的样本，推送给人工审核，最终实现模型的自我进化与持续优化。

### 🎯 核心特性

| 特性 | 描述 |
|------|------|
| **实时目标检测** | 基于 YOLOv11，支持 RTSP 摄像头流和本地视频文件 |
| **不确定性评估** | 通过信息熵计算预测不确定性，智能标记可疑目标 |
| **智能报警过滤** | 时空抑制算法，避免对同一目标重复报警 |
| **人机交互标注** | 前端实时核查与一键标注，10秒超时自动收入待处理列表 |
| **闭环增量训练** | 积累足够样本后自动触发 Fine-tuning，模型热更新无停机 |
| **参数热调节** | 所有关键参数支持前端实时调整，无需重启服务 |

### 🏗️ 系统架构

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              用户浏览器                                  │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  Vue 3 前端 (Ethereal Glass 设计)                                │   │
│  │  - 实时视频 Canvas 渲染                                          │   │
│  │  - 检测框覆盖层绘制                                              │   │
│  │  - 报警队列管理                                                  │   │
│  │  - 参数控制台                                                    │   │
│  └───────────────────────────┬─────────────────────────────────────┘   │
└──────────────────────────────┼──────────────────────────────────────────┘
                               │ WebSocket (JSON + Base64 图像帧)
                               ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                          Go 业务后端 (Gin)                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌────────────┐  │
│  │ 视频采集管线  │  │ gRPC 通信池  │  │ WebSocket   │  │ 数据存储   │  │
│  │ (ffmpeg)     │──│              │──│ 广播器      │  │ (SQLite)   │  │
│  │ - RTSP 流    │  │ - 有序协程池 │  │             │  │            │  │
│  │ - 本地视频   │  │ - 流量控制   │  │             │  │            │  │
│  └──────────────┘  └──────┬───────┘  └─────────────┘  └────────────┘  │
│                           │                                            │
│  ┌────────────────────────┴────────────────────────────────────────┐   │
│  │ 智能过滤器: 时空抑制算法 (ActiveAlerts 列表, IoU 去重)           │   │
│  └─────────────────────────────────────────────────────────────────┘   │
└──────────────────────────────┼──────────────────────────────────────────┘
                               │ gRPC (Protobuf)
                               ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                      Python AI 微服务 (gRPC)                            │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │ YOLOv11 推理引擎 (ultralytics)                                    │  │
│  │ - NMS 非极大值抑制                                                │  │
│  │ - 不确定性计算 (信息熵)                                           │  │
│  │ - 模型热更新 (双缓冲指针切换)                                      │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │ 增量训练模块: Fine-tuning → 自动发布 → 热更新通知                  │  │
│  └──────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 📁 项目结构

```
anomaly_detection_system/
├── ai_service/              # Python AI 微服务
│   ├── pb/                  # gRPC 生成代码
│   ├── models/              # 模型文件
│   ├── server.py            # gRPC 服务主入口
│   ├── detector.py          # YOLOv11 检测器
│   ├── train.py             # 增量训练脚本
│   ├── requirements.txt     # Python 依赖
│   └── Dockerfile           # Docker 镜像
├── backend/                 # Go 业务后端
│   ├── pb/                  # gRPC 生成代码
│   ├── cmd/                 # 程序入口
│   ├── internal/            # 内部模块
│   │   ├── config/          # 配置管理
│   │   ├── grpcclient/      # gRPC 客户端
│   │   ├── pipeline/        # 管线处理
│   │   ├── filter/          # 智能过滤
│   │   ├── storage/         # 数据存储
│   │   └── websocket/       # WebSocket 广播
│   └── Dockerfile           # Docker 镜像
├── frontend/                # Vue 3 前端
│   ├── src/
│   │   ├── components/      # 组件
│   │   ├── views/           # 页面视图
│   │   └── App.vue          # 主应用
│   └── Dockerfile           # Docker 镜像
├── proto/                   # Protobuf 定义文件
│   └── detection.proto      # gRPC 接口定义
├── data/                    # 数据目录
│   ├── images/              # 样本图片
│   ├── labels/              # 标注文件
│   ├── models/              # 模型权重
│   └── database/            # SQLite 数据库
├── Makefile                 # 构建脚本
├── docker-compose.yml       # Docker 编排
└── README.md                # 本文件
```

---

## 🔧 技术栈

| 层级 | 技术选型 |
|------|----------|
| **AI 微服务** | Python 3.9+ · gRPC · YOLOv11 (ultralytics) · PyTorch |
| **业务后端** | Go 1.23+ · Gin · ffmpeg · nhooyr.io/websocket · GORM · SQLite |
| **前端** | Vue 3 · TypeScript · Vite · TailwindCSS · Lucide Icons |
| **通信协议** | gRPC (AI↔Backend) · WebSocket (Backend↔Frontend) |
| **容器化** | Docker · Docker Compose |

---

## 📋 环境依赖

### 🐳 方式一：Docker 启动（推荐，环境要求最少）

| 依赖 | 检查命令 | 最低版本 |
|------|----------|----------|
| **Docker** | `docker --version` | 20.10+ |
| **Docker Compose** | `docker compose version` | 1.29+ |
| **NVIDIA Container Toolkit** _(可选，GPU 加速)_ | `docker run --rm --gpus all nvidia/cuda:11.0-base nvidia-smi` | - |

### 💻 方式二：本地启动（开发调试推荐）

#### 1️⃣ Python 环境（AI 服务）

| 依赖 | 检查命令 | 最低版本 |
|------|----------|----------|
| **Python** | `python3 --version` | 3.9+ |
| **pip** | `pip3 --version` | 21.0+ |
| **CUDA** _(可选，GPU 加速)_ | `nvcc --version` | 11.0+ |
| **cuDNN** _(可选)_ | 检查 `/usr/local/cuda/include/cudnn_version.h` | 8.0+ |

Python 依赖包（通过 `pip install -r requirements.txt` 安装）：
- `grpcio`, `grpcio-tools` - gRPC 通信
- `ultralytics` - YOLOv11 模型
- `torch`, `torchvision` - PyTorch 深度学习框架
- `numpy`, `opencv-python` - 数值计算与图像处理

#### 2️⃣ Go 环境（后端服务）

| 依赖 | 检查命令 | 最低版本 |
|------|----------|----------|
| **Go** | `go version` | 1.23+ |
| **ffmpeg** | `ffmpeg -version` | 4.0+ |
| **SQLite** | `sqlite3 --version` | 3.0+ |

> 💡 **提示**：视频采集使用 ffmpeg 命令行方式实现，无需安装复杂的 OpenCV/GoCV 依赖。

#### 3️⃣ Node.js 环境（前端）

| 依赖 | 检查命令 | 最低版本 |
|------|----------|----------|
| **Node.js** | `node --version` | 18.0+ |
| **npm** | `npm --version` | 9.0+ |

#### 4️⃣ Protobuf 编译器

| 依赖 | 检查命令 | 最低版本 |
|------|----------|----------|
| **protoc** | `protoc --version` | 3.19+ |
| **protoc-gen-go** | `protoc-gen-go --version` | - |
| **protoc-gen-go-grpc** | `protoc-gen-go-grpc --version` | - |

---

## 🚀 启动方式

### 🐳 方式一：Docker Compose 启动（推荐生产环境）

**优势**：环境隔离、一键启动、无需安装复杂依赖

```bash
# 1. 构建镜像
make docker-build

# 2. 启动所有服务（CPU 模式，默认）
make docker-up

# 2-GPU. 如果有 NVIDIA GPU，可使用 GPU 加速模式
make docker-up-gpu

# 3. 查看服务状态
make docker-ps

# 4. 查看日志
make docker-logs

# 5. 停止服务
make docker-down
```

**服务端口**：
| 服务 | 端口 | 说明 |
|------|------|------|
| AI 服务 | 50051 | gRPC 接口 |
| Go 后端 | 8080 | HTTP/WebSocket |
| Vue 前端 | 3000 | Web 界面 |

> 💡 **提示**：
> - Docker 构建时会自动编译 Proto 文件生成 gRPC 代码，无需手动执行 `make proto`
> - **CPU 模式**：默认使用 CPU 进行推理，适用于大多数开发和测试环境
> - **GPU 模式**：需要安装 [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/install-guide.html)，推理速度更快

访问前端：打开浏览器访问 `http://localhost:3000`

### 💻 方式二：本地启动（推荐开发调试）

**优势**：代码热更新、方便调试、日志直观

#### 步骤 1：安装依赖

```bash
# 安装所有依赖
make install

# 或分别安装
make install-ai       # Python 依赖
make install-backend  # Go 依赖
make install-frontend # Node 依赖
```

#### 步骤 2：编译 Proto 文件

```bash
make proto
```

#### 步骤 3：启动服务（需要 3 个终端）

**终端 1 - Python AI 服务**：
```bash
make ai-service
# 或
cd ai_service && python server.py
```

**终端 2 - Go 后端**：
```bash
make backend
# 或
cd backend && go run cmd/main.go
```

**终端 3 - Vue 前端**：
```bash
make frontend
# 或
cd frontend && npm run dev
```

#### 步骤 4：访问系统

打开浏览器访问 `http://localhost:5173`（Vite 开发服务器默认端口）

### 🔄 两种方式对比

| 特性 | Docker 方式 | 本地方式 |
|------|-------------|----------|
| **环境配置** | 极简（仅需 Docker） | 复杂（需安装多种依赖） |
| **启动速度** | 首次较慢（构建镜像） | 较快 |
| **代码热更新** | 需要重新构建镜像 | ✅ 实时生效 |
| **调试便利性** | 一般 | ✅ 方便 |
| **生产部署** | ✅ 推荐 | 不推荐 |
| **GPU 支持** | 需要 NVIDIA Container Toolkit | 原生支持 |

---

## ⚙️ 可配置参数

系统支持通过前端控制台实时调整以下参数：

### 视频源配置

| 参数 | 选项 | 默认值 | 说明 |
|------|------|--------|------|
| 视频源类型 | `RTSP` / `本地文件` | RTSP | 切换视频输入方式 |
| RTSP 地址 | URL 字符串 | - | 摄像头流地址 |
| 本地文件路径 | 文件路径 | - | 本地视频文件 |
| 采集帧率 | `30` / `60` FPS | 30 | 视频采集速度 |

> 💡 **交互说明**：选择视频源类型、输入地址、选择帧率后，需点击 **「应用」按钮** 才会生效，避免误触发请求。

### AI 检测参数

| 参数 | 范围 | 默认值 | 说明 |
|------|------|--------|------|
| 置信度阈值 | 0.0 - 1.0 | 0.5 | 低于此值的检测结果将被过滤 |
| NMS IoU 阈值 | 0.5 - 1.0 | 0.8 | 非极大值抑制的重叠阈值 |
| 不确定性熵值阈值 | 0.0 - 1.0 | 0.3 | 高于此值标记为不确定 |

### 过滤与报警参数

| 参数 | 范围 | 默认值 | 说明 |
|------|------|--------|------|
| 空间抑制 IoU | 0.0 - 1.0 | 0.5 | 与历史报警框重叠度超过此值则不重复报警 |
| 时间抑制窗口 | 1 - 120 秒 | 60 | ActiveAlerts 列表保留时长 |

### 训练参数

| 参数 | 范围 | 默认值 | 说明 |
|------|------|--------|------|
| 自动训练触发阈值 | 50 - 500 | 100 | 积累多少已标注样本后触发训练 |

---

## 📝 API 接口

### gRPC 接口 (AI 服务)

```protobuf
service DetectionService {
  rpc Detect(DetectionRequest) returns (DetectionResponse);
  rpc UpdateConfig(ConfigRequest) returns (ConfigResponse);
  rpc ReloadModel(ReloadRequest) returns (ReloadResponse);
}
```

### WebSocket 接口 (前端通信)

```
ws://localhost:8080/ws

// 下行消息格式
{
  "type": "frame",
  "frame_id": 12345,
  "image": "base64...",
  "detections": [...],
  "alerts": [...]
}

// 上行消息格式 - 标注
{
  "type": "label",
  "alert_id": "xxx",
  "label": "abnormal" | "normal"
}

// 上行消息格式 - 心跳 (每30秒发送一次)
{
  "type": "ping"
}
```

---

## 🛠️ 常用命令

```bash
# 查看所有可用命令
make help

# 构建
make build          # 构建所有
make proto          # 编译 Proto 文件

# 运行
make ai-service     # 启动 AI 服务
make backend        # 启动后端
make frontend       # 启动前端

# Docker
make docker-build   # 构建 Docker 镜像
make docker-up      # 启动容器 (CPU 模式)
make docker-up-gpu  # 启动容器 (GPU 模式)
make docker-down    # 停止容器
make docker-logs    # 查看实时日志
make docker-ps      # 查看服务状态
make docker-restart # 重启服务
make docker-clean   # 清理镜像和卷

# 清理
make clean          # 清理构建产物
```

---

## 📄 许可证

MIT License

---

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！
