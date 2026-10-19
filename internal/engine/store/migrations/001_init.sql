-- One row per scan attempt. Only 'completed' scans can be the active index.
CREATE TABLE scan_runs (
  id          INTEGER PRIMARY KEY,
  root        TEXT    NOT NULL,
  status      TEXT    NOT NULL CHECK (status IN ('running', 'completed', 'cancelled', 'failed')),
  started_at  INTEGER NOT NULL,
  finished_at INTEGER,
  files       INTEGER NOT NULL DEFAULT 0,
  dirs        INTEGER NOT NULL DEFAULT 0,
  bytes       INTEGER NOT NULL DEFAULT 0,
  errors      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX scan_runs_root ON scan_runs (root, status, id);

-- Entries of a scan. (scan_id, id) is the natural key, so the table is
-- clustered on it (WITHOUT ROWID) instead of carrying a hidden rowid.
CREATE TABLE entries (
  scan_id   INTEGER NOT NULL REFERENCES scan_runs (id),
  id        INTEGER NOT NULL,
  parent_id INTEGER NOT NULL,
  name      TEXT    NOT NULL,
  kind      INTEGER NOT NULL,
  size      INTEGER NOT NULL,
  files     INTEGER NOT NULL,
  mtime     INTEGER NOT NULL,
  err       TEXT    NOT NULL DEFAULT '',
  PRIMARY KEY (scan_id, id)
) WITHOUT ROWID;

-- Serves "children of a directory, biggest first" without sorting.
CREATE INDEX entries_children ON entries (scan_id, parent_id, size DESC);
