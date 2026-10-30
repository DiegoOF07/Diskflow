import type {
  Backend,
  Hit,
  ListQuery,
  Node,
  Page,
  Progress,
  RootInfo,
  ScanState,
  SearchResult,
  Summary,
} from './types'

/**
 * In-memory Backend for tests. children() and listChildren() answer immediately
 * unless the parent is held with hold(), and search() unless the query is held:
 * that lets a test decide the order of responses.
 */
export class FakeBackend implements Backend {
  picked: string | null = null // folder the dialog returns
  catalog: RootInfo[] = []
  failRemove: string | null = null
  removeCalls = 0
  scanStates = new Map<string, ScanState>()
  roots = new Map<string, Node>()
  kids = new Map<number, Node[]>()
  failStart: string | null = null
  failChildren: string | null = null
  failSearch: string | null = null
  searches: string[] = []
  started: string[] = []
  cancelled = 0
  searchesCancelled = 0
  shown: number[] = [] // ids passed to showInExplorer
  copied: number[] = [] // ids passed to copyPath
  failAction: string | null = null

  private progressCbs = new Set<(p: Progress) => void>()
  private savingCbs = new Set<() => void>()
  private doneCbs = new Set<(s: Summary) => void>()
  private held = new Map<number | string, (() => void)[]>() // several requests can wait on one key
  private holding = new Set<number | string>()

  async pickFolder() {
    if (!this.picked) return null
    const path = this.picked
    let r = this.catalog.find((c) => c.path === path)
    if (!r) {
      r = rootInfo(Math.max(0, ...this.catalog.map((c) => c.id)) + 1, path)
      this.catalog.push(r)
    }
    return { ...r }
  }
  async listRoots() {
    return this.catalog.map((r) => ({ ...r }))
  }
  async getRoot(id: number) {
    const r = this.catalog.find((c) => c.id === id)
    if (!r) throw 'engine: folder not in the catalog'
    return { ...r }
  }
  async renameRoot(id: number, name: string) {
    const r = await this.getRoot(id)
    this.catalog = this.catalog.map((c) =>
      c.id === id ? { ...r, name: name.trim() || baseName(r.path) } : c,
    )
  }
  async removeRoot(id: number) {
    this.removeCalls++
    await this.gate(`remove:${id}`)
    if (this.failRemove) throw this.failRemove
    this.catalog = this.catalog.filter((c) => c.id !== id) // already gone is fine, like the engine
  }
  async startScan(root: string) {
    if (this.failStart) throw this.failStart // Wails rejects with the Go error string
    this.started.push(root)
  }
  async cancelScan() {
    this.cancelled++
  }
  async scanState(root: string) {
    return this.scanStates.get(root) ?? ''
  }
  async root(root: string) {
    return this.roots.get(root) ?? null
  }
  async children(_root: string, parentId: number, limit: number) {
    const kids = await this.kidsOf(parentId)
    return [...kids].sort((a, b) => b.diskSize - a.diskSize || a.id - b.id).slice(0, limit)
  }
  async listChildren(_root: string, parentId: number, q: ListQuery): Promise<Page> {
    const kids = await this.kidsOf(parentId)
    const key = {
      size: (n: Node) => n.diskSize,
      name: (n: Node) => n.name.toLowerCase(),
      modified: (n: Node) => n.modified,
    }[q.sort]
    const dir = q.desc ? -1 : 1
    const sorted = [...kids].sort((a, b) => {
      const ka = key(a)
      const kb = key(b)
      return (ka < kb ? -dir : ka > kb ? dir : 0) || a.id - b.id // ties by id, like the store
    })
    return { items: sorted.slice(q.offset, q.offset + q.limit), total: kids.length }
  }
  async search(root: string, q: string, limit: number): Promise<SearchResult> {
    this.searches.push(q)
    await this.gate(q)
    if (this.failSearch) throw this.failSearch
    const needle = q.toLowerCase()
    const hits: Hit[] = []
    for (const [parentId, kids] of this.kids) {
      for (const n of kids) {
        if (!n.name.toLowerCase().includes(needle)) continue
        const dir = (await this.trail(root, parentId)).map((p) => p.name).join('\\')
        hits.push({ ...n, parentId, dir })
      }
    }
    hits.sort((a, b) => b.diskSize - a.diskSize || a.id - b.id)
    return { hits: hits.slice(0, limit), more: hits.length > limit }
  }
  async cancelSearch() {
    this.searchesCancelled++
  }
  async showInExplorer(_root: string, id: number) {
    if (this.failAction) throw this.failAction
    this.shown.push(id)
  }
  async copyPath(_root: string, id: number) {
    if (this.failAction) throw this.failAction
    this.copied.push(id)
  }
  async trail(root: string, id: number) {
    const top = this.roots.get(root)
    const out: Node[] = []
    for (let cur = id; cur !== top?.id;) {
      const parent = [...this.kids].find(([, kids]) => kids.some((n) => n.id === cur))
      if (!parent) throw 'store: not found'
      out.unshift(parent[1].find((n) => n.id === cur)!)
      cur = parent[0]
    }
    return [top!, ...out]
  }
  private async kidsOf(parentId: number) {
    await this.gate(parentId)
    if (this.failChildren) throw this.failChildren
    return this.kids.get(parentId) ?? []
  }
  private async gate(key: number | string) {
    if (!this.holding.has(key)) return
    await new Promise<void>((resolve) => this.held.set(key, [...(this.held.get(key) ?? []), resolve]))
  }
  onProgress(cb: (p: Progress) => void) {
    this.progressCbs.add(cb)
    return () => this.progressCbs.delete(cb)
  }
  onSaving(cb: () => void) {
    this.savingCbs.add(cb)
    return () => this.savingCbs.delete(cb)
  }
  onDone(cb: (s: Summary) => void) {
    this.doneCbs.add(cb)
    return () => this.doneCbs.delete(cb)
  }

  /** Holds the requests for a parent id, the searches for a query, or `remove:<id>`. */
  hold(key: number | string) {
    this.holding.add(key)
  }
  release(key: number | string) {
    this.holding.delete(key)
    this.held.get(key)?.forEach((resolve) => resolve())
    this.held.delete(key)
  }
  emitProgress(p: Progress) {
    this.progressCbs.forEach((cb) => cb(p))
  }
  emitSaving() {
    this.savingCbs.forEach((cb) => cb())
  }
  emitDone(s: Summary) {
    this.doneCbs.forEach((cb) => cb(s))
  }
  get listeners() {
    return this.progressCbs.size + this.savingCbs.size + this.doneCbs.size
  }
}

// diskSize defaults to size: most tests don't care about the difference.
export const dir = (id: number, name: string, size = 0, diskSize = size, flags = 0): Node => ({
  id,
  name,
  kind: 'dir',
  size,
  diskSize,
  flags,
  files: 0,
  modified: 0,
})
export const file = (id: number, name: string, size = 0, diskSize = size, flags = 0): Node => ({
  id,
  name,
  kind: 'file',
  size,
  diskSize,
  flags,
  files: 1,
  modified: 0,
})

const baseName = (path: string) => path.split(/[\\/]/).filter(Boolean).at(-1) ?? path

/** A catalog entry that was never scanned. */
export const rootInfo = (id: number, path: string, name = baseName(path)): RootInfo => ({
  id,
  name,
  path,
  createdAt: 0,
  index: null,
  last: null,
})
