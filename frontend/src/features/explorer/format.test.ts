import { describe, expect, it } from 'vitest'
import { formatAgo, formatBytes, formatCount, formatDate, formatPercent } from './format'

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

describe('formatAgo', () => {
  const now = 1_800_000_000 // unix seconds
  it.each([
    [now - 20, 'hace un momento'],
    [now - 5 * 60, 'hace 5 minutos'],
    [now - 3 * 3600, 'hace 3 horas'],
    [now - 26 * 3600, 'ayer'],
    [now - 10 * 86400, 'hace 10 días'],
  ])('%d -> %s', (then, want) => {
    expect(formatAgo(then, now)).toBe(want)
  })
})

describe('formatDate', () => {
  it('shows a dash for an unknown date', () => {
    expect(formatDate(0)).toBe('—')
  })

  it('shows the local date and time', () => {
    const unix = new Date(2026, 9, 26, 14, 5).getTime() / 1000
    expect(formatDate(unix)).toMatch(/26\/10\/2026.*14:05/)
  })
})
