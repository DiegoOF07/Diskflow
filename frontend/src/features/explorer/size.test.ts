import { describe, expect, it } from 'vitest'
import { dir, file } from '../../backend/fake'
import { Flag } from '../../backend/types'
import { flagLabels, isEstimated, showLogical, sizeNote } from './size'

const GiB = 2 ** 30

describe('flagLabels', () => {
  it('names each flag, in a fixed order', () => {
    expect(flagLabels(Flag.Cloud | Flag.Sparse)).toEqual(['disperso', 'solo en la nube'])
    expect(flagLabels(Flag.Compressed)).toEqual(['comprimido'])
    expect(flagLabels(0)).toEqual([])
  })
})

describe('sizeNote', () => {
  it('explains a sparse file', () => {
    expect(sizeNote(file(1, 'userdata.img', 512 * GiB, 2.9 * GiB, Flag.Sparse))).toBe(
      'Disperso: reserva 512,0 GiB pero ocupa 2,9 GiB',
    )
  })
  it('explains a compressed file', () => {
    expect(sizeNote(file(1, 'log.txt', 10 * GiB, 2 * GiB, Flag.Compressed))).toBe(
      'Comprimido: 10,0 GiB sin comprimir, ocupa 2,0 GiB',
    )
  })
  it('explains a cloud placeholder', () => {
    expect(sizeNote(file(1, 'video.mp4', GiB, 0, Flag.Cloud))).toBe(
      'Solo en la nube (OneDrive): 1,0 GiB que no ocupan espacio en este disco',
    )
  })
  it('explains a directory by what it contains', () => {
    expect(sizeNote(dir(1, 'Local', 757 * GiB, 250 * GiB, Flag.Sparse | Flag.Cloud))).toBe(
      'Contiene archivos dispersos y solo en la nube: 757,0 GiB lógicos, 250,0 GiB en disco',
    )
  })
  it('has nothing to say without flags', () => {
    expect(sizeNote(file(1, 'a.txt', 100, 4096))).toBe('')
  })
})

describe('showLogical', () => {
  it('shows the logical column when the folder contains flagged files', () => {
    expect(showLogical(dir(1, 'Local', 10, 5, Flag.Sparse))).toBe(true)
  })
  it('hides it when the only difference is cluster rounding', () => {
    expect(showLogical(dir(1, 'src', 10_000, 16_384))).toBe(false)
  })
  it('hides it without a folder', () => {
    expect(showLogical(null)).toBe(false)
  })
})

describe('isEstimated', () => {
  it('detects an index migrated from logical sizes', () => {
    expect(isEstimated(dir(1, 'C:\\', 5000, 5000))).toBe(true)
  })
  it('trusts a real scan, where rounding or flags make the sizes differ', () => {
    expect(isEstimated(dir(1, 'C:\\', 5000, 8192))).toBe(false)
    expect(isEstimated(dir(1, 'C:\\', 5000, 5000, Flag.Compressed))).toBe(false)
  })
  it('does not warn about an empty folder', () => {
    expect(isEstimated(dir(1, 'C:\\', 0, 0))).toBe(false)
  })
})
