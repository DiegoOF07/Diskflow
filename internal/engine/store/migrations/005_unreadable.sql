-- Entries the scan could not read. Few rows have an error, so a partial index
-- lists them without reading the millions that don't.
CREATE INDEX entries_unreadable ON entries (scan_id, id) WHERE err <> '';
