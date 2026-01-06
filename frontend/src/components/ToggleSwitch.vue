<template>
  <div class="flex items-center justify-between">
    <span class="text-sm font-medium text-gray-700">{{ label }}</span>
    <button
      class="ios-toggle"
      :class="{ 'active': value }"
      @click="$emit('update', !value)"
      role="switch"
      :aria-checked="value"
    ></button>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  label: string
  value: boolean
}>()

defineEmits<{
  update: [value: boolean]
}>()
</script>
