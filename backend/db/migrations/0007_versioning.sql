-- 0007_versioning.sql
-- Phase 1 (Part B): make the bi-temporal columns REAL.
--
-- THE DEFECT THIS FIXES
-- Until now every UpsertX did `ON CONFLICT(id) DO UPDATE SET ...`, and nothing
-- ever assigned tx_to / valid_to. The four bi-temporal columns were decorative:
-- amending a circular OVERWROTE the prior clause text, so "what did this clause
-- say before the amendment?" was unanswerable. History was destroyed, not
-- versioned.
--
-- THE FIX
-- A logical fact (circular / clause / obligation) may now have MANY rows - one
-- per system-time version. The row identity moves from `id` to a surrogate
-- `row_uid`; `id` becomes the LOGICAL key shared by every version of the fact.
--
--   row_uid = id || '@' || tx_from
--
-- is deterministic (the project's core idiom), so re-seeding an unchanged fact
-- still upserts the same physical row and stays idempotent.
--
-- "The current version of fact X" is `WHERE id = X AND tx_to IS NULL`, and the
-- partial unique index below makes "at most one current version" a database
-- guarantee rather than a convention.
--
-- WHY THE FOREIGN KEYS ARE DROPPED FROM THE VERSIONED TABLES
-- SQLite requires a foreign key's parent columns to be covered by a NON-PARTIAL
-- unique index. A partial index (`UNIQUE(id) WHERE tx_to IS NULL`) does not
-- qualify - an INSERT into a child fails with "foreign key mismatch". This was
-- verified empirically, not assumed. The alternative - a full `UNIQUE(id)` -
-- is self-defeating: it forbids a second version of the same id, which is the
-- exact capability this migration exists to add.
--
-- So the FK clauses on columns pointing at circular/clause/obligation are
-- removed. What replaces them:
--   * the partial unique index, which still guarantees at most one CURRENT row
--     per logical id (the row every existing query and join resolves against,
--     since every query already filters `tx_to IS NULL`);
--   * store-layer validation, which already re-validates every write.
-- Foreign keys to the NON-versioned tables (control, evidence, policy) are kept
-- exactly as they were.
--
-- Deferred FK checking lets the whole rebuild run inside the migration runner's
-- single transaction; SQLite ignores `PRAGMA foreign_keys` inside a transaction,
-- but honours `defer_foreign_keys`, which re-checks everything at COMMIT.

PRAGMA defer_foreign_keys = ON;

-------------------------------------------------------------------------------
-- Step 1: rebuild the children that reference obligation(id), dropping only
-- that FK. Done first so nothing references `obligation` when it is replaced.
-------------------------------------------------------------------------------
CREATE TABLE obligation_control_new (
    id            TEXT PRIMARY KEY,
    obligation_id TEXT NOT NULL,               -- versioned parent: see header
    control_id    TEXT NOT NULL REFERENCES control(id),
    valid_from    TEXT NOT NULL,
    valid_to      TEXT,
    tx_from       TEXT NOT NULL,
    tx_to         TEXT
);
INSERT INTO obligation_control_new
    SELECT id, obligation_id, control_id, valid_from, valid_to, tx_from, tx_to
    FROM obligation_control;
DROP TABLE obligation_control;
ALTER TABLE obligation_control_new RENAME TO obligation_control;
CREATE INDEX IF NOT EXISTS idx_obl_ctrl_obl  ON obligation_control(obligation_id);
CREATE INDEX IF NOT EXISTS idx_obl_ctrl_ctrl ON obligation_control(control_id);

CREATE TABLE ticket_new (
    id            TEXT PRIMARY KEY,
    obligation_id TEXT NOT NULL,               -- versioned parent: see header
    clause_ref    TEXT NOT NULL,
    title         TEXT NOT NULL,
    detail        TEXT,
    owner         TEXT NOT NULL,
    deadline      TEXT,
    citation      TEXT NOT NULL,
    state         TEXT NOT NULL DEFAULT 'draft'
                    CHECK (state IN ('draft','filed','resolved')),
    valid_from    TEXT NOT NULL,
    valid_to      TEXT,
    tx_from       TEXT NOT NULL,
    tx_to         TEXT
);
INSERT INTO ticket_new
    SELECT id, obligation_id, clause_ref, title, detail, owner, deadline,
           citation, state, valid_from, valid_to, tx_from, tx_to
    FROM ticket;
DROP TABLE ticket;
ALTER TABLE ticket_new RENAME TO ticket;
CREATE INDEX IF NOT EXISTS idx_ticket_obligation ON ticket(obligation_id);
CREATE INDEX IF NOT EXISTS idx_ticket_state      ON ticket(state);

CREATE TABLE signoff_new (
    id              TEXT PRIMARY KEY,
    obligation_id   TEXT NOT NULL,             -- versioned parent: see header
    action          TEXT NOT NULL CHECK (action IN ('approve','reject')),
    obligation_hash TEXT NOT NULL,
    signature       TEXT,
    public_key      TEXT,
    signed_by       TEXT NOT NULL,
    justification   TEXT NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    valid_from      TEXT NOT NULL,
    valid_to        TEXT,
    tx_from         TEXT NOT NULL,
    tx_to           TEXT
);
INSERT INTO signoff_new
    SELECT id, obligation_id, action, obligation_hash, signature, public_key,
           signed_by, justification, created_at, valid_from, valid_to, tx_from, tx_to
    FROM signoff;
DROP TABLE signoff;
ALTER TABLE signoff_new RENAME TO signoff;
CREATE INDEX IF NOT EXISTS idx_signoff_obligation ON signoff(obligation_id);

-- policy_eval's foreign key to policy(id) stays valid (policy is not versioned),
-- but its ROWS must step out of the way first. Dropping a parent table records
-- one deferred violation per surviving child row, and that count is not cleared
-- by rebuilding the child afterwards - it is only cleared by the child rows not
-- existing when the parent is dropped. So the rows are parked in a constraint-free
-- table and restored once the new policy table is in place.
CREATE TABLE policy_eval_parked AS SELECT * FROM policy_eval;
DROP TABLE policy_eval;

CREATE TABLE policy_new (
    id            TEXT PRIMARY KEY,
    obligation_id TEXT NOT NULL,               -- versioned parent: see header
    package_name  TEXT NOT NULL,
    rego          TEXT NOT NULL,
    stage         TEXT NOT NULL DEFAULT 'audit'
                    CHECK (stage IN ('audit','soft','hard')),
    compiled_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    valid_from    TEXT NOT NULL,
    valid_to      TEXT,
    tx_from       TEXT NOT NULL,
    tx_to         TEXT
);
INSERT INTO policy_new
    SELECT id, obligation_id, package_name, rego, stage, compiled_at,
           valid_from, valid_to, tx_from, tx_to
    FROM policy;
DROP TABLE policy;
ALTER TABLE policy_new RENAME TO policy;
CREATE INDEX IF NOT EXISTS idx_policy_obligation ON policy(obligation_id);

-- Restore policy_eval against the rebuilt policy table.
CREATE TABLE policy_eval (
    id            TEXT PRIMARY KEY,
    policy_id     TEXT NOT NULL REFERENCES policy(id),
    obligation_id TEXT NOT NULL,
    input_json    TEXT NOT NULL,
    compliant     INTEGER NOT NULL,
    applicable    INTEGER NOT NULL,
    deny_json     TEXT NOT NULL DEFAULT '[]',
    stage         TEXT NOT NULL,
    blocked       INTEGER NOT NULL DEFAULT 0,
    trace         TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    valid_from    TEXT NOT NULL,
    valid_to      TEXT,
    tx_from       TEXT NOT NULL,
    tx_to         TEXT
);
INSERT INTO policy_eval
    SELECT id, policy_id, obligation_id, input_json, compliant, applicable,
           deny_json, stage, blocked, trace, created_at,
           valid_from, valid_to, tx_from, tx_to
    FROM policy_eval_parked;
DROP TABLE policy_eval_parked;
CREATE INDEX IF NOT EXISTS idx_policy_eval_obligation ON policy_eval(obligation_id);

-------------------------------------------------------------------------------
-- Step 2: obligation gains row_uid. Nothing references it any more.
-------------------------------------------------------------------------------
CREATE TABLE obligation_new (
    row_uid           TEXT PRIMARY KEY,        -- deterministic: id || '@' || tx_from
    id                TEXT NOT NULL,           -- LOGICAL key, shared by all versions
    clause_id         TEXT NOT NULL,           -- versioned parent: see header
    bearer            TEXT NOT NULL,
    deontic_type      TEXT NOT NULL CHECK (deontic_type IN ('MUST','MUST_NOT','MAY')),
    condition         TEXT,
    threshold_json    TEXT NOT NULL DEFAULT '{}',
    deadline          TEXT,
    penalty           TEXT,
    -- Provenance stays MANDATORY (safety invariant #5) on every version.
    source_clause_ref TEXT NOT NULL,
    source_sentence   TEXT NOT NULL,
    confidence        REAL NOT NULL DEFAULT 0,
    status            TEXT NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','needs_review','approved','rejected')),
    -- Added by 0003 via ALTER TABLE. A table rebuild must carry over every
    -- column added by a later migration, not just those in the original CREATE.
    embedding_json    TEXT NOT NULL DEFAULT '[]',
    valid_from        TEXT NOT NULL,
    valid_to          TEXT,
    tx_from           TEXT NOT NULL,
    tx_to             TEXT
);
-- Backfill is lossless: there is exactly one version per id today.
INSERT INTO obligation_new
    SELECT id || '@' || tx_from, id, clause_id, bearer, deontic_type, condition,
           threshold_json, deadline, penalty, source_clause_ref, source_sentence,
           confidence, status, embedding_json, valid_from, valid_to, tx_from, tx_to
    FROM obligation;
DROP TABLE obligation;
ALTER TABLE obligation_new RENAME TO obligation;
CREATE UNIQUE INDEX IF NOT EXISTS ux_obligation_current ON obligation(id) WHERE tx_to IS NULL;
CREATE INDEX IF NOT EXISTS idx_obligation_id     ON obligation(id);
CREATE INDEX IF NOT EXISTS idx_obligation_clause ON obligation(clause_id);
CREATE INDEX IF NOT EXISTS idx_obligation_status ON obligation(status);

-------------------------------------------------------------------------------
-- Step 3: clause gains row_uid (self-referential parent_id FK also dropped -
-- clause is its own child).
-------------------------------------------------------------------------------
CREATE TABLE clause_new (
    row_uid     TEXT PRIMARY KEY,              -- deterministic: id || '@' || tx_from
    id          TEXT NOT NULL,                 -- LOGICAL key: "<circular_id>#<clause_ref>"
    circular_id TEXT NOT NULL,                 -- versioned parent: see header
    clause_ref  TEXT NOT NULL,
    parent_id   TEXT,                          -- versioned parent (self): see header
    heading     TEXT,
    text        TEXT NOT NULL,
    ordinal     INTEGER NOT NULL,
    valid_from  TEXT NOT NULL,
    valid_to    TEXT,
    tx_from     TEXT NOT NULL,
    tx_to       TEXT
);
INSERT INTO clause_new
    SELECT id || '@' || tx_from, id, circular_id, clause_ref, parent_id, heading,
           text, ordinal, valid_from, valid_to, tx_from, tx_to
    FROM clause;
DROP TABLE clause;
ALTER TABLE clause_new RENAME TO clause;
CREATE UNIQUE INDEX IF NOT EXISTS ux_clause_current ON clause(id) WHERE tx_to IS NULL;
CREATE INDEX IF NOT EXISTS idx_clause_id       ON clause(id);
CREATE INDEX IF NOT EXISTS idx_clause_circular ON clause(circular_id);
CREATE INDEX IF NOT EXISTS idx_clause_parent   ON clause(parent_id);
CREATE INDEX IF NOT EXISTS idx_clause_temporal ON clause(valid_from, valid_to, tx_to);

-------------------------------------------------------------------------------
-- Step 4: circular gains row_uid.
-------------------------------------------------------------------------------
CREATE TABLE circular_new (
    row_uid    TEXT PRIMARY KEY,               -- deterministic: id || '@' || tx_from
    id         TEXT NOT NULL,                  -- LOGICAL key, e.g. "SEBI/IA/MC/2024"
    title      TEXT NOT NULL,
    regulator  TEXT NOT NULL,
    issued_on  TEXT NOT NULL,
    source_url TEXT,
    valid_from TEXT NOT NULL,
    valid_to   TEXT,
    tx_from    TEXT NOT NULL,
    tx_to      TEXT
);
INSERT INTO circular_new
    SELECT id || '@' || tx_from, id, title, regulator, issued_on, source_url,
           valid_from, valid_to, tx_from, tx_to
    FROM circular;
DROP TABLE circular;
ALTER TABLE circular_new RENAME TO circular;
CREATE UNIQUE INDEX IF NOT EXISTS ux_circular_current ON circular(id) WHERE tx_to IS NULL;
CREATE INDEX IF NOT EXISTS idx_circular_id ON circular(id);

INSERT INTO app_meta (key, value) VALUES ('schema_phase', '8')
ON CONFLICT(key) DO UPDATE SET value = excluded.value,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now');
