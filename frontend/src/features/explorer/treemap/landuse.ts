import type { Node } from '../../../backend/types'
import { OTHERS_ID } from './layout'

/**
 * The map colour of a cell, named after the land-use colours of a topographic
 * map (CSS tokens --forest, --sand, --rock). In Fase 3 they become categories.
 */
export type LandUse = 'forest' | 'sand' | 'rock' | 'grouped'

export function landUse(n: Node): LandUse {
  if (n.id === OTHERS_ID) return 'grouped'
  if (n.kind === 'dir') return 'forest'
  if (n.kind === 'file') return 'sand'
  return 'rock'
}

const LEGEND: { use: LandUse; label: string }[] = [
  { use: 'forest', label: 'Carpeta' },
  { use: 'sand', label: 'Archivo' },
  { use: 'rock', label: 'Enlace u otro' },
  { use: 'grouped', label: 'Agrupado en «Otros»' },
]

/** The legend of a map: only the colours it shows, in a fixed order. */
export function legendFor(cells: readonly Node[]) {
  const shown = new Set(cells.map(landUse))
  return LEGEND.filter((l) => shown.has(l.use))
}
