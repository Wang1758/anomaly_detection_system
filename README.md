# Anomaly Detection System

基于主动学习 (Active Learning) 的实时计算机视觉闭环系统，采用 Human-in-the-loop 架构实现模型自我进化。

## 架构

```
┌─────────────────┐     gRPC      ┌──────────────────┐    MJPEG/WS     ┌─────────────────┐
│   AI Service    │◄─────────────►│   Go Backend     │◄───────────────►│   React Frontend│
│   (Python)      │               │   (Gin)          │                 │   (Vite+TS)     │
│                 │               │                  │                 │                 │
│ • YOLOv11       │               │ • Pipeline       │                 │ • 实时监控       │
│ • 不确定性度量    │               │ • 有序协程池      │                 │ • 异常标注       │
│ • OpenCV 可视化  │               │ • 时空过滤        │                 │ • 配置管理       │
│ • 模型热更新     │               │ • SQLite/GORM    │                 │ • 拖拽改标       │
└─────────────────┘               └──────────────────┘                 └─────────────────┘
```

## 技术栈

| 层级 | 技术 |
|------|------|
| AI 计算层 | Python 3.11, gRPC, YOLOv11 (ultralytics), OpenCV, PyTorch |
| 业务调度层 | Go 1.25, Gin, GoCV, GORM, SQLite, nhooyr.io/websocket |
| 交互展示层 | React 19, TypeScript, Vite, Tailwind CSS, Framer Motion, Zustand |

## 项目结构

```
anomaly_detection_system/
├── proto/              # gRPC Protobuf 定义
├── ai_service/         # Python AI 微服务
├── backend/            # Go 业务后端
├── frontend/           # React 前端
├── data/               # 数据目录 (图片/标签/模型/数据库)
├── docker-compose.yml
└── README.md
```

## 快速开始
### 本地开发前置环境配置

本项目本地开发需先安装以下环境，其中ai_service服务运行在windows主机上，go和前端运行在linux上。

1. **Python 3.11+**
2. **Go 1.25+**
3. **Node.js 20+**
4. **OpenCV 开发库 + 编译工具链**

目录与文件准备要求：

- 数据集目录位于 `data/indoor`，应该放到与ai_service平级的目录。
- `ai_service/models/best.pt` 或 `data/models/` 下至少存在一个可加载权重。
- 若使用“AI 一键判断”的多模态模型，需要提前配置 `LLM_API_KEY`（可选，不配置会回退到 YOLO 重检测）。

python建议使用conda虚拟环境管理，并安装pytorch和nvidia驱动。如果不安装nvidia驱动，则默认使用cpu进行运算。

### 本地开发启动
**1. AI Service (Python)**

```bash
cd ai_service
pip install -r requirements.txt
python server.py
```

开启性能日志（可选）：

```bash
python server.py --perf-log
```

**2. Backend (Go)**

```bash
cd backend
go mod tidy
go run ./cmd/server/
```

开启性能日志（可选）：

```bash
go run ./cmd/server/ --perf-log
```

> 注：后端仅支持 GoCV 模式，请先安装 OpenCV 后再启动。

### 宿主机 + 虚拟机 (NAT) 部署说明

Python `ai_service` 跑在宿主机，而 Go 后端跑在 VMware 虚拟机：

- 不要使用 `localhost:50051` 或 `127.0.0.1:50051`（这只会指向虚拟机自身）。
- 请将 `AI_SERVICE_ADDR` 设置为**宿主机在 VMnet8/NAT 网卡上的 IP**，例如：

```bash
cd backend
AI_SERVICE_ADDR=192.168.***.***:50051 SERVER_PORT=:8080 DATA_DIR=../data go run ./cmd/server/
```

在虚拟机内可先做连通性检查：

```bash
nc -vz 192.168.***.*** 50051
```

若不通，请检查宿主机防火墙是否放行 `50051/tcp`，并确认 Python 服务已启动（`python server.py`）。

如需诊断性能瓶颈，可在两端启动时追加 `--perf-log` 参数。

**3. Frontend (React)**

```bash
cd frontend
npm install
npm run dev
```

### 常见问题排查（本地开发）

1. **Backend 启动时报 OpenCV/GoCV 相关错误**
	- 确认已安装 GoCV 所需要的环境

2. **Backend 无法连接 AI Service (`connection refused` / 超时)**
	- 确认 `ai_service` 已启动并监听 `50051`。
	- 本机开发建议使用 `AI_SERVICE_ADDR=localhost:50051`。
	- NAT 场景请改为宿主机 NAT 网卡 IP，并使用 `nc -vz <host-ip> 50051` 检查连通性。

3. **训练或推理时报模型文件不存在**
	- 确认 `ai_service/models/best.pt` 或 `data/models/` 下存在可加载权重。
	- 若使用训练热更新，确认 `DATA_DIR/models/latest.pt` 路径与 `AI_RELOAD_MODEL_PATH` 配置一致。

## API 端点

| 方法 | 路径 | 用途 |
|------|------|------|
| GET | `/api/config` | 获取当前配置 |
| PUT | `/api/config` | 更新配置参数 |
| POST | `/api/samples/:id/label` | 提交人工标注 |
| POST | `/api/samples/ai-judge` | AI 批量研判（优先调用多模态大模型看图判断，未配置 `LLM_API_KEY` 时回退到 YOLO 重检测） |
| GET | `/api/samples?status=labeled` | 获取样本列表 |
| PATCH | `/api/samples/:id/relabel` | 修改标签 |
| POST | `/api/training/trigger` | 触发增量训练 |
| GET | `/api/training/history` | 训练历史 |
| GET | `/api/stream/mjpeg` | MJPEG 视频流 |
| WS | `/ws/events` | WebSocket 异常推送 |
| POST | `/api/pipeline/start` | 启动 Pipeline |
| POST | `/api/pipeline/stop` | 停止 Pipeline |
| GET | `/api/pipeline/status` | 获取 Pipeline 运行状态 |

## 环境变量

项目中所有通过环境变量配置的参数如下。本地开发时在启动命令前添加。

### AI Service (Python)

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `GRPC_PORT` | gRPC 服务监听端口 | `50051` |
| `MODEL_DIR` | YOLO 模型文件目录（容器内路径） | `models` |

### Backend (Go)

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `AI_SERVICE_ADDR` | Python AI 服务的 gRPC 地址 | `192.168.3.23:50051` |
| `SERVER_PORT` | 后端 HTTP 监听端口（自动补 `:` 前缀） | `:8080` |
| `DATA_DIR` | 数据根目录（图片/数据库/模型） | `../data` |
| `TRAINING_PYTHON` | 增量训练 Python 解释器 | `python3` |
| `TRAINING_SCRIPT` | 增量训练脚本路径 | `../ai_service/retrain.py` |
| `TRAINING_BASE_MODEL` | 训练基线权重路径（可选） | 空（自动选择） |
| `TRAINING_BASE_DATASET` | 原始训练集路径（YOLO 格式，`images/train`+`labels/train`） | 空（可选） |
| `TRAINING_EPOCHS` | 增量训练 epoch 数（可选） | `10` |
| `TRAINING_BATCH` | 增量训练 batch size（可选） | `8` |
| `TRAINING_IMGSZ` | 增量训练输入尺寸（可选） | `640` |
| `TRAINING_DEVICE` | 增量训练 device（可选） | 空（ultralytics 自动） |
| `AI_RELOAD_MODEL_PATH` | 训练后通知 AI 服务热更新的模型路径 | 默认与 `DATA_DIR/models/latest.pt` 一致 |
| `LLM_API_KEY` | 多模态大模型 API 密钥（不填则"AI一键判断"回退到 YOLO 重检测） | 无（未启用） |
| `LLM_BASE_URL` | 多模态大模型 API 端点（兼容 OpenAI / 通义千问 / DeepSeek 等） | `https://api.openai.com/v1` |
| `LLM_MODEL` | 多模态大模型名称 | `gpt-4o` |

> `LLM_API_KEY` 不会通过 `GET /api/config` 返回给前端。

**国产模型配置示例：**

```bash
# 通义千问
LLM_API_KEY=sk-xxx LLM_BASE_URL=https://dashscope.aliyuncs.com/compatible-mode/v1 LLM_MODEL=qwen-vl-max

# DeepSeek
LLM_API_KEY=sk-xxx LLM_BASE_URL=https://api.deepseek.com/v1 LLM_MODEL=deepseek-chat
```

### Frontend (React)

前端本身不读取环境变量。开发模式下 Vite 代理 `/api` 和 `/ws` 到 `http://localhost:8080`（见 `vite.config.ts`）；生产模式由 Nginx 反向代理到 `backend:8080`（见 `nginx.conf`）。

## 核心工作流

1. **视频采集** → Producer 协程读取 RTSP/本地视频
2. **AI 推理** → 有序协程池并发调用 Python gRPC，min-heap 重排保序
3. **可视化推流** → Broadcaster 将标注画面通过 MJPEG 推送前端
4. **异常检测** → 不确定性目标经时空过滤后通过 WebSocket 推送
5. **人工标注** → 前端待处理队列，支持确认/误报二分类
6. **增量训练** → 标注数据回流触发模型微调，双缓冲热更新

### 增量训练执行链路（已实现）

1. 前端点击“触发增量训练”调用 `POST /api/training/trigger`
2. 后端创建 `training_runs` 记录（`running`）并异步执行 `ai_service/retrain.py`
3. `retrain.py` 从 `data/db/app.db` 读取 `status=labeled` 样本，构建 YOLO 数据集并训练
	- 人工标注为“乌骨鸡”时：保留不确定检测框坐标作为正样本框
	- 人工标注为“不是乌骨鸡”时：清空该样本检测框，仅作为困难背景样本
	- 若配置 `TRAINING_BASE_DATASET`，会将人工筛选样本混入原始训练集做迭代训练
4. 训练产出 `DATA_DIR/models/latest.pt` 和 `latest_metrics.json`
5. 后端调用 gRPC `ReloadModel(model_path)` 通知 AI Service 热更新
6. Python `ModelManager` 采用双缓冲原子切换模型；成功后 run 状态变 `succeeded`，并把样本状态改为 `trained`

> 容器部署时，如果 backend 与 ai_service 的容器内路径不同，请设置 `AI_RELOAD_MODEL_PATH`（例如 `/app/models/latest.pt`）。
