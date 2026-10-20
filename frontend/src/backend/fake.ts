import type { Backend, Node, Progress, Summary } from './types'

/**
 * In-memory Backend for tests. children() answers immediately unless the
 * parent is held with hold(), which lets a test decide the order of responses.
 */
export class FakeBackend implements Backend {
  picked: string | null = null
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
    return this.picked
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

export const dir = (id: number, name: string, size = 0): Node => ({ id, name, kind: 'dir', size, files: 0 })
export const file = (id: number, name: string, size = 0): Node => ({ id, name, kind: 'file', size, files: 1 })
