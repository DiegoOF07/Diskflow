<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useTemplateRef } from 'vue'
import { useRouter } from 'vue-router'
import { useBackend } from '../../backend/inject'
import type { RootInfo } from '../../backend/types'
import AppIcon from '../../components/AppIcon.vue'
import { onMenuKey } from '../../components/menuKeys'
import { formatBytes, formatPercent } from '../explorer/format'
import ContourMotif from './ContourMotif.vue'
import { scaleOf } from './scale'
import { rootStatus } from './status'
import { useRoots } from './useRoots'

const { roots, loaded, loading, adding, removing, error, load, add, rename, remove } = useRoots(useBackend())
const router = useRouter()
void load()

const scale = computed(() => scaleOf(roots.value))
const largest = computed(() => roots.value.find((r) => scale.value.get(r.id) === 1))

// Only one row is edited or asked for confirmation at a time.
const renaming = ref<number | null>(null)
const draft = ref('')
const confirming = ref<number | null>(null)
const nameInput = ref<HTMLInputElement[]>([])

const open = (r: RootInfo) => router.push({ name: 'explorer', params: { id: r.id } })
const scan = (r: RootInfo) => router.push({ name: 'explorer', params: { id: r.id }, query: { scan: '1' } })
const running = (r: RootInfo) => r.last?.status === 'running'

async function addFolder() {
  const r = await add()
  if (r) await open(r)
}

async function startRename(r: RootInfo) {
  confirming.value = null
  renaming.value = r.id
  draft.value = r.name
  await nextTick()
  nameInput.value[0]?.select()
}

async function saveRename(id: number) {
  if (renaming.value !== id) return // Enter already saved; this is the blur that follows
  renaming.value = null
  await rename(id, draft.value)
}

// The confirmation replaces the row's buttons, so it takes the focus.
async function askRemove(r: RootInfo) {
  confirming.value = r.id
  await nextTick()
  document.querySelector<HTMLElement>('.confirm button')?.focus()
}

// The confirmation stays, showing "Quitando…", until the folder is gone.
async function confirmRemove(id: number) {
  await remove(id)
  if (confirming.value === id) confirming.value = null
}

// Renaming and removing are rare, so they wait in a menu and a row fits a narrow window.
const MENU_WIDTH = 180
const menu = ref<{ root: RootInfo; x: number; y: number; trigger: HTMLElement } | null>(null)
const menuEl = useTemplateRef<HTMLElement>('menuEl')
async function openMenu(e: MouseEvent, root: RootInfo) {
  const trigger = e.currentTarget as HTMLElement
  const r = trigger.getBoundingClientRect()
  menu.value = { root, x: Math.max(8, r.right - MENU_WIDTH), y: r.bottom + 4, trigger }
  await nextTick()
  menuEl.value?.querySelector('button')?.focus()
}
function closeMenu() {
  menu.value = null
}
function act(fn: (r: RootInfo) => unknown) {
  const root = menu.value?.root
  closeMenu()
  if (root) void fn(root)
}
// Closes the menu from the keyboard, giving the focus back to its button.
function dismiss() {
  const trigger = menu.value?.trigger
  closeMenu()
  trigger?.focus()
}
function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape' && menu.value) dismiss()
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
  <main class="home">
    <header class="toolbar">
      <h1>DiskFlow</h1>
      <button type="button" class="primary" :disabled="!loaded || adding" @click="addFolder">
        Agregar carpeta
      </button>
    </header>

    <p v-if="!loaded && loading" class="loading" aria-live="polite">Cargando carpetas…</p>
    <p v-else-if="!loaded" class="notice problem" role="alert">
      <strong>No se pudo cargar la lista de carpetas.</strong> {{ error }}
      <button type="button" @click="load">Reintentar</button>
    </p>
    <p v-else-if="error" class="notice problem" role="alert">{{ error }}</p>

    <section v-if="loaded && !roots.length" class="first-run">
      <ContourMotif />
      <h2>Agrega una carpeta para ver qué ocupa su espacio</h2>
      <p>
        DiskFlow la recorre y dibuja un mapa de sus tamaños. Solo lee: nunca borra, mueve ni abre tus
        archivos.
      </p>
      <button type="button" class="primary" :disabled="adding" @click="addFolder">Agregar carpeta</button>
    </section>

    <p v-if="loaded && largest && scale.size > 1" class="scale-key">
      Cada barra compara el tamaño en disco de la carpeta con el de <strong>{{ largest.name }}</strong
      >, la más grande.
    </p>
    <ul v-if="loaded && roots.length" class="roots">
      <li v-for="r in roots" :key="r.id" :class="['root', { unavailable: !r.available }]">
        <div class="info">
          <input
            v-if="renaming === r.id"
            ref="nameInput"
            v-model="draft"
            class="rename"
            aria-label="Nombre de la carpeta"
            @keydown.enter="saveRename(r.id)"
            @keydown.esc="renaming = null"
            @blur="saveRename(r.id)"
          />
          <button v-else type="button" class="link name" @click="open(r)">{{ r.name }}</button>
          <code class="path" :title="r.path">{{ r.path }}</code>
        </div>

        <span
          :class="['size', 'num', { missing: !r.index }]"
          :title="r.index ? `${formatBytes(r.index.bytes)} lógicos` : undefined"
        >
          {{ r.index ? formatBytes(r.index.diskBytes) : 'Sin datos' }}
        </span>

        <!-- Sizes are in the text; the bar only compares them at a glance. -->
        <div
          v-if="scale.has(r.id)"
          class="scale"
          aria-hidden="true"
          :title="
            largest && largest.id !== r.id
              ? `${formatPercent(r.index!.diskBytes, largest.index!.diskBytes)} de ${largest.name}`
              : undefined
          "
        >
          <span class="fill" :style="{ width: `${(scale.get(r.id) ?? 0) * 100}%` }"></span>
        </div>

        <p class="status">
          <span
            v-if="!r.available"
            class="badge problem"
            title="No se encuentra la carpeta: puede que se haya movido o borrado, o que su unidad esté desconectada. Si ya no existe, quítala."
            >No disponible</span
          >
          <span :class="{ 'problem-text': rootStatus(r).problem && !rootStatus(r).note }">{{
            rootStatus(r).text
          }}</span>
          <span v-if="rootStatus(r).note" :class="['badge', { problem: rootStatus(r).problem }]">{{
            rootStatus(r).note
          }}</span>
        </p>

        <div v-if="confirming === r.id" class="actions confirm" role="group" aria-label="Confirmar">
          <span>¿Quitar del catálogo? Se borra su análisis, no sus archivos.</span>
          <button
            type="button"
            class="danger"
            :disabled="removing.includes(r.id)"
            @click="confirmRemove(r.id)"
          >
            {{ removing.includes(r.id) ? 'Quitando…' : 'Quitar' }}
          </button>
          <button type="button" @click="confirming = null">No</button>
        </div>
        <div v-else class="actions">
          <button type="button" @click="open(r)">Abrir</button>
          <button
            type="button"
            :disabled="!r.available || running(r)"
            :title="!r.available ? 'La carpeta no está disponible' : undefined"
            @click="scan(r)"
          >
            {{ running(r) ? 'Analizando…' : 'Analizar' }}
          </button>
          <button
            type="button"
            class="more"
            aria-haspopup="menu"
            :aria-expanded="menu?.root.id === r.id"
            :aria-label="`Más acciones para ${r.name}`"
            title="Más acciones"
            @click.stop="openMenu($event, r)"
          >
            <AppIcon name="more" />
          </button>
        </div>
      </li>
    </ul>

    <div
      v-if="menu"
      ref="menuEl"
      class="menu"
      role="menu"
      :aria-label="menu.root.name"
      :style="{ left: `${menu.x}px`, top: `${menu.y}px`, width: `${MENU_WIDTH}px` }"
      @click.stop
      @keydown="onMenuKey($event, dismiss)"
    >
      <button type="button" role="menuitem" @click="act(startRename)">
        <AppIcon name="rename" />Renombrar
      </button>
      <button type="button" role="menuitem" @click="act(askRemove)">
        <AppIcon name="remove" />Quitar del catálogo
      </button>
    </div>
  </main>
</template>

<style scoped>
/* Grows with the window up to a width where a row still reads as one line. */
.home {
  padding: var(--space-4);
  max-width: 1600px;
  margin: 0 auto;
}
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
}
h1 {
  font-size: var(--text-lg);
  margin: 0;
}
.loading {
  color: var(--graphite);
}
.notice button {
  margin-left: var(--space-2);
}

.first-run {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-3);
  margin: var(--space-6) auto;
  max-width: 460px;
  text-align: center;
}
.first-run h2 {
  margin: var(--space-2) 0 0;
  font-size: var(--text-lg);
  text-wrap: balance;
}
.first-run p {
  margin: 0 0 var(--space-2);
  color: var(--graphite);
}

.scale-key {
  margin: var(--space-4) 0 0;
  color: var(--graphite);
  font-size: var(--text-sm);
}
.scale-key strong {
  font-weight: 600;
  color: var(--ink);
}
/* The columns belong to the list and each row is a subgrid, so the bars and
   sizes of every folder line up whatever the length of its buttons. */
.roots {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  list-style: none;
  padding: 0;
  margin: var(--space-2) 0 var(--space-4);
}
/* Narrow: name and size on top, the scale bar across, then status and actions.
   Wide: one line per folder, and the bar takes the room the window gives. */
.root {
  grid-column: 1 / -1;
  display: grid;
  grid-template-columns: subgrid;
  grid-template-areas:
    'info size'
    'scale scale'
    'status actions';
  align-items: center;
  gap: var(--space-1) var(--space-4);
  padding: var(--space-3) var(--space-2);
  border-bottom: 1px solid var(--contour);
}
.info {
  grid-area: info;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.name {
  font-size: var(--text-md);
  font-weight: 600;
  color: var(--ink);
}
.rename {
  font-size: var(--text-md);
  max-width: 320px;
}
.path {
  color: var(--graphite);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.size {
  grid-area: size;
  align-self: start;
  justify-self: end;
  font-size: var(--text-md);
  font-weight: 600;
  white-space: nowrap;
}
.size.missing {
  font-weight: 400;
  color: var(--graphite);
}
.unavailable .name,
.unavailable .size {
  color: var(--graphite);
}

/* A map scale bar: the track is the largest folder, split in tenths by gaps
   that cut through the fill too, so they read on both. */
.scale {
  grid-area: scale;
  position: relative;
  height: 10px;
  margin: var(--space-1) 0;
  background: var(--contour);
  border-radius: 2px;
  overflow: hidden;
}
.scale::after {
  content: '';
  position: absolute;
  inset: 0;
  background: repeating-linear-gradient(
    to right,
    transparent 0 calc(10% - 2px),
    var(--paper) calc(10% - 2px) 10%
  );
}
.fill {
  position: absolute;
  inset: 0 auto 0 0;
  min-width: 2px;
  background: var(--forest-strong);
}

.status {
  grid-area: status;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-1) var(--space-2);
  margin: 0;
  color: var(--graphite);
  font-size: var(--text-sm);
}
.problem-text {
  color: var(--boundary);
}
.badge {
  cursor: default;
}
.badge[title] {
  cursor: help;
}
.actions {
  grid-area: actions;
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 6px;
  align-items: center;
}
.more {
  padding: var(--space-1) 6px;
  display: inline-flex;
  align-items: center;
}
.confirm {
  grid-area: auto;
  grid-column: 1 / -1;
}

@media (min-width: 1100px) {
  .roots {
    grid-template-columns: minmax(260px, 2fr) minmax(200px, 3fr) auto auto;
    column-gap: var(--space-6);
  }
  .root {
    grid-template-areas:
      'info scale size actions'
      'status scale size actions';
  }
  .scale,
  .size {
    align-self: center;
  }
  .scale {
    margin: 0;
  }
}
.confirm span {
  color: var(--graphite);
  font-size: var(--text-sm);
  margin-right: auto;
}
</style>
