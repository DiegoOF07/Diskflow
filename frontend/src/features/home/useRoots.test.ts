import { effectScope } from 'vue'
import { beforeEach, describe, expect, it } from 'vitest'
import { FakeBackend, rootInfo } from '../../backend/fake'
import { useRoots } from './useRoots'

let backend: FakeBackend
let scope: ReturnType<typeof effectScope>

function setup() {
  scope = effectScope()
  return scope.run(() => useRoots(backend))!
}

beforeEach(() => {
  backend = new FakeBackend()
  backend.catalog = [rootInfo(1, 'C:\\work'), rootInfo(2, 'D:\\fotos')]
})

it('loads the catalog', async () => {
  const r = setup()
  await r.load()
  expect(r.roots.value.map((x) => x.name)).toEqual(['work', 'fotos'])
  expect(r.loaded.value).toBe(true)
})

describe('add', () => {
  it('adds the picked folder and returns it', async () => {
    backend.picked = 'E:\\juegos'
    const r = setup()
    await r.load()
    const added = await r.add()
    expect(added?.path).toBe('E:\\juegos')
    expect(r.roots.value.map((x) => x.path)).toContain('E:\\juegos')
  })

  it('does nothing when the dialog is closed', async () => {
    const r = setup()
    await r.load()
    expect(await r.add()).toBeNull()
    expect(r.roots.value).toHaveLength(2)
  })
})

it('renames and reloads', async () => {
  const r = setup()
  await r.load()
  await r.rename(1, 'Trabajo')
  expect(r.roots.value[0].name).toBe('Trabajo')
})

describe('remove', () => {
  it('removes and reloads', async () => {
    const r = setup()
    await r.load()
    await r.remove(1)
    expect(r.roots.value.map((x) => x.id)).toEqual([2])
  })

  it('ignores a second click while the first removal runs', async () => {
    backend.hold('remove:1')
    const r = setup()
    await r.load()
    const first = r.remove(1)
    const second = r.remove(1)
    expect(r.removing.value).toEqual([1])
    backend.release('remove:1')
    await Promise.all([first, second])
    expect(backend.removeCalls).toBe(1)
    expect(r.removing.value).toEqual([])
    expect(r.error.value).toBe('')
    expect(r.roots.value.map((x) => x.id)).toEqual([2])
  })

  it('shows the error and keeps the list', async () => {
    backend.failRemove = 'engine: a scan is already running'
    const r = setup()
    await r.load()
    await r.remove(1)
    expect(r.error.value).toBe('engine: a scan is already running')
    expect(r.roots.value).toHaveLength(2)
  })
})

it('reloads when a scan finishes, so the summaries stay current', async () => {
  const r = setup()
  await r.load()
  backend.catalog.push(rootInfo(3, 'F:\\otra'))
  backend.emitDone({
    scanId: 9,
    status: 'completed',
    elapsedMs: 1,
    files: 0,
    dirs: 0,
    bytes: 0,
    diskBytes: 0,
    errors: 0,
  })
  await new Promise((res) => setTimeout(res, 0))
  expect(r.roots.value).toHaveLength(3)
})

it('unsubscribes when its scope is disposed', () => {
  setup()
  expect(backend.listeners).toBe(1)
  scope.stop()
  expect(backend.listeners).toBe(0)
})
