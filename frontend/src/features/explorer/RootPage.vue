<script setup lang="ts">
import { onScopeDispose, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { errorCode, errorMessage } from '../../backend/errors'
import { useBackend } from '../../backend/inject'
import type { RootInfo } from '../../backend/types'
import ExplorerView from './ExplorerView.vue'

// The route only carries the catalog id; this page resolves it to a folder.
const props = defineProps<{ id: number; scan: boolean }>()
const backend = useBackend()
const router = useRouter()
const info = ref<RootInfo | null>(null)
const error = ref('')
const gone = ref(false) // removed from the catalog meanwhile

async function load(id: number) {
  try {
    const r = await backend.getRoot(id)
    if (id === props.id) info.value = r
  } catch (e) {
    if (id !== props.id) return
    error.value = errorMessage(e)
    gone.value = errorCode(e) === 'not_found'
  }
}

watch(
  () => props.id,
  (id) => {
    info.value = null
    error.value = ''
    gone.value = false
    void load(id)
  },
  { immediate: true },
)

// A finished scan changes the folder's index and last attempt.
const off = backend.onDone(() => void load(props.id))
onScopeDispose(off)

// "Analizar" from the home screen arrives as ?scan=1. Drop it from the URL,
// so going back to this page later doesn't start another scan.
const autoscan = props.scan
if (props.scan) void router.replace({ query: {} })
</script>

<template>
  <ExplorerView v-if="info" :key="info.id" :info="info" :autoscan="autoscan" />
  <main v-else-if="error" class="missing">
    <p role="alert">{{ gone ? 'Esta carpeta ya no está en el catálogo.' : error }}</p>
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
