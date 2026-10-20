import { computed, onScopeDispose, readonly, ref } from 'vue'
import type { Backend, Node, Progress, Summary } from '../../backend/types'

/** Children shown per directory: the UI never loads a whole directory at once. */
export const PAGE_SIZE = 200

/**
 * State and actions of the explorer: pick a folder, scan it and browse the
 * result. It holds no UI, so it can be tested without rendering components.
 * Call it inside a component's setup (or an effect scope): it unsubscribes
 * from scan events when that scope ends.
 */
export function useExplorer(backend: Backend) {
  const root = ref<string | null>(null)
  const trail = ref<Node[]>([]) // breadcrumbs; the last one is the directory shown
  const children = ref<Node[]>([])
  const progress = ref<Progress | null>(null)
  const summary = ref<Summary | null>(null)
  const scanning = ref(false)
  const error = ref('')

  const current = computed(() => trail.value.at(-1) ?? null)
  const truncated = computed(() => children.value.length >= PAGE_SIZE)

  const offs = [
    backend.onProgress((p) => (progress.value = p)),
    backend.onDone((s) => {
      scanning.value = false
      summary.value = s
      // Only a completed scan replaces the index; otherwise the view stays valid.
      if (s.status === 'completed') void attempt(loadRoot)
    }),
  ]
  onScopeDispose(() => offs.forEach((off) => off()))

  async function attempt(fn: () => Promise<void>) {
    error.value = ''
    try {
      await fn()
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e) // Wails rejects with the Go error string
    }
  }

  // Each navigation takes a number; a response is applied only if no newer
  // navigation started while it was in flight.
  let seq = 0

  async function show(newTrail: Node[]) {
    const id = ++seq
    const dir = newTrail[newTrail.length - 1]
    const kids = await backend.children(root.value!, dir.id, PAGE_SIZE)
    if (id !== seq) return
    trail.value = newTrail
    children.value = kids
  }

  async function loadRoot() {
    const r = await backend.root(root.value!)
    if (r) return show([r])
    seq++ // drops any navigation still in flight
    trail.value = []
    children.value = []
  }

  const pickFolder = () =>
    attempt(async () => {
      if (scanning.value) return
      const picked = await backend.pickFolder()
      if (!picked) return
      root.value = picked
      progress.value = null
      summary.value = null
      trail.value = []
      children.value = []
      await loadRoot()
    })

  const scan = () =>
    attempt(async () => {
      if (!root.value || scanning.value) return
      progress.value = null
      summary.value = null
      scanning.value = true
      try {
        await backend.startScan(root.value)
      } catch (e) {
        scanning.value = false
        throw e
      }
    })

  const cancel = () => attempt(() => backend.cancelScan())

  const open = (node: Node) =>
    attempt(async () => {
      if (node.kind === 'dir') await show([...trail.value, node])
    })

  const goTo = (index: number) => attempt(() => show(trail.value.slice(0, index + 1)))

  return {
    root: readonly(root),
    trail: readonly(trail),
    children: readonly(children),
    progress: readonly(progress),
    summary: readonly(summary),
    scanning: readonly(scanning),
    error: readonly(error),
    current,
    truncated,
    pickFolder,
    scan,
    cancel,
    open,
    goTo,
  }
}
