<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useTemplateRef, watch } from 'vue'
import { useBackend } from '../../backend/inject'
import AppIcon from '../../components/AppIcon.vue'
import type { Hit, Node, RootInfo, SortKey } from '../../backend/types'
import { previousIndexNote } from '../home/status'
import { progressFacts } from './facts'
import { formatAgo, formatBytes, formatCount, formatDate, formatPercent } from './format'
import { flagLabels, isEstimated, showLogical, sizeNote } from './size'
import { landUse } from './treemap/landuse'
import TreemapChart from './treemap/TreemapChart.vue'
import { useExplorer } from './useExplorer'
import { SEARCH_LIMIT, useSearch } from './useSearch'
import { UNREADABLE_LIMIT, unreadableReason, useUnreadable } from './useUnreadable'

const props = defineProps<{
  info: RootInfo // the explorer is recreated for another folder, so its path never changes
  autoscan?: boolean // start a scan as soon as the current index is shown
}>()

const backend = useBackend()
const ex = useExplorer(backend, props.info.path)
const {
  trail,
  children,
  rows,
  total,
  sort,
  found,
  copied,
  progress,
  summary,
  scanning,
  saving,
  error,
  errorCode,
  current,
  hasMore,
} = ex
const search = useSearch(backend, props.info.path)
const unreadable = useUnreadable(backend, props.info.path)
const { query, hits, more, searching } = search
const searchBox = useTemplateRef<HTMLInputElement>('searchBox')
const searchActive = computed(() => query.value.trim() !== '')
if (props.autoscan) void ex.ready.then(ex.scan)

const STATUS_TEXT = { completed: 'Completado', cancelled: 'Cancelado', failed: 'Falló' } as const
const hovered = ref<number | null>(null) // shared by the table and the treemap
watch(current, () => (hovered.value = null))
const share = (diskSize: number) => (current.value?.diskSize ? diskSize / current.value.diskSize : 0)
const logicalCol = computed(() => showLogical(current.value))
const estimated = computed(() => !!trail.value[0] && isEstimated(trail.value[0]))

const kindIcon = (n: Node) => (n.kind === 'dir' ? 'folder' : n.kind === 'file' ? 'file' : 'link')

const ariaSort = (key: SortKey) =>
  sort.value.key !== key ? 'none' : sort.value.desc ? 'descending' : 'ascending'
const arrow = (key: SortKey) => (sort.value.key !== key ? '' : sort.value.desc ? ' ↓' : ' ↑')

// Context menu of a row, opened with the right button, the Menu key or Shift+F10.
const MENU_WIDTH = 220
const menu = ref<{ node: Node; x: number; y: number } | null>(null)
const menuEl = useTemplateRef<HTMLElement>('menuEl')
async function openMenu(node: Node, x: number, y: number) {
  menu.value = { node, x: Math.min(x, window.innerWidth - MENU_WIDTH), y }
  await nextTick()
  menuEl.value?.querySelector('button')?.focus()
}
function menuAt(e: MouseEvent, node: Node) {
  void openMenu(node, e.clientX, e.clientY)
}
function closeMenu() {
  menu.value = null
}
function act(fn: (n: Node) => unknown) {
  const node = menu.value?.node
  closeMenu()
  if (node) void fn(node)
}

function pick(hit: Hit) {
  search.clear()
  void ex.reveal(hit)
}
// Bring the revealed row into view once it is rendered.
watch(found, async (id) => {
  if (id === null) return
  await nextTick()
  const row = document.querySelector<HTMLElement>(`tr[data-id="${id}"]`)
  row?.focus({ preventScroll: true })
  row?.scrollIntoView({ block: 'center' })
})

// Keyboard: each row is one tab stop; arrows move between rows, Enter acts on
// the row (opens a folder, shows a search hit) and Backspace goes up a level.
function onRowKey(e: KeyboardEvent, enter: () => unknown, node?: Node) {
  const row = e.currentTarget as HTMLElement
  if (node && (e.key === 'ContextMenu' || (e.shiftKey && e.key === 'F10'))) {
    const r = row.getBoundingClientRect()
    void openMenu(node, r.left + 24, r.bottom)
  } else if (e.key === 'Enter') void enter()
  else if (e.key === 'ArrowDown') (row.nextElementSibling as HTMLElement | null)?.focus()
  else if (e.key === 'ArrowUp') (row.previousElementSibling as HTMLElement | null)?.focus()
  else return
  e.preventDefault()
}
function onKey(e: KeyboardEvent) {
  if (menu.value) {
    if (e.key === 'Escape') closeMenu()
    return
  }
  if (e.ctrlKey && e.key === 'f' && searchBox.value) {
    e.preventDefault()
    searchBox.value.focus()
    return
  }
  const t = e.target as HTMLElement
  if (e.key !== 'Backspace' || t.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(t.tagName))
    return
  e.preventDefault()
  void ex.up()
}
onMounted(() => {
  window.addEventListener('keydown', onKey)
  window.addEventListener('click', closeMenu)
  window.addEventListener('blur', closeMenu)
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKey)
  window.removeEventListener('click', closeMenu)
  window.removeEventListener('blur', closeMenu)
})
</script>

<template>
  <main class="explorer">
    <header class="toolbar">
      <RouterLink to="/" class="back"><AppIcon name="back" />Inicio</RouterLink>
      <div class="heading">
        <h1 class="title">{{ info.name }}</h1>
        <code class="root" :title="info.path">{{ info.path }}</code>
      </div>
      <div class="commands">
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
      </div>
    </header>

    <p class="facts" aria-live="polite">
      <template v-if="scanning || summary">
        <span v-for="f in progress ? progressFacts(progress) : []" :key="f" class="num">{{ f }}</span>
        <span v-if="scanning" class="phase">{{ saving ? 'Guardando resultados…' : 'Analizando…' }}</span>
        <span v-else-if="summary" class="phase">
          {{ STATUS_TEXT[summary.status] }} en {{ (summary.elapsedMs / 1000).toFixed(1) }} s<template
            v-if="summary.error"
            >: {{ summary.error }}</template
          >
        </span>
      </template>
      <span v-else-if="info.index" :title="formatDate(info.index.finishedAt)">
        Analizado {{ formatAgo(info.index.finishedAt) }}
      </span>
      <button
        v-if="info.index?.errors && trail.length && !scanning"
        type="button"
        class="link"
        :aria-expanded="unreadable.open.value"
        title="No se pudieron leer, así que los totales no los incluyen"
        @click="unreadable.toggle"
      >
        {{ formatCount(info.index.errors) }} elementos sin acceso
      </button>
    </p>

    <p v-if="error" class="notice problem" role="alert">
      {{ error }}
      <RouterLink v-if="errorCode === 'root_unavailable'" to="/">Quitarla desde Inicio</RouterLink>
    </p>
    <p v-if="!info.available && errorCode !== 'root_unavailable'" class="notice problem" role="status">
      No se encuentra esta carpeta: puede que se haya movido o borrado, o que su unidad esté desconectada.
      <template v-if="trail.length">Se muestra su último análisis. </template>
      Si ya no existe, puedes <RouterLink to="/">quitarla desde Inicio</RouterLink>.
    </p>
    <p v-if="!scanning && previousIndexNote(info)" class="notice" role="status">
      {{ previousIndexNote(info) }}
    </p>
    <p v-if="estimated && !scanning" class="notice">
      Este análisis es anterior al cálculo del tamaño en disco y muestra tamaños lógicos. Analiza de nuevo
      para ver el espacio real.
    </p>

    <div v-if="!trail.length && !scanning" class="empty-state">
      <p>Todavía no hay un mapa de esta carpeta. Analízala para ver qué ocupa su espacio.</p>
      <button type="button" class="primary" @click="ex.scan">Analizar</button>
    </div>

    <div v-if="trail.length" class="wayfinding">
      <nav class="crumbs" aria-label="Ruta">
        <template v-for="(n, i) in trail" :key="n.id">
          <button
            type="button"
            class="link"
            :aria-current="i === trail.length - 1 ? 'page' : undefined"
            @click="ex.goTo(i)"
          >
            {{ n.name }}
          </button>
          <span v-if="i < trail.length - 1" class="sep" aria-hidden="true">›</span>
        </template>
      </nav>
      <div class="search" role="search">
        <input
          ref="searchBox"
          v-model="query"
          type="search"
          placeholder="Buscar por nombre (Ctrl+F)"
          aria-label="Buscar por nombre"
          @keydown.esc="search.clear()"
        />
      </div>
    </div>

    <section v-if="searchActive" class="results" aria-label="Resultados de la búsqueda">
      <p class="facts" aria-live="polite">
        <template v-if="searching">Buscando…</template>
        <span v-else-if="search.error.value" class="error-text">{{ search.error.value }}</span>
        <template v-else-if="!hits.length">Sin resultados para «{{ query.trim() }}».</template>
        <template v-else-if="more">
          Más de {{ SEARCH_LIMIT }} resultados; se muestran los más grandes. Escribe algo más específico.
        </template>
        <template v-else>{{ formatCount(hits.length) }} resultados.</template>
      </p>
      <table v-if="hits.length" class="hits">
        <thead>
          <tr>
            <th scope="col">Nombre</th>
            <th scope="col">Carpeta</th>
            <th scope="col" class="num">En disco</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="h in hits"
            :key="h.id"
            tabindex="0"
            class="hit"
            @click="pick(h)"
            @keydown="onRowKey($event, () => pick(h))"
          >
            <td class="name"><AppIcon :name="kindIcon(h)" class="kind" />{{ h.name }}</td>
            <td class="dir" :title="h.dir">{{ h.dir }}</td>
            <td class="num">{{ formatBytes(h.diskSize) }}</td>
          </tr>
        </tbody>
      </table>
    </section>

    <div v-else-if="trail.length" class="workspace">
      <div v-if="current && children.length" class="map-pane">
        <TreemapChart
          :nodes="children"
          :parent="current"
          :highlighted="hovered ?? found"
          :estimated="estimated"
          @hover="hovered = $event"
          @open="ex.open"
        />
      </div>
      <div class="table-pane">
        <section v-if="unreadable.open.value" class="unreadable" aria-label="Elementos sin acceso">
          <div class="panel-head">
            <h2>Elementos sin acceso</h2>
            <button type="button" @click="unreadable.toggle">Cerrar</button>
          </div>
          <p class="facts" aria-live="polite">
            <template v-if="unreadable.loading.value">Cargando…</template>
            <span v-else-if="unreadable.error.value" class="error-text">{{ unreadable.error.value }}</span>
            <template v-else>
              No se pudieron leer, así que los totales no los incluyen. Elige uno para ver dónde está.
              <template v-if="unreadable.more.value"
                >Se muestran los primeros {{ UNREADABLE_LIMIT }}.</template
              >
            </template>
          </p>
          <table v-if="unreadable.hits.value.length" class="hits">
            <thead>
              <tr>
                <th scope="col">Nombre</th>
                <th scope="col">Carpeta</th>
                <th scope="col">Motivo</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="h in unreadable.hits.value"
                :key="h.id"
                tabindex="0"
                class="hit"
                @click="pick(h)"
                @keydown="onRowKey($event, () => pick(h))"
              >
                <td class="name"><AppIcon :name="kindIcon(h)" class="kind" />{{ h.name }}</td>
                <td class="dir" :title="h.dir">{{ h.dir }}</td>
                <td>{{ unreadableReason(h.err) }}</td>
              </tr>
            </tbody>
          </table>
        </section>

        <table v-if="rows.length" class="entries">
          <thead>
            <tr>
              <th scope="col" :aria-sort="ariaSort('name')">
                <button type="button" class="sort" @click="ex.sortBy('name')">
                  Nombre{{ arrow('name') }}
                </button>
              </th>
              <th scope="col" class="num" :aria-sort="ariaSort('size')">
                <button type="button" class="sort" @click="ex.sortBy('size')">
                  En disco{{ arrow('size') }}
                </button>
              </th>
              <th v-if="logicalCol" scope="col" class="num col-logical">Lógico</th>
              <th scope="col" class="num share">% de la carpeta</th>
              <th scope="col" class="num col-files">Archivos</th>
              <th scope="col" class="num col-date" :aria-sort="ariaSort('modified')">
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
              :data-id="n.id"
              :class="{ hovered: hovered === n.id, found: found === n.id }"
              @mouseenter="hovered = n.id"
              @mouseleave="hovered = null"
              @keydown="onRowKey($event, () => ex.open(n), n)"
              @contextmenu.prevent="menuAt($event, n)"
            >
              <td class="name">
                <button v-if="n.kind === 'dir'" type="button" class="entry" tabindex="-1" @click="ex.open(n)">
                  <AppIcon name="folder" class="kind" />{{ n.name }}
                </button>
                <span v-else class="entry"><AppIcon :name="kindIcon(n)" class="kind" />{{ n.name }}</span>
                <span v-if="n.err" class="badge problem" :title="unreadableReason(n.err)">sin acceso</span>
                <span
                  v-for="label in flagLabels(n.flags)"
                  :key="label"
                  class="badge info"
                  :title="sizeNote(n)"
                >
                  {{ label }}
                </span>
                <span v-if="copied === n.id" class="copied" role="status">Ruta copiada</span>
                <span class="actions">
                  <button
                    type="button"
                    tabindex="-1"
                    title="Mostrar en el Explorador"
                    aria-label="Mostrar en el Explorador"
                    @click="ex.showInExplorer(n)"
                  >
                    <AppIcon name="reveal" />
                  </button>
                  <button
                    type="button"
                    tabindex="-1"
                    title="Copiar ruta"
                    aria-label="Copiar ruta"
                    @click="ex.copyPath(n)"
                  >
                    <AppIcon name="copy" />
                  </button>
                </span>
              </td>
              <td class="num">{{ formatBytes(n.diskSize) }}</td>
              <td v-if="logicalCol" class="num logical col-logical">{{ formatBytes(n.size) }}</td>
              <td class="num share">
                <span
                  :class="['bar', landUse(n)]"
                  :style="{ width: `${share(n.diskSize) * 100}%` }"
                  aria-hidden="true"
                ></span>
                <span class="pct">{{ formatPercent(n.diskSize, current?.diskSize ?? 0) }}</span>
              </td>
              <td class="num col-files">{{ formatCount(n.files) }}</td>
              <td class="num date col-date">{{ formatDate(n.modified) }}</td>
            </tr>
          </tbody>
        </table>
        <p v-else class="facts">Carpeta vacía.</p>
        <p v-if="rows.length" class="facts more">
          <span class="num">Mostrando {{ formatCount(rows.length) }} de {{ formatCount(total) }}</span>
          <button v-if="hasMore" type="button" @click="ex.loadMore">Cargar más</button>
        </p>
      </div>
    </div>

    <div
      v-if="menu"
      ref="menuEl"
      class="menu"
      role="menu"
      :aria-label="menu.node.name"
      :style="{ left: `${menu.x}px`, top: `${menu.y}px`, width: `${MENU_WIDTH}px` }"
      @click.stop
    >
      <button type="button" role="menuitem" @click="act(ex.showInExplorer)">
        <AppIcon name="reveal" />Mostrar en el Explorador
      </button>
      <button type="button" role="menuitem" @click="act(ex.copyPath)">
        <AppIcon name="copy" />Copiar ruta
      </button>
    </div>
  </main>
</template>

<style scoped>
.explorer {
  padding: var(--space-4) var(--space-4) 0;
}

/* Header: back, the folder as the page title, and the commands. */
.toolbar {
  display: flex;
  gap: var(--space-2) var(--space-4);
  align-items: center;
  flex-wrap: wrap;
}
.back {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  color: var(--water);
  text-decoration: none;
}
.back:hover {
  text-decoration: underline;
}
.heading {
  flex: 1;
  min-width: 0; /* lets a long path ellipsize instead of widening the page */
}
.title {
  font-size: var(--text-xl);
  margin: 0;
  overflow-wrap: anywhere;
}
.root {
  display: block;
  color: var(--graphite);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.commands {
  display: flex;
  gap: var(--space-2);
}

/* A line of separate facts: spacing separates them, not punctuation. */
.facts {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: var(--space-1) var(--space-4);
  min-height: 1.45em;
  margin: var(--space-2) 0;
  color: var(--graphite);
}
.phase {
  color: var(--ink);
}

.notice {
  margin: var(--space-2) 0;
  padding: var(--space-2) var(--space-3);
  background: var(--water-soft);
  border-left: 3px solid var(--water);
  border-radius: 0 var(--radius-control) var(--radius-control) 0;
}
.notice.problem {
  background: var(--boundary-soft);
  border-left-color: var(--boundary);
}
.error-text {
  color: var(--boundary);
}

.empty-state {
  margin: var(--space-6) 0;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: var(--space-3);
}
.empty-state p {
  margin: 0;
  font-size: var(--text-md);
  max-width: 60ch;
}

/* Where you are and how to find something else, on one line when it fits. */
.wayfinding {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2) var(--space-4);
  margin: var(--space-2) 0 var(--space-3);
}
.crumbs {
  flex: 1 1 200px;
  min-width: 0;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 2px var(--space-1);
}
.crumbs .link {
  color: var(--graphite);
}
.crumbs [aria-current] {
  color: var(--ink);
  font-weight: 600;
}
.sep {
  color: var(--contour);
}
.search {
  flex: 0 1 280px;
}
.search input {
  width: 100%;
}

/* The map and the table: stacked by default, side by side in a wide window,
   where the page fits the window and each pane scrolls on its own. */
.workspace {
  display: grid;
  gap: var(--space-4);
}
.map-pane {
  height: clamp(220px, 45vh, 400px);
}
.table-pane {
  container-type: inline-size;
  min-width: 0;
  padding-bottom: var(--space-4);
}
@media (min-width: 960px) {
  .explorer {
    height: 100vh;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
  }
  .workspace,
  .results {
    flex: 1;
    min-height: 0;
  }
  .workspace {
    grid-template-columns: minmax(0, 3fr) minmax(360px, 2fr);
  }
  .map-pane {
    height: auto;
    padding-bottom: var(--space-4);
  }
  .table-pane,
  .results {
    overflow: auto;
  }
  thead th {
    position: sticky;
    top: 0;
    background: var(--paper);
    z-index: 1;
  }
}

table {
  width: 100%;
  border-collapse: collapse;
}
th,
td {
  padding: 6px var(--space-2);
  border-bottom: 1px solid var(--contour);
  text-align: left;
}
th {
  font-size: var(--text-sm);
  font-weight: 600;
  color: var(--graphite);
}
th[aria-sort='ascending'],
th[aria-sort='descending'] {
  color: var(--ink);
}
th .sort {
  min-height: 0;
  font: inherit;
  color: inherit;
  background: none;
  border: none;
  padding: 0;
  cursor: pointer;
}
tbody tr:hover,
tr.hovered {
  background: var(--row-hover);
}
tr.found {
  background: var(--water-soft);
}
tbody tr:focus-visible {
  outline: 2px solid var(--water);
  outline-offset: -2px;
}
.num {
  text-align: right;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.name {
  word-break: break-all;
}
.entry {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 0;
  padding: 0;
  border: 0;
  background: none;
  color: var(--ink);
  font: inherit;
  text-align: left;
}
button.entry {
  cursor: pointer;
}
button.entry:hover {
  text-decoration: underline;
}
.kind {
  color: var(--graphite);
}
.hits .name .kind {
  margin-right: 6px;
}
.logical,
.date {
  color: var(--graphite);
}

/* The share bar takes the map colour of the entry, tying each row to its cell. */
.share {
  position: relative;
  width: 160px;
}
.bar {
  position: absolute;
  inset: 5px auto 5px 0;
  border-radius: 2px;
  background: var(--forest);
  opacity: 0.55;
}
.bar.sand {
  background: var(--sand);
}
.bar.rock {
  background: var(--rock);
}
.pct {
  position: relative;
}
.badge {
  margin-left: var(--space-2);
}
.badge.info {
  cursor: help;
}
.copied {
  margin-left: var(--space-2);
  font-size: 12px;
  color: var(--water);
}
.actions {
  visibility: hidden;
  margin-left: var(--space-2);
  white-space: nowrap;
}
tr:hover .actions,
tr:focus-within .actions {
  visibility: visible;
}
.actions button {
  min-height: 0;
  padding: 2px 4px;
  border: 0;
  background: none;
  color: var(--graphite);
}
.actions button:hover {
  color: var(--water);
}

/* Secondary columns go first as the table gets narrower. */
@container (max-width: 640px) {
  .col-logical,
  .col-files {
    display: none;
  }
}
@container (max-width: 480px) {
  .col-date {
    display: none;
  }
  .entries .share {
    width: 90px;
  }
}

.more {
  justify-content: space-between;
}
.hit {
  cursor: pointer;
}
.dir {
  color: var(--graphite);
  max-width: 0; /* lets the column shrink and ellipsize instead of widening the table */
  width: 50%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.unreadable {
  margin-bottom: var(--space-6);
  padding: var(--space-3);
  background: var(--surface);
  border: 1px solid var(--contour);
  border-radius: var(--radius-panel);
}
.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
}
.panel-head h2 {
  margin: 0;
  font-size: var(--text-md);
}

.menu {
  position: fixed;
  z-index: 10;
  display: flex;
  flex-direction: column;
  padding: var(--space-1);
  background: var(--surface);
  border: 1px solid var(--contour);
  border-radius: var(--radius-panel);
  box-shadow: 0 6px 20px rgb(0 0 0 / 0.18);
}
.menu button {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  text-align: left;
  color: var(--ink);
  background: none;
  border: none;
  padding: 6px 10px;
  border-radius: var(--radius-control);
}
.menu button:hover,
.menu button:focus-visible {
  background: var(--row-hover);
  outline: none;
}
</style>
