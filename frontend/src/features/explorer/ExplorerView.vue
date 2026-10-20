<script setup lang="ts">
import { ref, watch } from 'vue'
import { useBackend } from '../../backend/inject'
import { formatBytes, formatCount, formatPercent } from './format'
import Treemap from './treemap/Treemap.vue'
import { PAGE_SIZE, useExplorer } from './useExplorer'

const ex = useExplorer(useBackend())
const { root, trail, children, progress, summary, scanning, error, current, truncated } = ex

const STATUS_TEXT = { completed: 'Completado', cancelled: 'Cancelado', failed: 'Falló' } as const
const hovered = ref<number | null>(null) // shared by the table and the treemap
watch(current, () => (hovered.value = null))
const share = (size: number) => (current.value?.size ? size / current.value.size : 0)
</script>

<template>
  <main class="explorer">
    <header class="toolbar">
      <button type="button" :disabled="scanning" @click="ex.pickFolder">Elegir carpeta</button>
      <button type="button" class="primary" :disabled="!root || scanning" @click="ex.scan">Analizar</button>
      <button type="button" :disabled="!scanning" @click="ex.cancel">Cancelar</button>
      <code v-if="root" class="root" :title="root ?? ''">{{ root ?? 'Ninguna carpeta elegida' }}</code>
    </header>

    <div v-if="!root" class="empty-card">
      <code class="empty-message" :title="root ?? ''">{{ root ?? 'Ninguna carpeta elegida' }}</code>
    </div>

    <p class="status" aria-live="polite">
      <template v-if="progress">
        {{ formatCount(progress.files) }} archivos · {{ formatCount(progress.dirs) }} carpetas ·
        {{ formatBytes(progress.bytes) }}
        <template v-if="progress.errors"> · {{ formatCount(progress.errors) }} sin acceso</template>
      </template>
      <span v-if="scanning"> — analizando…</span>
      <span v-else-if="summary">
        — {{ STATUS_TEXT[summary.status] }} en {{ (summary.elapsedMs / 1000).toFixed(1) }} s
        <template v-if="summary.error">: {{ summary.error }}</template>
      </span>
    </p>

    <p v-if="error" class="error" role="alert">{{ error }}</p>

    <p v-if="root && !trail.length && !scanning" class="empty">
      Esta carpeta aún no se ha analizado. Pulsa <strong>Analizar</strong>.
    </p>

    <nav v-if="trail.length" class="crumbs" aria-label="Ruta">
      <template v-for="(n, i) in trail" :key="n.id">
        <button type="button" class="link" :aria-current="i === trail.length - 1 ? 'page' : undefined" @click="ex.goTo(i)">
          {{ n.name }}
        </button>
        <span v-if="i < trail.length - 1" aria-hidden="true">›</span>
      </template>
      <span class="total" v-if="current">{{ formatBytes(current.size) }} · {{ formatCount(current.files) }} archivos</span>
    </nav>

    <Treemap
      v-if="current && children.length"
      :nodes="children"
      :parent-size="current.size"
      :highlighted="hovered"
      @hover="hovered = $event"
      @open="ex.open"
    />

    <table v-if="children.length">
      <thead>
        <tr>
          <th scope="col">Nombre</th>
          <th scope="col" class="num">Tamaño</th>
          <th scope="col" class="num">% de la carpeta</th>
          <th scope="col" class="num">Archivos</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="n in children"
          :key="n.id"
          :class="{ hovered: hovered === n.id }"
          @mouseenter="hovered = n.id"
          @mouseleave="hovered = null"
        >
          <td class="name">
            <button v-if="n.kind === 'dir'" type="button" class="link" @click="ex.open(n)">📁 {{ n.name }}</button>
            <span v-else>{{ n.kind === 'link' ? '🔗' : '📄' }} {{ n.name }}</span>
            <span v-if="n.err" class="badge" :title="n.err">sin acceso</span>
          </td>
          <td class="num">{{ formatBytes(n.size) }}</td>
          <td class="num share">
            <span class="bar" :style="{ width: `${share(n.size) * 100}%` }" aria-hidden="true"></span>
            <span class="pct">{{ formatPercent(n.size, current?.size ?? 0) }}</span>
          </td>
          <td class="num">{{ formatCount(n.files) }}</td>
        </tr>
      </tbody>
    </table>
    <p v-else-if="trail.length" class="empty">Carpeta vacía.</p>
    <p v-if="truncated" class="status">Se muestran los {{ PAGE_SIZE }} elementos más grandes.</p>
  </main>
</template>

<style scoped>
.explorer { padding: 16px 20px; }
.toolbar { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.root { color: var(--muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; flex: 1; }
.status { min-height: 1.4em; color: var(--muted); margin: 10px 0; }
.error { background: var(--danger-bg); color: var(--danger); padding: 8px 12px; border-radius: 6px; }
.empty { color: var(--muted); }
.empty-card { display: flex; align-items: center; justify-content: center; height: 85dvh; width: 100%; color: var(--muted); }
.empty-message { font-size: 14px; text-align: center; margin: 5rem;}
.crumbs { display: flex; gap: 6px; align-items: center; flex-wrap: wrap; margin: 8px 0; }
.crumbs [aria-current] { font-weight: 600; }
.total { margin-left: auto; color: var(--muted); }
table { width: 100%; border-collapse: collapse; }
th, td { padding: 6px 8px; border-bottom: 1px solid var(--border); text-align: left; }
th { font-weight: 600; color: var(--muted); }
tr.hovered { background: var(--row-hover); }
.num { text-align: right; font-variant-numeric: tabular-nums; white-space: nowrap; }
.name { word-break: break-all; }
.share { position: relative; width: 160px; }
.bar { position: absolute; inset: 4px auto 4px 0; background: var(--accent-soft); border-radius: 3px; }
.pct { position: relative; }
.badge { margin-left: 8px; font-size: 11px; color: var(--danger); border: 1px solid currentColor; border-radius: 4px; padding: 0 4px; }
</style>
