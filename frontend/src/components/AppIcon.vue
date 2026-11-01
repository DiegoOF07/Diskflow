<script setup lang="ts">
// Line icons drawn on a 16px grid in the text colour, so they follow the theme
// (emoji look different on every system and ignore it).
const PATHS = {
  folder: 'M2 4.5h4l1.5 1.5H14v6.5H2z',
  file: 'M4 2h5l3 3v9H4zM9 2v3h3',
  link: 'M6.5 9.5l5-5M8.5 4.5h3v3M11.5 10v2.5h-8v-8H6',
  reveal: 'M2 4.5h4l1.5 1.5H13v1.5M2 4.5v8h10.5L14 8H4.5L3 12.5',
  copy: 'M5.5 5.5h7v8h-7zM3.5 10.5v-8h7',
  back: 'M10 3.5L5.5 8l4.5 4.5',
} as const

defineProps<{ name: keyof typeof PATHS }>()
</script>

<template>
  <svg class="icon" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true" focusable="false">
    <path :d="PATHS[name]" />
  </svg>
</template>

<style scoped>
.icon {
  flex: none;
  vertical-align: -3px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.4;
  stroke-linejoin: round;
  stroke-linecap: round;
}
</style>
