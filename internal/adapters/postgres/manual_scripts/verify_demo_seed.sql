-- Read-only check of a node seeded from data/federated_mvp/nodes/<site>/.
-- Mirrors the demo cohort (benchmark/query_definition.json) with the same
-- LATEST_IN_RANGE rule as CountMatchingCohort; compare the last three columns
-- with the `<site>,all,...` row of benchmark/expected_queries.csv.
WITH latest AS (
    SELECT DISTINCT ON (patient_id) id AS specimen_id, patient_id
    FROM specimens
    WHERE collected_at BETWEEN DATE '2024-01-01' AND DATE '2024-12-31'
    ORDER BY patient_id, collected_at DESC, external_specimen_id COLLATE "C" DESC
),
panel AS (
    SELECT
        l.patient_id,
        COUNT(DISTINCT o.field_code) FILTER (
            WHERE o.field_code IN ('HB', 'MCV', 'MCH', 'RBC') AND NOT o.censored
        ) = 4 AS complete_cbc,
        BOOL_OR(o.field_code = 'MCV' AND o.value::numeric < 80) AS mcv_low,
        BOOL_OR(o.field_code = 'MCH' AND o.value::numeric < 27) AS mch_low,
        COUNT(DISTINCT o.field_code) FILTER (
            WHERE o.field_code IN ('HBA0', 'HBA2', 'HBF')
        ) = 3 AS complete_hplc
    FROM latest l
    JOIN lab_observations o ON o.specimen_id = l.specimen_id
    GROUP BY l.patient_id
)
SELECT
    (SELECT COUNT(*) FROM patients) AS patients,
    (SELECT COUNT(*) FROM specimens) AS specimens,
    (SELECT COUNT(*) FROM lab_observations) AS lab_observations,
    COUNT(*) FILTER (WHERE complete_cbc) AS complete_cbc,
    COUNT(*) FILTER (WHERE complete_cbc AND mcv_low AND mch_low) AS matches_rule,
    COUNT(*) FILTER (WHERE complete_cbc AND mcv_low AND mch_low AND complete_hplc) AS matches_with_hplc
FROM panel;
