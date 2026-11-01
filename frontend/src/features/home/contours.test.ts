import { describe, expect, it } from 'vitest'
import { hillContours } from './contours'

describe('hillContours', () => {
  it('draws one closed path per level', () => {
    const paths = hillContours(5, 100, 60, 50)
    expect(paths).toHaveLength(5)
    for (const d of paths) expect(d).toMatch(/^M[\d.\s-]+C.*Z$/)
  })
  it('is deterministic', () => {
    expect(hillContours(3, 0, 0, 10)).toEqual(hillContours(3, 0, 0, 10))
  })
})
