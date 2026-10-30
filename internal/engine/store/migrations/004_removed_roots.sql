-- Removing a folder only marks its scans; Prune deletes their entries later,
-- in short chunks, and then the rows. Removed scans are invisible meanwhile.
ALTER TABLE scan_runs ADD COLUMN removed INTEGER NOT NULL DEFAULT 0;
