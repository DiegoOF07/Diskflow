import { effectScope } from 'vue'
import { beforeEach, describe, expect, it } from 'vitest'
import { dir, FakeBackend, file } from '../../backend/fake'
import type { Summary } from '../../backend/types'
import { PAGE_SIZE, useExplorer } from './useExplorer'

const ROOT = 'C:\\work'
const flush = () => new Promise((r) => setTimeout(r, 0))
const summary = (status: Summary['status']): Summary => ({
  scanId: 1,
  status,
  elapsedMs: 10,
  files: 2,
  dirs: 1,
  bytes: 300,
  diskBytes: 8192,
  errors: 0,
})

let backend: FakeBackend
let scope: ReturnType<typeof effectScope>

// useExplorer registers cleanups with onScopeDispose, so it runs inside an effect scope.
function setup() {
  scope = effectScope()
  return scope.run(() => useExplorer(backend, ROOT))!
}

beforeEach(() => {
  backend = new FakeBackend()
  backend.roots.set(ROOT, dir(1, ROOT, 300))
  backend.kids.set(1, [dir(2, 'src', 200), file(3, 'a.txt', 100)])
  backend.kids.set(2, [file(4, 'main.go', 200)])
})

describe('loading', () => {
  it('shows the current index of its folder', async () => {
    const ex = setup()
    await ex.ready
    expect(ex.root).toBe(ROOT)
    expect(ex.trail.value.map((n) => n.name)).toEqual([ROOT])
    expect(ex.children.value.map((n) => n.name)).toEqual(['src', 'a.txt'])
  })

  it('accepts a folder that was never scanned', async () => {
    backend.roots.clear()
    const ex = setup()
    await ex.ready
    expect(ex.trail.value).toEqual([])
    expect(ex.error.value).toBe('')
  })
})

describe('scan', () => {
  it('tracks progress and reloads the index when it completes', async () => {
    backend.roots.clear()
    const ex = setup()
    await ex.ready
    await ex.scan()
    expect(backend.started).toEqual([ROOT])
    expect(ex.scanning.value).toBe(true)

    backend.emitProgress({ files: 1, dirs: 1, bytes: 100, diskBytes: 4096, errors: 0 })
    expect(ex.progress.value?.files).toBe(1)
    expect(ex.saving.value).toBe(false)

    backend.emitSaving()
    expect(ex.saving.value).toBe(true)

    backend.roots.set(ROOT, dir(1, ROOT, 300))
    backend.emitDone(summary('completed'))
    await flush()
    expect(ex.scanning.value).toBe(false)
    expect(ex.saving.value).toBe(false)
    expect(ex.summary.value?.status).toBe('completed')
    expect(ex.children.value.map((n) => n.name)).toEqual(['src', 'a.txt'])
  })

  it('keeps the current view when the scan is cancelled', async () => {
    const ex = setup()
    await ex.ready
    await ex.open(ex.children.value[0])
    await ex.scan()
    await ex.cancel()
    expect(backend.cancelled).toBe(1)

    backend.emitDone(summary('cancelled'))
    await flush()
    expect(ex.scanning.value).toBe(false)
    expect(ex.trail.value.map((n) => n.name)).toEqual([ROOT, 'src'])
  })

  it('shows the error when the scan cannot start', async () => {
    backend.failStart = 'engine: a scan is already running'
    const ex = setup()
    await ex.ready
    await ex.scan()
    expect(ex.scanning.value).toBe(false)
    expect(ex.error.value).toBe('engine: a scan is already running')
  })
})

describe('navigation', () => {
  it('opens directories and goes back through the breadcrumbs', async () => {
    const ex = setup()
    await ex.ready
    await ex.open(ex.children.value[0])
    expect(ex.trail.value.map((n) => n.name)).toEqual([ROOT, 'src'])
    expect(ex.children.value.map((n) => n.name)).toEqual(['main.go'])

    await ex.goTo(0)
    expect(ex.trail.value.map((n) => n.name)).toEqual([ROOT])
    expect(ex.children.value).toHaveLength(2)
  })

  it('does not open files', async () => {
    const ex = setup()
    await ex.ready
    await ex.open(ex.children.value[1])
    expect(ex.trail.value).toHaveLength(1)
  })

  it('ignores a response that arrives after a newer click', async () => {
    backend.kids.set(1, [dir(2, 'slow'), dir(5, 'fast')])
    backend.kids.set(5, [file(6, 'x')])
    const ex = setup()
    await ex.ready
    const [slow, fast] = ex.children.value

    backend.hold(2)
    const first = ex.open(slow)
    await ex.open(fast)
    backend.release(2)
    await first

    expect(ex.trail.value.map((n) => n.name)).toEqual([ROOT, 'fast'])
    expect(ex.children.value.map((n) => n.name)).toEqual(['x'])
  })

  it('shows load errors and keeps the current view', async () => {
    const ex = setup()
    await ex.ready
    backend.failChildren = 'store: children: disk I/O error'
    await ex.open(ex.children.value[0])
    expect(ex.error.value).toBe('store: children: disk I/O error')
    expect(ex.trail.value).toHaveLength(1)
  })
})

describe('table', () => {
  const rowNames = (ex: ReturnType<typeof setup>) => ex.rows.value.map((n) => n.name)

  beforeEach(() => {
    backend.kids.set(1, [file(3, 'b.txt', 300), dir(2, 'C', 200), file(4, 'a.txt', 100)])
  })

  it('starts biggest first and sorts without reordering the treemap', async () => {
    const ex = setup()
    await ex.ready
    expect(rowNames(ex)).toEqual(['b.txt', 'C', 'a.txt'])
    expect(ex.total.value).toBe(3)

    await ex.sortBy('name')
    expect(ex.sort.value).toEqual({ key: 'name', desc: false })
    expect(rowNames(ex)).toEqual(['a.txt', 'b.txt', 'C'])
    expect(ex.children.value.map((n) => n.name)).toEqual(['b.txt', 'C', 'a.txt'])
  })

  it('flips the direction when the same column is chosen again', async () => {
    const ex = setup()
    await ex.ready
    await ex.sortBy('size')
    expect(ex.sort.value).toEqual({ key: 'size', desc: false })
    expect(rowNames(ex)).toEqual(['a.txt', 'C', 'b.txt'])
  })

  it('keeps the order while navigating', async () => {
    backend.kids.set(2, [file(5, 'z', 900), file(6, 'y', 1)])
    const ex = setup()
    await ex.ready
    await ex.sortBy('name')
    await ex.open(dir(2, 'C', 200))
    expect(rowNames(ex)).toEqual(['y', 'z'])
  })

  it('loads more rows until the folder is complete', async () => {
    const many = Array.from({ length: PAGE_SIZE + 5 }, (_, i) => file(10 + i, `f${i}`, 1000 - i))
    backend.kids.set(1, many)
    const ex = setup()
    await ex.ready
    expect(ex.rows.value).toHaveLength(PAGE_SIZE)
    expect(ex.hasMore.value).toBe(true)

    await ex.loadMore()
    expect(ex.rows.value).toHaveLength(PAGE_SIZE + 5)
    expect(ex.rows.value.at(-1)?.name).toBe(`f${PAGE_SIZE + 4}`)
    expect(ex.hasMore.value).toBe(false)
  })

  it('drops a page that arrives after navigating away', async () => {
    const many = Array.from({ length: PAGE_SIZE + 5 }, (_, i) => file(10 + i, `f${i}`, 1000 - i))
    backend.kids.set(1, [dir(2, 'src', 5000), ...many])
    const ex = setup()
    await ex.ready

    backend.hold(1)
    const more = ex.loadMore()
    await ex.open(ex.rows.value[0])
    backend.release(1)
    await more

    expect(ex.trail.value.map((n) => n.name)).toEqual([ROOT, 'src'])
    expect(rowNames(ex)).toEqual(['main.go'])
  })

  it('goes up one level', async () => {
    const ex = setup()
    await ex.ready
    await ex.up() // already at the top: nothing happens
    expect(ex.trail.value).toHaveLength(1)

    await ex.open(dir(2, 'C', 200))
    await ex.up()
    expect(ex.trail.value.map((n) => n.name)).toEqual([ROOT])
  })
})

describe('reveal', () => {
  it('opens the folder of a search hit and marks it', async () => {
    const ex = setup()
    await ex.ready
    await ex.reveal({ ...file(4, 'main.go', 200), parentId: 2, dir: `${ROOT}\\src` })
    expect(ex.trail.value.map((n) => n.name)).toEqual([ROOT, 'src'])
    expect(ex.found.value).toBe(4)
    await ex.up()
    expect(ex.found.value).toBeNull()
  })

  it('loads pages until the hit is in the table', async () => {
    backend.kids.set(
      2,
      Array.from({ length: PAGE_SIZE * 2 }, (_, i) => file(100 + i, `f${i}`, PAGE_SIZE * 2 - i)),
    )
    const ex = setup()
    await ex.ready
    const last = backend.kids.get(2)!.at(-1)!
    await ex.reveal({ ...last, parentId: 2, dir: '' })
    expect(ex.rows.value).toHaveLength(PAGE_SIZE * 2)
    expect(ex.found.value).toBe(last.id)
  })

  it('shows the error of a hit that is no longer in the index', async () => {
    const ex = setup()
    await ex.ready
    await ex.reveal({ ...file(99, 'gone', 1), parentId: 98, dir: '' })
    expect(ex.error.value).toBe('store: not found')
    expect(ex.trail.value.map((n) => n.name)).toEqual([ROOT])
  })
})

it('unsubscribes from scan events when its scope is disposed', () => {
  setup()
  expect(backend.listeners).toBe(3)
  scope.stop()
  expect(backend.listeners).toBe(0)
})
