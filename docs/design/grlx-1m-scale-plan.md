# grlx: Phase-Wise Plan for 1 Million Endpoints

Companion to `grlx-fork-roadmap.md` and `grlx-nats-jwt-auth-design.md`. Those documents describe *what* to build; this one sequences it against a concrete scale target and names the parts that only become problems at real scale — several of which don't show up until Phase 3 below.

## Assumptions (confirm before treating numbers as final)

- "1 million endpoints" = 1M concurrently-connected sprouts, not 1M total-ever-registered with most offline.
- Tenant distribution unknown — assumed a mix of tenant sizes rather than one tenant with 1M sprouts. This materially affects the NATS resolver mode (Phase 3) and etcd sharding key (Phase 3) — worth confirming early.
- Per-sprout metadata (facts/PKI record) averages 1–2KB — not a binding constraint under the PXC-based design below, but relevant to PXC storage/backup sizing regardless.
- **Storage decision (settled):** PXC (Percona XtraDB Cluster) for durable metadata (PKI, RBAC, tenant accounts, props/facts) — chosen over etcd given the team's existing PXC operational depth, no size ceiling to design around, and mature sharding tooling if ever needed. Redis, populated from NATS's own connection lifecycle events, for connection-state/heartbeat — keeping ephemeral, loss-tolerant, high-churn data off the durable/quorum-committed path entirely. Object storage for job logs, unchanged throughout. No etcd, no Kine — Kine exists specifically to satisfy Kubernetes's etcd-API-shaped storage interface, a lock-in grlx doesn't have, so it was never actually in scope once going relational.

## Phase 0 — Foundational correctness (target: validated at thousands of sprouts)

Nothing here is scale-specific — it's the trust and correctness baseline everything else stands on, and it's cheaper to get right before scale amplifies any mistake.

- NATS decentralized JWT auth (Operator → Account-per-tenant → User-per-sprout), replacing the custom NKey allow-list. *(full design in companion doc)*
- OpenBao-backed key custody for the Operator/Account signing keys and TLS certs.
- DMZ / non-DMZ split (`RunNATSServer()` / `ConnectFarmer()` extracted into separate binaries).
- Queue-group fix (`nc.Subscribe` → `nc.QueueSubscribe`) in `natsapi/router.go`.

**Exit criteria:** a tenant can be onboarded, a sprout accepted, and a job dispatched, entirely through the JWT/Account model, with the bus in the DMZ and core outside it — at a scale small enough that none of Phase 1–3's problems are in play yet.

## Phase 1 — Storage layer (target: tens of thousands of sprouts)

- PKI, props/facts, RBAC → PXC. Props specifically drops its write-through in-memory cache in favor of reading through to PXC — fixes the cross-replica divergence bug, not just a storage swap. **No native watch/notify here** (unlike etcd) — change-notification for props/facts needs to be built explicitly: either short-interval polling (`WHERE updated_at > ?`, likely fine given props/facts don't need sub-second reactivity) or binlog-based CDC if tighter propagation is ever needed. Worth picking the simpler polling approach first and only reaching for CDC if it proves insufficient.
- Job logs → object storage (S3/MinIO), unchanged from earlier scoping — clean, mechanical, no cache to fight.
- **Connection-state/heartbeat, settled design:** Redis TTL keys, driven primarily by NATS's own system-account connection lifecycle events (`$SYS.ACCOUNT.*.CONNECT`/`DISCONNECT`) rather than a sprout-initiated heartbeat protocol — core subscribes once, sets the key on CONNECT, deletes it on DISCONNECT. A generous TTL on the key acts as a safety net for ungraceful disconnects (crash, hard partition), which NATS's own protocol-level ping/pong keepalive (`PingInterval`/`MaxPingsOut`) will eventually surface as a DISCONNECT event, self-healing the key within a bounded window. This avoids inventing a separate app-level heartbeat message entirely — at 1M sprouts, an app-level heartbeat every 30s would add ~33,000 messages/second system-wide purely for liveness, which this design avoids by reusing what NATS already tracks. Redis doesn't need PXC-grade HA rigor here — the data is inherently ephemeral and self-healing on reconnect, so a bare Redis instance is a reasonable starting point, with Sentinel/Redis Cluster available later purely for continuity through node failure, not for durability.
- **Tenant isolation on PXC is application-enforced**, not protocol-enforced the way NATS Accounts give you for the messaging layer — `tenant_id` scoping on every query (or schema-per-tenant) needs the same security-review discipline as every other tenant-boundary-touching piece of this plan, since it doesn't come for free just from picking a mature database.

**Exit criteria:** multiple core replicas run against shared PXC + Redis + S3 state with no divergence under normal restart/failover testing at this scale; Redis-driven connection state confirmed accurate against actual NATS connection lifecycle under a forced bus-node restart.

## Phase 2 — NATS horizontal scale-out (target: hundreds of thousands of sprouts)

- Cluster the bus (multiple `nats-server` nodes, full mesh). Core NATS's connection-handling scales by adding nodes — this is a real, validated capability: NATS's own guidance is 100,000+ concurrent connections per well-tuned node, and clustered deployments are documented handling millions of total connections. This is *not* the same constraint as JetStream's single-leader-per-stream limit — grlx uses core pub/sub, so this scales more favorably than the write-throughput ceilings discussed elsewhere in this plan.
- Sprout config gains a list of seed URLs (not one hardcoded `FarmerBusURL`) so initial connection has somewhere to go if one seed node is down — small, contained config change flagged back when multi-server support was discussed.
- **Reconnect-storm hardening — new finding, worth taking seriously before this phase, not after an incident.** Today's sprout reconnect logic is a fixed `nats.ReconnectWait(15 * time.Second)` with unlimited retries. At 1M sprouts, a single bus-side restart or network blip disconnects everyone roughly simultaneously, and a fixed wait means everyone retries at roughly the same moment — a textbook thundering herd. NATS's own vendor guidance names this exact scenario: illustratively, 1,000,000 devices reconnecting on a shared schedule can produce reconnect rates in the thousands per second, each consuming CPU to re-authenticate even when rejected. **Fix: add jittered/randomized backoff to the sprout's reconnect logic** (`nats.CustomReconnectDelay` instead of a fixed `ReconnectWait`) — a small, contained change, but one that matters far more at 1M than at the scale everything's been tested at so far.
- Load-test core's queue-group replica count against real request-rate patterns (job dispatch frequency, fact refresh cadence) — this number is workload-dependent and shouldn't be guessed; treat it as a benchmarking task, not a fixed target.

**Exit criteria:** simulated connection load (hundreds of thousands of concurrent sprout connections, via a load-testing harness rather than real fleet) sustained through at least one full bus-node restart without a reconnect-storm-induced outage.

## Phase 3 — Metadata layer at real scale (target: 1 million sprouts)

With PXC + Redis as the settled Phase 1 design, the two hard ceilings that would have forced a redesign here under etcd don't apply: PXC has no comparable size limit, and Redis's TTL-key model was already the right shape for heartbeat volume rather than a placeholder needing replacement. What's left for this phase is scale validation and tuning, not a design pivot:

- **PXC write/certification load at 1M-sprout metadata churn** (facts updates, PKI accept/deny transitions, job dispatch bookkeeping) — load-test against realistic update rates; if a single PXC cluster's certification overhead becomes a bottleneck, tenant-based sharding (application-level routing, or Vitess) is the standard, well-tooled path — more mature tooling here than etcd sharding would have offered.
- **Redis throughput at full connection-event volume** — 1M CONNECT/DISCONNECT events plus reconnect-storm bursts (see Phase 2) is a straightforward Redis workload well within normal operating envelopes, but worth confirming under the same load test rather than assuming.
- NATS Account/JWT resolver: revisit "full" (every node holds every tenant's Account JWT) vs. "cache" (nodes fetch on demand) based on actual tenant count at this scale — deferred from the original JWT design doc, now worth deciding concretely.
- Full-scale load and chaos testing: simulate 1M connections, kill PXC/Redis/NATS nodes independently mid-operation, confirm the JWT auth model's fencing/CAS behavior and PXC's certification-conflict retry logic both hold under real failure injection — the standard this plan has applied to every other correctness-critical piece (lockd, the JWT auth model) applies here too.

**Exit criteria:** sustained 1M simulated concurrent connections, survives a bus node failure, a PXC node failure, and a Redis failure independently without data loss (PXC) or worse than bounded staleness (Redis), at validated write/certification throughput.

## Running in parallel throughout (no dependency on any phase above)

- **Windows sprout support** — service provider, `snack`/winget wiring, user/group provider, registry ingredient. Sprout-side, no dependency on farmer/core scale work.
- **Multi-tenancy correctness review** — as the JWT/Accounts model and etcd sharding both land, worth a dedicated pass confirming tenant isolation holds under the combined design, not just each piece independently.

## Sequencing at a glance

```
Phase 0  Foundational correctness         (auth model, DMZ split, queue groups)
Phase 1  Storage layer                    (PXC + Redis + S3, thousands → tens of thousands)
Phase 2  NATS scale-out + reconnect       (hundreds of thousands)
Phase 3  Metadata layer at real scale     (PXC/Redis load validation, sharding if needed, 1M)
Phase 4  Operational hardening            (monitoring, alerting, runbooks, security review)
─────────────────────────────────────────
Windows sprout support: parallel throughout, all phases
```

**What was the biggest open question is now settled:** PXC (durable metadata) + Redis (NATS-event-driven heartbeat) + object storage (job logs) replaces the etcd-based design throughout, chosen for the team's existing PXC operational depth, PXC's lack of a size ceiling at this scale, and Redis's native TTL model being a cleaner fit for heartbeat churn than forcing it through any fully-durable, quorum-committed store. What remains open is narrower and more standard: building props/facts change-notification without etcd's native watch (polling first, CDC if needed), and enforcing tenant isolation at the application layer on PXC rather than getting it protocol-enforced the way NATS Accounts give the messaging layer.
