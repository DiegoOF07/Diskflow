import { CancelScan, Children, PickFolder, Root, StartScan } from '../../wailsjs/go/desktop/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import type { Backend, Node, Progress, Summary } from './types'

// Event names emitted by internal/desktop/app.go.
const EVENT_PROGRESS = 'scan:progress'
const EVENT_DONE = 'scan:done'

export const wailsBackend: Backend = {
  pickFolder: async () => (await PickFolder()) || null,
  startScan: (root) => StartScan(root),
  cancelScan: () => CancelScan(),
  // The generated types say model.Node, but Go returns nil for a never-scanned root.
  root: async (root) => ((await Root(root)) as Node | null) ?? null,
  children: async (root, parentId, limit) => (await Children(root, parentId, limit)) as Node[],
  // EventsOn returns its own unsubscribe function; EventsOff would remove every listener of the event.
  onProgress: (cb) => EventsOn(EVENT_PROGRESS, (p: Progress) => cb(p)),
  onDone: (cb) => EventsOn(EVENT_DONE, (s: Summary) => cb(s)),
}
