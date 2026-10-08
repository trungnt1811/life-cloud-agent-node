CREATE TABLE IF NOT EXISTS hospital_governance_states (
    node_id VARCHAR(128) PRIMARY KEY,
    state JSONB NOT NULL CHECK (jsonb_typeof(state) = 'object')
);

CREATE TABLE IF NOT EXISTS hospital_governance_commands (
    node_id VARCHAR(128) NOT NULL,
    actor VARCHAR(128) NOT NULL CHECK (actor <> ''),
    command_key VARCHAR(255) NOT NULL CHECK (command_key <> ''),
    fingerprint VARCHAR(80) NOT NULL,
    response JSONB NOT NULL CHECK (jsonb_typeof(response) = 'object'),
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (node_id, actor, command_key)
);

CREATE TABLE IF NOT EXISTS hospital_governance_audit (
    event_id UUID PRIMARY KEY,
    node_id VARCHAR(128) NOT NULL,
    event_type VARCHAR(64) NOT NULL CHECK (event_type <> ''),
    actor VARCHAR(128) NOT NULL CHECK (actor <> ''),
    local_revision BIGINT NOT NULL CHECK (local_revision > 0),
    snapshot_revision BIGINT NOT NULL DEFAULT 0 CHECK (snapshot_revision >= 0),
    permit_id VARCHAR(128) NOT NULL DEFAULT '',
    permit_version BIGINT NOT NULL DEFAULT 0,
    policy_hash VARCHAR(80) NOT NULL DEFAULT '',
    occurred_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_hospital_governance_audit_node_time
    ON hospital_governance_audit (node_id, occurred_at DESC, event_id);
