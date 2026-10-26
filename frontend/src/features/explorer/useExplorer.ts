import { computed, onScopeDispose, readonly, ref } from 'vue'
import type { Backend, Node, Progress, SortKey, Summary } from '../../backend/types'

/** Children loaded at a time: the UI never loads a whole directory at once. */
export const PAGE_SIZE = 200

// The first click on a column sorts it the way it is most useful.
const FIRST_DESC: Record<SortKey, boolean> = { size: true, name: false, modified: true }

/**
 * State and actions of the explorer of one folder: scan it and browse the
 * result. It holds no UI, so it can be tested without rendering components.
 * Call it inside a component's setup (or an effect scope): it unsubscribes
 * from scan events when that scope ends.
 */
export function useExplorer(backend: Backend, root: string) {
  const trail = ref<Node[]>([]) // breadcrumbs; the last one is the directory shown
  const children = ref<Node[]>([]) // biggest first, for the treemap
  const rows = ref<Node[]>([]) // the table, in the chosen order and loaded page by page
  const total = ref(0) // children of the directory shown
  const sort = ref({ key: 'size' as SortKey, desc: true })
  const progress = ref<Progress | null>(null)
  const summary = ref<Summary | null>(null)
  const scanning = ref(false)
  const saving = ref(false) // the scan is past the point where it can be cancelled
  const error = ref('')

  const current = computed(() => trail.value.at(-1) ?? null)
  const hasMore = computed(() => rows.value.length < total.value)

  const offs = [
    backend.onProgress((p) => (progress.value = p)),
    backend.onSaving(() => (saving.value = true)),
    backend.onDone((s) => {
      scanning.value = false
      saving.value = false
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

  // Each navigation or sort takes a number; a response is applied only if no
  // newer one started while it was in flight.
  let seq = 0

  const page = (dir: Node, offset: number) =>
    backend.listChildren(root, dir.id, {
      sort: sort.value.key,
      desc: sort.value.desc,
      offset,
      limit: PAGE_SIZE,
    })

  async function show(newTrail: Node[]) {
    const id = ++seq
    const dir = newTrail[newTrail.length - 1]
    const [kids, first] = await Promise.all([backend.children(root, dir.id, PAGE_SIZE), page(dir, 0)])
    if (id !== seq) return
    trail.value = newTrail
    children.value = kids
    rows.value = first.items
    total.value = first.total
  }

  async function loadRoot() {
    const r = await backend.root(root)
    if (r) return show([r])
    seq++ // drops any navigation still in flight
    trail.value = []
    children.value = []
    rows.value = []
    total.value = 0
  }

  const scan = () =>
    attempt(async () => {
      if (scanning.value) return
      progress.value = null
      summary.value = null
      scanning.value = true
      try {
        await backend.startScan(root)
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

  const up = () => (trail.value.length > 1 ? goTo(trail.value.length - 2) : Promise.resolve())

  /** Sorts the table by key; choosing the current key again flips the direction. */
  const sortBy = (key: SortKey) =>
    attempt(async () => {
      const desc = key === sort.value.key ? !sort.value.desc : FIRST_DESC[key]
      sort.value = { key, desc }
      if (trail.value.length) await show(trail.value)
    })

  let loadingMore = false

  const loadMore = () =>
    attempt(async () => {
      const dir = current.value
      if (!dir || !hasMore.value || loadingMore) return
      const id = seq
      loadingMore = true
      try {
        const next = await page(dir, rows.value.length)
        if (id !== seq) return // the user navigated or re-sorted meanwhile
        rows.value = [...rows.value, ...next.items]
        total.value = next.total
      } finally {
        loadingMore = false
      }
    })

  // Settles once the current index is shown, or the error is.
  const ready = attempt(loadRoot)

  return {
    root,
    ready,
    trail: readonly(trail),
    children: readonly(children),
    rows: readonly(rows),
    total: readonly(total),
    sort: readonly(sort),
    progress: readonly(progress),
    summary: readonly(summary),
    scanning: readonly(scanning),
    saving: readonly(saving),
    error: readonly(error),
    current,
    hasMore,
    scan,
    cancel,
    open,
    goTo,
    up,
    sortBy,
    loadMore,
  }
}
