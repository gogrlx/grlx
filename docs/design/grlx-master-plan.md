# grlx: Master Phase-Wise Plan

Supersedes the phase structure in `grlx-1m-scale-plan.md`. Companion detail docs: `grlx-fork-roadmap.md` (per-workstream effort/risk/Claude-Code-fit table) and `grlx-nats-jwt-auth-design.md` (JWT/Account model detail). This document sequences everything decided across the full design conversation against delivery phases.

## Settled architecture, referenced throughout

- **Messaging:** NATS core pub/sub only (no JetStream) — bus in DMZ, farmer/core in non-DMZ, outbound-only connection from core to bus.
- **Identity:** Operator → Account-per-tenant → User-per-sprout JWT hierarchy. One signing identity per sprout, expressed as **two paired JWTs minted together at enrollment** (and together at every rotation): a NATS User JWT (`ed25519-nkey` alg, Account-signed) consumed only by nats-server's own decentralized auth, and a standard EdDSA JWT (gateway-signed) presented to Envoy on both the websocket route and the recipe HTTP endpoint. They're not interchangeable — NATS's `ed25519-nkey` header alg isn't a registered JOSE algorithm, so a standard `jwt_authn`-style validator like Envoy's can't consume the NATS JWT directly. Full rationale and JWKS design in `grlx-envoy-enrollment-design.md`.
- **Gateway:** Envoy in front of NATS, gateway-JWT validation before traffic reaches nats-server — mitigates the 2026 pre-auth websocket CVE class as a side effect, not just an auth convenience.
- **Storage:** PXC (durable metadata — PKI, RBAC, tenant accounts, sprout X25519 public keys), Valkey Cluster (heartbeat/connection-state, fed from NATS connection lifecycle events, no tenant hash-tagging), object storage (job logs, recipes). No etcd, no Kine.
- **Secrets custody (CloudXP's own):** OpenBao — operator/account signing keys, TLS cert issuance, per-tenant X25519 private keys.
- **Licensing:** PXC (GPLv2, under commercial Percona agreement) and OpenBao (MPL-2.0) accepted as exceptions to the Apache/MIT default.
- **Platform:** Kubernetes for NATS, Valkey, farmer, PXC — separate DMZ and non-DMZ node pools/namespaces, not one flat cluster.

---

## Phase 0 — Identity, gateway, and enrollment foundation

Everything a sprout needs to become a trusted, addressable identity before any of the scale or storage work matters.

- NATS decentralized JWT auth (Operator → Account → User), replacing the custom NKey allow-list.
- Envoy gateway in front of NATS — JWT validation on the websocket route, plus a second route proxying authenticated recipe requests through to non-DMZ.
- **Enrollment subsystem (new):** one-time registration keys, short-lived and tenant-scoped, usage-capped (`kubeadm`-join-token shape) — own PXC table (tenant_id, key hash, expiry, max/used count, revoked flag), own endpoint (not JWT-gated, since the sprout has no JWT yet), issues the sprout's JWT + NKey identity + tenant X25519 public key in one response.
- DMZ / non-DMZ process split (`RunNATSServer()` / `ConnectFarmer()`).
- Queue-group fix (`nc.Subscribe` → `nc.QueueSubscribe`).
- OpenBao-backed custody for Operator/Account signing keys and TLS certs.

**Exit criteria:** a tenant can be onboarded, a fresh VM can run the Ansible playbook with a registration key and come up as a fully-identified sprout, entirely through this pipeline — before any scale or storage work is layered on.

## Phase 1 — Storage and recipe layer

- PKI, props/facts, RBAC → PXC (read-through, no in-memory cache — fixes the cross-replica divergence bug directly).
- **Recipes → object storage, moved off local disk (new, corrected from the original NATS-based design).** Farmer's `basepath` local-disk recipe tree doesn't survive horizontal scaling — any replica needs to serve any recipe. Write path: git-as-source-of-truth, synced to S3/MinIO on merge. Read path: the new dedicated HTTP endpoint (behind the same Envoy JWT gate as NATS) serves from object storage instead of local disk.
- Job logs → object storage, unchanged.
- Connection-state/heartbeat → Valkey TTL keys, driven by NATS's own `$SYS.ACCOUNT.*.CONNECT`/`DISCONNECT` events rather than an app-level heartbeat protocol — this is what makes the <300ms budget (Phase 2) achievable at all, since the old `probeSprout()` synchronous ping loop's 3-second worst case is ~10x over budget by itself.

**Exit criteria:** multiple core replicas run against shared PXC + Valkey + S3 state with no divergence; any replica can serve any recipe; Valkey-driven connection state matches actual NATS connection lifecycle under a forced restart.

## Phase 2 — Connectivity, scale-out, and the latency budget

- Cluster the NATS bus (multiple nodes, full mesh) — validated capability at this scale, not a risk in itself.
- Sprout config gains a list of seed URLs (not one hardcoded `FarmerBusURL`).
- **Reconnect-storm hardening:** jittered/randomized backoff (`nats.CustomReconnectDelay`) replacing the fixed 15-second `ReconnectWait` — real risk at 1M sprouts reconnecting on a shared schedule after any bus restart.
- **Sprout proxy support (new):** eased specifically by the websocket transport already chosen — `wss://` is an HTTP Upgrade, so standard HTTP-proxy traversal applies; wire a proxy-aware dialer into the sprout's NATS client and the recipe-endpoint HTTP client (Go's `net/http` gets most of this for free via `ProxyFromEnvironment`).
- **<300ms response validation (new, explicit SLA):** load-test end-to-end (sprout → Envoy → NATS → handler) latency. Known risks to the budget: PXC certification-conflict retries on write-heavy paths, not NATS's own messaging latency or Envoy's JWT check (both comfortably sub-budget under normal conditions).
- Load-test core's queue-group replica count against real request-rate patterns.

**Exit criteria:** hundreds of thousands of simulated connections sustained through a bus-node restart without a reconnect storm; measured p99 response time under 300ms for standard job-dispatch requests.

## Phase 3 — Payload security

- **Application-level payload encryption (new):** NaCl `box` (X25519), one keypair per tenant (farmer-side, OpenBao-custodied private key) and one per sprout (generated locally at enrollment, private key never leaves the sprout). Protects payload content even from a fully compromised DMZ bus — the bus only ever sees ciphertext and routing metadata.
- **Key rotation (new, corrected from the originally-proposed design):** sprout-initiated only — sprout generates a new keypair locally and re-registers the new public key; farmer may trigger rotation but never generates or transmits a sprout's private key. Grace-period overlap (old and new public key both valid briefly) reusing the existing PKI accept/deny/revoke lifecycle.
- Decide and document the static-key forward-secrecy tradeoff explicitly (accepted, with a rotation schedule) rather than leaving it implicit.

**Exit criteria:** payload encryption live end-to-end; a full rotation cycle (trigger → sprout regenerates → grace period → old key revoked) exercised without a dropped in-flight message.

## Phase 4 — Metadata layer at 1M-endpoint scale

- PXC write/certification load-tested against realistic 1M-sprout metadata churn; tenant-based sharding (application-level or Vitess) as the standard fallback if a single cluster's certification overhead becomes a bottleneck.
- Valkey throughput confirmed against full CONNECT/DISCONNECT volume plus reconnect-storm bursts.
- NATS Account/JWT resolver mode ("full" vs "cache") decided against actual tenant-count distribution.
- Full chaos testing: kill PXC, Valkey, and NATS nodes independently mid-operation; confirm JWT fencing/CAS and PXC certification-retry logic hold.

**Exit criteria:** sustained 1M simulated concurrent connections; survives a PXC, Valkey, or bus node failure independently without data loss or a reconnect storm.

## Phase 5 — Sprout ingredients, cook-engine primitives, and farmer orchestration

Three related but distinct pieces of work, deliberately kept separate rather than folded into one expanded "probe" ingredient — see the architecture decision below for why.

**Architecture decision: one orchestrator per sprout.** The existing recipe/cook engine (`internal/cook`) is the only thing that sequences steps within a sprout's execution of a recipe. Probe, and everything borrowed from Spot below, are atomic ingredients or engine-level primitives the cook engine invokes — none of them get their own internal workflow logic. Before this decision, three separate things could each have ended up implementing their own notion of step ordering and conditionals: the recipe engine, a self-contained probe workflow, and a ported version of Spot's playbook engine. Collapsing to one avoids three places that could each get "run this only if that succeeded" subtly wrong in different ways. Confirmed against the actual code that this is architecturally sound: cooking happens entirely in `internal/cook/sproutcook.go`; farmer's own cook-related code only resolves and serves recipe content, never executes anything — so sprout-side orchestration and farmer-side dispatch are already naturally separate layers, not something this decision has to force apart.

**New atomic ingredients**, several derived from `umputun/spot` (MIT-licensed, SSH-based deployment tool — its execution model doesn't transfer, since sprout is an always-connected agent rather than an ephemeral SSH session, but several of its individual command types are good, validated ideas):
- `probe/http`, `probe/database` — single check + validation each, no internal sequencing. Modeled as async jobs (dispatch acknowledges within the 300ms budget, result arrives via separate job-status publish) — never synchronous NATS request-reply, since a slow external target has no business competing against the latency SLA.
- `wait` (from Spot's `wait: {cmd, timeout, interval}`) — poll a command/condition until success or timeout; an atomic action that retries internally, not a sequencer of other ingredients.
- `file.sync`, `file.line` (from Spot's `sync` and `line`) — directory sync with delete-orphans/exclude, and regex-based line delete/replace/append. Check against the existing `file` ingredient's current coverage before treating these as net-new; Salt has `file.line`/`file.replace` equivalents this may partially mirror already.
- `file.copy` enrichment (from Spot's `copy`) — bidirectional push/pull, glob patterns, exclude lists, `mkdir`, `chmod+x` — enrichment of the existing ingredient rather than a new one, if not already covered.

**New recipe/cook-engine orchestration primitives** (apply to any ingredient, not probe-specific):
- Conditional step execution (`cond`, with negation) — confirm no equivalent already exists before building as new.
- Deferred/`on_exit` actions — cleanup that runs regardless of a step's outcome.
- Variable registration and passing between steps — the general mechanism. Probe's sensitive-value chaining becomes a property of a registered variable (`sensitive: true`) rather than a probe-specific bolt-on: never written to the persisted job log (redacted to a fixed placeholder) or to PXC/Valkey, held only in-memory for the run's duration, and — a gap worth deliberately closing that Spot itself doesn't (its masking only covers its `options.secrets` path, not its separate `register` mechanism) — redaction extends to any verbose/debug/diagnostic output, not just the persisted log.
- Runtime context variables (`{GRLX_SPROUT_ID}`, `{GRLX_TENANT_ID}`, etc.), from Spot's `{SPOT_REMOTE_HOST}` family.
- Secret injection into step input wired directly to the deferred SDB `sdb://` interface (reference by name, resolve at execution time) rather than a separate probe-only secrets mechanism — Spot's own `options.secrets` pattern independently converged on nearly the same shape already designed for SDB, which is a good confidence signal for that interface, not a reason to duplicate it.

**Farmer-side staged rollout — grlx's equivalent of Salt's Orchestrate runner (`orchestrate.sls`).** Salt's orchestration runner executes on the master and coordinates *multiple minions* — sequencing which minions run a state and when, gating later batches on earlier ones' results — as distinct from a state run's own internal step ordering on one minion. Nothing in this design has an equivalent yet: today, a job dispatches to every target sprout in one action, with no staged pacing. The gap, and the fix, mirror Spot's `--concurrent=N` + `wait` combination translated to fleet scope: farmer dispatches a recipe to a batch of sprouts, waits on a probe or job-status success signal from that batch, then proceeds to the next batch — canary-style rollout, not per-sprout cooking. This is farmer logic exclusively; it decides *which sprouts* and *when*, and never executes a step itself. Naturally sequenced alongside probe, since it consumes probe's async results as its gating signal, but it is a distinct component from anything in the sprout-side ingredient/primitive list above.

**Explicit non-goals:** no probe-internal workflow engine; no ported Spot playbook/task engine; no separate probe-only secrets mechanism.

**Exit criteria:** a probe sequence runs as a tracked async job with results retrievable independent of dispatch latency; a chained-credential probe run leaves no sensitive value in the persisted job log, PXC/Valkey, or verbose/debug output; a recipe can be rolled out to a fleet in gated batches, halting before the next batch if a probe-based health signal fails.

## Deferred track — SDB secret resolution (not in the near-term sequence)

- Sprout-side `SecretProvider` interface behind the `sdb://` URI scheme. Tier 1: OpenBao/HashiCorp Vault via customer-supplied client certificate (hot-reloaded, no restart on rotation), Azure/AWS/GCP via managed identity only (same-cloud limitation accepted for v1). Tier 2 (later still): CyberArk and Delinea via bespoke REST clients and a separate bootstrap-issued credential, since neither has a maintained Go SDK or JWT-federation equivalent.
- Explicitly split out from the probe workstream and scheduled later — no dependency blocking Phase 5 from shipping without it.

**Exit criteria (when resumed):** a recipe can resolve a secret from OpenBao and from one cloud provider's managed-identity path end to end.

## Phase 6 — Packaging and deployment automation

- **Linux packaging:** deb, rpm, apk already exist (`nfpm` via goreleaser, systemd units already checked in) — zypper needs an explicit validation pass on SUSE conventions (firewall, AppArmor, packaging norms) rather than assuming rpm compatibility is sufficient.
- **Windows packaging (new, two distinct pieces):** (1) a `golang.org/x/sys/windows/svc` wrapper giving the sprout binary itself Windows Service Control Manager lifecycle hooks — separate from and more basic than the service-management *ingredient* from workstream G; (2) an MSI installer (WiX or `go-msi`) so Ansible's `win_package` module can drive fully silent install/uninstall/upgrade, matching enterprise Windows deployment norms.
- **Ansible playbooks (new, customer-run):** installs the correct package for the target OS, supplies the one-time registration key from Phase 0's enrollment subsystem, starts the service. Scope confirmed as sprout-fleet bootstrap only — not backend infrastructure provisioning, which stays on Kubernetes.

**Exit criteria:** a customer can run one playbook against a mixed Linux/Windows inventory and have every host come up as an enrolled, running sprout with no manual per-host steps.

## Running in parallel throughout (no dependency on phase sequence)

- **Windows ingredient support** — service provider, `snack`/winget wiring, user/group provider, registry ingredient. Still the least-scrutinized piece of the overall plan relative to the storage/messaging stack; worth a dedicated design pass at the same depth as everything else here, not just "scoped."
- **Multi-tenancy correctness review** — as the JWT/Account model, PXC tenant-scoping, and payload encryption all land, a dedicated cross-cutting review confirming isolation holds under the combined system, not just per-piece.
- **CERT-In / DPDP / data sovereignty review** — not yet undertaken in this design process at all; needs explicit scoping into a phase or confirmed owner elsewhere, not left implicit.

## Sequencing at a glance

```
Phase 0  Identity, gateway, enrollment      (JWT/Account model, Envoy, one-time registration keys)
Phase 1  Storage and recipe layer           (PXC + Valkey + S3, recipes off local disk)
Phase 2  Connectivity and scale-out         (NATS clustering, reconnect jitter, proxies, <300ms validation)
Phase 3  Payload security                   (per-sprout/tenant encryption, sprout-initiated rotation)
Phase 4  Metadata layer at 1M scale         (PXC/Valkey load validation, sharding if needed)
Phase 5  Ingredients, cook primitives, rollout (probe/wait/file ingredients, engine primitives, staged rollout)
Phase 6  Packaging and deployment           (zypper validation, Windows MSI+svc, Ansible playbooks)
──────────────────────────────────────────
Deferred: SDB secret resolution             (resumed after Phase 6, not blocking)
Windows ingredient support: parallel throughout
Multi-tenancy + CERT-In/DPDP review: parallel, cross-cutting
```

**Biggest structural dependency worth calling out:** Phase 0's enrollment subsystem and JWT model is now a harder prerequisite than it looked in the original plan — Phases 1 through 6 all assume a sprout already has its identity, keys, and trust chain established, and that identity is what threads through storage (PXC key), messaging (JWT), encryption (X25519 keypair), and secrets resolution (customer-cert or managed-identity binding). Getting Phase 0 right isn't just "foundational" in the abstract — every later phase's design is written assuming its output exists.
