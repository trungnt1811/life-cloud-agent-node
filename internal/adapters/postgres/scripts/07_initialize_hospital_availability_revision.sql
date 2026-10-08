UPDATE hospital_governance_states
SET state = jsonb_set(state, '{AvailabilityRevision}', '1'::jsonb)
WHERE NOT (state ? 'AvailabilityRevision');

UPDATE hospital_governance_commands
SET response = jsonb_set(response, '{AvailabilityRevision}', '1'::jsonb)
WHERE NOT (response ? 'AvailabilityRevision');
