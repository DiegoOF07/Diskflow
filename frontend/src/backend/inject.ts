import { inject, type InjectionKey } from 'vue'
import type { Backend } from './types'

export const BackendKey: InjectionKey<Backend> = Symbol('backend')

/** Returns the Backend provided by main.ts (or by a test). */
export function useBackend(): Backend {
  const backend = inject(BackendKey)
  if (!backend) throw new Error('no Backend provided')
  return backend
}
