# Runbook: Seed a Node with Demo Hospital Data

## Scope

Bring up one Life Cloud data node (this agent) with a populated local patient
registry (D3) from the demo hospital exports in `data/federated_mvp/`, enable
its field whitelist (D5), and verify the node answers the demo cohort exactly
as the benchmark expects. Works for a local machine and for a server running
the repository's `docker-compose.yml`.

Connecting the node to a control center is a separate step; see
`docs/product/deployed-node-instance.md` and
`docs/product/node-agent-connection-guide.md`.

## The One Rule: One Node = One Site

Each node instance (each Postgres database) must be seeded with **exactly one**
site: `VN_A`, `VN_B`, or `VN_C`.

The three sites reuse the same local IDs (patient `0000001`, specimen
`S0000001`, same `source_dataset`), and D3 identifies patients and specimens by
those IDs without a site column. Loading a second site into the same database
silently merges it into the first. Observed in rehearsal: loading `VN_B` into a
`VN_A` database left the patient count at 6211 and turned the demo answer into
1309 instead of 1089. If this happens, reset (see below) and seed again.

## Prerequisites

- A checkout of this repository (the data ships in it: `data/federated_mvp/`).
- Docker. Go 1.24+ on the host is optional — a `golang` container works too.
- Postgres 15 reachable by the node (the compose `db` service is fine).
- Admin credentials chosen for `ADMIN_BASIC_AUTH_USER` / `ADMIN_BASIC_AUTH_PASS`
  (needed to enable D5 fields; the admin routes are not registered without
  both).

## Steps

Pick the site for this node once and reuse it throughout:

```bash
SITE=VN_A   # or VN_B, VN_C
```

### 1. Start Postgres and apply migrations

With the repository's compose file:

```bash
docker compose up -d db
docker compose run --rm app ./migrate
```

`./migrate` applies every file in `internal/adapters/postgres/scripts/`; all of
them are safe to re-run. It creates the D3/D4/D5 tables and seeds the nine D5
fields as **disabled**. Setting `ENABLE_AUTO_MIGRATE=true` on the `app`
service does the same at startup.

Without compose: `make migrate` with `DB_*` pointing at your database.

### 2. Ingest the site's exports

`cmd/ingest` is not in the runtime image, so run it from the checkout. Always
pass both files: `reexport.csv` exercises re-export deduplication and revision
handling.

Option A — Go on the host (compose publishes Postgres on `localhost:5432`,
password `password`):

```bash
DB_HOST=localhost DB_PORT=5432 DB_USER=postgres DB_PASSWORD=password \
DB_NAME=life_cloud_agent_node \
go run ./cmd/ingest -profile "$SITE" \
  "data/federated_mvp/nodes/$SITE/raw/results.csv" \
  "data/federated_mvp/nodes/$SITE/raw/reexport.csv"
```

Option B — no Go on the host; join the compose network (find its name with
`docker network ls | grep app-network`, usually
`life-cloud-agent-node_app-network`):

```bash
docker run --rm --network life-cloud-agent-node_app-network \
  -v "$PWD":/src -w /src \
  -e DB_HOST=db -e DB_PORT=5432 -e DB_USER=postgres -e DB_PASSWORD=password \
  -e DB_NAME=life_cloud_agent_node \
  golang:1.24.7-alpine \
  go run ./cmd/ingest -profile "$SITE" \
    "data/federated_mvp/nodes/$SITE/raw/results.csv" \
    "data/federated_mvp/nodes/$SITE/raw/reexport.csv"
```

A successful run ends with one line like:

```text
ingest complete profile=VN_A files=2 specimens_saved=6288 observations_saved=56454 anomalies=140
```

`specimens_saved` / `observations_saved` count rows processed, including
duplicates that were deduplicated — use step 3 for the real totals. The
anomaly count is expected: the data deliberately contains unknown codes,
missing units, ambiguous dates, and similar faults, which are recorded and
skipped, never guessed. Ingest takes seconds per site and is idempotent:
running it again leaves every count unchanged.

### 3. Verify the registry against the benchmark

```bash
docker compose exec -T db psql -U postgres -d life_cloud_agent_node \
  < internal/adapters/postgres/manual_scripts/verify_demo_seed.sql
```

The script is read-only. It counts D3 rows and evaluates the demo cohort
(`data/federated_mvp/benchmark/query_definition.json`: 2024 window, latest
specimen per patient, CBC complete and exact, `MCV < 80`, `MCH < 27`, HPLC
complete) with the same specimen-selection rule the node uses. Expected output:

| Site | patients | specimens | lab_observations | complete_cbc | matches_rule | matches_with_hplc | ingest anomalies |
|---|---:|---:|---:|---:|---:|---:|---:|
| VN_A | 6211 | 6212 | 55770 | 6076 | 1090 | 1089 | 140 |
| VN_B | 2957 | 2958 | 26575 | 2913 | 813 | 812 | 65 |
| VN_C | 3893 | 3894 | 34845 | 3701 | 805 | 804 | 203 |

The last three cohort columns must equal the `<site>,all,...` row of
`data/federated_mvp/benchmark/expected_queries.csv`. `patients` is two lower
than that file's `patients` column because the benchmark counts
`patient_registry.csv`, while the node creates a patient only from an
ingestible result row.

### 4. Enable the field whitelist (D5)

Which fields a hospital exposes is its data-sharing decision (decision 0002).
For a demo node, enable all nine schema-v1 fields; the demo cohort needs
`HB, MCV, MCH, RBC, HBA0, HBA2, HBF`, and any disabled field makes a query come
back `REJECTED_INVALID_QUERY`.

Add `ADMIN_BASIC_AUTH_USER` / `ADMIN_BASIC_AUTH_PASS` to the `app` service's
environment (the stock `docker-compose.yml` does not set them), start it, then
use the same values here:

```bash
ADMIN_USER=...   # = ADMIN_BASIC_AUTH_USER
ADMIN_PASS=...   # = ADMIN_BASIC_AUTH_PASS
docker compose up -d app
curl -sf http://localhost:8080/health

for f in HB MCV MCH RBC MCHC RDW HBA0 HBA2 HBF; do
  curl -s -o /dev/null -w "$f=%{http_code}\n" -u "$ADMIN_USER:$ADMIN_PASS" \
    -X PUT -H 'Content-Type: application/json' -d '{"enabled":true}' \
    "http://localhost:8080/admin/query-fields/$f"
done

curl -s -u "$ADMIN_USER:$ADMIN_PASS" http://localhost:8080/admin/query-fields
```

Every `PUT` returns `200`, and the list shows `"enabled":true` with
`updated_by` set to the admin user. D5 lives in Postgres, so it survives app
restarts and re-ingests.

## Readiness

The node is seeded and ready for queries when:

- `GET /health` returns `200`;
- step 3 matches the table for this node's site;
- `GET /admin/query-fields` shows the intended fields enabled.

Once a control center sends the demo `QueryTask`, the node should return
`status: OK` with `matching_count` equal to `matches_with_hplc`
(1089 / 812 / 804). Suppression (`SUPPRESSION_THRESHOLD`, default 5) only
hides counts from 1 to 4, so it does not affect these numbers.

## Reset

To reload the registry (wrong site loaded, or a newer copy of the data):

```bash
docker compose exec -T db psql -U postgres -d life_cloud_agent_node \
  -c "TRUNCATE patients, job_progress CASCADE;"
```

This clears D3 (the cascade covers `specimens` and `lab_observations`) and D4
checkpoints, so a finished job cannot hand back a count from the old data. The
D5 whitelist is kept. Then repeat steps 2 and 3. To wipe everything including
D5, remove the database volume instead
(`docker compose down -v` — this destroys all node state).

## Validation Record

Rehearsed on 2026-09-29 against scratch Postgres 15 databases, one per site:
migrate → ingest (host Go and `golang` container) → the cohort count from the
node's own `CountMatchingCohort` → `verify_demo_seed.sql`. Both gave the
numbers in the table above, matching `expected_queries.csv` for all three
sites. Also checked: re-ingest is idempotent, `./migrate` from the built image
re-runs cleanly on a seeded database, the reset above followed by re-ingest
restores the same counts, and the D5 `PUT` calls return `200`.

## Unknowns

- The runtime image has no `ingest` binary and does not contain the data, so
  seeding a server needs the repository checkout (and Go or Docker) on that
  server.
- `patient_registry.csv` (sex, age, source diagnosis) is not ingested; D3 has
  no demographics.
- This is demo data. Seeding a real hospital needs that hospital's own exports
  and, if their format differs from VN_A/B/C, a new adapter.
