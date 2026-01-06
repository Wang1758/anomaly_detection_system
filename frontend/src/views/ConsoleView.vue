<template>
  <div class="h-full w-full overflow-y-auto p-6">
    <div class="max-w-4xl mx-auto space-y-6">
      <!-- 标题 -->
      <div class="mb-8">
        <h1 class="text-2xl font-bold text-gray-900">控制台</h1>
        <p class="text-gray-500 mt-1">调整系统参数和触发模型训练</p>
      </div>

      <!-- AI 检测参数 -->
      <div class="glass rounded-3xl p-6">
        <h2 class="text-lg font-semibold text-gray-900 mb-4 flex items-center">
          <Brain class="w-5 h-5 mr-2 text-apple-blue" />
          AI 检测参数
        </h2>
        <div class="space-y-6">
          <!-- 置信度阈值 -->
          <SliderControl
            label="置信度阈值"
            :value="localConfig.ai.confidence_threshold"
            :min="0"
            :max="1"
            :step="0.05"
            @update="(v) => updateLocalAI('confidence_threshold', v)"
          />
          
          <!-- 熵值阈值 -->
          <SliderControl
            label="不确定性熵值阈值"
            :value="localConfig.ai.entropy_threshold"
            :min="0"
            :max="1"
            :step="0.05"
            @update="(v) => updateLocalAI('entropy_threshold', v)"
          />

          <!-- NMS IoU 阈值 -->
          <SliderControl
            label="NMS IoU 阈值"
            :value="localConfig.ai.nms_iou_threshold"
            :min="0.5"
            :max="1"
            :step="0.05"
            @update="(v) => updateLocalAI('nms_iou_threshold', v)"
          />
        </div>
      </div>

      <!-- 报警过滤参数 -->
      <div class="glass rounded-3xl p-6">
        <h2 class="text-lg font-semibold text-gray-900 mb-4 flex items-center">
          <Filter class="w-5 h-5 mr-2 text-apple-purple" />
          报警过滤参数
        </h2>
        <div class="space-y-6">
          <!-- 空间抑制 IoU -->
          <SliderControl
            label="空间抑制 IoU 阈值"
            :value="localConfig.filter.spatial_iou_threshold"
            :min="0"
            :max="1"
            :step="0.05"
            @update="(v) => updateLocalFilter('spatial_iou_threshold', v)"
          />
        </div>
      </div>

      <!-- 训练配置 -->
      <div class="glass rounded-3xl p-6 border-2 border-apple-blue/20">
        <h2 class="text-lg font-semibold text-gray-900 mb-4 flex items-center">
          <Sparkles class="w-5 h-5 mr-2 text-apple-orange" />
          模型训练
        </h2>

        <!-- 状态显示 -->
        <div class="bg-gray-50 rounded-2xl p-4 mb-6">
          <div class="flex items-center justify-between mb-2">
            <span class="text-gray-600">当前待训练样本</span>
            <span class="text-2xl font-bold text-gray-900">
              {{ trainingStatus.labeled_samples_count }} 张
            </span>
          </div>
          <div class="w-full bg-gray-200 rounded-full h-2">
            <div 
              class="h-2 rounded-full transition-all duration-500"
              :class="trainingStatus.can_train ? 'bg-apple-green' : 'bg-apple-blue'"
              :style="{ width: `${Math.min(100, (trainingStatus.labeled_samples_count / trainingStatus.trigger_threshold) * 100)}%` }"
            ></div>
          </div>
          <div class="text-xs text-gray-500 mt-1 text-right">
            目标: {{ trainingStatus.trigger_threshold }} 张
          </div>
        </div>

        <!-- 触发阈值 -->
        <SliderControl
          label="自动训练触发阈值"
          :value="localConfig.training.trigger_threshold"
          :min="50"
          :max="500"
          :step="10"
          suffix="张"
          @update="(v) => updateLocalTraining('trigger_threshold', v)"
        />

        <!-- 训练按钮 -->
        <button
          class="w-full mt-6 py-4 rounded-2xl font-semibold text-lg transition-all"
          :class="trainingStatus.is_training 
            ? 'bg-gray-200 text-gray-500 cursor-not-allowed' 
            : 'bg-gradient-to-r from-apple-blue to-apple-purple text-white hover:shadow-glow-blue'"
          :disabled="trainingStatus.is_training"
          @click="$emit('triggerTraining')"
        >
          <template v-if="trainingStatus.is_training">
            <Loader2 class="w-5 h-5 inline mr-2 animate-spin" />
            训练中...
          </template>
          <template v-else>
            <Zap class="w-5 h-5 inline mr-2" />
            立即开始训练
          </template>
        </button>

        <!-- 训练完成提示 -->
        <div 
          v-if="trainingStatus.latest_training?.status === 'completed'"
          class="mt-4 p-3 bg-apple-green/10 rounded-xl text-apple-green text-sm flex items-center"
        >
          <CheckCircle class="w-5 h-5 mr-2" />
          训练完成，模型已热更新
        </div>
      </div>

      <!-- 系统开关 -->
      <div class="glass rounded-3xl p-6">
        <h2 class="text-lg font-semibold text-gray-900 mb-4 flex items-center">
          <Settings class="w-5 h-5 mr-2 text-gray-600" />
          系统开关
        </h2>
        <div class="space-y-4">
          <ToggleSwitch
            label="启用报警推送"
            :value="localConfig.filter.enable_alert_push"
            @update="(v) => updateLocalFilter('enable_alert_push', v)"
          />
          <ToggleSwitch
            label="自动保存样本"
            :value="localConfig.filter.auto_save_sample"
            @update="(v) => updateLocalFilter('auto_save_sample', v)"
          />
        </div>
      </div>

      <!-- 应用设置按钮 -->
      <div class="sticky bottom-6">
        <button
          class="w-full py-4 rounded-2xl font-semibold text-lg transition-all"
          :class="hasChanges 
            ? 'bg-gradient-to-r from-apple-green to-apple-blue text-white hover:shadow-glow-blue' 
            : 'bg-gray-200 text-gray-400 cursor-not-allowed'"
          :disabled="!hasChanges || isApplying"
          @click="applySettings"
        >
          <template v-if="isApplying">
            <Loader2 class="w-5 h-5 inline mr-2 animate-spin" />
            应用中...
          </template>
          <template v-else>
            <Save class="w-5 h-5 inline mr-2" />
            {{ hasChanges ? '应用设置' : '无修改' }}
          </template>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { Brain, Filter, Sparkles, Settings, Zap, Loader2, CheckCircle, Save } from 'lucide-vue-next'
import SliderControl from '../components/SliderControl.vue'
import ToggleSwitch from '../components/ToggleSwitch.vue'
import type { AIConfig, FilterConfig, TrainingConfig, TrainingStatus } from '../types'

const props = defineProps<{
  config: {
    ai: AIConfig
    filter: FilterConfig
    training: TrainingConfig
  }
  trainingStatus: TrainingStatus
}>()

const emit = defineEmits<{
  updateAI: [config: Partial<AIConfig>]
  updateFilter: [config: Partial<FilterConfig>]
  updateTraining: [config: Partial<TrainingConfig>]
  triggerTraining: []
}>()

// 本地配置副本（用于编辑）
const localConfig = reactive({
  ai: {
    confidence_threshold: props.config.ai.confidence_threshold,
    entropy_threshold: props.config.ai.entropy_threshold,
    nms_iou_threshold: props.config.ai.nms_iou_threshold,
    input_size: props.config.ai.input_size
  },
  filter: {
    spatial_iou_threshold: props.config.filter.spatial_iou_threshold,
    time_window_seconds: props.config.filter.time_window_seconds,
    enable_alert_push: props.config.filter.enable_alert_push,
    auto_save_sample: props.config.filter.auto_save_sample
  },
  training: {
    trigger_threshold: props.config.training.trigger_threshold
  }
})

// 应用状态
const isApplying = ref(false)

// 当 props 变化时同步到本地（比如服务端更新后）
watch(() => props.config, (newConfig) => {
  localConfig.ai.confidence_threshold = newConfig.ai.confidence_threshold
  localConfig.ai.entropy_threshold = newConfig.ai.entropy_threshold
  localConfig.ai.nms_iou_threshold = newConfig.ai.nms_iou_threshold
  localConfig.ai.input_size = newConfig.ai.input_size
  localConfig.filter.spatial_iou_threshold = newConfig.filter.spatial_iou_threshold
  localConfig.filter.time_window_seconds = newConfig.filter.time_window_seconds
  localConfig.filter.enable_alert_push = newConfig.filter.enable_alert_push
  localConfig.filter.auto_save_sample = newConfig.filter.auto_save_sample
  localConfig.training.trigger_threshold = newConfig.training.trigger_threshold
}, { deep: true })

// 检查是否有未保存的修改
const hasChanges = computed(() => {
  return (
    localConfig.ai.confidence_threshold !== props.config.ai.confidence_threshold ||
    localConfig.ai.entropy_threshold !== props.config.ai.entropy_threshold ||
    localConfig.ai.nms_iou_threshold !== props.config.ai.nms_iou_threshold ||
    localConfig.filter.spatial_iou_threshold !== props.config.filter.spatial_iou_threshold ||
    localConfig.filter.enable_alert_push !== props.config.filter.enable_alert_push ||
    localConfig.filter.auto_save_sample !== props.config.filter.auto_save_sample ||
    localConfig.training.trigger_threshold !== props.config.training.trigger_threshold
  )
})

// 更新本地 AI 配置
function updateLocalAI(key: keyof AIConfig, value: number) {
  (localConfig.ai as any)[key] = value
}

// 更新本地 Filter 配置
function updateLocalFilter(key: keyof FilterConfig, value: number | boolean) {
  (localConfig.filter as any)[key] = value
}

// 更新本地 Training 配置
function updateLocalTraining(key: keyof TrainingConfig, value: number) {
  (localConfig.training as any)[key] = value
}

// 应用所有设置
async function applySettings() {
  if (!hasChanges.value || isApplying.value) return
  
  isApplying.value = true
  
  try {
    // 检查并提交 AI 配置变更
    const aiChanges: Partial<AIConfig> = {}
    if (localConfig.ai.confidence_threshold !== props.config.ai.confidence_threshold) {
      aiChanges.confidence_threshold = localConfig.ai.confidence_threshold
    }
    if (localConfig.ai.entropy_threshold !== props.config.ai.entropy_threshold) {
      aiChanges.entropy_threshold = localConfig.ai.entropy_threshold
    }
    if (localConfig.ai.nms_iou_threshold !== props.config.ai.nms_iou_threshold) {
      aiChanges.nms_iou_threshold = localConfig.ai.nms_iou_threshold
    }
    if (Object.keys(aiChanges).length > 0) {
      emit('updateAI', aiChanges)
    }
    
    // 检查并提交 Filter 配置变更
    const filterChanges: Partial<FilterConfig> = {}
    if (localConfig.filter.spatial_iou_threshold !== props.config.filter.spatial_iou_threshold) {
      filterChanges.spatial_iou_threshold = localConfig.filter.spatial_iou_threshold
    }
    if (localConfig.filter.enable_alert_push !== props.config.filter.enable_alert_push) {
      filterChanges.enable_alert_push = localConfig.filter.enable_alert_push
    }
    if (localConfig.filter.auto_save_sample !== props.config.filter.auto_save_sample) {
      filterChanges.auto_save_sample = localConfig.filter.auto_save_sample
    }
    if (Object.keys(filterChanges).length > 0) {
      emit('updateFilter', filterChanges)
    }
    
    // 检查并提交 Training 配置变更
    const trainingChanges: Partial<TrainingConfig> = {}
    if (localConfig.training.trigger_threshold !== props.config.training.trigger_threshold) {
      trainingChanges.trigger_threshold = localConfig.training.trigger_threshold
    }
    if (Object.keys(trainingChanges).length > 0) {
      emit('updateTraining', trainingChanges)
    }
    
    // 等待一小段时间让父组件处理完成
    await new Promise(resolve => setTimeout(resolve, 500))
  } finally {
    isApplying.value = false
  }
}
</script>
