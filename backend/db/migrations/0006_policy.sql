-- 0006_policy.sql
-- Phase 7: Policy-as-Code. A SIGNED (approved) obligation is compiled into a
-- deterministic Rego policy; firm-state input is evaluated by the embedded OPA
-- engine and the result is recorded. Enforcement is STAGED: audit → soft →
-- hard. Policies start at 'audit'; 'hard' (blocking) is only ever reached after
-- an explicit promotion, and only a signed obligation has a policy at all.

CREATE TABLE IF NOT EXISTS policy (
    id            TEXT PRIMARY KEY,          -- deterministic: "pol:" + obligation_id
    obligation_id TEXT NOT NULL REFERENCES obligation(id),
    package_name  TEXT NOT NULL,
    rego          TEXT NOT NULL,
    stage         TEXT NOT NULL DEFAULT 'audit' CHECK (stage IN ('audit','soft','hard')),
    compiled_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    valid_from    TEXT NOT NULL,
    valid_to      TEXT,
    tx_from       TEXT NOT NULL,
    tx_to         TEXT
);
CREATE INDEX IF NOT EXISTS idx_policy_obligation ON policy(obligation_id);

CREATE TABLE IF NOT EXISTS policy_eval (
    id            TEXT PRIMARY KEY,          -- "ev:" + obligation_id (latest kept)
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
CREATE INDEX IF NOT EXISTS idx_policy_eval_obligation ON policy_eval(obligation_id);

INSERT INTO app_meta (key, value) VALUES ('schema_phase', '7')
ON CONFLICT(key) DO UPDATE SET value = excluded.value,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now');
