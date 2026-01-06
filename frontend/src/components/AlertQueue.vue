<template>
  <aside class="w-80 glass rounded-4xl p-4 flex flex-col">
    <!-- 标题 -->
    <div class="flex items-center justify-between mb-4">
      <h2 class="text-lg font-semibold text-gray-900">
        <AlertTriangle class="w-5 h-5 inline text-apple-orange mr-2" />
        待核查队列
      </h2>
      <span class="text-sm text-gray-500">{{ alerts.length }} 项</span>
    </div>

    <!-- 报警列表 -->
    <div class="flex-1 overflow-y-auto space-y-3">
      <TransitionGroup name="alert">
        <AlertCard
          v-for="alert in alerts"
          :key="alert.id"
          :alert="alert"
          @feedback="(status) => $emit('feedback', alert.id, status)"
        />
      </TransitionGroup>

      <!-- 空状态 -->
      <div 
        v-if="alerts.length === 0"
        class="h-full flex flex-col items-center justify-center text-gray-400"
      >
        <CheckCircle class="w-12 h-12 mb-3" />
        <span class="text-sm">暂无待核查项目</span>
      </div>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { AlertTriangle, CheckCircle } from 'lucide-vue-next'
import AlertCard from './AlertCard.vue'
import type { Alert } from '../types'

defineProps<{
  alerts: Alert[]
}>()

defineEmits<{
  feedback: [alertId: number, status: 'normal' | 'abnormal']
}>()
</script>

<style scoped>
.alert-enter-active,
.alert-leave-active {
  transition: all 0.3s ease;
}

.alert-enter-from {
  opacity: 0;
  transform: translateX(20px);
}

.alert-leave-to {
  opacity: 0;
  transform: translateX(-20px);
}
</style>
