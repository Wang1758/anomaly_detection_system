<template>
  <div class="space-y-2">
    <div class="flex justify-between items-center">
      <label class="text-sm font-medium text-gray-700">{{ label }}</label>
      <span class="text-sm font-mono text-apple-blue">
        {{ displayValue }}{{ suffix }}
      </span>
    </div>
    <input
      type="range"
      :value="value"
      :min="min"
      :max="max"
      :step="step"
      class="w-full h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer"
      :style="trackStyle"
      @input="handleInput"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  label: string
  value: number
  min?: number
  max?: number
  step?: number
  suffix?: string
}>(), {
  min: 0,
  max: 100,
  step: 1,
  suffix: ''
})

const emit = defineEmits<{
  update: [value: number]
}>()

const displayValue = computed(() => {
  if (props.step < 1) {
    return props.value.toFixed(2)
  }
  return props.value
})

const trackStyle = computed(() => {
  const percent = ((props.value - props.min) / (props.max - props.min)) * 100
  return {
    background: `linear-gradient(to right, #007AFF 0%, #007AFF ${percent}%, #e5e7eb ${percent}%, #e5e7eb 100%)`
  }
})

function handleInput(event: Event) {
  const target = event.target as HTMLInputElement
  emit('update', parseFloat(target.value))
}
</script>

<style scoped>
input[type="range"]::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: white;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.2);
  cursor: pointer;
  transition: transform 0.1s ease;
}

input[type="range"]::-webkit-slider-thumb:hover {
  transform: scale(1.1);
}

input[type="range"]::-webkit-slider-thumb:active {
  transform: scale(0.95);
}
</style>
