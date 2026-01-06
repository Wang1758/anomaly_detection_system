<template>
  <div 
    class="bg-white/40 rounded-2xl p-3 border-l-4 border-apple-orange hover-lift cursor-pointer"
    :class="{ 'bg-white/80': isHovered }"
    @mouseenter="isHovered = true"
    @mouseleave="isHovered = false"
  >
    <div class="flex gap-3">
      <!-- 缩略图 -->
      <div class="w-16 h-16 rounded-xl overflow-hidden bg-gray-200 flex-shrink-0">
        <img 
          v-if="alert.imageData"
          :src="'data:image/jpeg;base64,' + alert.imageData"
          class="w-full h-full object-cover"
          alt="报警截图"
        />
        <div v-else class="w-full h-full flex items-center justify-center">
          <ImageIcon class="w-6 h-6 text-gray-400" />
        </div>
      </div>

      <!-- 信息 -->
      <div class="flex-1 min-w-0">
        <div class="flex items-center justify-between">
          <span class="text-sm font-medium text-gray-900 truncate">
            {{ alert.className }} #{{ alert.id }}
          </span>
          <!-- 倒计时 -->
          <span 
            class="text-xs px-2 py-0.5 rounded-full"
            :class="alert.countdown <= 3 ? 'bg-apple-red/20 text-apple-red' : 'bg-gray-100 text-gray-500'"
          >
            {{ alert.countdown }}s
          </span>
        </div>
        <div class="text-xs text-gray-500 mt-1">
          置信度: {{ (alert.confidence * 100).toFixed(1) }}%
        </div>
        <div class="text-xs text-gray-400 mt-0.5">
          {{ formatTime(alert.timestamp) }}
        </div>
      </div>
    </div>

    <!-- 操作按钮 -->
    <div class="flex gap-2 mt-3">
      <button
        class="flex-1 py-1.5 rounded-lg text-sm font-medium bg-apple-green/10 text-apple-green hover:bg-apple-green/20 transition-colors"
        @click.stop="$emit('feedback', 'normal')"
      >
        <Check class="w-4 h-4 inline mr-1" />
        正常
      </button>
      <button
        class="flex-1 py-1.5 rounded-lg text-sm font-medium bg-apple-red/10 text-apple-red hover:bg-apple-red/20 transition-colors"
        @click.stop="$emit('feedback', 'abnormal')"
      >
        <X class="w-4 h-4 inline mr-1" />
        异常
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { Check, X, Image as ImageIcon } from 'lucide-vue-next'
import type { Alert } from '../types'

defineProps<{
  alert: Alert
}>()

defineEmits<{
  feedback: [status: 'normal' | 'abnormal']
}>()

const isHovered = ref(false)

function formatTime(timestamp: number): string {
  const date = new Date(timestamp)
  return date.toLocaleTimeString('zh-CN', { 
    hour: '2-digit', 
    minute: '2-digit', 
    second: '2-digit' 
  })
}
</script>
