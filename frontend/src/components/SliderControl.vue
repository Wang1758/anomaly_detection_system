<template>
  <div class="w-full space-y-3 select-none">
    <div class="flex justify-between items-center px-1">
      <label class="text-sm font-semibold text-gray-500 tracking-tight">{{ label }}</label>
      <span class="font-mono text-sm font-bold text-apple-blue bg-blue-50 px-2 py-0.5 rounded-md border border-blue-100/50">
        {{ displayValue }}{{ suffix }}
      </span>
    </div>

    <div class="relative w-full h-6 flex items-center group cursor-pointer">
      
      <div class="absolute w-full h-1.5 bg-gray-200/80 rounded-full overflow-hidden">
        <div 
          class="h-full bg-apple-blue rounded-full transition-all duration-75 ease-out" 
          :style="{ width: `${percentage}%` }" 
        />
      </div>

      <div 
        class="absolute h-5 w-5 bg-white rounded-full shadow-[0_2px_4px_rgba(0,0,0,0.15)] border border-black/5 transform -translate-x-1/2 transition-transform duration-200 ease-[cubic-bezier(0.34,1.56,0.64,1)] group-hover:scale-110 group-active:scale-95"
        :style="{ left: `${percentage}%` }" 
      />

      <input
        type="range"
        :value="value"
        :min="min"
        :max="max"
        :step="step"
        class="absolute inset-0 w-full h-full opacity-0 cursor-pointer z-10 margin-0"
        @input="handleInput"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

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

const percentage = computed(() => {
  return ((props.value - props.min) / (props.max - props.min)) * 100
})

const displayValue = computed(() => {
  return props.step < 1 ? props.value.toFixed(2) : props.value
})

function handleInput(event: Event) {
  const target = event.target as HTMLInputElement
  emit('update', parseFloat(target.value))
}
</script>
