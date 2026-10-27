import { onScopeDispose, readonly, ref, watch } from 'vue'
import type { Backend, Hit } from '../../backend/types'

/** Hits shown at most; beyond that the user is asked to refine the search. */
export const SEARCH_LIMIT = 200
/** Typing pause, in ms, before a search starts. */
export const SEARCH_DELAY = 250

/**
 * Searches the current index of one folder by name while the user types. Like
 * useExplorer, call it inside a component's setup or an effect scope.
 */
export function useSearch(backend: Backend, root: string) {
  const query = ref('')
  const hits = ref<Hit[]>([])
  const more = ref(false)
  const searching = ref(false) // waiting for the typing pause or for the backend
  const error = ref('')

  // Every keystroke takes a number; only the response of the latest one is applied.
  let seq = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  let inFlight = 0 // requests sent and not answered yet

  // A search left running would hold the database and delay other screens.
  function stop() {
    if (inFlight) void backend.cancelSearch()
  }

  function schedule(delay: number) {
    clearTimeout(timer)
    const id = ++seq
    const text = query.value.trim()
    if (!text) {
      stop()
      hits.value = []
      more.value = false
      searching.value = false
      error.value = ''
      return
    }
    searching.value = true
    timer = setTimeout(() => void run(id, text), delay)
  }

  async function run(id: number, text: string) {
    inFlight++
    try {
      const res = await backend.search(root, text, SEARCH_LIMIT)
      if (id !== seq) return
      hits.value = res.hits
      more.value = res.more
      error.value = ''
    } catch (e) {
      if (id !== seq) return // a newer search cancelled this one in the backend
      hits.value = []
      more.value = false
      error.value = e instanceof Error ? e.message : String(e)
    } finally {
      inFlight--
      if (id === seq) searching.value = false
    }
  }

  watch(query, () => schedule(SEARCH_DELAY))

  // Hit ids belong to one index: a completed scan makes them stale.
  const off = backend.onDone((s) => {
    if (s.status === 'completed' && query.value.trim()) schedule(0)
  })
  onScopeDispose(() => {
    clearTimeout(timer)
    stop()
    off()
  })

  const clear = () => (query.value = '')

  return {
    query,
    hits: readonly(hits),
    more: readonly(more),
    searching: readonly(searching),
    error: readonly(error),
    clear,
  }
}
