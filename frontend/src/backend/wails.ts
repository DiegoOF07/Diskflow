import {
  CancelScan,
  Children,
  GetRoot,
  ListChildren,
  ListRoots,
  PickFolder,
  RemoveRoot,
  RenameRoot,
  Root,
  Search,
  StartScan,
  Trail,
} from '../../wailsjs/go/desktop/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import type { Backend, Node, Page, Progress, RootInfo, SearchResult, Summary } from './types'

// Event names emitted by internal/desktop/app.go.
const EVENT_PROGRESS = 'scan:progress'
const EVENT_SAVING = 'scan:saving'
const EVENT_DONE = 'scan:done'

export const wailsBackend: Backend = {
  // Go returns nil when the dialog is closed; the generated types don't say so.
  pickFolder: async () => ((await PickFolder()) as RootInfo | null) ?? null,
  listRoots: async () => ((await ListRoots()) as RootInfo[] | null) ?? [],
  getRoot: async (id) => (await GetRoot(id)) as RootInfo,
  renameRoot: (id, name) => RenameRoot(id, name),
  removeRoot: (id) => RemoveRoot(id),
  startScan: (root) => StartScan(root),
  cancelScan: () => CancelScan(),
  // The generated types say model.Node, but Go returns nil for a never-scanned root.
  root: async (root) => ((await Root(root)) as Node | null) ?? null,
  children: async (root, parentId, limit) => (await Children(root, parentId, limit)) as Node[],
  listChildren: async (root, parentId, q) => (await ListChildren(root, parentId, q)) as Page,
  search: async (root, q, limit) => (await Search(root, q, limit)) as SearchResult,
  trail: async (root, id) => (await Trail(root, id)) as Node[],
  // EventsOn returns its own unsubscribe function; EventsOff would remove every listener of the event.
  onProgress: (cb) => EventsOn(EVENT_PROGRESS, (p: Progress) => cb(p)),
  onSaving: (cb) => EventsOn(EVENT_SAVING, () => cb()),
  onDone: (cb) => EventsOn(EVENT_DONE, (s: Summary) => cb(s)),
}
