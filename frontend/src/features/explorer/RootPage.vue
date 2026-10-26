<script setup lang="ts">
import { ref, watchEffect } from 'vue'
import { useRouter } from 'vue-router'
import { useBackend } from '../../backend/inject'
import type { RootInfo } from '../../backend/types'
import ExplorerView from './ExplorerView.vue'

// The route only carries the catalog id; this page resolves it to a folder.
const props = defineProps<{ id: number; scan: boolean }>()
const backend = useBackend()
const router = useRouter()
const info = ref<RootInfo | null>(null)
const error = ref('')

watchEffect(async () => {
  const id = props.id
  info.value = null
  error.value = ''
  try {
    const r = await backend.getRoot(id)
    if (id === props.id) info.value = r
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  }
})

// "Analizar" from the home screen arrives as ?scan=1. Drop it from the URL,
// so going back to this page later doesn't start another scan.
const autoscan = props.scan
if (props.scan) void router.replace({ query: {} })
</script>

<template>
  <ExplorerView v-if="info" :key="info.id" :root="info.path" :name="info.name" :autoscan="autoscan" />
  <main v-else-if="error" class="missing">
    <p role="alert">Esta carpeta ya no está en el catálogo.</p>
    <RouterLink to="/">Volver al inicio</RouterLink>
  </main>
</template>

<style scoped>
.missing {
  padding: 16px 20px;
}
.missing a {
  color: var(--accent);
}
</style>
