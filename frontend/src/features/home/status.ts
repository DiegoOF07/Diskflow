import type { RootInfo } from '../../backend/types'
import { formatAgo, formatDate } from '../explorer/format'

/** One line describing where a folder's scans stand, for the home screen. */
export function rootStatus(r: RootInfo, nowSeconds = Date.now() / 1000): string {
  const { index, last } = r
  if (last?.status === 'running') return 'Analizando…'
  if (index) {
    const age = `Analizado ${formatAgo(index.finishedAt, nowSeconds)}`
    if (last && last.id !== index.id) {
      return `${age} · el último intento ${last.status === 'failed' ? 'falló' : 'se canceló'}`
    }
    return age
  }
  if (last?.status === 'failed') return 'El análisis falló'
  if (last?.status === 'cancelled') return 'Análisis cancelado'
  return 'Sin analizar'
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
