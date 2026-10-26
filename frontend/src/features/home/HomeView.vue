<script setup lang="ts">
import { nextTick, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useBackend } from '../../backend/inject'
import type { RootInfo } from '../../backend/types'
import { formatBytes } from '../explorer/format'
import { rootStatus } from './status'
import { useRoots } from './useRoots'

const { roots, loaded, error, load, add, rename, remove } = useRoots(useBackend())
const router = useRouter()
void load()

// Only one row is edited or asked for confirmation at a time.
const renaming = ref<number | null>(null)
const draft = ref('')
const confirming = ref<number | null>(null)
const nameInput = ref<HTMLInputElement[]>([])

const open = (r: RootInfo) => router.push({ name: 'explorer', params: { id: r.id } })
const scan = (r: RootInfo) => router.push({ name: 'explorer', params: { id: r.id }, query: { scan: '1' } })

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

async function confirmRemove(id: number) {
  confirming.value = null
  await remove(id)
}
</script>

<template>
  <main class="home">
    <header class="toolbar">
      <h1>DiskFlow</h1>
      <button type="button" class="primary" @click="addFolder">Agregar carpeta</button>
    </header>

    <p v-if="error" class="error" role="alert">{{ error }}</p>

    <div v-if="loaded && !roots.length" class="first-run">
      <p>Agrega una carpeta para ver en qué se va su espacio.</p>
      <p class="hint">DiskFlow solo lee: nunca borra, mueve ni abre tus archivos.</p>
      <button type="button" class="primary" @click="addFolder">Agregar carpeta</button>
    </div>

    <ul v-else class="roots">
      <li v-for="r in roots" :key="r.id" class="root">
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
          <span class="status">{{ rootStatus(r) }}</span>
        </div>

        <span class="size" :title="r.index ? `${formatBytes(r.index.bytes)} lógicos` : undefined">
          {{ r.index ? formatBytes(r.index.diskBytes) : '—' }}
        </span>

        <div v-if="confirming === r.id" class="actions confirm" role="group" aria-label="Confirmar">
          <span>¿Quitar del catálogo? Se borra su análisis, no sus archivos.</span>
          <button type="button" class="danger" @click="confirmRemove(r.id)">Quitar</button>
          <button type="button" @click="confirming = null">No</button>
        </div>
        <div v-else class="actions">
          <button type="button" @click="open(r)">Abrir</button>
          <button type="button" @click="scan(r)">Analizar</button>
          <button type="button" @click="startRename(r)">Renombrar</button>
          <button type="button" @click="confirming = r.id">Quitar</button>
        </div>
      </li>
    </ul>
  </main>
</template>

<style scoped>
.home {
  padding: 16px 20px;
  max-width: 1100px;
}
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
h1 {
  font-size: 20px;
  margin: 0;
}
.error {
  background: var(--danger-bg);
  color: var(--danger);
  padding: 8px 12px;
  border-radius: 6px;
}
.first-run {
  margin: 64px auto;
  text-align: center;
}
.hint {
  color: var(--muted);
}
.roots {
  list-style: none;
  padding: 0;
  margin: 16px 0;
}
.root {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 12px 8px;
  border-bottom: 1px solid var(--border);
}
.info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}
.name {
  font-size: 15px;
  font-weight: 600;
}
.rename {
  font: inherit;
  font-size: 15px;
  padding: 2px 4px;
  max-width: 320px;
}
.path {
  color: var(--muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.status {
  color: var(--muted);
  font-size: 12px;
}
.size {
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  min-width: 80px;
  text-align: right;
}
.actions {
  display: flex;
  gap: 6px;
  align-items: center;
}
.confirm span {
  color: var(--muted);
  font-size: 12px;
  max-width: 220px;
}
button.danger {
  border-color: var(--danger);
  color: var(--danger);
}
</style>
