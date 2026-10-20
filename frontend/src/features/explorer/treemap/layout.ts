import { hierarchy, treemap, treemapSquarify } from 'd3-hierarchy'
import type { Node } from '../../../backend/types'

/** Id of the synthetic "Otros" cell; real entries always have positive ids. */
export const OTHERS_ID = -1
/** Most cells drawn for one directory (product plan §19.7). */
export const MAX_CELLS = 50
/** Children below this share of the parent go to "Otros" (§19.7). */
const MIN_SHARE = 0.01

export interface Rect {
  node: Node
  x0: number
  y0: number
  x1: number
  y1: number
}

/**
 * Picks the cells to draw for a directory of parentSize bytes: the biggest
 * MAX_CELLS children with at least MIN_SHARE of the parent. The rest, and the
 * children that were never loaded, become one "Otros" cell.
 */
export function groupOthers(children: readonly Node[], parentSize: number): Node[] {
  if (parentSize <= 0) return []
  const sorted = [...children].sort((a, b) => b.size - a.size)
  const kept: Node[] = []
  let otherFiles = 0
  for (const c of sorted) {
    if (kept.length < MAX_CELLS && c.size > 0 && c.size >= parentSize * MIN_SHARE) kept.push(c)
    else otherFiles += c.files
  }
  const rest = parentSize - kept.reduce((sum, c) => sum + c.size, 0)
  if (rest > 0) kept.push({ id: OTHERS_ID, name: 'Otros', kind: 'other', size: rest, files: otherFiles })
  return kept
}

type Datum = Node | { children: Node[] }

/** Lays out nodes as a squarified treemap filling width × height. */
export function layoutTreemap(nodes: readonly Node[], width: number, height: number): Rect[] {
  if (nodes.length === 0 || width <= 0 || height <= 0) return []
  const root = hierarchy<Datum>({ children: [...nodes] }, (d) => ('children' in d ? d.children : undefined))
    .sum((d) => ('size' in d ? d.size : 0))
    .sort((a, b) => (b.value ?? 0) - (a.value ?? 0))
  return treemap<Datum>()
    .size([width, height])
    .tile(treemapSquarify)(root)
    .leaves()
    .map((l) => ({ node: l.data as Node, x0: l.x0, y0: l.y0, x1: l.x1, y1: l.y1 }))
}

const CHAR_WIDTH = 7 // average px per character at the treemap's 12px font
const PADDING = 8

/** Shortens name to fit width px, or returns "" if not even one letter fits. */
export function fitLabel(name: string, width: number): string {
  const chars = Math.floor((width - PADDING) / CHAR_WIDTH)
  if (name.length <= chars) return name
  return chars >= 2 ? name.slice(0, chars - 1) + '…' : ''
}
