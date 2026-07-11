-- 0005_signoff.sql
-- Phase 6: human sign-off records. Approving an obligation produces an Ed25519
-- signature over a canonical hash of the obligation's CONTENT (not its review
-- status), stored here with the mandatory typed justification. Enforcement
-- (Phase 7) is gated on the existence of a valid 'approve' sign-off — the LLM
-- never approves, only a human does.

CREATE TABLE IF NOT EXISTS signoff (
    id              TEXT PRIMARY KEY,        -- deterministic: "so:" + obligation_id
    obligation_id   TEXT NOT NULL REFERENCES obligation(id),
    action          TEXT NOT NULL CHECK (action IN ('approve','reject')),
    obligation_hash TEXT NOT NULL,           -- sha256 hex of canonical obligation
    signature       TEXT,                    -- base64 Ed25519 signature (null on reject)
    public_key      TEXT,                    -- base64 Ed25519 public key
    signed_by       TEXT NOT NULL,
    justification   TEXT NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    valid_from      TEXT NOT NULL,
    valid_to        TEXT,
    tx_from         TEXT NOT NULL,
    tx_to           TEXT
);
CREATE INDEX IF NOT EXISTS idx_signoff_obligation ON signoff(obligation_id);

INSERT INTO app_meta (key, value) VALUES ('schema_phase', '6')
ON CONFLICT(key) DO UPDATE SET value = excluded.value,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now');
