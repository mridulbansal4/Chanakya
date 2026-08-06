-- 0011_jobs.sql
-- Phase 2: an in-process job queue, inside SQLite.
--
-- WHY A QUEUE AT ALL
-- A real ingestion run with a live LLM is 40-150s (30+ per-clause extraction
-- calls). That cannot live inside an HTTP request: the client would time out,
-- a refresh would restart the work, and `store.go` pins MaxOpenConns(1) so a
-- long-running handler would block every other request behind it.
--
-- WHY NOT A BROKER
-- Adding Redis or RabbitMQ would break the project's central claim that one
-- SQLite file IS the product. A single-writer database with an atomic
-- claim-by-UPDATE is sufficient for an in-process pool, and it means the job
-- history lives in the same file as the audit trail it belongs to.
--
-- Job rows are NEVER deleted: they are the record of what was ingested, when,
-- and whether it failed. Later phases build the audit pack on top of them.

CREATE TABLE IF NOT EXISTS job (
    id            TEXT PRIMARY KEY,
    kind          TEXT NOT NULL,             -- e.g. "ingest"
    payload_json  TEXT NOT NULL DEFAULT '{}',
    state         TEXT NOT NULL DEFAULT 'queued'
                    CHECK (state IN ('queued','running','succeeded','failed')),
    attempts      INTEGER NOT NULL DEFAULT 0,
    error         TEXT,
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    started_at    TEXT,
    finished_at   TEXT,
    progress_json TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_job_state   ON job(state, created_at);
CREATE INDEX IF NOT EXISTS idx_job_kind    ON job(kind);

INSERT INTO app_meta (key, value) VALUES ('schema_phase', '12')
ON CONFLICT(key) DO UPDATE SET value = excluded.value,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now');
