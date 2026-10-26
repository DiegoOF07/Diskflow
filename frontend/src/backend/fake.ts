import type { Backend, Node, Progress, RootInfo, Summary } from './types'

/**
 * In-memory Backend for tests. children() answers immediately unless the
 * parent is held with hold(), which lets a test decide the order of responses.
 */
export class FakeBackend implements Backend {
  picked: string | null = null // folder the dialog returns
  catalog: RootInfo[] = []
  failRemove: string | null = null
  roots = new Map<string, Node>()
  kids = new Map<number, Node[]>()
  failStart: string | null = null
  failChildren: string | null = null
  started: string[] = []
  cancelled = 0

  private progressCbs = new Set<(p: Progress) => void>()
  private doneCbs = new Set<(s: Summary) => void>()
  private held = new Map<number, () => void>()
  private holding = new Set<number>()

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
    if (this.failRemove) throw this.failRemove
    await this.getRoot(id)
    this.catalog = this.catalog.filter((c) => c.id !== id)
  }
  async startScan(root: string) {
    if (this.failStart) throw this.failStart // Wails rejects with the Go error string
    this.started.push(root)
  }
  async cancelScan() {
    this.cancelled++
  }
  async root(root: string) {
    return this.roots.get(root) ?? null
  }
  async children(_root: string, parentId: number, limit: number) {
    if (this.holding.has(parentId)) {
      await new Promise<void>((resolve) => this.held.set(parentId, resolve))
    }
    if (this.failChildren) throw this.failChildren
    return (this.kids.get(parentId) ?? []).slice(0, limit)
  }
  onProgress(cb: (p: Progress) => void) {
    this.progressCbs.add(cb)
    return () => this.progressCbs.delete(cb)
  }
  onDone(cb: (s: Summary) => void) {
    this.doneCbs.add(cb)
    return () => this.doneCbs.delete(cb)
  }

  hold(parentId: number) {
    this.holding.add(parentId)
  }
  release(parentId: number) {
    this.holding.delete(parentId)
    this.held.get(parentId)?.()
  }
  emitProgress(p: Progress) {
    this.progressCbs.forEach((cb) => cb(p))
  }
  emitDone(s: Summary) {
    this.doneCbs.forEach((cb) => cb(s))
  }
  get listeners() {
    return this.progressCbs.size + this.doneCbs.size
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
})
export const file = (id: number, name: string, size = 0, diskSize = size, flags = 0): Node => ({
  id,
  name,
  kind: 'file',
  size,
  diskSize,
  flags,
  files: 1,
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
