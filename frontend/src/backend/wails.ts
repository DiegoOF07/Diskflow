import {
  CancelScan,
  CancelSearch,
  Children,
  CopyPath,
  GetRoot,
  ListChildren,
  ListRoots,
  PickFolder,
  RemoveRoot,
  RenameRoot,
  ScanState,
  Root,
  Search,
  ShowInExplorer,
  StartScan,
  Trail,
  Unreadable,
} from '../../wailsjs/go/desktop/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { toBackendError } from './errors'
import type {
  Backend,
  Node,
  Page,
  Progress,
  RootInfo,
  ScanState as State,
  SearchResult,
  Summary,
} from './types'

// Event names emitted by internal/desktop/app.go.
const EVENT_PROGRESS = 'scan:progress'
const EVENT_SAVING = 'scan:saving'
const EVENT_DONE = 'scan:done'

// Go rejects with {code, detail} (internal/desktop/errors.go); call() turns
// that into a BackendError, so callers never see what Wails sends.
const call = <T>(p: Promise<T>): Promise<T> =>
  p.catch((e: unknown) => {
    throw toBackendError(e)
  })

export const wailsBackend: Backend = {
  // Go returns nil when the dialog is closed; the generated types don't say so.
  pickFolder: async () => ((await call(PickFolder())) as RootInfo | null) ?? null,
  listRoots: async () => ((await call(ListRoots())) as RootInfo[] | null) ?? [],
  getRoot: async (id) => (await call(GetRoot(id))) as RootInfo,
  renameRoot: (id, name) => call(RenameRoot(id, name)),
  removeRoot: (id) => call(RemoveRoot(id)),
  startScan: (root) => call(StartScan(root)),
  cancelScan: () => call(CancelScan()),
  scanState: async (root) => (await call(ScanState(root))) as State,
  // The generated types say model.Node, but Go returns nil for a never-scanned root.
  root: async (root) => ((await call(Root(root))) as Node | null) ?? null,
  children: async (root, parentId, limit) => (await call(Children(root, parentId, limit))) as Node[],
  listChildren: async (root, parentId, q) => (await call(ListChildren(root, parentId, q))) as Page,
  search: async (root, q, limit) => (await call(Search(root, q, limit))) as SearchResult,
  cancelSearch: () => call(CancelSearch()),
  showInExplorer: (root, id) => call(ShowInExplorer(root, id)),
  copyPath: (root, id) => call(CopyPath(root, id)),
  unreadable: async (root, limit) => (await call(Unreadable(root, limit))) as SearchResult,
  trail: async (root, id) => (await call(Trail(root, id))) as Node[],
  // EventsOn returns its own unsubscribe function; EventsOff would remove every listener of the event.
  onProgress: (cb) => EventsOn(EVENT_PROGRESS, (p: Progress) => cb(p)),
  onSaving: (cb) => EventsOn(EVENT_SAVING, () => cb()),
  onDone: (cb) => EventsOn(EVENT_DONE, (s: Summary) => cb(s)),
}
