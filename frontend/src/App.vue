<template>
  <div class="h-screen w-screen aurora-bg overflow-hidden p-4 flex gap-4">
    <!-- Toast 弹窗容器 -->
    <div class="toast-container">
      <div
        v-for="toast in toasts"
        :key="toast.id"
        class="toast"
        :class="[`toast-${toast.type}`, { 'toast-out': toast.leaving }]"
      >
        <CheckCircle v-if="toast.type === 'success'" class="w-5 h-5" />
        <XCircle v-else-if="toast.type === 'error'" class="w-5 h-5" />
        <AlertCircle v-else-if="toast.type === 'warning'" class="w-5 h-5" />
        <Info v-else class="w-5 h-5" />
        <span>{{ toast.message }}</span>
      </div>
    </div>

    <!-- 左侧悬浮导航 Dock -->
    <FloatingDock 
      :activeTab="activeTab" 
      @update:activeTab="activeTab = $event" 
    />

    <!-- 中央主内容区 -->
    <main class="flex-1 flex flex-col gap-4">
      <!-- 顶部视频源选择器 -->
      <VideoSourceSelector 
        v-if="activeTab === 'monitor'"
        :config="videoConfig"
        @apply="applyVideoConfig"
      />

      <!-- 主内容 -->
      <div class="flex-1 glass rounded-4xl overflow-hidden">
        <MonitorView 
          v-if="activeTab === 'monitor'" 
          :wsConnected="wsConnected"
          :currentFrame="currentFrame"
          :detections="detections"
        />
        <ConsoleView 
          v-else 
          :config="systemConfig"
          :trainingStatus="trainingStatus"
          @updateAI="updateAIConfig"
          @updateFilter="updateFilterConfig"
          @updateTraining="updateTrainingConfig"
          @triggerTraining="triggerTraining"
        />
      </div>
    </main>

    <!-- 右侧报警队列（仅监控视图显示） -->
    <AlertQueue 
      v-if="activeTab === 'monitor'"
      :alerts="alerts"
      @feedback="submitFeedback"
    />
  </div>
</template>

<script setup lang="ts">
import { AlertCircle, CheckCircle, Info, XCircle } from 'lucide-vue-next'
import { onMounted, onUnmounted, reactive, ref } from 'vue'
import AlertQueue from './components/AlertQueue.vue'
import FloatingDock from './components/FloatingDock.vue'
import VideoSourceSelector from './components/VideoSourceSelector.vue'
import type {
  AIConfig,
  Alert,
  Detection,
  FilterConfig,
  FrameData,
  TrainingConfig,
  TrainingStatus,
  VideoConfig
} from './types'
import ConsoleView from './views/ConsoleView.vue'
import MonitorView from './views/MonitorView.vue'

// 当前激活的 Tab
const activeTab = ref<'monitor' | 'console'>('monitor')

// Toast 弹窗相关
interface Toast {
  id: number
  type: 'success' | 'error' | 'warning' | 'info'
  message: string
  leaving?: boolean
}
const toasts = ref<Toast[]>([])
let toastId = 0

function showToast(type: Toast['type'], message: string, duration = 3000) {
  const id = ++toastId
  toasts.value.push({ id, type, message })
  
  setTimeout(() => {
    const index = toasts.value.findIndex(t => t.id === id)
    if (index !== -1) {
      toasts.value[index].leaving = true
      setTimeout(() => {
        toasts.value = toasts.value.filter(t => t.id !== id)
      }, 300)
    }
  }, duration)
}

// WebSocket 连接状态
const wsConnected = ref(false)
let ws: WebSocket | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let heartbeatTimer: ReturnType<typeof setInterval> | null = null

// 当前帧数据
const currentFrame = ref<FrameData | null>(null)
const detections = ref<Detection[]>([])

// 报警队列
const alerts = ref<Alert[]>([])

// 配置
const videoConfig = reactive<VideoConfig>({
  source_type: 'rtsp',
  rtsp_url: '',
  local_path: '',
  fps: 30
})

const systemConfig = reactive({
  ai: {
    confidence_threshold: 0.5,
    entropy_threshold: 0.5,
    nms_iou_threshold: 0.8,
    input_size: 640
  } as AIConfig,
  filter: {
    spatial_iou_threshold: 0.5,
    time_window_seconds: 60,
    enable_alert_push: true,
    auto_save_sample: true
  } as FilterConfig,
  training: {
    trigger_threshold: 100
  } as TrainingConfig
})

// 训练状态
const trainingStatus = ref<TrainingStatus>({
  labeled_samples_count: 0,
  trigger_threshold: 100,
  can_train: false,
  is_training: false
})

// 清理 WebSocket 相关资源
function cleanupWebSocket() {
  if (heartbeatTimer) {
    clearInterval(heartbeatTimer)
    heartbeatTimer = null
  }
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
  if (ws) {
    // 移除所有事件监听器，防止触发 onclose 重连
    ws.onopen = null
    ws.onmessage = null
    ws.onclose = null
    ws.onerror = null
    if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
      ws.close()
    }
    ws = null
  }
  wsConnected.value = false
}

// WebSocket 连接
function connectWebSocket() {
  // 清理已有连接
  cleanupWebSocket()
  
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/ws`
  
  console.log('WebSocket 正在连接:', wsUrl)
  ws = new WebSocket(wsUrl)
  
  ws.onopen = () => {
    console.log('WebSocket 已连接')
    wsConnected.value = true
    // 启动心跳
    startHeartbeat()
  }
  
  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)
      handleWebSocketMessage(msg)
    } catch (e) {
      console.error('WebSocket 消息解析失败:', e)
    }
  }
  
  ws.onclose = (event) => {
    console.log('WebSocket 已断开, code:', event.code, 'reason:', event.reason)
    wsConnected.value = false
    stopHeartbeat()
    // 非正常关闭时5秒后重连
    if (event.code !== 1000) {
      scheduleReconnect()
    }
  }
  
  ws.onerror = (error) => {
    console.error('WebSocket 错误:', error)
  }
}

// 启动心跳
function startHeartbeat() {
  stopHeartbeat()
  heartbeatTimer = setInterval(() => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      // 发送心跳消息
      ws.send(JSON.stringify({ type: 'ping' }))
    }
  }, 30000) // 每30秒发送一次心跳
}

// 停止心跳
function stopHeartbeat() {
  if (heartbeatTimer) {
    clearInterval(heartbeatTimer)
    heartbeatTimer = null
  }
}

// 安排重连
function scheduleReconnect() {
  if (reconnectTimer) {
    return // 已经有重连计划
  }
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null
    connectWebSocket()
  }, 5000)
}

// 处理 WebSocket 消息
function handleWebSocketMessage(msg: any) {
  switch (msg.type) {
    case 'frame':
      currentFrame.value = {
        frameId: msg.data.frame_id,
        imageData: msg.data.image_data,
        width: msg.data.width,
        height: msg.data.height,
        inferenceTime: msg.data.inference_time
      }
      detections.value = msg.data.detections || []
      break
      
    case 'alert':
      // 添加到报警队列
      const alert: Alert = {
        id: msg.data.id,
        frameId: msg.data.frame_id,
        timestamp: msg.data.timestamp,
        imageData: msg.data.image_data,
        bbox: {
          x1: msg.data.x1,
          y1: msg.data.y1,
          x2: msg.data.x2,
          y2: msg.data.y2
        },
        className: msg.data.class_name,
        confidence: msg.data.confidence,
        entropy: msg.data.entropy,
        countdown: 10
      }
      alerts.value.unshift(alert)
      
      // 启动倒计时
      startAlertCountdown(alert)
      break
  }
}

// 报警倒计时
function startAlertCountdown(alert: Alert) {
  const interval = setInterval(() => {
    const idx = alerts.value.findIndex(a => a.id === alert.id)
    if (idx === -1) {
      clearInterval(interval)
      return
    }
    
    alerts.value[idx].countdown--
    
    if (alerts.value[idx].countdown <= 0) {
      clearInterval(interval)
      // 移入待处理列表（这里简化处理）
    }
  }, 1000)
}

// API 调用
async function fetchConfig() {
  try {
    const res = await fetch('/api/config')
    const data = await res.json()
    Object.assign(videoConfig, data.video)
    Object.assign(systemConfig.ai, data.ai)
    Object.assign(systemConfig.filter, data.filter)
    Object.assign(systemConfig.training, data.training)
  } catch (e) {
    console.error('获取配置失败:', e)
  }
}

async function fetchTrainingStatus() {
  try {
    const res = await fetch('/api/training/status')
    const data = await res.json()
    trainingStatus.value = data
  } catch (e) {
    console.error('获取训练状态失败:', e)
  }
}

async function applyVideoConfig(config: VideoConfig) {
  try {
    const res = await fetch('/api/config/video', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(config)
    })
    const data = await res.json()
    if (data.success) {
      Object.assign(videoConfig, data.config)
      showToast('success', '视频源配置已应用')
    } else {
      showToast('error', '视频配置失败: ' + (data.error || data.message))
      console.error('更新视频配置失败:', data.error || data.message)
    }
  } catch (e) {
    showToast('error', '视频配置失败: 网络错误')
    console.error('更新视频配置失败:', e)
  }
}

async function updateAIConfig(config: Partial<AIConfig>) {
  try {
    const res = await fetch('/api/config/ai', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(config)
    })
    const data = await res.json()
    if (data.success) {
      Object.assign(systemConfig.ai, data.config)
      showToast('success', 'AI 检测参数已更新')
    } else {
      showToast('error', 'AI 参数更新失败: ' + (data.error || data.message))
    }
  } catch (e) {
    showToast('error', 'AI 参数更新失败: 网络错误')
    console.error('更新 AI 配置失败:', e)
  }
}

async function updateFilterConfig(config: Partial<FilterConfig>) {
  try {
    const res = await fetch('/api/config/filter', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(config)
    })
    const data = await res.json()
    if (data.success) {
      Object.assign(systemConfig.filter, data.config)
      showToast('success', '过滤参数已更新')
    } else {
      showToast('error', '过滤参数更新失败: ' + (data.error || data.message))
    }
  } catch (e) {
    showToast('error', '过滤参数更新失败: 网络错误')
    console.error('更新过滤配置失败:', e)
  }
}

async function updateTrainingConfig(config: Partial<TrainingConfig>) {
  try {
    const res = await fetch('/api/config/training', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(config)
    })
    const data = await res.json()
    if (data.success) {
      Object.assign(systemConfig.training, data.config)
      showToast('success', '训练配置已更新')
    } else {
      showToast('error', '训练配置更新失败: ' + (data.error || data.message))
    }
  } catch (e) {
    showToast('error', '训练配置更新失败: 网络错误')
    console.error('更新训练配置失败:', e)
  }
}

async function triggerTraining() {
  try {
    trainingStatus.value.is_training = true
    showToast('info', '模型训练已启动...')
    const res = await fetch('/api/training/trigger', {
      method: 'POST'
    })
    const data = await res.json()
    if (data.success) {
      // 轮询训练状态
      pollTrainingStatus()
    } else {
      showToast('error', '训练启动失败: ' + (data.error || data.message))
      trainingStatus.value.is_training = false
    }
  } catch (e) {
    showToast('error', '训练启动失败: 网络错误')
    console.error('触发训练失败:', e)
    trainingStatus.value.is_training = false
  }
}

function pollTrainingStatus() {
  const interval = setInterval(async () => {
    await fetchTrainingStatus()
    if (!trainingStatus.value.is_training) {
      clearInterval(interval)
      // 训练结束，显示结果
      if (trainingStatus.value.latest_training?.status === 'completed') {
        showToast('success', '模型训练完成！模型已热更新')
      } else if (trainingStatus.value.latest_training?.status === 'failed') {
        showToast('error', '模型训练失败: ' + (trainingStatus.value.latest_training?.error_message || '未知错误'))
      }
    }
  }, 2000)
}

async function submitFeedback(alertId: number, status: 'normal' | 'abnormal') {
  try {
    const res = await fetch('/api/feedback', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        sample_id: alertId,
        label_status: status
      })
    })
    const data = await res.json()
    
    if (data.success !== false) {
      // 从队列中移除
      alerts.value = alerts.value.filter(a => a.id !== alertId)
      showToast('success', status === 'abnormal' ? '已标记为异常' : '已标记为正常')
      // 刷新训练状态
      fetchTrainingStatus()
    } else {
      showToast('error', '标记失败: ' + (data.error || data.message))
    }
  } catch (e) {
    showToast('error', '标记失败: 网络错误')
    console.error('提交反馈失败:', e)
  }
}

// 生命周期
onMounted(() => {
  fetchConfig()
  fetchTrainingStatus()
  connectWebSocket()
})

onUnmounted(() => {
  cleanupWebSocket()
})
</script>
