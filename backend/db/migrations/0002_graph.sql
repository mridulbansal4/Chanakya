-- 0002_graph.sql
-- Phase 1: the bi-temporal Living Obligation Graph.
--
-- BI-TEMPORAL MODEL (every node/edge table carries all four columns):
--   valid_from / valid_to : WORLD time  — when the fact is/was true in reality
--                           (e.g. a clause in force from its issue date).
--   tx_from   / tx_to     : SYSTEM time — when CHANAKYA knew/knows the fact.
-- Open-ended intervals use NULL for valid_to / tx_to. All timestamps are stored
-- as RFC3339 UTC strings ("2024-05-15T00:00:00Z") so lexical comparison equals
-- chronological comparison — which is what the as-of queries rely on.
--
-- "Current, as-of :asof" therefore means:
--   valid_from <= :asof AND (valid_to IS NULL OR valid_to > :asof)  -- world
--   AND tx_to IS NULL                                               -- latest knowledge
--
-- Graph shape:
--   circular 1──* clause (self-referential tree via parent_id)
--   clause   1──* obligation                    (clause → obligation edge)
--   obligation *──* control  via obligation_control
--   control    *──* evidence via control_evidence
--   entity : the regulated firm(s) an obligation bears upon.

-------------------------------------------------------------------------------
-- Source documents
-------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS circular (
    id         TEXT PRIMARY KEY,          -- e.g. "SEBI/IA/MC/2024"
    title      TEXT NOT NULL,
    regulator  TEXT NOT NULL,
    issued_on  TEXT NOT NULL,             -- RFC3339 UTC
    source_url TEXT,
    valid_from TEXT NOT NULL,
    valid_to   TEXT,
    tx_from    TEXT NOT NULL,
    tx_to      TEXT
);

-------------------------------------------------------------------------------
-- Clause tree (the parsed structure of a circular)
-------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS clause (
    id          TEXT PRIMARY KEY,         -- deterministic: "<circular_id>#<clause_ref>"
    circular_id TEXT NOT NULL REFERENCES circular(id),
    clause_ref  TEXT NOT NULL,            -- human id, e.g. "3.1"
    parent_id   TEXT REFERENCES clause(id),
    heading     TEXT,
    text        TEXT NOT NULL,
    ordinal     INTEGER NOT NULL,         -- document order (for tree sort)
    valid_from  TEXT NOT NULL,
    valid_to    TEXT,
    tx_from     TEXT NOT NULL,
    tx_to       TEXT
);
CREATE INDEX IF NOT EXISTS idx_clause_circular ON clause(circular_id);
CREATE INDEX IF NOT EXISTS idx_clause_parent   ON clause(parent_id);
CREATE INDEX IF NOT EXISTS idx_clause_temporal ON clause(valid_from, valid_to, tx_to);

-------------------------------------------------------------------------------
-- Regulated entities (the bearers of obligations)
-------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS entity (
    id         TEXT PRIMARY KEY,
    kind       TEXT NOT NULL,             -- e.g. "investment_adviser"
    name       TEXT NOT NULL,
    pan        TEXT,
    meta_json  TEXT NOT NULL DEFAULT '{}',
    valid_from TEXT NOT NULL,
    valid_to   TEXT,
    tx_from    TEXT NOT NULL,
    tx_to      TEXT
);

-------------------------------------------------------------------------------
-- Obligations (typed, cited; populated by the Regulation Compiler in Phase 2)
-------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS obligation (
    id                TEXT PRIMARY KEY,
    clause_id         TEXT NOT NULL REFERENCES clause(id),
    bearer            TEXT NOT NULL,
    deontic_type      TEXT NOT NULL CHECK (deontic_type IN ('MUST','MUST_NOT','MAY')),
    condition         TEXT,
    threshold_json    TEXT NOT NULL DEFAULT '{}',
    deadline          TEXT,
    penalty           TEXT,
    -- Provenance is MANDATORY (safety invariant #5): an obligation with no
    -- source clause ref + exact source sentence must never enter the graph.
    source_clause_ref TEXT NOT NULL,
    source_sentence   TEXT NOT NULL,
    confidence        REAL NOT NULL DEFAULT 0,
    status            TEXT NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','needs_review','approved','rejected')),
    valid_from        TEXT NOT NULL,
    valid_to          TEXT,
    tx_from           TEXT NOT NULL,
    tx_to             TEXT
);
CREATE INDEX IF NOT EXISTS idx_obligation_clause ON obligation(clause_id);
CREATE INDEX IF NOT EXISTS idx_obligation_status ON obligation(status);

-------------------------------------------------------------------------------
-- Controls and evidence
-------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS control (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    kind        TEXT,
    valid_from  TEXT NOT NULL,
    valid_to    TEXT,
    tx_from     TEXT NOT NULL,
    tx_to       TEXT
);

CREATE TABLE IF NOT EXISTS evidence (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    source_system TEXT,                   -- read-only origin, e.g. "firm_crm"
    description   TEXT,
    kind          TEXT,
    valid_from    TEXT NOT NULL,
    valid_to      TEXT,
    tx_from       TEXT NOT NULL,
    tx_to         TEXT
);

-------------------------------------------------------------------------------
-- Edges (bi-temporal association tables)
-------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS obligation_control (
    id            TEXT PRIMARY KEY,
    obligation_id TEXT NOT NULL REFERENCES obligation(id),
    control_id    TEXT NOT NULL REFERENCES control(id),
    valid_from    TEXT NOT NULL,
    valid_to      TEXT,
    tx_from       TEXT NOT NULL,
    tx_to         TEXT
);
CREATE INDEX IF NOT EXISTS idx_obl_ctrl_obl  ON obligation_control(obligation_id);
CREATE INDEX IF NOT EXISTS idx_obl_ctrl_ctrl ON obligation_control(control_id);

CREATE TABLE IF NOT EXISTS control_evidence (
    id          TEXT PRIMARY KEY,
    control_id  TEXT NOT NULL REFERENCES control(id),
    evidence_id TEXT NOT NULL REFERENCES evidence(id),
    valid_from  TEXT NOT NULL,
    valid_to    TEXT,
    tx_from     TEXT NOT NULL,
    tx_to       TEXT
);
CREATE INDEX IF NOT EXISTS idx_ctrl_ev_ctrl ON control_evidence(control_id);
CREATE INDEX IF NOT EXISTS idx_ctrl_ev_ev   ON control_evidence(evidence_id);

-- Advance the recorded schema phase.
INSERT INTO app_meta (key, value) VALUES ('schema_phase', '1')
ON CONFLICT(key) DO UPDATE SET value = excluded.value,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now');
