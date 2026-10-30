import { onScopeDispose, readonly, ref } from 'vue'
import { errorMessage } from '../../backend/errors'
import type { Backend, Hit } from '../../backend/types'

const REASONS: Record<string, string> = {
  access_denied: 'Sin permiso de lectura',
  not_found: 'Desapareció durante el análisis',
}

/** Why an entry could not be read, from its Node.err code (model.Err*). */
export const unreadableReason = (err = '') => REASONS[err] ?? 'Error de lectura'

/** Entries listed at most in the partial coverage panel. */
export const UNREADABLE_LIMIT = 200

/**
 * The partial coverage panel: what the last scan of root could not read. The
 * list is loaded when the panel opens, not before, since few users open it.
 * Like useExplorer, call it inside a component's setup or an effect scope.
 */
export function useUnreadable(backend: Backend, root: string) {
  const open = ref(false)
  const hits = ref<Hit[]>([])
  const more = ref(false)
  const loading = ref(false)
  const error = ref('')

  let seq = 0 // a reload after a scan replaces a load still in flight

  async function load() {
    const id = ++seq
    loading.value = true
    error.value = ''
    try {
      const res = await backend.unreadable(root, UNREADABLE_LIMIT)
      if (id !== seq) return
      hits.value = res.hits
      more.value = res.more
    } catch (e) {
      if (id !== seq) return
      hits.value = []
      more.value = false
      error.value = errorMessage(e)
    } finally {
      if (id === seq) loading.value = false
    }
  }

  async function toggle() {
    open.value = !open.value
    if (open.value) await load()
  }

  // A completed scan replaces the index, and with it the list.
  const off = backend.onDone((s) => {
    if (s.status === 'completed' && open.value) void load()
  })
  onScopeDispose(off)

  return {
    open: readonly(open),
    hits: readonly(hits),
    more: readonly(more),
    loading: readonly(loading),
    error: readonly(error),
    toggle,
  }
}
