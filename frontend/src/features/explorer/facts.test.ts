import { expect, it } from 'vitest'
import { progressFacts } from './facts'

const progress = { files: 1175176, dirs: 229000, bytes: 900, diskBytes: 3 * 1024 ** 3, errors: 0 }

it('lists what a scan has seen, one fact per item', () => {
  expect(progressFacts(progress)).toEqual(['1.175.176 archivos', '229.000 carpetas', '3,0 GiB en disco'])
})

it('adds the entries it could not read only when there are some', () => {
  expect(progressFacts({ ...progress, errors: 12 }).at(-1)).toBe('12 sin acceso')
})

it('uses the singular for one', () => {
  expect(progressFacts({ ...progress, files: 1, dirs: 1 }).slice(0, 2)).toEqual(['1 archivo', '1 carpeta'])
})
