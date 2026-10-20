import { describe, expect, it } from 'vitest'
import { formatBytes, formatCount, formatPercent } from './format'

describe('formatBytes', () => {
  it.each([
    [0, '0 B'],
    [1023, '1023 B'],
    [1024, '1,0 KiB'],
    [1536, '1,5 KiB'],
    [5 * 2 ** 30, '5,0 GiB'],
    [3 * 2 ** 40, '3,0 TiB'],
  ])('%d -> %s', (n, want) => {
    expect(formatBytes(n)).toBe(want)
  })
})

describe('formatPercent', () => {
  it('formats one decimal with a Spanish comma', () => {
    expect(formatPercent(25, 200)).toBe('12,5 %')
  })
  it('has no percentage of an empty parent', () => {
    expect(formatPercent(0, 0)).toBe('—')
  })
})

describe('formatCount', () => {
  it('groups thousands', () => {
    expect(formatCount(12345)).toBe('12.345')
  })
})
