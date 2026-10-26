<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useBackend } from '../../backend/inject'
import type { Node, SortKey } from '../../backend/types'
import { formatBytes, formatCount, formatDate, formatPercent } from './format'
import { flagLabels, isEstimated, showLogical, sizeNote } from './size'
import TreemapChart from './treemap/TreemapChart.vue'
import { useExplorer } from './useExplorer'

const props = defineProps<{
  root: string // folder path; the explorer is recreated for another folder
  name: string
  autoscan?: boolean // start a scan as soon as the current index is shown
}>()

const ex = useExplorer(useBackend(), props.root)
const { trail, children, rows, total, sort, progress, summary, scanning, saving, error, current, hasMore } =
  ex
if (props.autoscan) void ex.ready.then(ex.scan)

const STATUS_TEXT = { completed: 'Completado', cancelled: 'Cancelado', failed: 'Falló' } as const
const hovered = ref<number | null>(null) // shared by the table and the treemap
watch(current, () => (hovered.value = null))
const share = (diskSize: number) => (current.value?.diskSize ? diskSize / current.value.diskSize : 0)
const logicalCol = computed(() => showLogical(current.value))
const estimated = computed(() => !!trail.value[0] && isEstimated(trail.value[0]))

const ariaSort = (key: SortKey) =>
  sort.value.key !== key ? 'none' : sort.value.desc ? 'descending' : 'ascending'
const arrow = (key: SortKey) => (sort.value.key !== key ? '' : sort.value.desc ? ' ↓' : ' ↑')

// Keyboard: each row is one tab stop; arrows move between rows, Enter opens a
// folder and Backspace goes up a level.
function onRowKey(e: KeyboardEvent, n: Node) {
  const row = e.currentTarget as HTMLElement
  if (e.key === 'Enter') void ex.open(n)
  else if (e.key === 'ArrowDown') (row.nextElementSibling as HTMLElement | null)?.focus()
  else if (e.key === 'ArrowUp') (row.previousElementSibling as HTMLElement | null)?.focus()
  else return
  e.preventDefault()
}
function onKey(e: KeyboardEvent) {
  const t = e.target as HTMLElement
  if (e.key !== 'Backspace' || t.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(t.tagName))
    return
  e.preventDefault()
  void ex.up()
}
onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <main class="explorer">
    <header class="toolbar">
      <RouterLink to="/" class="back">← Inicio</RouterLink>
      <h1 class="title">{{ name }}</h1>
      <code class="root" :title="root">{{ root }}</code>
      <button type="button" class="primary" :disabled="scanning" @click="ex.scan">Analizar</button>
      <button
        type="button"
        :disabled="!scanning || saving"
        :title="
          saving ? 'Los resultados ya se están guardando; cancelar ahora perdería el análisis' : undefined
        "
        @click="ex.cancel"
      >
        Cancelar
      </button>
    </header>

    <p class="status" aria-live="polite">
      <template v-if="progress">
        {{ formatCount(progress.files) }} archivos · {{ formatCount(progress.dirs) }} carpetas ·
        <span :title="`${formatBytes(progress.bytes)} lógicos`"
          >{{ formatBytes(progress.diskBytes) }} en disco</span
        >
        <template v-if="progress.errors"> · {{ formatCount(progress.errors) }} sin acceso</template>
      </template>
      <span v-if="scanning"> - {{ saving ? 'guardando resultados…' : 'analizando…' }}</span>
      <span v-else-if="summary">
        — {{ STATUS_TEXT[summary.status] }} en {{ (summary.elapsedMs / 1000).toFixed(1) }} s
        <template v-if="summary.error">: {{ summary.error }}</template>
      </span>
    </p>

    <p v-if="error" class="error" role="alert">{{ error }}</p>

    <p v-if="!trail.length && !scanning" class="empty">
      Esta carpeta aún no se ha analizado. Pulsa <strong>Analizar</strong>.
    </p>

    <p v-if="estimated && !scanning" class="notice">
      Este análisis es anterior al cálculo del tamaño en disco y muestra tamaños lógicos. Pulsa
      <strong>Analizar</strong> para ver el espacio real.
    </p>

    <nav v-if="trail.length" class="crumbs" aria-label="Ruta">
      <template v-for="(n, i) in trail" :key="n.id">
        <button
          type="button"
          class="link"
          :aria-current="i === trail.length - 1 ? 'page' : undefined"
          @click="ex.goTo(i)"
        >
          {{ n.name }}
        </button>
        <span v-if="i < trail.length - 1" aria-hidden="true">›</span>
      </template>
      <span v-if="current" class="total" :title="sizeNote(current) || `${formatBytes(current.size)} lógicos`"
        >{{ formatBytes(current.diskSize) }}{{ estimated ? '' : ' en disco' }} ·
        {{ formatCount(current.files) }} archivos</span
      >
    </nav>

    <TreemapChart
      v-if="current && children.length"
      :nodes="children"
      :parent="current"
      :highlighted="hovered"
      @hover="hovered = $event"
      @open="ex.open"
    />

    <table v-if="rows.length">
      <thead>
        <tr>
          <th scope="col" :aria-sort="ariaSort('name')">
            <button type="button" class="sort" @click="ex.sortBy('name')">Nombre{{ arrow('name') }}</button>
          </th>
          <th scope="col" class="num" :aria-sort="ariaSort('size')">
            <button type="button" class="sort" @click="ex.sortBy('size')">En disco{{ arrow('size') }}</button>
          </th>
          <th v-if="logicalCol" scope="col" class="num">Lógico</th>
          <th scope="col" class="num">% de la carpeta</th>
          <th scope="col" class="num">Archivos</th>
          <th scope="col" class="num" :aria-sort="ariaSort('modified')">
            <button type="button" class="sort" @click="ex.sortBy('modified')">
              Modificado{{ arrow('modified') }}
            </button>
          </th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="n in rows"
          :key="n.id"
          tabindex="0"
          :class="{ hovered: hovered === n.id }"
          @mouseenter="hovered = n.id"
          @mouseleave="hovered = null"
          @keydown="onRowKey($event, n)"
        >
          <td class="name">
            <button v-if="n.kind === 'dir'" type="button" class="link" tabindex="-1" @click="ex.open(n)">
              📁 {{ n.name }}
            </button>
            <span v-else>{{ n.kind === 'link' ? '🔗' : '📄' }} {{ n.name }}</span>
            <span v-if="n.err" class="badge" :title="n.err">sin acceso</span>
            <span v-for="label in flagLabels(n.flags)" :key="label" class="badge info" :title="sizeNote(n)">
              {{ label }}
            </span>
          </td>
          <td class="num">{{ formatBytes(n.diskSize) }}</td>
          <td v-if="logicalCol" class="num logical">{{ formatBytes(n.size) }}</td>
          <td class="num share">
            <span class="bar" :style="{ width: `${share(n.diskSize) * 100}%` }" aria-hidden="true"></span>
            <span class="pct">{{ formatPercent(n.diskSize, current?.diskSize ?? 0) }}</span>
          </td>
          <td class="num">{{ formatCount(n.files) }}</td>
          <td class="num date">{{ formatDate(n.modified) }}</td>
        </tr>
      </tbody>
    </table>
    <p v-else-if="trail.length" class="empty">Carpeta vacía.</p>
    <p v-if="rows.length" class="status more">
      Mostrando {{ formatCount(rows.length) }} de {{ formatCount(total) }}
      <button v-if="hasMore" type="button" @click="ex.loadMore">Cargar más</button>
    </p>
  </main>
</template>

<style scoped>
.explorer {
  padding: 16px 20px;
}
.toolbar {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}
.back {
  color: var(--accent);
  text-decoration: none;
}
.title {
  font-size: 16px;
  margin: 0;
}
.root {
  color: var(--muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
  flex: 1;
}
.status {
  min-height: 1.4em;
  color: var(--muted);
  margin: 10px 0;
}
.error {
  background: var(--danger-bg);
  color: var(--danger);
  padding: 8px 12px;
  border-radius: 6px;
}
.empty {
  color: var(--muted);
}
.crumbs {
  display: flex;
  gap: 6px;
  align-items: center;
  flex-wrap: wrap;
  margin: 8px 0;
}
.crumbs [aria-current] {
  font-weight: 600;
}
.total {
  margin-left: auto;
  color: var(--muted);
}
table {
  width: 100%;
  border-collapse: collapse;
}
th,
td {
  padding: 6px 8px;
  border-bottom: 1px solid var(--border);
  text-align: left;
}
th {
  font-weight: 600;
  color: var(--muted);
}
tr.hovered {
  background: var(--row-hover);
}
.num {
  text-align: right;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.name {
  word-break: break-all;
}
.share {
  position: relative;
  width: 160px;
}
.bar {
  position: absolute;
  inset: 4px auto 4px 0;
  background: var(--accent-soft);
  border-radius: 3px;
}
.pct {
  position: relative;
}
.badge {
  margin-left: 8px;
  font-size: 11px;
  color: var(--danger);
  border: 1px solid currentColor;
  border-radius: 4px;
  padding: 0 4px;
}
.badge.info {
  color: var(--muted);
  cursor: help;
}
.logical,
.date {
  color: var(--muted);
}
th .sort {
  font: inherit;
  color: inherit;
  background: none;
  border: none;
  padding: 0;
  cursor: pointer;
}
th[aria-sort='ascending'],
th[aria-sort='descending'] {
  color: var(--fg);
}
tbody tr:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: -2px;
}
.more {
  display: flex;
  gap: 12px;
  align-items: center;
}
.notice {
  background: var(--accent-soft);
  padding: 8px 12px;
  border-radius: 6px;
}
</style>
