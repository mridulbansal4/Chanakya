-- 0012_ingest_pipeline.sql
-- Phase 2: the tables Stages 3-6 and the approval gate need.
--
-- 0008_ingest.sql has already shipped and been applied, so these arrive as a
-- separate migration rather than an edit to a migration other databases have
-- already recorded.
--
-- THE CENTRAL INVARIANT THESE TABLES EXIST TO SUPPORT
-- An uploaded circular does NOT enter the regulatory graph until a human
-- approves it. Everything a run produces is held here, outside circular/clause/
-- obligation, until POST /api/ingest/:id/approve commits it in ONE transaction.
-- A proposal is therefore always distinguishable from an accepted fact.

-------------------------------------------------------------------------------
-- One row per ingestion attempt: the audit record of what was proposed.
-------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ingest_run (
    id            TEXT PRIMARY KEY,          -- deterministic: "ing:" + sha256[:16]
    job_id        TEXT,
    sha256        TEXT NOT NULL,             -- the document_blob content address
    filename      TEXT NOT NULL,
    state         TEXT NOT NULL DEFAULT 'queued'
                    CHECK (state IN ('queued','running','preview','approved','discarded','failed')),
    stage         TEXT NOT NULL DEFAULT '',  -- last stage entered, for error reporting
    doc_kind      TEXT NOT NULL DEFAULT '',
    circular_id   TEXT NOT NULL DEFAULT '',
    -- The whole proposal (meta + clauses + obligations + relations + refs) as
    -- JSON. Held opaquely on purpose: it is NOT graph data yet, and giving it
    -- real tables would blur the line the approval gate draws.
    proposal_json TEXT NOT NULL DEFAULT '{}',
    error         TEXT,
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    approved_at   TEXT,
    approved_by   TEXT,
    valid_from    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    valid_to      TEXT,
    tx_from       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    tx_to         TEXT
);
CREATE INDEX IF NOT EXISTS idx_ingest_run_sha   ON ingest_run(sha256);
CREATE INDEX IF NOT EXISTS idx_ingest_run_state ON ingest_run(state);

-------------------------------------------------------------------------------
-- Stage 5 output: atomic normative units within a clause.
-------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS semantic_unit (
    id           TEXT PRIMARY KEY,           -- deterministic: clause_id + "/u" + ordinal
    clause_id    TEXT NOT NULL,              -- versioned parent (see 0007 header)
    ordinal      INTEGER NOT NULL,
    role         TEXT NOT NULL CHECK (role IN
                    ('norm','condition','exception','deadline','penalty',
                     'definition','cross_ref','scope')),
    text         TEXT NOT NULL,
    -- Character offsets INTO THE PARENT CLAUSE TEXT. Without these the citation
    -- gate could not operate at unit level: a unit's text must remain locatable
    -- in the verbatim clause it was cut from.
    start_offset INTEGER NOT NULL,
    end_offset   INTEGER NOT NULL,
    valid_from   TEXT NOT NULL,
    valid_to     TEXT,
    tx_from      TEXT NOT NULL,
    tx_to        TEXT
);
CREATE INDEX IF NOT EXISTS idx_semantic_unit_clause ON semantic_unit(clause_id);

-------------------------------------------------------------------------------
-- How a clause version relates to its predecessor (populated by Phase 3's
-- amendment matcher; created here so the shape is settled).
-------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS clause_lineage (
    id             TEXT PRIMARY KEY,
    new_clause_id  TEXT NOT NULL,
    old_clause_id  TEXT,
    relation       TEXT NOT NULL CHECK (relation IN ('unchanged','modified','added','deleted')),
    score          REAL NOT NULL DEFAULT 0,
    valid_from     TEXT NOT NULL,
    valid_to       TEXT,
    tx_from        TEXT NOT NULL,
    tx_to          TEXT
);
CREATE INDEX IF NOT EXISTS idx_clause_lineage_new ON clause_lineage(new_clause_id);
CREATE INDEX IF NOT EXISTS idx_clause_lineage_old ON clause_lineage(old_clause_id);

-------------------------------------------------------------------------------
-- Document-to-document relations extracted by Stage 3 and resolved by Stage 6.
-------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS circular_relation (
    id            TEXT PRIMARY KEY,          -- deterministic: from|kind|to
    from_circular TEXT NOT NULL,
    to_ref        TEXT NOT NULL,             -- the cited circular number as printed
    to_circular   TEXT,                      -- resolved id, NULL while unresolved
    kind          TEXT NOT NULL CHECK (kind IN ('supersedes','amends','references')),
    valid_from    TEXT NOT NULL,
    valid_to      TEXT,
    tx_from       TEXT NOT NULL,
    tx_to         TEXT
);
CREATE INDEX IF NOT EXISTS idx_circular_relation_from ON circular_relation(from_circular);

-------------------------------------------------------------------------------
-- A reference Stage 6 could not resolve.
--
-- These are recorded, never dropped: a citation the pipeline could not follow is
-- exactly the kind of thing a reviewer must see. Silently discarding it would
-- make the graph look more complete than it is.
-------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dangling_reference (
    id            TEXT PRIMARY KEY,
    circular_id   TEXT NOT NULL,
    clause_id     TEXT NOT NULL,
    raw_text      TEXT NOT NULL,             -- verbatim, as printed in the clause
    kind          TEXT NOT NULL,             -- clause | regulation | annexure | document | vague
    reason        TEXT NOT NULL,
    valid_from    TEXT NOT NULL,
    valid_to      TEXT,
    tx_from       TEXT NOT NULL,
    tx_to         TEXT
);
CREATE INDEX IF NOT EXISTS idx_dangling_reference_circular ON dangling_reference(circular_id);

INSERT INTO app_meta (key, value) VALUES ('schema_phase', '13')
ON CONFLICT(key) DO UPDATE SET value = excluded.value,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now');
