import type { RootInfo } from '../../backend/types'

/** Each scanned folder's size on disk as a fraction of the largest one, for the home scale bars. */
export function scaleOf(roots: readonly RootInfo[]): Map<number, number> {
  const scanned = roots.filter((r) => r.index)
  const max = Math.max(0, ...scanned.map((r) => r.index!.diskBytes))
  return new Map(scanned.map((r) => [r.id, max ? r.index!.diskBytes / max : 0]))
}
