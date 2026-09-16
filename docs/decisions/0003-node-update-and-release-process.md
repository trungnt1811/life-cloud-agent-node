# 0003 Node Update And Release Process

Date: 2026-09-16

## Status

Accepted

## Context

The node agent (decisions 0001, 0002) runs on infrastructure the hospital
hosts, not infrastructure Life Cloud owns — the control center cannot reach
in to a node (0001), and cannot assume it is safe to unilaterally restart
software adjacent to a hospital's clinical data flow. A process was needed
for how a new node-agent version reaches an already-deployed hospital
instance.

## Decision

**The control center only advises; it never triggers or performs a
restart.** It tracks a latest-available version (populated out of band by a
release admin publishing a new container image) and pushes an
`UpdateAdvisory(version, changelog_url, severity)` message down the same
long-lived gRPC stream already used for query traffic (decision 0001) to
every connected node.

**The node takes no automatic action on an advisory.** It logs the advisory
and exposes it locally (e.g. `GET /admin/status`) for whoever operates that
site to see.

**Applying an update reuses the existing Docker-based deployment.** A human
operator — hospital IT or the Life Cloud team managing that site — runs
`docker compose pull` followed by `docker compose up -d` against the new
image tag, at a time of their own choosing. No self-update logic (download,
verify, replace the running binary) exists in, or is planned for, the Go
binary itself.

**A node re-registers with its version on every (re)connect.** On restart,
the new process reconnects (dial-out per 0001) and sends `Register` with its
`agent_version`, so the control center's node registry always reflects what
is actually deployed at each site.

**Rollback reuses the identical mechanism.** Pulling and running the
previous image tag is the rollback path; there is no bespoke rollback code.

**Nodes are explicitly allowed to run different `agent_version`s at the same
time.** Applying an update is a per-site decision with no forced
synchronization across the fleet. Decision 0002's `query_schema_version`
check exists specifically so the query contract tolerates this instead of
assuming every node is current.

## Alternatives Considered

1. Fully automatic, remote-triggered restart initiated by the control
   center. Rejected: hospital-hosted, clinical-data-adjacent software should
   not be force-restarted by a remote party without local awareness or
   consent.
2. A self-updating Go binary that downloads, verifies, and replaces itself.
   Rejected for now: it duplicates what the existing Docker deployment
   already solves, adding download/verify/replace/rollback logic that has
   to be written and kept secure for no stated benefit yet.
3. A mandatory, synchronized fleet-wide update window. Rejected: hospitals
   operate independently; forcing simultaneous rollout across sites isn't
   operationally realistic.

## Consequences

Positive:

- No self-update logic to write, maintain, or secure — updating is exactly
  today's `docker compose pull && up -d` deployment flow.
- Rollback is the same well-understood mechanism run against an older tag.
- Each hospital keeps control of its own maintenance window.

Tradeoffs:

- Nodes can be on meaningfully different versions at any given time; the
  query contract (decision 0002) has to be designed defensively around
  that, not around an assumption that every node is current.
- There is no way to force an urgent fix onto every node automatically — an
  urgent (e.g. security) advisory still depends on a human operator acting
  on it.

## Follow-Up

- Decide how a high-severity advisory is escalated beyond a passive status
  flag (e.g. paging the site operator) — not designed yet.
- Design `UpdateAdvisory` as part of the same `.proto` contract that will
  carry `Register`, `Heartbeat`, `QueryTask`, and `QueryResult`.
