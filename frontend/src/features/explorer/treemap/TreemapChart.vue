<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import type { Node } from '../../../backend/types'
import { formatBytes, formatPercent } from '../format'
import { fitLabel, groupOthers, layoutTreemap, OTHERS_ID } from './layout'

const props = defineProps<{
  nodes: readonly Node[]
  parentSize: number
  highlighted: number | null // id hovered in the table
}>()
const emit = defineEmits<{ open: [node: Node]; hover: [id: number | null] }>()

const HEIGHT = 300
const box = ref<HTMLDivElement>()
const width = ref(0)
let observer: ResizeObserver | undefined

// The SVG is sized in pixels, so it follows the container's width.
onMounted(() => {
  observer = new ResizeObserver(([entry]) => (width.value = Math.floor(entry.contentRect.width)))
  observer.observe(box.value!)
})
onBeforeUnmount(() => observer?.disconnect())

const cells = computed(() => groupOthers(props.nodes, props.parentSize))
const rects = computed(() => layoutTreemap(cells.value, width.value, HEIGHT))

// A row hidden inside "Otros" highlights the "Otros" cell.
const active = computed(() => {
  const id = props.highlighted
  if (id == null) return null
  return cells.value.some((c) => c.id === id) ? id : OTHERS_ID
})

const kindClass = (n: Node) => (n.id === OTHERS_ID ? 'others' : n.kind)
</script>

<template>
  <div ref="box" class="treemap" :style="{ height: `${HEIGHT}px` }" @mouseleave="emit('hover', null)">
    <svg :width="width" :height="HEIGHT" role="img" aria-label="Mapa de tamaños de la carpeta actual">
      <g
        v-for="r in rects"
        :key="r.node.id"
        :class="['cell', kindClass(r.node), { active: r.node.id === active }]"
        @mouseenter="emit('hover', r.node.id)"
        @click="r.node.kind === 'dir' && emit('open', r.node)"
      >
        <rect :x="r.x0" :y="r.y0" :width="r.x1 - r.x0" :height="r.y1 - r.y0" />
        <title>
          {{ r.node.name }} — {{ formatBytes(r.node.size) }} ({{ formatPercent(r.node.size, parentSize) }})
        </title>
        <template v-if="r.y1 - r.y0 > 18">
          <text :x="r.x0 + 4" :y="r.y0 + 15">{{ fitLabel(r.node.name, r.x1 - r.x0) }}</text>
          <text v-if="r.y1 - r.y0 > 36" class="size" :x="r.x0 + 4" :y="r.y0 + 30">
            {{ fitLabel(formatBytes(r.node.size), r.x1 - r.x0) }}
          </text>
        </template>
      </g>
    </svg>
  </div>
</template>

<style scoped>
.treemap {
  width: 100%;
  margin: 8px 0 12px;
}
svg {
  display: block;
}
rect {
  stroke: var(--bg);
  stroke-width: 1;
}
text {
  font-size: 12px;
  pointer-events: none;
  fill: var(--fg);
}
text.size {
  opacity: 0.8;
}
.cell.dir rect {
  fill: var(--cell-dir);
  cursor: pointer;
}
.cell.dir text {
  fill: #fff;
}
.cell.file rect {
  fill: var(--cell-file);
}
.cell.link rect,
.cell.other rect {
  fill: var(--cell-other);
}
.cell.others rect {
  fill: var(--cell-others);
}
.cell.active rect {
  stroke: var(--fg);
  stroke-width: 2;
}
.cell.dir.active rect,
.cell.dir:hover rect {
  fill: var(--cell-dir-hover);
}
</style>
