import type { Progress } from '../../backend/types'
import { formatBytes, formatCount } from './format'

const count = (n: number, one: string, many: string) => `${formatCount(n)} ${n === 1 ? one : many}`

/** What a scan has seen so far, one fact per item; the view lays them out. */
export function progressFacts(p: Progress): string[] {
  const facts = [
    count(p.files, 'archivo', 'archivos'),
    count(p.dirs, 'carpeta', 'carpetas'),
    `${formatBytes(p.diskBytes)} en disco`,
  ]
  if (p.errors) facts.push(`${formatCount(p.errors)} sin acceso`)
  return facts
}
