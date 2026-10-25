import { describe, expect, it } from 'vitest'
import type { Node } from '../../../backend/types'
import { fitLabel, groupOthers, layoutTreemap, MAX_CELLS, OTHERS_ID } from './layout'

// Logical and on-disk sizes are equal unless a test says otherwise.
const n = (id: number, size: number, files = 1, diskSize = size): Node => ({
  id,
  name: `n${id}`,
  kind: 'dir',
  size,
  diskSize,
  flags: 0,
  files,
})
const parent = (diskSize: number, size = diskSize) => ({ size, diskSize })
const ids = (nodes: Node[]) => nodes.map((x) => x.id)

describe('groupOthers', () => {
  it('groups children under 1% of the parent', () => {
    const out = groupOthers([n(1, 5000), n(2, 4950, 3), n(3, 49, 2), n(4, 1, 5)], parent(10_000))
    expect(ids(out)).toEqual([1, 2, OTHERS_ID])
    expect(out[2]).toMatchObject({ name: 'Otros', diskSize: 50, files: 7 })
  })

  it(`groups whatever falls outside the top ${MAX_CELLS}`, () => {
    const kids = Array.from({ length: MAX_CELLS + 10 }, (_, i) => n(i + 1, 100, 2))
    const out = groupOthers(kids, parent(100 * kids.length))
    expect(out).toHaveLength(MAX_CELLS + 1)
    expect(out.at(-1)).toMatchObject({ id: OTHERS_ID, diskSize: 1000, files: 20 })
  })

  it('counts the size of children that were not loaded', () => {
    // The table loads at most PAGE_SIZE children; the parent knows the full total.
    const out = groupOthers([n(1, 600), n(2, 300)], parent(1000))
    expect(out.at(-1)).toMatchObject({ id: OTHERS_ID, diskSize: 100 })
  })

  it('does not add an empty Otros', () => {
    expect(ids(groupOthers([n(1, 500), n(2, 300), n(3, 200)], parent(1000)))).toEqual([1, 2, 3])
  })

  it('keeps nothing from an empty parent', () => {
    expect(groupOthers([n(1, 0), n(2, 0)], parent(0))).toEqual([])
  })

  it('ranks by on-disk size, not logical size', () => {
    // A sparse image reserves 512 units but only takes 3; the video really takes 40.
    const out = groupOthers([n(1, 512, 1, 3), n(2, 40, 1, 40)], parent(100, 600))
    expect(ids(out)).toEqual([2, 1, OTHERS_ID])
    expect(out.at(-1)).toMatchObject({ diskSize: 57, size: 48 })
  })

  it('sorts by size even if the input is not sorted', () => {
    expect(ids(groupOthers([n(1, 100), n(2, 900)], parent(1000)))).toEqual([2, 1])
  })
})

describe('layoutTreemap', () => {
  const W = 400
  const H = 300
  const area = (r: { x0: number; y0: number; x1: number; y1: number }) => (r.x1 - r.x0) * (r.y1 - r.y0)

  it('gives each node an area proportional to its on-disk size', () => {
    const rects = layoutTreemap([n(1, 999, 1, 70), n(2, 1, 1, 20), n(3, 50, 1, 10)], W, H)
    expect(ids(rects.map((r) => r.node)).sort()).toEqual([1, 2, 3])
    for (const r of rects) {
      expect(area(r) / (W * H)).toBeCloseTo(r.node.diskSize / 100, 2)
    }
  })

  it('stays inside the box', () => {
    for (const r of layoutTreemap([n(1, 5), n(2, 3), n(3, 1), n(4, 1)], W, H)) {
      expect(r.x0).toBeGreaterThanOrEqual(0)
      expect(r.y0).toBeGreaterThanOrEqual(0)
      expect(r.x1).toBeLessThanOrEqual(W)
      expect(r.y1).toBeLessThanOrEqual(H)
    }
  })

  it('returns nothing without nodes or without room', () => {
    expect(layoutTreemap([], W, H)).toEqual([])
    expect(layoutTreemap([n(1, 10)], 0, H)).toEqual([])
  })
})

describe('fitLabel', () => {
  it('keeps names that fit', () => {
    expect(fitLabel('src', 100)).toBe('src')
  })
  it('cuts long names with an ellipsis', () => {
    const label = fitLabel('node_modules', 50)
    expect(label.endsWith('…')).toBe(true)
    expect(label.length).toBeLessThan('node_modules'.length)
  })
  it('hides the label when not even one letter fits', () => {
    expect(fitLabel('src', 10)).toBe('')
  })
})
