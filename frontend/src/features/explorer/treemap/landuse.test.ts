import { describe, expect, it } from 'vitest'
import { dir, file } from '../../../backend/fake'
import type { Node } from '../../../backend/types'
import { OTHERS_ID } from './layout'
import { landUse, legendFor } from './landuse'

const others: Node = { ...dir(OTHERS_ID, 'Otros', 10), kind: 'other' }
const link: Node = { ...file(9, 'acceso'), kind: 'link' }

describe('landUse', () => {
  it('gives each kind of entry its map colour', () => {
    expect(landUse(dir(2, 'src'))).toBe('forest')
    expect(landUse(file(3, 'a.txt'))).toBe('sand')
    expect(landUse(link)).toBe('rock')
  })
  it('marks the grouped cell apart from a real entry of kind other', () => {
    expect(landUse(others)).toBe('grouped')
  })
})

describe('legendFor', () => {
  it('lists only what the map shows, always in the same order', () => {
    expect(legendFor([file(3, 'a'), others, dir(2, 'b'), file(4, 'c')]).map((l) => l.label)).toEqual([
      'Carpeta',
      'Archivo',
      'Agrupado en «Otros»',
    ])
  })
  it('is empty for an empty map', () => {
    expect(legendFor([])).toEqual([])
  })
})
