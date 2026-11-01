import { describe, expect, it } from 'vitest'
import { rootInfo } from '../../backend/fake'
import type { RootInfo } from '../../backend/types'
import { scaleOf } from './scale'

const sized = (id: number, diskBytes: number | null): RootInfo => ({
  ...rootInfo(id, `C:\\r${id}`),
  index:
    diskBytes === null
      ? null
      : {
          id,
          status: 'completed',
          startedAt: 0,
          finishedAt: 0,
          files: 0,
          dirs: 0,
          bytes: 0,
          diskBytes,
          errors: 0,
        },
})

describe('scaleOf', () => {
  it('measures each folder against the largest one', () => {
    const scale = scaleOf([sized(1, 400), sized(2, 100), sized(3, 0)])
    expect(scale.get(1)).toBe(1)
    expect(scale.get(2)).toBe(0.25)
    expect(scale.get(3)).toBe(0)
  })
  it('leaves out folders never scanned', () => {
    const scale = scaleOf([sized(1, 400), sized(2, null)])
    expect(scale.has(2)).toBe(false)
  })
  it('all empty folders measure zero, not NaN', () => {
    expect(scaleOf([sized(1, 0)]).get(1)).toBe(0)
  })
})
