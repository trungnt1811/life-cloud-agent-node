-- Job Progress Checkpoint (D4): one row per federated query job, so an
-- interrupted execution resumes from its last completed patient chunk
-- (decision 0005).
CREATE TABLE IF NOT EXISTS job_progress (
    job_id VARCHAR(255) PRIMARY KEY,
    -- SHA-256 hex of the canonical query criteria. A job_id re-sent with a
    -- different query must not inherit this row's count.
    criteria_hash VARCHAR(64) NOT NULL,
    -- Cursor into patients.id order; NULL until the first chunk completes.
    last_patient_id UUID,
    -- Matches counted so far. Always written in the same statement as
    -- last_patient_id, so the two can never disagree. Never suppressed.
    running_count BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(16) NOT NULL DEFAULT 'in_progress',
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT job_progress_status_check CHECK (status IN ('in_progress', 'done'))
);

CREATE INDEX IF NOT EXISTS job_progress_status_updated_at_idx
    ON job_progress (status, updated_at);
