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
  more: 'M2.5 8a1 1 0 1 0 2 0a1 1 0 1 0-2 0M7 8a1 1 0 1 0 2 0a1 1 0 1 0-2 0M11.5 8a1 1 0 1 0 2 0a1 1 0 1 0-2 0',
  rename: 'M10.5 3l2.5 2.5L6 12.5H3.5V10zM9 4.5L11.5 7',
  remove: 'M4.5 4.5l7 7M11.5 4.5l-7 7',
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
