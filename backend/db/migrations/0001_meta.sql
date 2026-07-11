-- 0001_meta.sql
-- Phase 0: bootstrap metadata only. The bi-temporal obligation graph schema
-- (Obligation, Control, Evidence, Entity + edges) arrives in Phase 1's
-- migration. This first migration exists so the migration runner has something
-- to apply and so we can stamp the schema/app version into the DB file itself.

CREATE TABLE IF NOT EXISTS app_meta (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

INSERT INTO app_meta (key, value)
VALUES ('schema_phase', '0')
ON CONFLICT(key) DO UPDATE SET value = excluded.value;
