import { describe, expect, it, vi } from 'vitest'
import { onMenuKey } from './menuKeys'

const menu = () => {
  const items = ['a', 'b', 'c'].map((name) => ({ name, focus: vi.fn() }))
  return { items, el: { querySelectorAll: () => items } }
}
const press = (key: string, target: unknown, el: unknown) => {
  const e = { key, target, currentTarget: el, preventDefault: vi.fn() }
  const close = vi.fn()
  onMenuKey(e as never, close)
  return { e, close }
}

describe('onMenuKey', () => {
  it('moves between items with the arrows and wraps at the ends', () => {
    const { items, el } = menu()
    press('ArrowDown', items[0], el)
    expect(items[1].focus).toHaveBeenCalled()
    press('ArrowDown', items[2], el)
    expect(items[0].focus).toHaveBeenCalled()
    press('ArrowUp', items[0], el)
    expect(items[2].focus).toHaveBeenCalled()
  })
  it('jumps to the ends with Home and End', () => {
    const { items, el } = menu()
    press('End', items[0], el)
    expect(items[2].focus).toHaveBeenCalled()
    press('Home', items[2], el)
    expect(items[0].focus).toHaveBeenCalled()
  })
  it('closes on Tab instead of leaving the focus on a menu that goes away', () => {
    const { items, el } = menu()
    const { e, close } = press('Tab', items[1], el)
    expect(close).toHaveBeenCalled()
    expect(e.preventDefault).toHaveBeenCalled()
  })
  it('ignores other keys', () => {
    const { items, el } = menu()
    const { e, close } = press('a', items[0], el)
    expect(close).not.toHaveBeenCalled()
    expect(e.preventDefault).not.toHaveBeenCalled()
  })
})
