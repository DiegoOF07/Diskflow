import { onScopeDispose, readonly, ref } from 'vue'
import { errorMessage } from '../../backend/errors'
import type { Backend, RootInfo } from '../../backend/types'

/**
 * State and actions of the catalog of folders shown on the home screen.
 * Call it inside a component's setup (or an effect scope): it unsubscribes
 * from scan events when that scope ends.
 */
export function useRoots(backend: Backend) {
  const roots = ref<RootInfo[]>([])
  const loaded = ref(false) // distinguishes "no folders yet" from "still loading"
  const loading = ref(false)
  const adding = ref(false) // the folder dialog is open
  const removing = ref<number[]>([]) // folders being removed; their button is disabled
  const error = ref('')

  async function attempt<T>(fn: () => Promise<T>): Promise<T | null> {
    error.value = ''
    try {
      return await fn()
    } catch (e) {
      error.value = errorMessage(e)
      return null
    }
  }

  const load = () =>
    attempt(async () => {
      loading.value = true
      try {
        roots.value = await backend.listRoots()
        loaded.value = true
      } finally {
        loading.value = false
      }
    })

  // A scan that finishes changes that folder's summary.
  const off = backend.onDone(() => void load())
  onScopeDispose(off)

  /** Asks for a folder and adds it; resolves to it, or null if the dialog was closed. */
  const add = async () => {
    if (adding.value) return null // one dialog at a time
    adding.value = true
    try {
      const added = await attempt(() => backend.pickFolder())
      if (added) await load()
      return added
    } finally {
      adding.value = false
    }
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
    loading: readonly(loading),
    adding: readonly(adding),
    removing: readonly(removing),
    error: readonly(error),
    load,
    add,
    rename,
    remove,
  }
}
