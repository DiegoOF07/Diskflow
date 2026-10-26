// Mirrors the JSON of internal/engine/model.
export type Kind = 'file' | 'dir' | 'link' | 'other'

/** Why an entry's on-disk size differs from its logical size (bitmask, model.Flags). */
export const Flag = { Sparse: 1, Compressed: 2, Cloud: 4 } as const

export interface Node {
  id: number
  name: string
  kind: Kind
  size: number // logical size
  diskSize: number // space allocated on disk; what the UI ranks by
  flags: number // a directory carries the union of its subtree's flags
  files: number // files in the subtree (1 for a file)
  err?: string
}

export interface Progress {
  files: number
  dirs: number
  bytes: number
  diskBytes: number
  errors: number
}

export type ScanStatus = 'completed' | 'cancelled' | 'failed'

export interface Summary extends Progress {
  scanId: number
  status: ScanStatus
  elapsedMs: number
  error?: string
}

/** One scan attempt as the catalog stores it (model.ScanInfo). */
export interface ScanInfo extends Progress {
  id: number
  status: ScanStatus | 'running'
  startedAt: number // unix seconds
  finishedAt: number // 0 while running
}

/** A folder of the catalog with its scans at a glance (model.RootInfo). */
export interface RootInfo {
  id: number
  name: string // the user's name for it, or the folder's own name
  path: string
  createdAt: number
  index: ScanInfo | null // latest completed scan, the one the explorer shows
  last: ScanInfo | null // latest attempt, whatever its status
}

/** Unsubscribes an event listener. */
export type Off = () => void

/**
 * Everything the UI needs from the desktop shell. Components depend on this
 * interface, never on Wails, so tests can pass a fake and a future Wails v3
 * migration only touches the implementation.
 */
export interface Backend {
  /** Adds the chosen folder to the catalog; resolves to null if the user closes the dialog. */
  pickFolder(): Promise<RootInfo | null>
  listRoots(): Promise<RootInfo[]>
  getRoot(id: number): Promise<RootInfo>
  /** A blank name restores the folder's own name. */
  renameRoot(id: number, name: string): Promise<void>
  /** Takes the folder out of the catalog and deletes its scans; never touches its files. */
  removeRoot(id: number): Promise<void>
  startScan(root: string): Promise<void>
  cancelScan(): Promise<void>
  /** Resolves to null if root has never been scanned. */
  root(root: string): Promise<Node | null>
  children(root: string, parentId: number, limit: number): Promise<Node[]>
  onProgress(cb: (p: Progress) => void): Off
  /** The walk is over and the results are being saved; the scan can no longer be cancelled. */
  onSaving(cb: () => void): Off
  onDone(cb: (s: Summary) => void): Off
}
