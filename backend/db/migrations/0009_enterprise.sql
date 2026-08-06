-- 0009_enterprise.sql
-- Phase 3: the living enterprise graph.
--
-- WHY THIS EXISTS
-- Until now the mock firm existed only as PDFs on disk and constants in a
-- frontend file. Neither is queryable and neither is as-of-able, so the
-- product's central claim - "a real company changes when a regulation changes" -
-- could only be asserted, never demonstrated. Promoting the firm into the
-- database makes it the same kind of object as the regulation: a bi-temporal
-- graph you can traverse and reconstruct as of any date.
--
-- TWO NAMESPACES, ONE SEAM
-- The regulatory graph (external, immutable, regulator-authored) and the
-- enterprise graph (internal, mutable, firm-authored) are deliberately SEPARATE.
-- They are different kinds of fact with different owners and different rates of
-- change, and merging them would mean an inference about the firm could be
-- mistaken for something the regulator wrote. They join at exactly one place:
-- `control` - the firm's answer to an obligation - plus the `binds_to` edges
-- below, which are explicitly marked as INFERENCE, not assertion.
--
-- Every table carries the same four bi-temporal columns as every existing table,
-- so "which agreements were on template v1 on 1 March 2025?" is answerable.

-------------------------------------------------------------------------------
-- Org
-------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS department (
    id               TEXT PRIMARY KEY,
    name             TEXT NOT NULL,
    head_employee_id TEXT,
    function         TEXT,
    valid_from       TEXT NOT NULL,
    valid_to         TEXT,
    tx_from          TEXT NOT NULL,
    tx_to            TEXT
);

CREATE TABLE IF NOT EXISTS employee (
    id             TEXT PRIMARY KEY,
    name           TEXT NOT NULL,
    role           TEXT NOT NULL,
    department_id  TEXT,
    email          TEXT,
    certifications TEXT NOT NULL DEFAULT '[]',   -- JSON array
    manager_id     TEXT,                          -- org chart; may be NULL
    valid_from     TEXT NOT NULL,
    valid_to       TEXT,
    tx_from        TEXT NOT NULL,
    tx_to          TEXT
);
CREATE INDEX IF NOT EXISTS idx_employee_dept    ON employee(department_id);
CREATE INDEX IF NOT EXISTS idx_employee_manager ON employee(manager_id);

-------------------------------------------------------------------------------
-- Clients and their agreements
-------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS client (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    segment      TEXT NOT NULL,                  -- retail | hni | institutional | nri
    onboarded_on TEXT NOT NULL,
    risk_profile TEXT,
    adviser_id   TEXT,
    -- service_kind is what makes the clause 4.2 segregation breach DISCOVERABLE
    -- by traversal (an adviser holding both advisory and distribution clients)
    -- rather than hardcoded anywhere as "the demo bug".
    service_kind TEXT NOT NULL DEFAULT 'advisory', -- advisory | distribution
    valid_from   TEXT NOT NULL,
    valid_to     TEXT,
    tx_from      TEXT NOT NULL,
    tx_to        TEXT
);
CREATE INDEX IF NOT EXISTS idx_client_adviser ON client(adviser_id);
CREATE INDEX IF NOT EXISTS idx_client_segment ON client(segment);

CREATE TABLE IF NOT EXISTS agreement (
    id               TEXT PRIMARY KEY,
    client_id        TEXT NOT NULL,
    template_version TEXT NOT NULL,              -- "v1" | "v2"
    signed_on        TEXT NOT NULL,
    doc_id           TEXT,
    valid_from       TEXT NOT NULL,
    valid_to         TEXT,
    tx_from          TEXT NOT NULL,
    tx_to            TEXT
);
CREATE INDEX IF NOT EXISTS idx_agreement_client   ON agreement(client_id);
CREATE INDEX IF NOT EXISTS idx_agreement_template ON agreement(template_version);

-------------------------------------------------------------------------------
-- Firm artefacts and systems
-------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS document (
    id            TEXT PRIMARY KEY,
    kind          TEXT NOT NULL,                 -- policy | sop | manual | register | corporate
    title         TEXT NOT NULL,
    version       INTEGER NOT NULL DEFAULT 1,
    owner_dept    TEXT,
    blob_sha      TEXT,
    status        TEXT NOT NULL DEFAULT 'current',
    last_reviewed TEXT,
    valid_from    TEXT NOT NULL,
    valid_to      TEXT,
    tx_from       TEXT NOT NULL,
    tx_to         TEXT
);
CREATE INDEX IF NOT EXISTS idx_document_dept ON document(owner_dept);

CREATE TABLE IF NOT EXISTS register (
    id             TEXT PRIMARY KEY,
    kind           TEXT NOT NULL,                -- client | complaint | training | exception | ...
    schema_json    TEXT NOT NULL DEFAULT '{}',
    row_count      INTEGER NOT NULL DEFAULT 0,
    source_system  TEXT,
    last_updated   TEXT,
    owner_dept     TEXT,
    valid_from     TEXT NOT NULL,
    valid_to       TEXT,
    tx_from        TEXT NOT NULL,
    tx_to          TEXT
);

CREATE TABLE IF NOT EXISTS system (
    id           TEXT PRIMARY KEY,
    kind         TEXT NOT NULL,                  -- crm | hrms | dms | email | billing | archive | grc
    vendor       TEXT,
    connector_id TEXT,                           -- wired to a read-only adapter in Phase 4
    criticality  TEXT,
    owner_dept   TEXT,
    valid_from   TEXT NOT NULL,
    valid_to     TEXT,
    tx_from      TEXT NOT NULL,
    tx_to        TEXT
);

-------------------------------------------------------------------------------
-- Workflow + task + approval.
-- Created here with their final shape so Phase 4 needs no schema migration;
-- Phase 3 does not populate them.
-------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS workflow (
    id            TEXT PRIMARY KEY,
    template      TEXT NOT NULL,
    obligation_id TEXT NOT NULL,
    state         TEXT NOT NULL DEFAULT 'draft',
    sla           TEXT,
    title         TEXT,
    valid_from    TEXT NOT NULL,
    valid_to      TEXT,
    tx_from       TEXT NOT NULL,
    tx_to         TEXT
);
CREATE INDEX IF NOT EXISTS idx_workflow_obligation ON workflow(obligation_id);

CREATE TABLE IF NOT EXISTS task (
    id                TEXT PRIMARY KEY,
    workflow_id       TEXT NOT NULL,
    owner_employee_id TEXT,                      -- NULL when no owner could be resolved
    owner_role        TEXT,
    title             TEXT NOT NULL,
    -- Draft-only, forever. CHANAKYA never dispatches: no email is sent and no
    -- ticket is filed. The other states exist for lifecycle completeness only.
    state             TEXT NOT NULL DEFAULT 'draft'
                        CHECK (state IN ('draft','dispatched','done')),
    deadline          TEXT,
    ordinal           INTEGER NOT NULL DEFAULT 0,
    depends_on        TEXT NOT NULL DEFAULT '[]', -- JSON array of task ids (the DAG)
    valid_from        TEXT NOT NULL,
    valid_to          TEXT,
    tx_from           TEXT NOT NULL,
    tx_to             TEXT
);
CREATE INDEX IF NOT EXISTS idx_task_workflow ON task(workflow_id);

CREATE TABLE IF NOT EXISTS approval (
    id           TEXT PRIMARY KEY,
    subject_type TEXT NOT NULL,                  -- workflow | document | ...
    subject_id   TEXT NOT NULL,
    approver_id  TEXT,
    approver     TEXT,
    decision     TEXT NOT NULL CHECK (decision IN ('approved','rejected')),
    note         TEXT,
    decided_at   TEXT,
    valid_from   TEXT NOT NULL,
    valid_to     TEXT,
    tx_from      TEXT NOT NULL,
    tx_to        TEXT
);
CREATE INDEX IF NOT EXISTS idx_approval_subject ON approval(subject_type, subject_id);

-------------------------------------------------------------------------------
-- Risk, training, communications, calendar
-------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS risk (
    id         TEXT PRIMARY KEY,
    title      TEXT NOT NULL,
    likelihood TEXT,
    impact     TEXT,
    owner_dept TEXT,
    control_id TEXT,
    valid_from TEXT NOT NULL,
    valid_to   TEXT,
    tx_from    TEXT NOT NULL,
    tx_to      TEXT
);

CREATE TABLE IF NOT EXISTS training (
    id               TEXT PRIMARY KEY,
    employee_id      TEXT NOT NULL,
    course           TEXT NOT NULL,
    completed_on     TEXT,                       -- NULL = not completed (a gap)
    certificate_doc  TEXT,
    period           TEXT,                       -- e.g. "2025-Q2"
    valid_from       TEXT NOT NULL,
    valid_to         TEXT,
    tx_from          TEXT NOT NULL,
    tx_to            TEXT
);
CREATE INDEX IF NOT EXISTS idx_training_employee ON training(employee_id);

CREATE TABLE IF NOT EXISTS communication (
    id           TEXT PRIMARY KEY,
    kind         TEXT NOT NULL CHECK (kind IN ('email','meeting','call')),
    subject      TEXT,
    participants TEXT NOT NULL DEFAULT '[]',     -- JSON array of employee/client ids
    thread_id    TEXT,
    sent_on      TEXT,
    system_id    TEXT,
    valid_from   TEXT NOT NULL,
    valid_to     TEXT,
    tx_from      TEXT NOT NULL,
    tx_to        TEXT
);
CREATE INDEX IF NOT EXISTS idx_communication_thread ON communication(thread_id);

CREATE TABLE IF NOT EXISTS calendar_event (
    id         TEXT PRIMARY KEY,
    title      TEXT NOT NULL,
    starts_at  TEXT NOT NULL,
    attendees  TEXT NOT NULL DEFAULT '[]',       -- JSON array
    kind       TEXT,
    valid_from TEXT NOT NULL,
    valid_to   TEXT,
    tx_from    TEXT NOT NULL,
    tx_to      TEXT
);

-------------------------------------------------------------------------------
-- The projection edge: obligation -> firm object.
--
-- This is INFERENCE, not assertion, and the schema says so: every binding
-- carries a confidence and a human-confirmation flag. Writing it as plain fact
-- would let a guess about which policy governs a clause look identical to a
-- regulator-authored statement, which is precisely the confusion the two
-- namespaces exist to prevent.
--
-- The UNIQUE constraint dedupes on (obligation, target_type, target_id): two
-- obligations binding to the same document is one relationship per obligation,
-- never a duplicate edge.
-------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS binds_to (
    id             TEXT PRIMARY KEY,
    obligation_id  TEXT NOT NULL,
    target_type    TEXT NOT NULL CHECK (target_type IN
                     ('document','register','agreement','system','client_segment')),
    target_id      TEXT NOT NULL,
    confidence     REAL NOT NULL DEFAULT 0,
    human_confirmed INTEGER NOT NULL DEFAULT 0,
    rationale      TEXT,
    valid_from     TEXT NOT NULL,
    valid_to       TEXT,
    tx_from        TEXT NOT NULL,
    tx_to          TEXT,
    UNIQUE (obligation_id, target_type, target_id)
);
CREATE INDEX IF NOT EXISTS idx_binds_to_obligation ON binds_to(obligation_id);
CREATE INDEX IF NOT EXISTS idx_binds_to_target     ON binds_to(target_type, target_id);

INSERT INTO app_meta (key, value) VALUES ('schema_phase', '10')
ON CONFLICT(key) DO UPDATE SET value = excluded.value,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now');
