import type { RootInfo } from '../../backend/types'
import { formatAgo } from '../explorer/format'

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
