<template>
  <nav class="w-20 dock-glass rounded-4xl p-3 flex flex-col items-center gap-4 transition-all duration-300">
    <div class="w-12 h-12 mt-2 rounded-2xl bg-gradient-to-tr from-gray-900 to-gray-700 flex items-center justify-center shadow-lg transform hover:scale-105 transition-transform duration-300">
      <svg class="w-6 h-6 text-white" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"/>
      </svg>
    </div>

    <div class="w-8 h-px bg-gray-200/50"></div>

    <div class="flex-1 flex flex-col gap-4 w-full px-1">
      <DockItem 
        :active="activeTab === 'monitor'"
        @click="$emit('update:activeTab', 'monitor')"
        title="实时监控"
      >
        <Camera class="w-6 h-6" />
      </DockItem>

      <DockItem 
        :active="activeTab === 'console'"
        @click="$emit('update:activeTab', 'console')"
        title="系统控制台"
      >
        <Settings2 class="w-6 h-6" />
      </DockItem>
    </div>

    <div class="mb-4">
      <div 
        class="w-3 h-3 rounded-full shadow-sm transition-colors duration-500"
        :class="[
          connected ? 'bg-apple-green shadow-[0_0_10px_rgba(52,199,89,0.4)]' : 'bg-apple-red',
          connected ? 'animate-pulse' : ''
        ]"
        :title="connected ? '系统在线' : '连接断开'"
      ></div>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { Camera, Settings2 } from 'lucide-vue-next';
import DockItem from './DockItem.vue';

defineProps<{
  activeTab: 'monitor' | 'console'
  connected?: boolean
}>()

defineEmits<{
  'update:activeTab': [value: 'monitor' | 'console']
}>()
</script>
