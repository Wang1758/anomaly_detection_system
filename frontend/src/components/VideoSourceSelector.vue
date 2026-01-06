<template>
  <div class="glass rounded-3xl p-4 flex items-center gap-4">
    <!-- 视频源类型选择 -->
    <div class="flex gap-2">
      <button
        class="px-4 py-2 rounded-xl text-sm font-medium transition-all"
        :class="localSourceType === 'rtsp' 
          ? 'bg-apple-blue text-white shadow-glow-blue' 
          : 'bg-white/50 text-gray-600 hover:bg-white/80'"
        @click="updateSourceType('rtsp')"
      >
        <Radio class="w-4 h-4 inline mr-2" />
        RTSP 流
      </button>
      <button
        class="px-4 py-2 rounded-xl text-sm font-medium transition-all"
        :class="localSourceType === 'local' 
          ? 'bg-apple-blue text-white shadow-glow-blue' 
          : 'bg-white/50 text-gray-600 hover:bg-white/80'"
        @click="updateSourceType('local')"
      >
        <Film class="w-4 h-4 inline mr-2" />
        本地视频
      </button>
    </div>

    <!-- 分隔线 -->
    <div class="w-px h-8 bg-gray-200"></div>

    <!-- 地址输入 -->
    <div class="flex-1">
      <input
        v-if="localSourceType === 'rtsp'"
        v-model="rtspUrl"
        type="text"
        placeholder="输入 RTSP 地址..."
        class="w-full px-4 py-2 rounded-xl bg-white/50 border border-white/80 text-sm focus:outline-none focus:ring-2 focus:ring-apple-blue/50"
      />
      <input
        v-else
        v-model="localPath"
        type="text"
        placeholder="输入本地视频路径..."
        class="w-full px-4 py-2 rounded-xl bg-white/50 border border-white/80 text-sm focus:outline-none focus:ring-2 focus:ring-apple-blue/50"
      />
    </div>

    <!-- 帧率选择 -->
    <div class="flex gap-2">
      <button
        class="px-3 py-2 rounded-xl text-sm font-medium transition-all"
        :class="localFps === 30 
          ? 'bg-apple-green text-white' 
          : 'bg-white/50 text-gray-600 hover:bg-white/80'"
        @click="updateFPS(30)"
      >
        30 FPS
      </button>
      <button
        class="px-3 py-2 rounded-xl text-sm font-medium transition-all"
        :class="localFps === 60 
          ? 'bg-apple-green text-white' 
          : 'bg-white/50 text-gray-600 hover:bg-white/80'"
        @click="updateFPS(60)"
      >
        60 FPS
      </button>
    </div>

    <!-- 应用按钮 -->
    <button
      class="px-6 py-2 rounded-xl bg-apple-blue text-white font-medium text-sm hover:bg-apple-blue/90 transition-all shadow-glow-blue"
      @click="apply"
    >
      <Play class="w-4 h-4 inline mr-1" />
      应用
    </button>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { Radio, Film, Play } from 'lucide-vue-next'
import type { VideoConfig } from '../types'

const props = defineProps<{
  config: VideoConfig
}>()

const emit = defineEmits<{
  apply: [config: VideoConfig]  // 只有点击应用按钮才触发
}>()

// 本地编辑状态（不会立即发送到后端）
const localSourceType = ref<'rtsp' | 'local'>(props.config.source_type)
const localFps = ref<30 | 60>(props.config.fps as 30 | 60)
const rtspUrl = ref(props.config.rtsp_url)
const localPath = ref(props.config.local_path)

// 监听外部配置变化，同步到本地状态
watch(() => props.config, (newConfig) => {
  localSourceType.value = newConfig.source_type
  localFps.value = newConfig.fps as 30 | 60
  rtspUrl.value = newConfig.rtsp_url
  localPath.value = newConfig.local_path
}, { deep: true })

// 这些函数只更新本地状态，不发送请求
function updateSourceType(type: 'rtsp' | 'local') {
  localSourceType.value = type
}

function updateFPS(fps: 30 | 60) {
  localFps.value = fps
}

// 只有点击应用按钮才发送完整配置
function apply() {
  emit('apply', {
    source_type: localSourceType.value,
    rtsp_url: rtspUrl.value,
    local_path: localPath.value,
    fps: localFps.value
  })
}
</script>
