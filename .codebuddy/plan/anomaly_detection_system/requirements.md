# 需求文档

## 引言

本项目旨在构建一个"基于主动学习的实时计算机视觉闭环系统"——智慧养殖场监控系统。该系统采用 Monorepo 架构，整合 Go 后端、Python AI 微服务和 Vue 3 前端三大技术栈，实现从视频采集、目标检测、不确定性评估、人工交互到模型增量训练的完整闭环。

**系统核心价值**：
- **实时检测**：对养殖场视频流进行实时目标检测
- **主动学习**：通过不确定性评估，智能筛选需要人工干预的样本
- **闭环训练**：人工标注数据自动回流，触发模型增量更新
- **零停机更新**：采用双缓冲机制实现模型热更新

**技术架构**：
- **AI 微服务 (Python)**：YOLOv11 + gRPC，负责推理与不确定性计算
- **业务后端 (Go)**：Gin + GoCV + WebSocket，负责视频采集、调度与数据管理
- **前端 (Vue 3 + TypeScript)**：Canvas 渲染 + WebSocket 通信，负责可视化与人机交互

---

## 需求

### 需求 1：Python AI 微服务

**用户故事**：作为系统架构师，我希望有一个高性能的 AI 推理微服务，以便实现实时目标检测和不确定性评估。

#### 验收标准

1. **gRPC 服务接口**
   - WHEN 收到包含图像字节数据的 gRPC 请求 THEN 系统 SHALL 返回包含检测结果的 JSON 列表
   - WHEN 返回检测结果 THEN 每个结果 SHALL 包含 `bbox`（边界框坐标）、`class`（类别）、`conf`（置信度）、`is_uncertain`（不确定性标记）字段

2. **不确定性计算**
   - WHEN 进行模型推理 THEN 系统 SHALL 计算每个检测框的预测概率分布信息熵
   - IF 信息熵值 > 预设阈值 THEN 系统 SHALL 将该检测框的 `is_uncertain` 字段设置为 `True`
   - WHEN 计算不确定性 THEN 系统 SHALL 支持混合加权公式：`Score = w1 * (1 - Conf) + w2 * IoU_With_Other_Boxes`

3. **非极大值抑制 (NMS)**
   - WHEN 执行检测 THEN 系统 SHALL 在计算不确定性之前执行严格的 NMS 处理
   - IF 两个检测框的 IoU > NMS IoU 阈值 且类别相同 THEN 系统 SHALL 强制合并为一个检测框
   - WHEN 收到 NMS IoU 阈值更新请求 THEN 系统 SHALL 动态更新 NMS 参数（默认值：0.8，可调范围：0.5 - 1.0）

4. **模型热更新机制**
   - WHEN 收到模型重载指令 THEN 系统 SHALL 在后台静默加载新模型权重（双缓冲机制）
   - WHEN 新模型加载完成 THEN 系统 SHALL 无缝切换推理指针，释放旧模型内存
   - WHEN 模型切换过程中 THEN 系统 SHALL 保证零停机时间

5. **协议定义**
   - WHEN 定义服务接口 THEN 系统 SHALL 使用 Protobuf 定义 gRPC 接口
   - WHEN 服务启动 THEN 系统 SHALL 使用 `concurrent.futures` 实现并发处理

6. **参数热更新**
   - WHEN 收到参数更新请求 THEN 系统 SHALL 支持动态更新以下参数：
     - NMS IoU 阈值（0.5 - 1.0）
     - 置信度阈值（0.0 - 1.0）
     - 不确定性熵值阈值

---

### 需求 2：Go 业务后端

**用户故事**：作为系统架构师，我希望有一个高性能的业务后端，以便处理 I/O 密集型任务、并发控制和业务逻辑调度。

#### 验收标准

1. **视频源管理**
   - WHEN 系统启动 THEN 系统 SHALL 支持两种视频源类型：RTSP 流和本地视频文件
   - WHEN 视频源类型为 RTSP THEN 系统 SHALL 接受 RTSP 地址作为输入参数
   - WHEN 视频源类型为本地文件 THEN 系统 SHALL 接受本地文件路径作为输入参数
   - WHEN 收到视频源切换请求 THEN 系统 SHALL 支持运行时动态切换视频源
   - IF 视频源切换成功 THEN 系统 SHALL 立即开始处理新的视频流

2. **视频采集管线**
   - WHEN 系统启动 THEN 协程 A SHALL 负责读取视频源
   - IF 视频源为本地文件 THEN 系统 SHALL 加入帧率控制（Ticker），强制休眠 `1/FPS` 秒模拟真实摄像头速度
   - WHEN 收到帧率更新请求 THEN 系统 SHALL 支持动态调整采集帧率（可选值：30 FPS 或 60 FPS，默认：30 FPS）
   - WHEN 读取视频帧 THEN 系统 SHALL 使用 GoCV 进行处理

3. **gRPC 通信管线**
   - WHEN 协程 B 工作时 THEN 系统 SHALL 负责与 Python gRPC 服务通信
   - WHEN 发送请求 THEN 系统 SHALL 使用有序协程池批量发送，确保返回结果按序号排序
   - WHEN 通信过程中 THEN 系统 SHALL 包含流量控制逻辑

4. **WebSocket 广播管线**
   - WHEN 协程 C 工作时 THEN 系统 SHALL 将检测结果坐标数据和原始图片帧通过 WebSocket 广播给前端
   - WHEN 广播数据 THEN 系统 SHALL 使用 `nhooyr.io/websocket` 库
   - WHEN 发送数据 THEN 系统 SHALL 以 JSON 格式传输坐标与 ID 数据

5. **智能过滤机制（空间及时间抑制）**
   - WHEN 收到"不确定目标"检测结果 THEN 系统 SHALL 检查过去 N 秒内是否已在相近位置发送过报警
   - WHEN 系统运行时 THEN 系统 SHALL 维护 `ActiveAlerts` 列表，存储过去 60s 内已发送的报警点（包含 CenterPoint、BBox、Timestamp）
   - IF 新报警与已有报警的 IoU > 空间抑制 IoU 阈值 THEN 系统 SHALL 直接丢弃该报警
   - WHEN 收到空间抑制 IoU 阈值更新请求 THEN 系统 SHALL 动态更新该参数（默认值：0.5，可调范围：0.0 - 1.0）
   - IF 新报警通过过滤 THEN 系统 SHALL 推送报警到前端

6. **数据快照与存储**
   - WHEN 触发有效异常且前端返回处理结果 THEN 系统 SHALL 将当前帧保存到本地磁盘
   - WHEN 保存样本 THEN 系统 SHALL 同时将元数据（ID、路径、时间、标注结果）保存到 SQLite 数据库
   - WHEN 操作数据库 THEN 系统 SHALL 使用 GORM 进行 ORM 操作

7. **闭环训练触发**
   - WHEN 数据库中"已人工修正"的样本数量 >= 自动训练触发阈值 THEN 系统 SHALL 触发 Shell 脚本调用 Python 训练脚本
   - WHEN 收到训练触发阈值更新请求 THEN 系统 SHALL 动态更新该参数（默认值：100，可调范围：50 - 500）
   - WHEN 训练完成 THEN 系统 SHALL 发送 `Reload` 指令给 Python 推理服务

8. **配置热更新**
   - WHEN 收到前端配置更新请求 THEN 系统 SHALL 实时更新相关参数
   - WHEN 需要更新 Python 服务参数 THEN 系统 SHALL 转发配置更新指令

---

### 需求 3：前端可视化与交互

**用户故事**：作为监控人员，我希望有一个直观、美观且具有沉浸感的监控界面，以便实时查看检测结果并高效处理不确定性样本。

#### 验收标准

##### 3.1 设计系统：Ethereal Glass（以太玻璃）

**设计理念**：采用 macOS Big Sur / VisionOS 设计语言，打造"悬浮在空间中"的沉浸式体验，摒弃传统 Admin Dashboard 布局。

1. **核心材质 (Material)**
   - WHEN 渲染容器（侧边栏、卡片、面板）THEN 系统 SHALL 使用高强度背景模糊 (`backdrop-blur-2xl` 或 `blur(24px)`)
   - WHEN 设置容器背景 THEN 系统 SHALL 使用高透明度白色背景 (`bg-white/60` 到 `bg-white/80`)
   - WHEN 设置边框 THEN 系统 SHALL 使用 1px 半透明白色边框 (`border-white/80`)，禁止使用黑色边框
   - WHEN 设置阴影 THEN 系统 SHALL 使用彩色弥散阴影 (`shadow-blue-500/20`) 或柔和大阴影 (`shadow-2xl`)，禁止使用生硬黑色投影
   - WHEN 设置圆角 THEN 系统 SHALL 使用极度圆润的圆角 (`rounded-[32px]` 或 `rounded-3xl`)

2. **悬浮布局 (Floating Layout)**
   - WHEN 渲染页面布局 THEN 系统 SHALL 采用悬浮岛设计，所有主要区域不得贴边显示
   - WHEN 布局左侧导航栏 THEN 系统 SHALL 将其渲染为独立悬浮长条，与屏幕边缘保持间距
   - WHEN 布局中央内容区 THEN 系统 SHALL 将其渲染为独立悬浮面板
   - WHEN 布局右侧报警栏 THEN 系统 SHALL 将其渲染为独立悬浮面板
   - WHEN 渲染各区域之间 THEN 系统 SHALL 保持明显间距以露出底部背景
   - WHEN 渲染页面背景 THEN 系统 SHALL 使用流体网格渐变 (Mesh Gradient) 或抽象极光背景，颜色为浅灰、淡蓝、淡紫的淡雅混合

3. **色彩规范 (Apple System Colors)**
   - WHEN 渲染激活状态/主按钮 THEN 系统 SHALL 使用 Apple Blue `#007AFF`
   - WHEN 渲染不确定目标/警告状态 THEN 系统 SHALL 使用 Apple Orange `#FF9500`
   - WHEN 渲染正常/成功状态 THEN 系统 SHALL 使用 Apple Green `#34C759`
   - WHEN 渲染主要文字 THEN 系统 SHALL 使用 `text-gray-900`
   - WHEN 渲染次要文字 THEN 系统 SHALL 使用 `text-gray-500`

##### 3.2 左侧悬浮导航 (Floating Dock)

1. **导航样式**
   - WHEN 渲染导航栏 THEN 系统 SHALL 采用类似 VisionOS 的垂直 Dock 设计
   - WHEN 渲染选中态图标 THEN 系统 SHALL 使用白色背景 + 强阴影 + 略微放大效果
   - WHEN 渲染未选中态图标 THEN 系统 SHALL 使用半透明背景
   - WHEN 鼠标悬停未选中图标 THEN 系统 SHALL 将背景变为白色

2. **导航功能**
   - WHEN 用户点击"监控 (Monitor)"图标 THEN 系统 SHALL 切换到监控视图并显示右侧报警队列
   - WHEN 用户点击"控制台 (Console)"图标 THEN 系统 SHALL 切换到控制台视图并隐藏右侧侧边栏
   - WHEN 使用图标 THEN 系统 SHALL 使用 Lucide 图标库（Camera、Settings2 等）

##### 3.3 中央监控区 (The Stage)

1. **视频源选择器（顶部）**
   - WHEN 渲染监控区顶部 THEN 系统 SHALL 显示视频源选择器组件
   - WHEN 用户选择视频源类型 THEN 系统 SHALL 提供两个选项：「RTSP 流」和「本地视频」
   - IF 用户选择「RTSP 流」THEN 系统 SHALL 显示 RTSP 地址输入框
   - IF 用户选择「本地视频」THEN 系统 SHALL 显示文件路径输入框或文件选择按钮
   - WHEN 用户确认视频源 THEN 系统 SHALL 发送配置更新请求到后端
   - WHEN 视频源切换成功 THEN 系统 SHALL 显示成功提示并开始播放新视频流

2. **视频容器**
   - WHEN 渲染视频播放区域 THEN 系统 SHALL 展示 16:9 比例的容器
   - WHEN 渲染视频边框 THEN 系统 SHALL 使用厚重白色边框 (`border-[6px] border-white`)，呈现"画框"效果
   - WHEN 通过 WebSocket 接收数据 THEN 系统 SHALL 实时更新视频帧和检测框坐标

3. **检测框覆盖层 (Overlay)**
   - WHEN 绘制检测框 THEN 系统 SHALL 使用 HTML5 Canvas 覆盖在视频之上
   - WHEN 绘制正常目标框 THEN 系统 SHALL 使用 Apple Blue (`#007AFF`) 细线
   - WHEN 绘制异常目标框 THEN 系统 SHALL 使用 Apple Orange (`#FF9500`) 细线 + 内部极淡橙色填充 + `backdrop-blur` 效果
   - WHEN 显示异常框标签 THEN 系统 SHALL 在框上方显示胶囊形状标签，内容为"? 疑似目标 (ID:xxx)"

4. **状态指示**
   - WHEN 显示实时状态 THEN 系统 SHALL 在视频区域显示 "LIVE" 状态标签，使用 Apple Green (`#34C759`)

##### 3.4 右侧报警队列 (Glass List)

1. **队列容器**
   - WHEN 渲染报警队列容器 THEN 系统 SHALL 使用玻璃材质悬浮面板
   - IF 当前为监控视图 THEN 系统 SHALL 显示报警队列
   - IF 当前为控制台视图 THEN 系统 SHALL 隐藏报警队列

2. **列表项卡片**
   - WHEN 渲染卡片默认状态 THEN 系统 SHALL 使用 `bg-white/40` 背景
   - WHEN 鼠标悬停卡片 THEN 系统 SHALL 将背景变为 `bg-white/80` 并产生轻微上浮效果
   - WHEN 渲染卡片内容 THEN 系统 SHALL 包含缩略图、时间戳和"进入详情"小箭头

3. **不确定性通知交互**
   - WHEN 收到 `is_uncertain=true` 信号 THEN 系统 SHALL 在队列中弹出带截图的小卡片
   - WHEN 截取缩略图 THEN 系统 SHALL 截取 BBox 向外扩大 50% 区域的图像
   - WHEN 显示新卡片 THEN 系统 SHALL 显示 10 秒倒计时
   - IF 用户点击卡片 THEN 系统 SHALL 提供"异常"或"正常"选项
   - IF 10 秒内无操作 THEN 系统 SHALL 自动将卡片收入"待处理列表"
   - WHEN 用户选择后 THEN 系统 SHALL 将结果发送给后端

##### 3.5 控制台 (Console Tab)

1. **整体布局**
   - WHEN 用户进入控制台 THEN 系统 SHALL 展示参数调节仪表盘
   - WHEN 渲染控制台面板 THEN 系统 SHALL 使用玻璃材质悬浮设计
   - WHEN 组织控制台模块 THEN 系统 SHALL 按以下分组显示：视频采集参数、AI 检测参数、报警过滤参数、训练配置、系统开关

2. **视频采集参数模块**
   - WHEN 渲染视频采集参数区 THEN 系统 SHALL 显示以下控件：
     - 视频源类型选择器（RTSP 流 / 本地视频）
     - RTSP 地址输入框（当选择 RTSP 流时显示）
     - 本地文件路径输入框 + 文件选择按钮（当选择本地视频时显示）
     - 采集帧率选择器（30 FPS / 60 FPS 两个选项，默认 30 FPS）
   - WHEN 用户修改视频源配置 THEN 系统 SHALL 提供"应用"按钮确认更改
   - WHEN 配置更新成功 THEN 系统 SHALL 显示成功提示

3. **AI 检测参数模块 (iOS Style Sliders)**
   - WHEN 渲染 AI 检测参数区 THEN 系统 SHALL 显示以下滑块控件：
     - 置信度阈值滑块（范围：0.0 - 1.0，默认：0.5）
     - 不确定性熵值阈值滑块（范围：0.0 - 1.0）
     - NMS IoU 阈值滑块（范围：0.5 - 1.0，默认：0.8）
   - WHEN 渲染滑块控件 THEN 系统 SHALL 模仿 iOS 的物理手感设计
   - WHEN 渲染滑块轨道 THEN 系统 SHALL 使用 Apple Blue (`#007AFF`) 作为激活色
   - WHEN 用户拖动滑块 THEN 系统 SHALL 实时显示当前数值
   - WHEN 用户释放滑块 THEN 系统 SHALL 发送参数更新请求到后端

4. **报警过滤参数模块**
   - WHEN 渲染报警过滤参数区 THEN 系统 SHALL 显示以下控件：
     - 空间抑制 IoU 阈值滑块（范围：0.0 - 1.0，默认：0.5）
   - WHEN 用户修改空间抑制阈值 THEN 系统 SHALL 发送参数更新请求到后端

5. **训练配置模块**
   - WHEN 渲染训练配置区 THEN 系统 SHALL 显示以下控件：
     - 自动训练触发阈值滑块（范围：50 - 500，默认：100，步进：10）
     - 当前待训练样本数量显示
     - "立即开始训练" 按钮
   - WHEN 用户点击"立即开始训练"按钮 THEN 系统 SHALL 进入 loading 状态并显示进度
   - WHEN 训练完成 THEN 系统 SHALL 提示"模型已热更新"
   - WHEN 用户修改触发阈值 THEN 系统 SHALL 发送参数更新请求到后端

6. **系统开关模块 (iOS Toggle)**
   - WHEN 渲染开关控件 THEN 系统 SHALL 完全复刻 iOS Toggle 样式
   - WHEN 显示系统开关 THEN 系统 SHALL 提供以下开关：
     - 启用报警推送（默认：开）
     - 自动保存样本（默认：开）

---

### 需求 4：项目结构与技术栈

**用户故事**：作为开发者，我希望有一个清晰的 Monorepo 项目结构，以便高效开发和维护。

#### 验收标准

1. **根目录结构**
   - WHEN 初始化项目 THEN 系统 SHALL 创建 `/anomaly_detection_system` 根目录
   - WHEN 组织代码 THEN 系统 SHALL 包含 `backend`、`ai_service`、`frontend`、`proto`、`data` 五个主要目录

2. **Go 后端技术栈**
   - WHEN 开发后端 THEN 系统 SHALL 使用 Gin 作为 Web 框架
   - WHEN 处理视频 THEN 系统 SHALL 使用 GoCV 进行视频读取
   - WHEN 实现 WebSocket THEN 系统 SHALL 使用 `nhooyr.io/websocket`
   - WHEN 操作数据库 THEN 系统 SHALL 使用 GORM + SQLite

3. **Python AI 服务技术栈**
   - WHEN 开发 AI 服务 THEN 系统 SHALL 使用 gRPC + `concurrent.futures`
   - WHEN 实现检测 THEN 系统 SHALL 使用 `ultralytics` 库的 YOLOv11 模型

4. **前端技术栈**
   - WHEN 开发前端 THEN 系统 SHALL 使用 Vue 3 + TypeScript (基于 Vite)
   - WHEN 实现样式 THEN 系统 SHALL 使用 TailwindCSS
   - WHEN 使用图标 THEN 系统 SHALL 使用 Lucide 图标库

5. **共享协议**
   - WHEN 定义接口 THEN 系统 SHALL 在 `proto` 目录存放共享的 `.proto` 文件

6. **数据存储**
   - WHEN 存储数据 THEN 系统 SHALL 在 `data` 目录存放图片、标签文件和 SQLite 数据库

---

### 需求 5：闭环自动化训练

**用户故事**：作为系统管理员，我希望系统能够自动触发增量训练，以便持续优化模型性能。

#### 验收标准

1. **训练触发条件**
   - WHEN 数据库中"已人工修正"样本数量 >= 自动训练触发阈值（可配置，默认 100）THEN 系统 SHALL 自动触发训练流程
   - WHEN 用户在控制台点击"立即开始训练" THEN 系统 SHALL 手动触发训练流程

2. **增量训练策略**
   - WHEN 执行训练 THEN 系统 SHALL 基于旧权重进行 Fine-tuning
   - WHEN 微调模型 THEN 系统 SHALL 冻结骨干网络，只微调 Head 层以加速训练

3. **训练与发布流程**
   - WHEN 训练开始 THEN Python 脚本 SHALL 读取新增标注数据
   - WHEN 训练结束 THEN 系统 SHALL 保存新权重文件
   - WHEN 新权重保存成功 THEN 系统 SHALL 通知 Go 后端
   - WHEN Go 后端收到通知 THEN 系统 SHALL 发送 `Reload` 指令给 Python 推理服务

---

## 边界情况与技术限制

### 边界情况

1. **视频源异常**
   - IF RTSP 流断开 THEN 系统 SHALL 自动重连并记录日志
   - IF 本地视频文件损坏 THEN 系统 SHALL 跳过损坏帧并继续处理
   - IF 用户输入的 RTSP 地址无效 THEN 系统 SHALL 显示错误提示并保持当前视频源
   - IF 用户选择的本地文件不存在或格式不支持 THEN 系统 SHALL 显示错误提示

2. **AI 服务不可用**
   - IF gRPC 服务无响应 THEN Go 后端 SHALL 进行重试（最多 3 次）
   - IF 重试失败 THEN 系统 SHALL 降级运行（仅显示原始视频流）

3. **WebSocket 断开**
   - IF WebSocket 连接断开 THEN 前端 SHALL 自动重连
   - WHEN 重连成功 THEN 系统 SHALL 恢复实时数据流

4. **模型加载失败**
   - IF 新模型权重加载失败 THEN 系统 SHALL 保持使用旧模型
   - WHEN 加载失败 THEN 系统 SHALL 记录错误日志并通知管理员

5. **参数更新失败**
   - IF 参数更新请求失败 THEN 前端 SHALL 显示错误提示并恢复到之前的值
   - WHEN 参数更新成功 THEN 前端 SHALL 显示成功提示

### 技术限制

1. **性能约束**
   - 系统 SHALL 支持至少 30 FPS 的实时处理能力
   - 系统 SHALL 支持可选的 60 FPS 高帧率模式（需要足够的硬件性能）
   - gRPC 单次推理延迟 SHALL 不超过 100ms

2. **存储约束**
   - 单次训练样本数量 SHALL 控制在 50-500 张（可配置）
   - 图片存储 SHALL 使用 JPEG 格式以节省空间

3. **并发约束**
   - WebSocket SHALL 支持至少 10 个并发客户端连接
   - 协程池大小 SHALL 根据 GPU 性能动态调整

---

## 可配置参数汇总

| 参数名称 | 所属模块 | 范围 | 默认值 | 说明 |
|---------|---------|------|--------|------|
| 视频源类型 | Go 后端 | RTSP / 本地视频 | RTSP | 视频输入源类型 |
| RTSP 地址 | Go 后端 | 字符串 | - | RTSP 流地址 |
| 本地视频路径 | Go 后端 | 文件路径 | - | 本地视频文件路径 |
| 采集帧率 | Go 后端 | 30 / 60 FPS | 30 FPS | 视频采集帧率 |
| 置信度阈值 | Python AI | 0.0 - 1.0 | 0.5 | 检测置信度过滤 |
| 不确定性熵值阈值 | Python AI | 0.0 - 1.0 | - | 判断是否为不确定目标 |
| NMS IoU 阈值 | Python AI | 0.5 - 1.0 | 0.8 | 非极大值抑制参数 |
| 空间抑制 IoU 阈值 | Go 后端 | 0.0 - 1.0 | 0.5 | 报警去重阈值 |
| 自动训练触发阈值 | Go 后端 | 50 - 500 | 100 | 触发自动训练的样本数 |
| 启用报警推送 | 前端 | 开 / 关 | 开 | 是否推送报警通知 |
| 自动保存样本 | 前端 | 开 / 关 | 开 | 是否自动保存异常样本 |

---

## 成功标准

1. **功能完整性**：所有核心功能模块按需求实现并通过测试
2. **实时性能**：视频处理延迟 < 200ms，用户交互响应 < 100ms
3. **系统稳定性**：连续运行 24 小时无崩溃
4. **闭环验证**：完成至少一次完整的"检测-标注-训练-更新"闭环流程
5. **用户体验**：前端界面响应流畅，视觉效果达到 Apple Design Award 级别的精致度
6. **配置灵活性**：所有可配置参数均可通过前端控制台实时调整，无需重启服务
