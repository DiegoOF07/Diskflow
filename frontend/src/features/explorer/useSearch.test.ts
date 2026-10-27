import { effectScope, nextTick } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { dir, FakeBackend, file } from '../../backend/fake'
import { SEARCH_DELAY, SEARCH_LIMIT, useSearch } from './useSearch'

const ROOT = 'C:\\work'

let backend: FakeBackend
let scope: ReturnType<typeof effectScope>

function setup() {
  scope = effectScope()
  return scope.run(() => useSearch(backend, ROOT))!
}

// Lets the watcher run, the typing pause pass and the fake answer.
async function type(s: ReturnType<typeof setup>, text: string, wait = SEARCH_DELAY) {
  s.query.value = text
  await nextTick()
  await vi.advanceTimersByTimeAsync(wait)
}

beforeEach(() => {
  vi.useFakeTimers()
  backend = new FakeBackend()
  backend.roots.set(ROOT, dir(1, ROOT, 600))
  backend.kids.set(1, [dir(2, 'Reports', 500), file(3, 'report.pdf', 100)])
  backend.kids.set(2, [file(4, 'old-report.txt', 400)])
})
afterEach(() => {
  scope.stop()
  vi.useRealTimers()
})

describe('search', () => {
  it('waits for a pause in typing and searches once', async () => {
    const s = setup()
    await type(s, 'r', 100)
    await type(s, 're', 100)
    expect(backend.searches).toEqual([])
    expect(s.searching.value).toBe(true)
    await type(s, 'rep')
    expect(backend.searches).toEqual(['rep'])
    expect(s.hits.value.map((h) => h.name)).toEqual(['Reports', 'old-report.txt', 'report.pdf'])
    expect(s.hits.value[1].dir).toBe(`${ROOT}\\Reports`)
    expect(s.searching.value).toBe(false)
  })

  it('ignores a response that arrives after a newer search', async () => {
    const s = setup()
    backend.hold('rep')
    await type(s, 'rep')
    await type(s, 'old')
    expect(s.hits.value.map((h) => h.name)).toEqual(['old-report.txt'])
    backend.release('rep')
    await vi.advanceTimersByTimeAsync(0)
    expect(s.hits.value.map((h) => h.name)).toEqual(['old-report.txt'])
  })

  it('clears the results without asking the backend', async () => {
    const s = setup()
    await type(s, 'rep')
    s.clear()
    await type(s, '')
    expect(s.hits.value).toEqual([])
    expect(backend.searches).toEqual(['rep'])
  })

  it('cancels the running search when the box is cleared', async () => {
    const s = setup()
    backend.hold('rep')
    await type(s, 'rep')
    await type(s, '')
    expect(backend.searchesCancelled).toBe(1)
    expect(s.searching.value).toBe(false)
    backend.release('rep')
  })

  it('cancels the running search when it is disposed', async () => {
    const s = setup()
    backend.hold('rep')
    await type(s, 'rep')
    scope.stop()
    expect(backend.searchesCancelled).toBe(1)
  })

  it('does not cancel when no search is running', async () => {
    const s = setup()
    await type(s, 'rep')
    s.clear()
    await type(s, '')
    expect(backend.searchesCancelled).toBe(0)
  })

  it('trims spaces and reports when there are more hits than shown', async () => {
    backend.kids.set(
      5,
      Array.from({ length: SEARCH_LIMIT + 1 }, (_, i) => file(10 + i, `log-${i}`)),
    )
    backend.kids.set(1, [...backend.kids.get(1)!, dir(5, 'logs')])
    const s = setup()
    await type(s, '  log-  ')
    expect(backend.searches).toEqual(['log-'])
    expect(s.hits.value).toHaveLength(SEARCH_LIMIT)
    expect(s.more.value).toBe(true)
  })

  it('shows errors and recovers on the next search', async () => {
    const s = setup()
    backend.failSearch = 'engine: no completed scan for this root'
    await type(s, 'rep')
    expect(s.error.value).toBe('engine: no completed scan for this root')
    backend.failSearch = null
    await type(s, 'old')
    expect(s.error.value).toBe('')
    expect(s.hits.value).toHaveLength(1)
  })

  it('ignores the error of a search the backend cancelled for a newer one', async () => {
    const s = setup()
    backend.hold('rep')
    await type(s, 'rep')
    await type(s, 'old')
    backend.failSearch = 'context canceled'
    backend.release('rep')
    await vi.advanceTimersByTimeAsync(0)
    expect(s.error.value).toBe('')
  })

  it('searches again when a scan replaces the index', async () => {
    const s = setup()
    await type(s, 'rep')
    backend.emitDone({
      scanId: 2,
      status: 'cancelled',
      elapsedMs: 1,
      files: 0,
      dirs: 0,
      bytes: 0,
      diskBytes: 0,
      errors: 0,
    })
    await vi.advanceTimersByTimeAsync(0)
    expect(backend.searches).toEqual(['rep'])
    backend.emitDone({
      scanId: 3,
      status: 'completed',
      elapsedMs: 1,
      files: 0,
      dirs: 0,
      bytes: 0,
      diskBytes: 0,
      errors: 0,
    })
    await vi.advanceTimersByTimeAsync(0)
    expect(backend.searches).toEqual(['rep', 'rep'])
  })

  it('stops listening when its scope is disposed', () => {
    setup()
    expect(backend.listeners).toBe(1)
    scope.stop()
    expect(backend.listeners).toBe(0)
  })
})
