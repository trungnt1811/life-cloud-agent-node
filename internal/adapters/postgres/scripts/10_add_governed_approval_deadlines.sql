ALTER TABLE governed_hospital_jobs
    ADD COLUMN IF NOT EXISTS approval_deadline TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_governed_hospital_approval_deadline
    ON governed_hospital_jobs (node_id, approval_deadline, job_id)
    WHERE phase = 'WAITING_APPROVAL' AND approval_deadline IS NOT NULL;
