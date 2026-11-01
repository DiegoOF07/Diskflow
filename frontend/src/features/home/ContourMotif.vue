<script setup lang="ts">
import { hillContours } from './contours'

const LEVELS = 11
const paths = hillContours(LEVELS, 160, 100, 64)
</script>

<template>
  <!-- Every fifth line (counted from the base) is an index contour, drawn heavier as on survey maps; the summit is the forest colour. -->
  <svg class="motif" viewBox="0 0 320 200" aria-hidden="true" focusable="false">
    <path
      v-for="(d, i) in paths"
      :key="i"
      :d="d"
      :class="{ index: (LEVELS - i) % 5 === 0, summit: i === paths.length - 1 }"
    />
  </svg>
</template>

<style scoped>
.motif {
  display: block;
  width: 100%;
  max-width: 360px;
  height: auto;
}
path {
  fill: none;
  stroke: var(--contour);
  stroke-width: 1.2;
}
path.index {
  stroke: var(--graphite);
  stroke-opacity: 0.6;
  stroke-width: 1.6;
}
path.summit {
  fill: var(--forest);
  stroke: var(--forest);
}
</style>
