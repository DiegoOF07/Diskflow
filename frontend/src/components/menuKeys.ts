/** The parts of a keyboard event a menu needs; a DOM KeyboardEvent fits. */
interface MenuKeyEvent {
  key: string
  target: unknown
  currentTarget: unknown // the menu element
  preventDefault(): void
}
interface Container {
  querySelectorAll(selector: string): Iterable<unknown>
}
interface Focusable {
  focus(): void
}

/**
 * Keyboard of a menu, as in the WAI-ARIA menu pattern: the arrows move between
 * its items and wrap, Home and End jump to the ends, and Tab closes it (close
 * puts the focus back where the menu was opened). Escape is left to the view.
 */
export function onMenuKey(e: MenuKeyEvent, close: () => void) {
  const menu = e.currentTarget as Container | null
  const items = [...(menu?.querySelectorAll('[role="menuitem"]') ?? [])] as Focusable[]
  if (!items.length) return
  const i = items.indexOf(e.target as Focusable)
  let next: Focusable | undefined
  if (e.key === 'ArrowDown') next = items[(i + 1) % items.length]
  else if (e.key === 'ArrowUp') next = items[(i - 1 + items.length) % items.length]
  else if (e.key === 'Home') next = items[0]
  else if (e.key === 'End') next = items[items.length - 1]
  else if (e.key !== 'Tab') return
  e.preventDefault()
  if (next) next.focus()
  else close()
}
