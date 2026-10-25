-- On-disk size next to the logical one. Older scans only know the logical
-- size, so it is copied as an approximation until the root is scanned again.
ALTER TABLE entries ADD COLUMN disk_size INTEGER NOT NULL DEFAULT 0;
ALTER TABLE entries ADD COLUMN flags INTEGER NOT NULL DEFAULT 0;
UPDATE entries SET disk_size = size;

ALTER TABLE scan_runs ADD COLUMN disk_bytes INTEGER NOT NULL DEFAULT 0;
UPDATE scan_runs SET disk_bytes = bytes;

-- Lists are now ordered by on-disk size. The old index would only slow writes.
DROP INDEX entries_children;
CREATE INDEX entries_children ON entries (scan_id, parent_id, disk_size DESC);
