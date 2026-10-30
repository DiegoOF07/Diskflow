import { onScopeDispose, readonly, ref } from 'vue'
import type { Backend, RootInfo } from '../../backend/types'

/**
 * State and actions of the catalog of folders shown on the home screen.
 * Call it inside a component's setup (or an effect scope): it unsubscribes
 * from scan events when that scope ends.
 */
export function useRoots(backend: Backend) {
  const roots = ref<RootInfo[]>([])
  const loaded = ref(false) // distinguishes "no folders yet" from "still loading"
  const removing = ref<number[]>([]) // folders being removed; their button is disabled
  const error = ref('')

  async function attempt<T>(fn: () => Promise<T>): Promise<T | null> {
    error.value = ''
    try {
      return await fn()
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e) // Wails rejects with the Go error string
      return null
    }
  }

  const load = () =>
    attempt(async () => {
      roots.value = await backend.listRoots()
      loaded.value = true
    })

  // A scan that finishes changes that folder's summary.
  const off = backend.onDone(() => void load())
  onScopeDispose(off)

  /** Asks for a folder and adds it; resolves to it, or null if the dialog was closed. */
  const add = async () => {
    const added = await attempt(() => backend.pickFolder())
    if (added) await load()
    return added
  }

  const rename = (id: number, name: string) =>
    attempt(async () => {
      await backend.renameRoot(id, name)
      await load()
    })

  const remove = (id: number) =>
    attempt(async () => {
      if (removing.value.includes(id)) return
      removing.value = [...removing.value, id]
      try {
        await backend.removeRoot(id)
      } finally {
        removing.value = removing.value.filter((r) => r !== id)
      }
      await load()
    })

  return {
    roots: readonly(roots),
    loaded: readonly(loaded),
    removing: readonly(removing),
    error: readonly(error),
    load,
    add,
    rename,
    remove,
  }
}
