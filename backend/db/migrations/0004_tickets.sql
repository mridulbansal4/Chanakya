-- 0004_tickets.sql
-- Phase 5: draft remediation tickets. A ticket is DRAFTED for each detected gap
-- (an obligation with no satisfying evidence path). CHANAKYA never *files*
-- tickets into a customer system — they stay 'draft' as internal records; the
-- state enum includes filed/resolved only for completeness of the lifecycle.

CREATE TABLE IF NOT EXISTS ticket (
    id            TEXT PRIMARY KEY,          -- deterministic: "tkt:" + obligation_id
    obligation_id TEXT NOT NULL REFERENCES obligation(id),
    clause_ref    TEXT NOT NULL,
    title         TEXT NOT NULL,
    detail        TEXT,
    owner         TEXT NOT NULL,
    deadline      TEXT,
    citation      TEXT NOT NULL,             -- the obligation's source sentence
    state         TEXT NOT NULL DEFAULT 'draft'
                    CHECK (state IN ('draft','filed','resolved')),
    valid_from    TEXT NOT NULL,
    valid_to      TEXT,
    tx_from       TEXT NOT NULL,
    tx_to         TEXT
);
CREATE INDEX IF NOT EXISTS idx_ticket_obligation ON ticket(obligation_id);
CREATE INDEX IF NOT EXISTS idx_ticket_state ON ticket(state);

INSERT INTO app_meta (key, value) VALUES ('schema_phase', '5')
ON CONFLICT(key) DO UPDATE SET value = excluded.value,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now');
