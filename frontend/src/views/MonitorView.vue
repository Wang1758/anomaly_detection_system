<template>
  <div class="h-full w-full relative flex items-center justify-center p-6 bg-gradient-to-br from-gray-50 to-gray-100">
    <!-- 视频容器 -->
    <div class="relative max-w-full max-h-full aspect-video">
      <!-- 画框效果 -->
      <div class="absolute inset-0 border-[6px] border-white rounded-2xl shadow-2xl overflow-hidden">
        <!-- 视频/图像 -->
        <img 
          v-if="currentFrame"
          :src="'data:image/jpeg;base64,' + currentFrame.imageData"
          class="w-full h-full object-contain bg-black"
          alt="监控画面"
        />
        <!-- 无信号占位 -->
        <div 
          v-else 
          class="w-full h-full bg-gray-900 flex flex-col items-center justify-center"
        >
          <VideoOff class="w-16 h-16 text-gray-600 mb-4" />
          <span class="text-gray-500 text-lg">等待视频信号...</span>
        </div>

        <!-- Canvas 覆盖层 - 绘制检测框 -->
        <canvas 
          ref="canvasRef"
          class="absolute inset-0 w-full h-full pointer-events-none"
        ></canvas>

        <!-- LIVE 状态指示 -->
        <div 
          v-if="wsConnected"
          class="absolute top-4 left-4 flex items-center gap-2 px-3 py-1.5 rounded-full bg-apple-green/90 text-white text-sm font-medium pulse-glow"
        >
          <div class="w-2 h-2 bg-white rounded-full animate-pulse"></div>
          LIVE
        </div>

        <!-- 推理耗时 -->
        <div 
          v-if="currentFrame"
          class="absolute top-4 right-4 px-3 py-1.5 rounded-full bg-black/50 text-white text-sm backdrop-blur"
        >
          {{ currentFrame.inferenceTime }}ms
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, nextTick } from 'vue'
import { VideoOff } from 'lucide-vue-next'
import type { FrameData, Detection } from '../types'

const props = defineProps<{
  wsConnected: boolean
  currentFrame: FrameData | null
  detections: Detection[]
}>()

const canvasRef = ref<HTMLCanvasElement | null>(null)

// 绘制检测框
function drawDetections() {
  const canvas = canvasRef.value
  if (!canvas || !props.currentFrame) return

  const ctx = canvas.getContext('2d')
  if (!ctx) return

  // 设置 canvas 尺寸
  canvas.width = props.currentFrame.width
  canvas.height = props.currentFrame.height

  // 清空画布
  ctx.clearRect(0, 0, canvas.width, canvas.height)

  // 绘制每个检测框
  for (const det of props.detections) {
    const x = det.x1
    const y = det.y1
    const w = det.x2 - det.x1
    const h = det.y2 - det.y1

    if (det.is_uncertain) {
      // 异常框 - 橙色
      ctx.strokeStyle = '#FF9500'
      ctx.lineWidth = 3
      ctx.fillStyle = 'rgba(255, 149, 0, 0.1)'
      ctx.fillRect(x, y, w, h)
      
      // 四角加粗
      const cornerLen = Math.min(w, h) * 0.15
      ctx.lineWidth = 4
      ctx.beginPath()
      // 左上角
      ctx.moveTo(x, y + cornerLen)
      ctx.lineTo(x, y)
      ctx.lineTo(x + cornerLen, y)
      // 右上角
      ctx.moveTo(x + w - cornerLen, y)
      ctx.lineTo(x + w, y)
      ctx.lineTo(x + w, y + cornerLen)
      // 右下角
      ctx.moveTo(x + w, y + h - cornerLen)
      ctx.lineTo(x + w, y + h)
      ctx.lineTo(x + w - cornerLen, y + h)
      // 左下角
      ctx.moveTo(x + cornerLen, y + h)
      ctx.lineTo(x, y + h)
      ctx.lineTo(x, y + h - cornerLen)
      ctx.stroke()

      // 标签
      const labelText = `? 疑似目标 (ID:${det.id})`
      ctx.font = 'bold 14px -apple-system, sans-serif'
      const labelWidth = ctx.measureText(labelText).width + 16
      const labelHeight = 24
      const labelX = x
      const labelY = y - labelHeight - 4

      // 胶囊标签背景
      ctx.fillStyle = '#FF9500'
      ctx.beginPath()
      ctx.roundRect(labelX, labelY, labelWidth, labelHeight, labelHeight / 2)
      ctx.fill()

      // 标签文字
      ctx.fillStyle = 'white'
      ctx.textBaseline = 'middle'
      ctx.fillText(labelText, labelX + 8, labelY + labelHeight / 2)

    } else {
      // 正常框 - 蓝色
      ctx.strokeStyle = '#007AFF'
      ctx.lineWidth = 2
      ctx.strokeRect(x, y, w, h)

      // 小标签
      const labelText = det.class_name
      ctx.font = '12px -apple-system, sans-serif'
      const labelWidth = ctx.measureText(labelText).width + 8
      ctx.fillStyle = '#007AFF'
      ctx.fillRect(x, y - 18, labelWidth, 18)
      ctx.fillStyle = 'white'
      ctx.textBaseline = 'middle'
      ctx.fillText(labelText, x + 4, y - 9)
    }
  }
}

// 监听检测结果变化
watch(() => props.detections, () => {
  nextTick(drawDetections)
}, { deep: true })

watch(() => props.currentFrame, () => {
  nextTick(drawDetections)
})

onMounted(() => {
  drawDetections()
})
</script>
