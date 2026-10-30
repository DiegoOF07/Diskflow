import { describe, expect, it } from 'vitest'
import { rootInfo } from '../../backend/fake'
import type { ScanInfo } from '../../backend/types'
import { formatDate } from '../explorer/format'
import { previousIndexNote, rootStatus } from './status'

const NOW = 1_800_000_000
const scan = (id: number, status: ScanInfo['status'], finishedAt = NOW - 3 * 3600): ScanInfo => ({
  id,
  status,
  startedAt: finishedAt - 60,
  finishedAt,
  files: 0,
  dirs: 0,
  bytes: 0,
  diskBytes: 0,
  errors: 0,
})
const root = (index: ScanInfo | null, last: ScanInfo | null) => ({ ...rootInfo(1, 'C:\\work'), index, last })

describe('rootStatus', () => {
  it('a folder never scanned', () => {
    expect(rootStatus(root(null, null), NOW)).toBe('Sin analizar')
  })
  it('a scan in progress', () => {
    expect(rootStatus(root(scan(1, 'completed'), scan(2, 'running', 0)), NOW)).toBe('Analizando…')
  })
  it('the age of the index', () => {
    const s = scan(1, 'completed')
    expect(rootStatus(root(s, s), NOW)).toBe('Analizado hace 3 horas')
  })
  it('a later attempt that did not complete', () => {
    expect(rootStatus(root(scan(1, 'completed'), scan(2, 'failed')), NOW)).toBe(
      'Analizado hace 3 horas · el último intento falló',
    )
    expect(rootStatus(root(scan(1, 'completed'), scan(2, 'cancelled')), NOW)).toBe(
      'Analizado hace 3 horas · el último intento se canceló',
    )
  })
  it('only attempts that did not complete', () => {
    expect(rootStatus(root(null, scan(2, 'failed')), NOW)).toBe('El análisis falló')
    expect(rootStatus(root(null, scan(2, 'cancelled')), NOW)).toBe('Análisis cancelado')
  })
})

describe('previousIndexNote', () => {
  it('says which index is shown after an attempt that did not complete', () => {
    const index = scan(1, 'completed')
    expect(previousIndexNote(root(index, scan(2, 'failed')))).toBe(
      `El último análisis falló; se muestra el del ${formatDate(index.finishedAt)}.`,
    )
    expect(previousIndexNote(root(index, scan(2, 'cancelled')))).toBe(
      `El último análisis se canceló; se muestra el del ${formatDate(index.finishedAt)}.`,
    )
  })
  it('says nothing when the index is the latest attempt, one is running or there is no index', () => {
    const index = scan(1, 'completed')
    expect(previousIndexNote(root(index, index))).toBe('')
    expect(previousIndexNote(root(index, scan(2, 'running', 0)))).toBe('')
    expect(previousIndexNote(root(null, scan(2, 'failed')))).toBe('')
  })
})
