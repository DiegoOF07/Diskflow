import { effectScope } from 'vue'
import { beforeEach, describe, expect, it } from 'vitest'
import { dir, FakeBackend, file } from '../../backend/fake'
import type { Summary } from '../../backend/types'
import { useExplorer } from './useExplorer'

const ROOT = 'C:\\work'
const flush = () => new Promise((r) => setTimeout(r, 0))
const summary = (status: Summary['status']): Summary => ({
  scanId: 1, status, elapsedMs: 10, files: 2, dirs: 1, bytes: 300, errors: 0,
})

let backend: FakeBackend
let scope: ReturnType<typeof effectScope>

// useExplorer registers cleanups with onScopeDispose, so it runs inside an effect scope.
function setup() {
  scope = effectScope()
  return scope.run(() => useExplorer(backend))!
}

beforeEach(() => {
  backend = new FakeBackend()
  backend.picked = ROOT
  backend.roots.set(ROOT, dir(1, ROOT, 300))
  backend.kids.set(1, [dir(2, 'src', 200), file(3, 'a.txt', 100)])
  backend.kids.set(2, [file(4, 'main.go', 200)])
})

describe('pickFolder', () => {
  it('shows the previous index of the folder', async () => {
    const ex = setup()
    await ex.pickFolder()
    expect(ex.root.value).toBe(ROOT)
    expect(ex.trail.value.map((n) => n.name)).toEqual([ROOT])
    expect(ex.children.value.map((n) => n.name)).toEqual(['src', 'a.txt'])
  })

  it('accepts a folder that was never scanned', async () => {
    backend.roots.clear()
    const ex = setup()
    await ex.pickFolder()
    expect(ex.root.value).toBe(ROOT)
    expect(ex.trail.value).toEqual([])
    expect(ex.error.value).toBe('')
  })

  it('keeps everything when the dialog is closed', async () => {
    const ex = setup()
    await ex.pickFolder()
    backend.picked = null
    await ex.pickFolder()
    expect(ex.root.value).toBe(ROOT)
    expect(ex.children.value).toHaveLength(2)
  })
})

describe('scan', () => {
  it('tracks progress and reloads the index when it completes', async () => {
    backend.roots.clear()
    const ex = setup()
    await ex.pickFolder()
    await ex.scan()
    expect(backend.started).toEqual([ROOT])
    expect(ex.scanning.value).toBe(true)

    backend.emitProgress({ files: 1, dirs: 1, bytes: 100, errors: 0 })
    expect(ex.progress.value?.files).toBe(1)

    backend.roots.set(ROOT, dir(1, ROOT, 300))
    backend.emitDone(summary('completed'))
    await flush()
    expect(ex.scanning.value).toBe(false)
    expect(ex.summary.value?.status).toBe('completed')
    expect(ex.children.value.map((n) => n.name)).toEqual(['src', 'a.txt'])
  })

  it('keeps the current view when the scan is cancelled', async () => {
    const ex = setup()
    await ex.pickFolder()
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
    await ex.pickFolder()
    await ex.scan()
    expect(ex.scanning.value).toBe(false)
    expect(ex.error.value).toBe('engine: a scan is already running')
  })
})

describe('navigation', () => {
  it('opens directories and goes back through the breadcrumbs', async () => {
    const ex = setup()
    await ex.pickFolder()
    await ex.open(ex.children.value[0])
    expect(ex.trail.value.map((n) => n.name)).toEqual([ROOT, 'src'])
    expect(ex.children.value.map((n) => n.name)).toEqual(['main.go'])

    await ex.goTo(0)
    expect(ex.trail.value.map((n) => n.name)).toEqual([ROOT])
    expect(ex.children.value).toHaveLength(2)
  })

  it('does not open files', async () => {
    const ex = setup()
    await ex.pickFolder()
    await ex.open(ex.children.value[1])
    expect(ex.trail.value).toHaveLength(1)
  })

  it('ignores a response that arrives after a newer click', async () => {
    backend.kids.set(1, [dir(2, 'slow'), dir(5, 'fast')])
    backend.kids.set(5, [file(6, 'x')])
    const ex = setup()
    await ex.pickFolder()
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
    await ex.pickFolder()
    backend.failChildren = 'store: children: disk I/O error'
    await ex.open(ex.children.value[0])
    expect(ex.error.value).toBe('store: children: disk I/O error')
    expect(ex.trail.value).toHaveLength(1)
  })
})

it('unsubscribes from scan events when its scope is disposed', () => {
  setup()
  expect(backend.listeners).toBe(2)
  scope.stop()
  expect(backend.listeners).toBe(0)
})
