# Deployed Node Instance: Connecting the Control Center

Audience: engineers standing up the control center, once it exists. This
document is the connection cover sheet for one already-deployed, live node
instance. For the general wire protocol every node implements — message
contracts, validation order, retry/resume semantics — see
`node-agent-connection-guide.md`; this document only adds what's specific to
*this* running instance.

## 1. Who connects to whom (read this first)

**The node dials out to the control center. The control center never dials
the node** (decision 0001). This instance currently has no control center to
call, so its gRPC client is idle — nothing is listening for you to connect
to on the node's side, and there is nothing to "connect to the node" in the
usual client-server sense.

To bring this node online against your control center:

1. You stand up your `NodeControl` gRPC server (implementing the service in
   `api/proto/lifecloud/node/v1/node_control.proto`) at a publicly reachable
   address.
2. You give the node operator that address, plus TLS details (§3).
3. The operator sets `CONTROL_CENTER_ADDRESS` (and TLS config, if needed) in
   this instance's environment and restarts the `app` container. The node
   then dials out, registers, and starts sending heartbeats — no deploy or
   code change on the node's side beyond that config value.

There is nothing for you to do on your end to "reach" this node beyond
having your server ready to accept its connection.

## 2. This instance's identity

| | |
|---|---|
| `node_id` (sent in every `Register`) | `vn-demo-node-1` — a placeholder; ask the operator to set a real site identifier before this is used for anything beyond a demo/smoke test. |
| `agent_version` | Tracks the exact deployed commit (currently `965e9d2`); bump on every redeploy per decision 0003. |
| `query_schema_version` | `1` (only version this build understands). |
| Host | A shared Ubuntu 24.04 server behind Cloudflare; the node process itself is not internet-reachable on any inbound port — only its local admin/health HTTP API is, at `https://life-cloud-agent.daihuongwedding.online`, and only for a human operator (Basic Auth), never for the control center. |
| Local D3 state | Empty patient registry as of this writing — no hospital export has been ingested into this instance yet. A query sent today would correctly report `matching_count: 0`, not because anything is broken, but because there is no data. |
| D5 whitelist | All 9 schema-v1 fields are enabled (`HB, MCV, MCH, RBC, MCHC, RDW, HBA0, HBA2, HBF`) — this instance will not reject a schema-v1 query on whitelist grounds. |

## 3. What we need from you before we can point this node at you

- **Your `NodeControl` gRPC server's address** (`host:port`).
- **TLS.** This node requires TLS by default (decision 0006) and will not
  fall back to plaintext in this deployment. Give us one of:
  - A certificate from a public CA (nothing else to configure on our side), or
  - A certificate from a private CA — send us the CA certificate so we can
    set `CONTROL_CENTER_CA_FILE`.
- **mTLS (optional but recommended for anything beyond a demo).** If you
  want cryptographic proof of which node is connecting (rather than trusting
  the self-asserted `node_id` string — see the connection guide's §2/§8 on
  this gap), issue us a client certificate/key pair and we'll configure
  `CONTROL_CENTER_CLIENT_CERT_FILE` / `CONTROL_CENTER_CLIENT_KEY_FILE`.

Once you have that, tell the node operator; the config change and restart
take under a minute, and the node will register on the next connection
attempt.

## 4. What you'll see once it's connected

- One `Register{node_id: "vn-demo-node-1", agent_version: "965e9d2",
  query_schema_version: 1}` immediately on connect.
- A `Heartbeat` roughly every 30 seconds.
- It will accept any schema-v1 `QueryTask` you send (all fields whitelisted
  — §2), but every result will be `matching_count: 0` until real hospital
  data is ingested into this instance.

## 5. Where this comes from

- `node-agent-connection-guide.md` — the protocol itself.
- `docs/decisions/0001-node-agent-role-and-grpc-channel.md` — why the node
  dials out and never listens.
- `docs/decisions/0006-grpc-client-transport-security.md` — the TLS/mTLS
  rules summarized in §3.
