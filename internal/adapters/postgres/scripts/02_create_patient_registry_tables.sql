-- Local Patient Registry (D3): patients → specimens → lab_observations
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS patients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_patient_id VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT patients_external_patient_id_key UNIQUE (external_patient_id)
);

CREATE TABLE IF NOT EXISTS specimens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id UUID NOT NULL REFERENCES patients (id) ON DELETE CASCADE,
    collected_at DATE NOT NULL,
    source_dataset VARCHAR(255) NOT NULL DEFAULT '',
    source_file VARCHAR(512) NOT NULL DEFAULT '',
    source_row_number INTEGER NOT NULL DEFAULT 0,
    source_record_id VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT specimens_source_provenance_key UNIQUE (source_dataset, source_file, source_record_id)
);

CREATE INDEX IF NOT EXISTS specimens_patient_id_collected_at_idx
    ON specimens (patient_id, collected_at);

CREATE TABLE IF NOT EXISTS lab_observations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    specimen_id UUID NOT NULL REFERENCES specimens (id) ON DELETE CASCADE,
    field_code VARCHAR(64) NOT NULL,
    value TEXT NOT NULL,
    censored BOOLEAN NOT NULL DEFAULT FALSE,
    raw_value TEXT NOT NULL DEFAULT '',
    raw_unit VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- The UNIQUE constraint already backs (specimen_id, field_code) with a
    -- btree index; no separate CREATE INDEX is needed on the same columns.
    CONSTRAINT lab_observations_specimen_id_field_code_key UNIQUE (specimen_id, field_code)
);
