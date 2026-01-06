<template>
  <nav class="w-20 glass-dark rounded-4xl p-3 flex flex-col items-center gap-2">
    <!-- Logo -->
    <div class="w-12 h-12 rounded-2xl bg-gradient-to-br from-apple-blue to-apple-purple flex items-center justify-center mb-4">
      <Egg class="w-6 h-6 text-white" />
    </div>

    <!-- 导航项 -->
    <div class="flex-1 flex flex-col gap-2">
      <DockItem 
        :active="activeTab === 'monitor'"
        @click="$emit('update:activeTab', 'monitor')"
        title="监控"
      >
        <Camera class="w-6 h-6" />
      </DockItem>

      <DockItem 
        :active="activeTab === 'console'"
        @click="$emit('update:activeTab', 'console')"
        title="控制台"
      >
        <Settings2 class="w-6 h-6" />
      </DockItem>
    </div>

    <!-- 底部状态 -->
    <div class="mt-auto">
      <div 
        class="w-10 h-10 rounded-xl flex items-center justify-center"
        :class="connected ? 'bg-apple-green/20' : 'bg-apple-red/20'"
      >
        <Wifi 
          class="w-5 h-5" 
          :class="connected ? 'text-apple-green' : 'text-apple-red'"
        />
      </div>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { Camera, Settings2, Wifi, Egg } from 'lucide-vue-next'
import DockItem from './DockItem.vue'

defineProps<{
  activeTab: 'monitor' | 'console'
  connected?: boolean
}>()

defineEmits<{
  'update:activeTab': [value: 'monitor' | 'console']
}>()
</script>
