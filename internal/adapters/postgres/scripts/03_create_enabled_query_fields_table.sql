-- Enabled Query Fields (D5): per-node whitelist subset of schema v1.
-- Seeded disabled so each hospital must explicitly opt in.
-- Keep this INSERT list identical to internal/domain/queryfields.SchemaV1FieldCodes
-- (enforced by queryfields.TestMigrationSeedMatchesSchemaV1FieldCodes).
CREATE TABLE IF NOT EXISTS enabled_query_fields (
    field_code VARCHAR(64) PRIMARY KEY,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL DEFAULT ''
);

INSERT INTO enabled_query_fields (field_code, enabled, updated_by) VALUES
    ('HB', FALSE, ''),
    ('MCV', FALSE, ''),
    ('MCH', FALSE, ''),
    ('RBC', FALSE, ''),
    ('MCHC', FALSE, ''),
    ('RDW', FALSE, ''),
    ('HBA0', FALSE, ''),
    ('HBA2', FALSE, ''),
    ('HBF', FALSE, '')
ON CONFLICT (field_code) DO NOTHING;
