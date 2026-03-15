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
| 业务调度层 | Go 1.24, Gin, GoCV, GORM, SQLite, nhooyr.io/websocket |
| 交互展示层 | React 18, TypeScript, Vite, Tailwind CSS, Framer Motion, Zustand |

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

### Docker 启动 (推荐)

```bash
# 在项目根目录
docker-compose up --build
```
    
服务端口：
- Frontend: http://localhost:3000
- Backend API: http://localhost:8080
- AI gRPC: localhost:50051

### 本地开发启动

**1. AI Service (Python)**

```bash
cd ai_service
pip install -r requirements.txt
python server.py
```

**2. Backend (Go)**

```bash
cd backend
go mod tidy
go run ./cmd/server

# 带真实视频采集（需要 OpenCV）
go run -tags gocv ./cmd/server/
```

> 注：GoCV 需要安装 OpenCV。未安装时系统自动使用 fallback 模式 (合成帧/图片目录)。

**3. Frontend (React)**

```bash
cd frontend
npm install
npm run dev
```

## API 端点

| 方法 | 路径 | 用途 |
|------|------|------|
| GET | `/api/config` | 获取当前配置 |
| PUT | `/api/config` | 更新配置参数 |
| POST | `/api/samples/:id/label` | 提交人工标注 |
| POST | `/api/samples/ai-judge` | AI 批量判断 |
| GET | `/api/samples?status=labeled` | 获取样本列表 |
| PATCH | `/api/samples/:id/relabel` | 修改标签 |
| POST | `/api/training/trigger` | 触发增量训练 |
| GET | `/api/training/history` | 训练历史 |
| GET | `/api/stream/mjpeg` | MJPEG 视频流 |
| WS | `/ws/events` | WebSocket 异常推送 |
| POST | `/api/pipeline/start` | 启动 Pipeline |
| POST | `/api/pipeline/stop` | 停止 Pipeline |

## 核心工作流

1. **视频采集** → Producer 协程读取 RTSP/本地视频
2. **AI 推理** → 有序协程池并发调用 Python gRPC，min-heap 重排保序
3. **可视化推流** → Broadcaster 将标注画面通过 MJPEG 推送前端
4. **异常检测** → 不确定性目标经时空过滤后通过 WebSocket 推送
5. **人工标注** → 前端待处理队列，支持确认/误报二分类
6. **增量训练** → 标注数据回流触发模型微调，双缓冲热更新
