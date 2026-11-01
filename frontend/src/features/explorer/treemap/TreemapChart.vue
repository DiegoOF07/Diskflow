<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import type { Node } from '../../../backend/types'
import { formatBytes, formatCount, formatPercent } from '../format'
import { sizeNote } from '../size'
import { landUse, legendFor } from './landuse'
import { fitLabel, groupOthers, layoutTreemap, OTHERS_ID } from './layout'

const props = defineProps<{
  nodes: readonly Node[]
  parent: Node
  highlighted: number | null // id hovered in the table
  estimated?: boolean // the index predates on-disk sizes, so the total is logical
}>()
const emit = defineEmits<{ open: [node: Node]; hover: [id: number | null] }>()

const box = ref<HTMLDivElement>()
const width = ref(0)
const height = ref(0)
let observer: ResizeObserver | undefined

// The SVG is sized in pixels, so it follows its box, whose size the page layout sets.
onMounted(() => {
  observer = new ResizeObserver(([entry]) => {
    width.value = Math.floor(entry.contentRect.width)
    height.value = Math.floor(entry.contentRect.height)
  })
  observer.observe(box.value!)
})
onBeforeUnmount(() => observer?.disconnect())

const cells = computed(() => groupOthers(props.nodes, props.parent))
const rects = computed(() => layoutTreemap(cells.value, width.value, height.value))
const legend = computed(() => legendFor(cells.value))

// A row hidden inside "Otros" highlights the "Otros" cell.
const active = computed(() => {
  const id = props.highlighted
  if (id == null) return null
  return cells.value.some((c) => c.id === id) ? id : OTHERS_ID
})

function cellTitle(n: Node) {
  const share = formatPercent(n.diskSize, props.parent.diskSize)
  const lines = [n.name, `${formatBytes(n.diskSize)} en disco, ${share} de la carpeta`, sizeNote(n)]
  return lines.filter(Boolean).join('\n')
}
</script>

<template>
  <figure class="map">
    <div ref="box" class="treemap" @mouseleave="emit('hover', null)">
      <svg :width="width" :height="height" role="img" aria-label="Mapa de tamaños de la carpeta actual">
        <defs>
          <!-- Hatching marks the area that groups many small entries, as maps do for mixed zones. -->
          <pattern
            id="treemap-hatch"
            width="6"
            height="6"
            patternUnits="userSpaceOnUse"
            patternTransform="rotate(45)"
          >
            <rect width="6" height="6" class="hatch-ground" />
            <line x1="0" y1="0" x2="0" y2="6" class="hatch-line" />
          </pattern>
        </defs>
        <g
          v-for="r in rects"
          :key="r.node.id"
          :class="['cell', landUse(r.node), { open: r.node.kind === 'dir', active: r.node.id === active }]"
          @mouseenter="emit('hover', r.node.id)"
          @click="r.node.kind === 'dir' && emit('open', r.node)"
        >
          <rect :x="r.x0" :y="r.y0" :width="r.x1 - r.x0" :height="r.y1 - r.y0" />
          <rect
            v-if="r.node.id === active"
            class="outline"
            :x="r.x0 + 1.5"
            :y="r.y0 + 1.5"
            :width="Math.max(r.x1 - r.x0 - 3, 0)"
            :height="Math.max(r.y1 - r.y0 - 3, 0)"
          />
          <title>{{ cellTitle(r.node) }}</title>
          <template v-if="r.y1 - r.y0 > 18">
            <text class="name" :x="r.x0 + 5" :y="r.y0 + 15">{{ fitLabel(r.node.name, r.x1 - r.x0) }}</text>
            <text v-if="r.y1 - r.y0 > 34" class="size" :x="r.x0 + 5" :y="r.y0 + 29">
              {{ fitLabel(formatBytes(r.node.diskSize), r.x1 - r.x0) }}
            </text>
          </template>
        </g>
      </svg>
    </div>
    <figcaption class="caption">
      <ul class="legend" aria-label="Leyenda">
        <li v-for="l in legend" :key="l.use"><span :class="['swatch', l.use]"></span>{{ l.label }}</li>
      </ul>
      <span class="totals" :title="sizeNote(parent) || `${formatBytes(parent.size)} lógicos`">
        <span class="num">{{ formatBytes(parent.diskSize) }}{{ estimated ? '' : ' en disco' }}</span>
        <span class="num">{{ formatCount(parent.files) }} archivos</span>
      </span>
    </figcaption>
  </figure>
</template>

<style scoped>
.map {
  margin: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}
.treemap {
  flex: 1;
  min-height: 0;
  overflow: hidden; /* the SVG must not push its box bigger, or the observer would grow it forever */
}
svg {
  display: block;
}
rect {
  stroke: var(--paper);
  stroke-width: 1.5;
}
text {
  font-family: var(--font-condensed);
  font-size: 12px;
  pointer-events: none;
  fill: var(--ink);
}
text.name {
  font-weight: 600;
}
.cell.forest rect {
  fill: var(--forest);
}
.cell.sand rect {
  fill: var(--sand);
}
.cell.rock rect {
  fill: var(--rock);
}
.cell.grouped > rect {
  fill: url(#treemap-hatch);
}
.hatch-ground {
  fill: var(--rock);
  stroke: none;
}
.hatch-line {
  stroke: var(--paper);
  stroke-width: 1.5;
}
.cell.open {
  cursor: pointer;
}
.cell.open:hover rect,
.cell.open.active rect {
  fill: var(--forest-active);
}
.cell rect.outline {
  fill: none;
  stroke: var(--ink);
  stroke-width: 2;
}

.caption {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: var(--space-1) var(--space-4);
  font-size: var(--text-sm);
  color: var(--graphite);
}
.legend {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-1) var(--space-4);
  margin: 0;
  padding: 0;
  list-style: none;
}
.legend li {
  display: flex;
  align-items: center;
  gap: 6px;
}
.swatch {
  width: 12px;
  height: 12px;
  border-radius: 2px;
}
.swatch.forest {
  background: var(--forest);
}
.swatch.sand {
  background: var(--sand);
}
.swatch.rock {
  background: var(--rock);
}
.swatch.grouped {
  background: repeating-linear-gradient(45deg, var(--rock) 0 3px, var(--paper) 3px 4.5px);
}
.totals {
  display: flex;
  gap: var(--space-4);
}
</style>
