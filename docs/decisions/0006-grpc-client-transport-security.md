# 0006 gRPC Client Transport Security

Date: 2026-09-22

## Status

Accepted

## Context

Decision 0001 made this node dial out to the control center over gRPC;
decision 0004 fixed the wire contract. Neither decided how the channel is
secured. The node sends clinical cohort filters and counts (decision 0002:
aggregate-only, but still hospital-derived) to a coordinator outside the
hospital's network, over infrastructure the hospital does not control end to
end (decision 0003). Choosing plaintext-by-default here would be an
externally observable security posture, not an implementation detail, so it
needs a decision rather than a default picked while writing the client.

## Decision

**TLS is required by default.** The node verifies the control center's
certificate using the system CA pool, or `CONTROL_CENTER_CA_FILE` when the
control center uses a private CA. Connecting fails loudly (the worker does
not start) if TLS setup fails; there is no silent fallback to plaintext.

**mTLS is enabled automatically when a client certificate is configured.**
Setting both `CONTROL_CENTER_CLIENT_CERT_FILE` and
`CONTROL_CENTER_CLIENT_KEY_FILE` presents that certificate to the control
center, so it can authenticate the node. Configuring only one of the two is
a startup error (same partial-config-fails-fast pattern as
`ADMIN_BASIC_AUTH_USER`/`PASS`, decision-adjacent to the Phase 3 hardening).

**Plaintext exists only as an explicit opt-out**, `CONTROL_CENTER_INSECURE=
true`, for local development and the in-repo stub control center (Phase 8).
Enabling it logs a warning naming the risk on every connect.

## Alternatives Considered

1. Plaintext by default, TLS opt-in. Rejected: the insecure path would be
   what every fresh deployment gets unless someone remembers to turn security
   on; defaults should be the safe choice.
2. Mandatory mTLS, no plaintext escape hatch at all. Rejected: blocks the
   Phase 8 in-repo stub and local development on certificate provisioning
   that has no other purpose in this repository; the explicit, logged opt-out
   keeps the unsafe path visible instead of removing it.
3. TLS version/cipher suite pinning in this decision. Rejected: `crypto/tls`
   and `google.golang.org/grpc/credentials` defaults are already conservative
   and change with the Go toolchain; pinning here would fight that instead of
   inheriting it.

## Consequences

Positive:

- A fresh deployment is secure without extra steps.
- mTLS is available without a second decision once the control center
  supports issuing node client certificates.

Tradeoffs:

- Local development and Phase 8's stub control center must pass
  `CONTROL_CENTER_INSECURE=true` explicitly.
- Certificate rotation is an operational concern for whoever runs the
  control center and each node; this decision does not cover rotation
  procedure.

## Follow-Up

- Phase 8's stub control center documents `CONTROL_CENTER_INSECURE=true` as
  its expected local configuration.
- Revisit if the control center later requires a specific mTLS certificate
  issuance flow (e.g. short-lived certs) — that would be a control-center-side
  decision this repository would need to consume, not originate.
