-- Folders the user keeps in the catalog. scan_runs.root holds the same value
-- as root_key; they are related by value, so scans of folders outside the
-- catalog (the developer CLI) keep working.
CREATE TABLE roots (
  id         INTEGER PRIMARY KEY,
  name       TEXT    NOT NULL DEFAULT '', -- '' shows the folder's own name
  path       TEXT    NOT NULL,            -- as the user picked it, original case
  root_key   TEXT    NOT NULL UNIQUE,
  created_at INTEGER NOT NULL
);

-- Folders scanned before the catalog existed. scan_runs only kept the
-- lower-cased key; the root entry of the latest index still has the original
-- path, so it is preferred when there is one.
INSERT INTO roots (path, root_key, created_at)
SELECT COALESCE(
         (SELECT e.name FROM entries e JOIN scan_runs i ON i.id = e.scan_id
          WHERE i.root = s.root AND e.id = 1 ORDER BY i.id DESC LIMIT 1),
         s.root),
       s.root, MIN(s.started_at)
FROM scan_runs s
GROUP BY s.root
ORDER BY MIN(s.started_at);
