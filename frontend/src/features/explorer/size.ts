import { Flag, type Node } from '../../backend/types'
import { formatBytes } from './format'

const FLAGS = [
  { flag: Flag.Sparse, label: 'disperso', plural: 'dispersos' },
  { flag: Flag.Compressed, label: 'comprimido', plural: 'comprimidos' },
  { flag: Flag.Cloud, label: 'solo en la nube', plural: 'solo en la nube' },
] as const

const list = new Intl.ListFormat('es', { type: 'conjunction' })

/** Short badge texts for the flags set in flags. */
export function flagLabels(flags: number): string[] {
  return FLAGS.filter((f) => flags & f.flag).map((f) => f.label)
}

/** Explains why n's on-disk size differs from its logical size, or "" if nothing notable. */
export function sizeNote(n: Node): string {
  if (!n.flags) return ''
  const logical = formatBytes(n.size)
  const disk = formatBytes(n.diskSize)
  if (n.kind === 'dir') {
    const kinds = list.format(FLAGS.filter((f) => n.flags & f.flag).map((f) => f.plural))
    return `Contiene archivos ${kinds}: ${logical} lógicos, ${disk} en disco`
  }
  if (n.flags & Flag.Cloud)
    return `Solo en la nube (OneDrive): ${logical} que no ocupan espacio en este disco`
  if (n.flags & Flag.Sparse) return `Disperso: reserva ${logical} pero ocupa ${disk}`
  return `Comprimido: ${logical} sin comprimir, ocupa ${disk}`
}

/**
 * Whether the table needs a logical size column for folder: only when it holds
 * flagged files. Rounding to clusters makes almost every size differ a little,
 * and that alone is not worth a column.
 */
export function showLogical(folder: Node | null): boolean {
  return !!folder?.flags
}

/**
 * Whether root comes from an index scanned before on-disk sizes existed: the
 * migration copied the logical size. A real scan almost never matches it
 * exactly, because files are rounded up to whole clusters.
 */
export function isEstimated(root: Node): boolean {
  return root.size > 0 && root.diskSize === root.size && root.flags === 0
}
