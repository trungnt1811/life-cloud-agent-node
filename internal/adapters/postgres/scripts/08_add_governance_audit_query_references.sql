ALTER TABLE hospital_governance_audit
    ADD COLUMN IF NOT EXISTS query_id uuid,
    ADD COLUMN IF NOT EXISTS command_id uuid;

CREATE INDEX IF NOT EXISTS idx_hospital_governance_audit_query
    ON hospital_governance_audit (query_id, occurred_at, event_id)
    WHERE query_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_hospital_governance_audit_command
    ON hospital_governance_audit (command_id)
    WHERE command_id IS NOT NULL;
