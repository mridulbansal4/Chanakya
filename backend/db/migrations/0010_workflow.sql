-- 0010_workflow.sql
-- Phase 4: what workflow synthesis needs on top of 0009's workflow/task/approval.
--
-- 0009 created those three tables with their final shape, so this migration adds
-- only what synthesis itself needs: the link from a generated task back to the
-- semantic unit that justified it, and the columns that let a workflow explain
-- itself to a reviewer.
--
-- THE INVARIANT THIS SCHEMA ENCODES: everything generated is 'draft'. CHANAKYA
-- never dispatches - no email is sent, no ticket is filed, no calendar invite
-- goes out - and the approval gate below is what a human passes before anything
-- is even marked dispatched. Even then, "dispatch" is logged, never performed.

ALTER TABLE workflow ADD COLUMN clause_ref TEXT NOT NULL DEFAULT '';
ALTER TABLE workflow ADD COLUMN verb TEXT NOT NULL DEFAULT '';
ALTER TABLE workflow ADD COLUMN rationale TEXT NOT NULL DEFAULT '';
ALTER TABLE workflow ADD COLUMN generated_at TEXT NOT NULL DEFAULT '';

ALTER TABLE task ADD COLUMN detail TEXT NOT NULL DEFAULT '';
-- The unit of the clause that justified this task, so a task can be traced back
-- to the exact sentence fragment it came from.
ALTER TABLE task ADD COLUMN source_unit_id TEXT NOT NULL DEFAULT '';
-- Set when owners.go could not resolve the role to a real person. The task stays
-- unassigned and is FLAGGED rather than handed to an arbitrary employee just to
-- satisfy a non-null column.
ALTER TABLE task ADD COLUMN owner_unresolved INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_workflow_state ON workflow(state);
CREATE INDEX IF NOT EXISTS idx_task_state     ON task(state);

INSERT INTO app_meta (key, value) VALUES ('schema_phase', '11')
ON CONFLICT(key) DO UPDATE SET value = excluded.value,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now');
