# 0005 Job Checkpoint Granularity And Resume Semantics

Date: 2026-09-21

## Status

Accepted

## Context

Decision 0002 requires query execution to be checkpointed and resumable, so an
interrupted job (node restart, stream drop, deadline) continues instead of
restarting or losing work, and named the store D4 (Job Progress Checkpoint).
It left open what a resumable unit is and how often progress is persisted.
Those choices are externally observable: they decide whether a resumed job can
return a different count than an uninterrupted one, and how long a finished
job's result stays re-servable. Phase 5 currently answers a task with one SQL
statement, which cannot be resumed.

## Decision

- **A resumable unit is a chunk of patients, walked in `patients.id` order
  (keyset: `patient_id > last_patient_id ORDER BY patient_id LIMIT N`).**
  Only patients with at least one specimen in the query's date window are
  candidates. Every patient falls wholly inside one chunk, so LATEST_IN_RANGE
  specimen selection (0004) is evaluated per patient exactly as in an
  unchunked run. Chunking by date is rejected: it would split one patient's
  specimens across chunks and break "latest".
- **One checkpoint is written after every completed chunk**, never on a timer.
  It stores `last_patient_id` and `running_count` in a single row upsert, so
  the cursor and the count can never disagree. A crash before the write
  recomputes exactly that one chunk; nothing is added twice because the count
  is derived per chunk and only committed together with the cursor.
- **Default chunk size is 5000 patients**, configurable with `JOB_CHUNK_SIZE`.
  A value of `0` or unset means the default. It may be lowered to 1 for tests.
- **Small-cell suppression (process 3.6) is applied only to the final total.**
  A checkpoint never stores a suppressed count.
- **A checkpoint carries a `criteria_hash`** (SHA-256 over a canonical form of
  the date window, conditions and required panels; order-insensitive). If a
  `job_id` arrives with a different hash the old checkpoint is discarded and
  the job starts from the beginning, instead of adding counts from another
  query.
- **A finished job keeps its row (`status = done`) with the final count.** A
  repeated `job_id` with the same hash returns that stored count without
  re-querying. Finished rows older than 7 days are deleted at the start of the
  next job.
- **`job_id` is required** (non-empty, at most 255 characters) because it is
  the checkpoint key.

## Alternatives Considered

1. Checkpoint on a timer. Rejected: resume cost depends on wall-clock, and a
   timer can fire between a chunk's count and its cursor.
2. Chunk by collection date or by specimen ID. Rejected: splits a patient's
   specimens across chunks (see above).
3. Stream every patient into memory and checkpoint per row. Rejected: one
   write per patient is far more I/O than the query it protects.
4. Discard finished rows immediately. Rejected: a control center that
   re-sends a `job_id` after a lost result would force a full re-run.

## Consequences

Positive:

- Resume cost is bounded by one chunk.
- A resumed run and an uninterrupted run over unchanged data return the same
  count, and this is testable by comparing them.
- No new wire fields; this is entirely node-local.

Tradeoffs:

- **Known limitation:** if ingestion writes to D3 between an interruption and
  the resume, the final count mixes data from before and after that write. v1
  accepts this because ingestion is an operator-run batch, not a continuous
  process. A job that must be exact should be re-sent with a fresh `job_id`.
- Two concurrent executions of the same `job_id` on one node are not
  coordinated. The gRPC client (Phase 7) runs one stream, so this does not
  arise today.

## Follow-Up

- Phase 7 passes the in-flight `job_id` set into `Heartbeat` and re-invokes
  execution after a reconnect; the checkpoint makes that idempotent.
- Revisit the ingestion-during-resume limitation if ingestion ever runs
  continuously.
