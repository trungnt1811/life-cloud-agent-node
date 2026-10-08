CREATE TABLE IF NOT EXISTS governed_hospital_jobs (
    node_id VARCHAR(128) NOT NULL REFERENCES hospital_governance_states(node_id),
    job_id UUID NOT NULL,
    query_id UUID NOT NULL,
    phase VARCHAR(32) NOT NULL CHECK (phase IN (
        'EXECUTING', 'READY_TO_RELEASE', 'WAITING_APPROVAL', 'SUCCEEDED',
        'POLICY_DENIED', 'DECLINED', 'APPROVAL_EXPIRED', 'TIMED_OUT', 'FAILED', 'CANCELED'
    )),
    record JSONB NOT NULL CHECK (jsonb_typeof(record) = 'object'),
    PRIMARY KEY (node_id, job_id)
);

CREATE TABLE IF NOT EXISTS governed_hospital_outbound_events (
    node_id VARCHAR(128) NOT NULL,
    event_id UUID NOT NULL,
    job_id UUID NOT NULL,
    delivery_state VARCHAR(32) NOT NULL CHECK (delivery_state IN (
        'PREPARED', 'SENT_UNCONFIRMED', 'RECEIVED', 'REJECTED'
    )),
    record JSONB NOT NULL CHECK (jsonb_typeof(record) = 'object'),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (node_id, event_id),
    FOREIGN KEY (node_id, job_id) REFERENCES governed_hospital_jobs(node_id, job_id)
);

CREATE INDEX IF NOT EXISTS idx_governed_hospital_jobs_pending
    ON governed_hospital_jobs (node_id, phase, job_id)
    WHERE phase IN ('EXECUTING', 'READY_TO_RELEASE', 'WAITING_APPROVAL');

CREATE INDEX IF NOT EXISTS idx_governed_hospital_outbound_created
    ON governed_hospital_outbound_events (node_id, created_at DESC, event_id);

ALTER TABLE hospital_governance_audit
    ADD COLUMN IF NOT EXISTS job_id UUID,
    ADD COLUMN IF NOT EXISTS outbound_event_id UUID;
