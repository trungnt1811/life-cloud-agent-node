# Query Field Dictionary (schema v1)

Global maximum set of `field_code` values the platform supports for
`query_schema_version = 1`. Life Cloud owns this list; it is identical for
every node. Each hospital’s local `enabled_query_fields` store must be a
**subset** of this set (decision 0002).

Wire representation: protobuf `string field_code` (decision 0004). This
document is the normative dictionary, not a protobuf enum.

Canonical codes match the demo mappings under
`life-cloud/data/federated_mvp/nodes/*/mapping.json` (`tests.*.code`).

| field_code | Kind | Canonical unit (demo) | Notes |
|---|---|---|---|
| `HB` | measurement | g/dL | Hemoglobin |
| `MCV` | measurement | fL | Mean corpuscular volume |
| `MCH` | measurement | pg | Mean corpuscular hemoglobin |
| `RBC` | measurement | 10^12/L | Red blood cell count |
| `MCHC` | measurement | g/dL | Mean corpuscular hemoglobin concentration |
| `RDW` | measurement | % | Red cell distribution width |
| `HBA0` | measurement | % | HbA0 (HPLC) |
| `HBA2` | measurement | % | HbA2 (HPLC) |
| `HBF` | measurement | % | HbF (HPLC); may be censored in source data |

## Operators (schema v1)

Defined on the wire as a protobuf enum (decision 0004): `EQ`, `NE`, `LT`,
`LTE`, `GT`, `GTE`. Semantic validation (layer 3) requires operators to match
field kinds; for v1 all listed fields are numeric measurements, so all six
ops are eligible when the condition uses `number_value`.

## Demo cohort compile hint

The engineering fixture
`life-cloud/data/federated_mvp/benchmark/query_definition.json` compiles to
schema v1 approximately as:

- `required_panels`: `HB,MCV,MCH,RBC` with `EXACT`; `HBA0,HBA2,HBF` with
  `ALLOW_CENSORED`
- `conditions`: `MCV LT 80`, `MCH LT 27` (`number_value` decimal strings)
- `time_range`: `2024-01-01` .. `2024-12-31`
- `specimen_policy`: `LATEST_IN_RANGE`

See decision 0004 for the full mapping table and rejection rules.
