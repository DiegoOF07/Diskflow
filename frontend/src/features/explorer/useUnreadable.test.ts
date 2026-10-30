import { effectScope } from 'vue'
import { beforeEach, expect, it } from 'vitest'
import { BackendError } from '../../backend/errors'
import { FakeBackend, file } from '../../backend/fake'
import type { Hit, Summary } from '../../backend/types'
import { UNREADABLE_LIMIT, unreadableReason, useUnreadable } from './useUnreadable'

const ROOT = 'C:\\work'
const hit = (id: number, name: string, err = 'access_denied'): Hit => ({
  ...file(id, name),
  err,
  parentId: 1,
  dir: ROOT,
})
const done = (status: Summary['status']): Summary => ({
  scanId: 2,
  status,
  elapsedMs: 1,
  files: 0,
  dirs: 0,
  bytes: 0,
  diskBytes: 0,
  errors: 0,
})

let backend: FakeBackend
let scope: ReturnType<typeof effectScope>

function setup() {
  scope = effectScope()
  return scope.run(() => useUnreadable(backend, ROOT))!
}

beforeEach(() => {
  backend = new FakeBackend()
  backend.unreadableHits = [hit(2, 'System Volume Information'), hit(3, 'gone.tmp', 'not_found')]
})

it('loads the list only when the panel is opened', async () => {
  const u = setup()
  expect(backend.unreadableCalls).toBe(0)
  await u.toggle()
  expect(u.open.value).toBe(true)
  expect(u.hits.value.map((h) => h.name)).toEqual(['System Volume Information', 'gone.tmp'])
  expect(u.more.value).toBe(false)
  await u.toggle()
  expect(u.open.value).toBe(false)
  expect(backend.unreadableCalls).toBe(1)
})

it('shows that the list is cut at the limit', async () => {
  backend.unreadableHits = Array.from({ length: UNREADABLE_LIMIT + 1 }, (_, i) => hit(i + 2, `d${i}`))
  const u = setup()
  await u.toggle()
  expect(u.hits.value).toHaveLength(UNREADABLE_LIMIT)
  expect(u.more.value).toBe(true)
})

it('is loading until the list arrives', async () => {
  backend.hold('unreadable')
  const u = setup()
  const opened = u.toggle()
  expect(u.loading.value).toBe(true)
  backend.release('unreadable')
  await opened
  expect(u.loading.value).toBe(false)
})

it('reloads after a completed scan only if it is open', async () => {
  const u = setup()
  await u.toggle()
  backend.unreadableHits = [hit(9, 'new')]
  backend.emitDone(done('cancelled'))
  backend.emitDone(done('completed'))
  await new Promise((r) => setTimeout(r, 0))
  expect(u.hits.value.map((h) => h.name)).toEqual(['new'])

  await u.toggle()
  backend.emitDone(done('completed'))
  expect(backend.unreadableCalls).toBe(2)
})

it('shows why the list could not be loaded', async () => {
  const u = setup()
  backend.unreadable = () => Promise.reject(new BackendError('no_scan'))
  await u.toggle()
  expect(u.error.value).toMatch(/análisis completo/)
  expect(u.hits.value).toEqual([])
})

it('unsubscribes when its scope is disposed', () => {
  setup()
  expect(backend.listeners).toBe(1)
  scope.stop()
  expect(backend.listeners).toBe(0)
})

it('names the reason of each error code', () => {
  expect(unreadableReason('access_denied')).toBe('Sin permiso de lectura')
  expect(unreadableReason('not_found')).toBe('Desapareció durante el análisis')
  expect(unreadableReason('error')).toBe('Error de lectura')
})
