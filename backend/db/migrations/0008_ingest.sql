-- 0008_ingest.sql
-- Phase 1 (Part C): content-addressed storage for uploaded source documents.
--
-- WHY THE BYTES LIVE IN SQLITE
-- "One file is the product": the database already holds the clause tree, the
-- obligations, and the sign-offs. Keeping the originating PDF beside them means
-- an audit pack can hand over the EXACT bytes that produced an obligation, not
-- a path to a file that may since have moved or changed. Content addressing by
-- sha256 makes re-uploading the same PDF a no-op rather than a duplicate.
--
-- The remaining ingest tables (ingest_run, semantic_unit, clause_lineage,
-- circular_relation, dangling_reference) arrive in Phase 2, when the async
-- pipeline that populates them exists.

CREATE TABLE IF NOT EXISTS document_blob (
    sha256      TEXT PRIMARY KEY,          -- lowercase hex; the content address
    bytes       BLOB NOT NULL,             -- the verbatim uploaded document
    filename    TEXT NOT NULL,
    size        INTEGER NOT NULL,
    page_count  INTEGER NOT NULL,
    uploaded_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

INSERT INTO app_meta (key, value) VALUES ('schema_phase', '9')
ON CONFLICT(key) DO UPDATE SET value = excluded.value,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now');
