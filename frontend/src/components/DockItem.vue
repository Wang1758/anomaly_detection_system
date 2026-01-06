<template>
  <button
    class="relative group w-full aspect-square rounded-2xl flex items-center justify-center transition-all duration-300 ease-[cubic-bezier(0.32,0.72,0,1)]"
    :class="active 
      ? 'bg-white text-apple-blue shadow-lg scale-100' 
      : 'text-gray-400 hover:bg-white/40 hover:text-gray-600 hover:scale-105 scale-95'"
    :title="title"
  >
    <slot />
    
    <div 
      v-if="active"
      class="absolute -right-1 top-1/2 -translate-y-1/2 w-1 h-1 bg-apple-blue rounded-full opacity-0 group-hover:opacity-100 transition-opacity"
    ></div>
  </button>
</template>

<script setup lang="ts">
defineProps<{
  active: boolean
  title?: string
}>()
</script>
