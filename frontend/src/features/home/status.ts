import type { RootInfo } from '../../backend/types'
import { formatAgo, formatDate } from '../explorer/format'

/**
 * Where a folder's scans stand, for the home screen. A later attempt that did
 * not complete goes in the note; problem marks a failure (the note, or the
 * text when there is no note).
 */
export interface RootStatus {
  text: string
  note?: string
  problem?: boolean
}

export function rootStatus(r: RootInfo, nowSeconds = Date.now() / 1000): RootStatus {
  const { index, last } = r
  if (last?.status === 'running') return { text: 'Analizando…' }
  if (index) {
    const text = `Analizado ${formatAgo(index.finishedAt, nowSeconds)}`
    if (last && last.id !== index.id) {
      return last.status === 'failed'
        ? { text, note: 'el último intento falló', problem: true }
        : { text, note: 'el último intento se canceló' }
    }
    return { text }
  }
  if (last?.status === 'failed') return { text: 'El análisis falló', problem: true }
  if (last?.status === 'cancelled') return { text: 'Análisis cancelado' }
  return { text: 'Sin analizar' }
}

/**
 * When the latest attempt did not complete, the explorer still shows the index
 * before it; this says so and how old that index is. Empty otherwise.
 */
export function previousIndexNote({ index, last }: RootInfo): string {
  if (!index || !last || last.id === index.id || last.status === 'running') return ''
  const what = last.status === 'failed' ? 'falló' : 'se canceló'
  return `El último análisis ${what}; se muestra el del ${formatDate(index.finishedAt)}.`
}
