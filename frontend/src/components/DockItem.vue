<template>
  <button
    class="w-14 h-14 rounded-2xl flex items-center justify-center transition-all duration-200"
    :class="active 
      ? 'bg-white text-gray-900 shadow-lg scale-110' 
      : 'bg-white/10 text-white/70 hover:bg-white/20 hover:text-white'"
    :title="title"
  >
    <slot />
  </button>
</template>

<script setup lang="ts">
defineProps<{
  active: boolean
  title?: string
}>()
</script>
