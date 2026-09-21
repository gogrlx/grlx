# CloudXP Machine Manager — API Design

Two codebases, one trust boundary:

- **SaaS API** — external, customer/CloudXP-facing. Standard Go REST service, GORM, `saas` schema.
- **Farmer** (+ sprout) — internal, data-plane. Existing grlx NATS API, `farmer` schema. Its entire API surface is now **privileged and internal-only** — callable exclusively by the SaaS API's service credential, never directly by a tenant, a human user, or CloudXP.

Both schemas live in one shared PXC cluster. Single-writer-per-schema: farmer writes only `farmer.*`, SaaS API writes only `saas.*`; each has read-only grants into the other's schema for local joins (see §5.1).

```
CloudXP / tenant admins / portal
        │  REST, bearer token (tenant-scoped claims)
        ▼
   SaaS API  ──── privileged internal calls (mTLS / system NATS user) ────►  Farmer
   (saas schema)                                                          (farmer schema)
        │                                                                       │
        └───────────────────── shared PXC cluster, cross-schema reads ─────────┘
```

---

## 1. External API — SaaS API

Base path: `/v1`. All endpoints require a bearer token whose claims include the caller's `tenant_id` — the gateway/SaaS API injects or validates this on every request; a client can never supply a `tenant_id` that doesn't match its own token.

Standard error shape for every endpoint:
```json
{ "error": "<snake_case_code>", "message": "<human readable>", "details": {} }
```

### 1.1 Tenants

| Method | Path | Notes |
|---|---|---|
| `POST` | `/tenants` | Create a tenant. **Async** — see below. |
| `GET` | `/tenants/{tenant_id}` | Fetch tenant + status. |
| `PATCH` | `/tenants/{tenant_id}` | Update name/plan/metadata. |
| `DELETE` | `/tenants/{tenant_id}` | Offboard. Async, same pattern as create. |
| `GET` | `/tenants/{tenant_id}/status` | Lightweight status-only poll. |

**`POST /tenants`**
```json
// request
{ "name": "Acme Bank", "plan_id": "plan_std" }

// response 202 Accepted
{ "tenant_id": "t_8f2a", "status": "pending" }
```
Status values: `pending → active`, or `pending → failed` (see `provisioning_jobs`, §5.2). `DELETE` follows the same `offboarding → offboarded`/`failed` shape.

### 1.2 Enrollment keys

Owned entirely by the SaaS API (`saas.enrollment_keys`); farmer validates against this table directly via its read grant at the moment a fresh sprout enrolls.

| Method | Path | Notes |
|---|---|---|
| `POST` | `/tenants/{tenant_id}/enrollment-keys` | Issue a one-time, tenant-scoped key. |
| `GET` | `/tenants/{tenant_id}/enrollment-keys` | List (with usage/expiry state). |
| `DELETE` | `/tenants/{tenant_id}/enrollment-keys/{key_id}` | Revoke. |

```json
// POST request
{ "expires_in_hours": 24, "max_uses": 50 }

// response
{ "key_id": "ek_91cd", "registration_key": "<opaque, shown once>", "expires_at": "2026-09-15T10:00:00Z", "max_uses": 50 }
```

### 1.3 Asset linking

The only place `asset_id` (CloudXP's own, stable-for-life VM identifier) enters the system. Purely an external attribute — farmer never sees it.

| Method | Path | Notes |
|---|---|---|
| `POST` | `/tenants/{tenant_id}/sprouts/{sprout_id}/asset-link` | Link. `{ "asset_id": "..." }` |
| `DELETE` | `/tenants/{tenant_id}/sprouts/{sprout_id}/asset-link` | Unlink. |

### 1.4 Sprouts — list by asset id

| Method | Path | Notes |
|---|---|---|
| `GET` | `/tenants/{tenant_id}/sprouts?asset_ids=a1,a2,...` | Max **100** ids per call → `400 too_many_asset_ids` above that. |

```json
// response
{
  "results": [
    { "sprout_id": "s_1", "asset_id": "a1", "key_state": "accepted", "connected": true },
    { "sprout_id": "s_2", "asset_id": "a2", "key_state": "accepted", "connected": false }
  ],
  "unresolved": ["a17"]
}
```
`unresolved` covers both "never linked" and "linked to a different tenant" — never distinguished in the response, to avoid leaking cross-tenant existence.

Implementation note: this is a single local SQL statement, no NATS round-trip —
```sql
SELECT s.* FROM farmer.sprouts s
JOIN saas.asset_links a ON a.sprout_id = s.id
WHERE a.tenant_id = ? AND a.asset_id IN (?, ...);
```

### 1.5 Sprout actions — batch, async

| Method | Path | Notes |
|---|---|---|
| `POST` | `/tenants/{tenant_id}/sprouts/actions` | Max 100 `asset_id`s. Returns a batch id. |
| `GET` | `/tenants/{tenant_id}/sprouts/actions/{batch_id}` | Poll status, per-item detail. |

```json
// POST request
{
  "asset_ids": ["a1", "a2", "a17"],
  "action": { "type": "cmd.run", "params": { "cmd": "systemctl restart nginx" } }
}
// or: { "type": "cook", "params": { "recipe": "nginx.harden" } }

// 202 response
{ "batch_id": "b_123" }

// GET status response
{
  "status": "in_progress",
  "items": [
    { "asset_id": "a1",  "sprout_id": "s_1", "status": "succeeded" },
    { "asset_id": "a2",  "sprout_id": "s_2", "status": "queued" },
    { "asset_id": "a17", "status": "unresolved" }
  ]
}
```
Batch/item status backed by `saas.asset_action_batches` / `saas.asset_action_items` (§5.2). Once an item's underlying farmer `jid` is known, its status is refreshed via a local read into `farmer.jobs` — no NATS call needed for polling, same pattern as §1.4.

### 1.6 Recipes, jobs, audit (read-through proxies)

Thin, tenant-scoped wrappers over farmer's existing `recipes.*`, `jobs.*`, and `audit.*` subjects (§2.1) — same shapes, tenant filter enforced by the SaaS API before forwarding.

| Method | Path |
|---|---|
| `GET` | `/tenants/{tenant_id}/recipes` |
| `GET` | `/tenants/{tenant_id}/recipes/{name}` |
| `GET` | `/tenants/{tenant_id}/jobs` |
| `GET` | `/tenants/{tenant_id}/jobs/{jid}` |
| `GET` | `/tenants/{tenant_id}/audit?date=...` |

### 1.7 Teams, API keys, webhooks, usage — *(surface only; not detailed yet)*

These close the gaps identified earlier but haven't been through a design pass the way §1.1–1.6 have:

| Method | Path |
|---|---|
| `POST/GET/DELETE` | `/tenants/{tenant_id}/api-keys` |
| `POST/GET/DELETE` | `/tenants/{tenant_id}/teams`, `/teams/{team_id}/members` |
| `POST/GET/DELETE` | `/tenants/{tenant_id}/webhooks` |
| `GET` | `/tenants/{tenant_id}/usage`, `/tenants/{tenant_id}/plan` |

Flagged explicitly as **not yet designed** — auth scheme for human users (bearer/API-key vs. SSO/OIDC), webhook delivery/retry semantics, and billing-system integration are all open. Two candidates worth evaluating when this gets designed, surfaced while working through the sprout-JWT design and set aside there as a better fit here instead: `gourdiantoken` (Go, MIT — access/refresh rotation, revocation, multi-tenant bulk revocation via a `tid` claim, matching this API's own tenant model closely; young/single-maintainer, worth weighing against a more established primitive), and OpenBao's own Identity/OIDC provider (native ID-token issuance against OpenBao's existing entity model, if human users end up modeled there) as an alternative to standing up a separate IdP.

### 1.8 Fleet updates

Owned by the SaaS API (`saas.fleet_versions`, `saas.tenant_update_policy`).
Dispatch reuses `saas.asset_action_batches`/`asset_action_items` (§5.2) —
no new batch/item tables — since an update rollout is just another batched
action, tracked and audited identically to a `cmd.run` or `cook` batch
(§1.5).

**Blocking dependency, stated up front:** upstream grlx's own
`internal/update` package is an explicitly disabled skeleton —
`PerformUpdate()` unconditionally returns `errUnsignedUpdatesDisabled`, and
the package header says outright "Do NOT enable it as-is" (tracked upstream
in `gogrlx/grlx#286`). Everything in this section is a design placeholder
for when sprout has a real, working, signed self-update path — not
something to wire live today. Don't expose `POST
/tenants/{tenant_id}/sprouts/updates` publicly, even behind a feature flag,
until that dependency resolves (also see §6).

| Method | Path | Notes |
|---|---|---|
| `GET` | `/versions` | List available sprout versions. **CloudXP's own published catalog**, not upstream's GitHub release feed — see rationale below. |
| `GET` | `/tenants/{tenant_id}/update-policy` | Fetch the tenant's approved version and rollout window. |
| `PATCH` | `/tenants/{tenant_id}/update-policy` | Set the approved/pinned version and rollout window. No sprout updates automatically without a tenant explicitly approving a version — this is opt-in, not upstream's ambient "check `/latest` every N minutes" model. |
| `POST` | `/tenants/{tenant_id}/sprouts/updates` | Trigger a staged update batch. Same request/response shape as §1.5's `/sprouts/actions`, with rollout-specific fields (below). |
| `GET` | `/tenants/{tenant_id}/sprouts/updates/{batch_id}` | Poll status — identical shape to §1.5's batch status endpoint. |

**`PATCH /tenants/{tenant_id}/update-policy`**
```json
// request
{ "approved_version": "v2.4.1", "auto_update": false }

// response
{ "tenant_id": "t_8f2a", "approved_version": "v2.4.1", "auto_update": false, "updated_at": "2026-09-15T10:00:00Z" }
```

**`POST /tenants/{tenant_id}/sprouts/updates`**
```json
// request
{
  "asset_ids": ["a1", "a2", "a17"],
  "target_version": "v2.4.1",
  "batch_size": 10,
  "gate": "probe"
}
// 202 response
{ "batch_id": "b_456" }
```
`batch_size`/`gate` default conservatively smaller and stricter than an
ordinary `cmd.run`/`cook` batch (§1.5) — see §2.3, Rollout safety, for why
a bad update is a worse failure mode than a bad recipe step.

**Why CloudXP's own version catalog, not upstream's release feed.**
Upstream's skeleton polls `<UpdateURL>/latest` directly from each sprout,
independent of farmer, with no tenant concept at all — every sprout across
every tenant would land on whatever upstream ships, whenever upstream ships
it. That's incompatible with a BFSI/government customer's own change-control
expectations. `GET /versions` and the per-tenant `update-policy` instead
give each tenant explicit, audited control over which version their fleet
runs and when — consistent with the tenant-scoping discipline applied
everywhere else in this design (§4's "every query includes `tenant_id`").

---

## 2. Internal API — Farmer (SaaS API only)

Transport: NATS subjects under `grlx.api.*` (existing) and a new `grlx.internal.*` prefix. Authenticated via a privileged internal NATS identity (system-account-scoped user or mTLS), distinct from any tenant's sprout/user credentials. Never exposed to a tenant, a sprout, or a human directly.

### 2.1 Existing subjects (unchanged shape, now tenant-scoped + internal-only)

| Subject | Purpose | Tenant-scoping note |
|---|---|---|
| `health`, `version` | Liveness/build info | Global, no scoping needed |
| `pki.{list,accept,reject,deny,unaccept,delete}` | Sprout key lifecycle | Scoped by NATS Account once JWT model lands |
| `sprouts.{list,get}` | Sprout metadata | **Superseded for external use by `internal.sprouts.list`** (§2.2) — paginated, tenant-scoped. Raw form stays for farmer-local/CLI use. |
| `test.ping` | Sprout health probe | — |
| `cmd.run`, `cook` | Command/recipe dispatch | Called by SaaS API only, per §1.5's batch flow |
| `jobs.{list,get,delete,cancel,forsprout}` | Job tracking | Proxied read-only via §1.6; SaaS API also reads `farmer.jobs` directly for batch-status polling |
| `props.{getall,get,set,delete}` | Sprout properties | — |
| `cohorts.*` | Sprout groupings | — |
| `auth.*` | Human RBAC (today: static config-file check) | Superseded long-term by SaaS API's `/api-keys`, `/teams` (§1.7) |
| `shell.start` | Interactive shell | — |
| `recipes.{list,get}` | Recipe catalog | Proxied via §1.6 |
| `audit.{dates,query}` | Audit log | Proxied via §1.6, tenant-filtered |

### 2.2 New subjects — tenant & sprout lifecycle

| Subject | Direction | Purpose |
|---|---|---|
| `internal.tenant.provision` | SaaS API → Farmer, fire-and-forget | Create the NATS Account + `farmer`-schema tenant partition. Async (§5.2) — no reply expected. |
| `internal.tenant.provisioned.{job_id}` | Farmer → SaaS API, published | Completion callback. Farmer has no write access to `saas` schema, so this event is how status gets back. |
| `internal.tenant.deprovision` | SaaS API → Farmer | Reverse of provision. Same async/callback shape. |
| `internal.sprout.mint` | SaaS API → Farmer, request-reply | Mint a User JWT + NKey identity for a newly-enrolled sprout, given `tenant_id` + submitted pubkeys. Called from farmer's own pre-enrollment HTTP path (which validates the registration key against `saas.enrollment_keys` directly, per §1.2) — this subject is the step after that validation succeeds — full sequence in §3. |
| `internal.sprout.revoke` | SaaS API → Farmer, request-reply | Revoke a sprout's JWT/NKey (backs "unenroll"). |
| `internal.sprouts.list` | SaaS API → Farmer, request-reply | Tenant-scoped, **paginated** version of `sprouts.list` (the existing subject returns an unbounded list — not viable at 1M sprouts). |
| `internal.sprout.action` | SaaS API → Farmer, request-reply | Dispatches `cmd.run`/`cook`/`self_update` for a resolved `sprout_id`, given the batch context from §1.5 (and, for updates, §1.8). Farmer independently checks the sprout's own stored `tenant_id` against the caller's asserted `tenant_id` before executing — the point-of-effect check that holds even if something upstream is wrong. |

```json
// internal.sprout.mint request
{ "tenant_id": "t_8f2a", "nkey_pub": "U...", "sprout_id_hint": "web-01" }
// reply
{
  "sprout_id": "s_1",
  "nats_jwt": "<signed NATS User JWT, ed25519-nkey alg, Account-signed>",
  "gateway_jwt": "<signed gateway JWT, EdDSA alg, gateway-key-signed, for Envoy jwt_authn>",
  "nkey_identity": "U..."
}
```
Two tokens, one signing event, minted together every time this subject (or the enrollment flow's internal equivalent, §3.3) is invoked — including at rotation, not just first enrollment. See `grlx-envoy-enrollment-design.md` for why a single JWT can't serve both the NATS and Envoy gates.

```json
// internal.sprouts.list request
{ "tenant_id": "t_8f2a", "limit": 100, "cursor": "s_87" }
// reply
{ "sprouts": [ /* ... */ ], "next_cursor": "s_142" }
```

`internal.sprout.action`'s `action.type` enum now includes `self_update`
alongside `cmd.run`/`cook` (§1.8):

```json
// internal.sprout.action request, self_update variant
{
  "tenant_id": "t_8f2a",
  "sprout_id": "s_1",
  "action": {
    "type": "self_update",
    "params": {
      "version": "v2.4.1",
      "artifact_url": "https://<farmer-recipe-endpoint>/artifacts/sprout-v2.4.1-linux-amd64",
      "checksum_sha256": "…"
    }
  }
}
```

`artifact_url` points at **CloudXP's own object storage**, served through
the same authenticated recipe HTTP endpoint already designed in Phase 1 of
the master plan (`grlx-master-plan.md`) — not upstream's release CDN. This
means the artifact-serving auth model, TLS, and object-storage backing are
all already-designed infrastructure being reused, not new plumbing.

### 2.3 Rollout safety — fleet updates

A failed `cmd.run` or `cook` step leaves a sprout in a knowable, recoverable
state. A failed self-update can leave a sprout **unable to reconnect to
farmer at all** — the failure mode is categorically worse. Before any
`self_update` action type is enabled, at minimum:

- **Smaller default batch size and stricter default gate** than §1.5's
  ordinary actions — don't inherit the general-purpose defaults.
- **The backup/restore-on-failure behavior sprout's own `internal/update`
  package already scaffolds** (`commitBinaryUpdate`'s rename-to-backup,
  swap-in-new, restore-on-failure sequence) must actually work and be
  exercised in testing before this ships — it's real code today, just not
  wired to anything.
- **Staged rollout should reuse workstream L's farmer-side batch-and-gate
  design** (`grlx-sprout-orchestration.md`'s open item: dispatch to a batch,
  wait on a probe/job-status health signal, proceed to the next batch) —
  self-update is the single best-motivating use case for that mechanism,
  arguably more than ordinary recipe rollout.
- A sprout that goes dark mid-update should be distinguishable, in
  `asset_action_items` status, from a sprout that failed for an unrelated
  reason — "unresponsive after update" deserves its own status value, not
  a generic `failed`, since the operational response differs (the
  backup/restore path may have already self-healed it).

---

### 2.4 JWKS endpoint — for Envoy, not internal-only

Unlike everything else in §2, this route is deliberately **not** on the privileged `grlx.internal.*`/system-NATS-identity path — it serves public key material only, so it carries no confidentiality requirement, the same trust model as any standard `/.well-known/jwks.json`.

| Method | Path | Notes |
|---|---|---|
| `GET` | `https://enroll.<region>/v1/.well-known/jwks.json` | Served by farmer, plain HTTP (TLS-terminated, unauthenticated). Envoy's `jwt_authn` filter polls this via `remote_jwks` (5–10 min refresh), not `local_jwks`. |

```json
// response
{
  "keys": [
    { "kty": "OKP", "crv": "Ed25519", "x": "<base64url pubkey>", "kid": "gw-2026-q3", "use": "sig" }
  ]
}
```

Contains the **gateway signing key's** public key only (see `grlx-nats-jwt-auth-design.md`'s key-custody section) — one entry, or two during the gateway key's own rotation overlap window (old + new `kid`). Never grows with tenant count: this is not a per-tenant Account-key JWKS, since Envoy's `jwt_authn` check never needs tenant granularity. Farmer already holds this public key (fetched from OpenBao alongside the signing operation itself), so this route is a pure data-transformation read, no new secret access.

## 3. Sprout enrollment flow

The one moment in the whole system where a caller has no credential yet. Borrowed deliberately from `kubeadm`'s join-token model, and it's the one place worth a genuine security review before trusting it in production — everything downstream assumes whatever identity this issues is real.

### 3.1 Token format

`{key_id}.{secret}` — not just one opaque blob. `key_id` is a short random public identifier (e.g. 8-char base32), stored in plaintext and used purely as an **indexed lookup key**; `secret` is a 32-byte random value whose SHA-256 hash is what's actually stored (`saas.enrollment_keys.key_hash`). This mirrors `kubeadm`'s split for exactly the same reason: without it, validating a token means scanning and hash-comparing against every live key in the table; with it, it's a single indexed `WHERE key_id = ?` followed by one constant-time hash comparison.

`POST /tenants/{tenant_id}/enrollment-keys` (§1.2) returns the full `{key_id}.{secret}` string once — that's what goes into the Ansible playbook's `grlx_join_token` variable.

### 3.2 The endpoint itself

`POST https://enroll.<region>/v1/enroll` — served by farmer, deliberately **not** behind Envoy's `jwt_authn` filter (a sprout enrolling has no JWT yet), but still TLS-terminated and, per the open item carried from the original enrollment design, deserving of Envoy-level IP-based rate limiting since it's the one DMZ-facing route without a JWT gate.

```json
// request
{ "join_token": "ab3f9k2q.9fT...longsecret", "nkey_pub": "U...", "hostname": "web-01" }

// success response
{
  "sprout_id": "s_1",
  "nats_jwt": "<signed NATS User JWT>",
  "gateway_jwt": "<signed gateway JWT, presented to Envoy on the ws upgrade and the recipe endpoint>",
  "nats_urls": ["wss://bus1.dmz...", "wss://bus2.dmz..."]
}

// failure response — deliberately generic, see §3.4
{ "error": "enrollment_failed" }
```

### 3.3 What farmer does, step by step

1. **Idempotency check first**: `SELECT * FROM farmer.sprouts WHERE nkey_pub = ?`. If a sprout already exists for this exact public key, return its existing JWT immediately and stop — no enrollment-key touched. This covers the ordinary case of Ansible retrying after a dropped connection: the sprout generates its keypair once, locally, before ever calling out, so a retry presents the same `nkey_pub` and gets the same identity back rather than burning a second use of a possibly single-use token.
2. **Look up the key**: `SELECT tenant_id, key_hash, expiry, max_uses, used_count, revoked, asset_id FROM saas.enrollment_keys WHERE key_id = ?` — a plain read via farmer's existing grant into `saas`.
3. **Validate**: unknown `key_id`, revoked, expired, or `used_count >= max_uses` all fail the same way (§3.4).
4. **Atomically redeem** — this is the one deliberate, narrow exception to the single-writer-per-schema rule:
   ```sql
   UPDATE saas.enrollment_keys
   SET used_count = used_count + 1, last_used_at = NOW()
   WHERE key_id = ? AND used_count < max_uses AND revoked = FALSE AND expiry > NOW();
   ```
   Checking `affected_rows = 1` after this statement *is* the concurrency control — two sprouts racing to redeem the same near-exhausted token can't both succeed, because the `WHERE` guard makes the whole check-and-increment atomic in one statement. This needs a narrow grant, not blanket write access:
   ```sql
   GRANT UPDATE (used_count, last_used_at) ON saas.enrollment_keys TO 'farmer_svc'@'%';
   ```
   Everything else in `saas` stays read-only to farmer, exactly as before.
5. **Mint**: generate the sprout's paired NATS User JWT (under the tenant's Account) and gateway JWT (under the platform-wide gateway signing key) in one signing step (`internal.sprout.mint`, §2.2), insert the row into `farmer.sprouts`.
6. **Respond** to the sprout synchronously — Ansible is blocking on this HTTP call, so this step can't be async.
7. **Notify the SaaS API**: publish `internal.sprout.enrolled` (`tenant_id`, `sprout_id`, `key_id`, `asset_id` if the key carried one). The SaaS API uses this to auto-create the `asset_links` row when the enrollment key was issued with a known `asset_id`, and to fire an `enrollment.succeeded` webhook (§1.7) — this is a fire-and-forget notification, not something the sprout's own response waits on.

### 3.4 Failure responses stay generic

Unknown key, expired, revoked, and exhausted all return the same `enrollment_failed` body. Distinguishing them in the response would hand an attacker a free oracle for guessing valid `key_id`s or timing a race against expiry — the specific reason is logged in farmer's audit trail (feeding the CERT-In/DPDP retention question already flagged in §6) but never returned over the wire.

---

## 4. Cross-cutting conventions

- **Pagination**: 100-item cap on caller-supplied ID batches (§1.4, §1.5); cursor-based for open-ended lists (`internal.sprouts.list`) — the existing `sprouts.list`/`jobs.list` farmer subjects lack this and should not be exposed externally as-is.
- **Async pattern**: every long-running operation (tenant provision/deprovision, batch actions, fleet updates) follows create-row-then-poll — `202` + a status endpoint, backed by an outbox-style table in `saas` schema, never a bare NATS publish, because NATS core (no JetStream, per the settled architecture) gives no redelivery guarantee.
- **Tenant safety**: every query that resolves a caller-supplied ID (`asset_id`, `sprout_id`) always includes `tenant_id` in the same `WHERE` clause — a mismatch resolves to "not found," never a distinguishable authorization error.
- **Versioning**: `/v1` prefix on the external API; internal subjects aren't versioned in the path — farmer and the SaaS API deploy together as one control plane, so subject compatibility is managed by coordinated release rather than version negotiation.

---

## 5. Schema reference

### 4.1 Grants
```sql
GRANT ALL    ON farmer.* TO 'farmer_svc'@'%';
GRANT SELECT ON saas.*   TO 'farmer_svc'@'%';
GRANT ALL    ON saas.*   TO 'saas_svc'@'%';
GRANT SELECT ON farmer.* TO 'saas_svc'@'%';
```
One narrow, deliberate exception to single-writer: farmer also gets `GRANT UPDATE (used_count, last_used_at) ON saas.enrollment_keys` — the atomic redemption counter needed at enrollment time (§3.3). Nothing else in `saas` is writable by farmer.

No cross-schema foreign keys — `tenant_id`/`sprout_id` are enforced by convention at the application layer, to avoid adding Galera certification overhead across schemas at 1M-sprout scale.

### 4.2 `saas` schema tables (new)
```sql
tenants               (id, name, status, plan_id, created_at, updated_at)
provisioning_jobs     (id, tenant_id, type, status, attempts, last_error, created_at, updated_at)
enrollment_keys        (id, tenant_id, key_hash, expiry, max_uses, used_count, revoked)
asset_links           (id, tenant_id, sprout_id UNIQUE, asset_id UNIQUE, linked_at)
asset_action_batches  (id, tenant_id, requested_asset_ids, created_at)
asset_action_items    (batch_id, asset_id, sprout_id, jid, status)
```

### 4.3 Fleet update tables (new)
```sql
fleet_versions        (id, version, artifact_url, checksum_sha256, released_at, notes)
tenant_update_policy  (tenant_id PK, approved_version, auto_update BOOLEAN,
                        rollout_window_start, rollout_window_end, updated_at)
```
No new batch/item tables — `asset_action_batches`/`asset_action_items` (§4.2)
already cover dispatch tracking for any `action.type`, including
`self_update`.

---

## 6. Open items (not yet designed)

- Human-user auth for the SaaS API itself (API keys vs. SSO/OIDC) — §1.7.
- Webhook delivery/retry semantics — §1.7.
- Billing/metering integration specifics — §1.7.
- Quota/rate-limit enforcement values and where exactly they're checked (SaaS API is the intended enforcement point, per earlier discussion, but no limits have been set).
- Can a sprout's `tenant_id` ever change post-enrollment, or does a tenant move always mean re-enrollment? Not decided.
- **Fleet update rollout is blocked on upstream.** `internal/update` is an
  explicitly disabled skeleton (`gogrlx/grlx#286`); `PerformUpdate()` fails
  closed today. §1.8/§2.2/§2.3's endpoints and subjects are ready to build
  against the SaaS API and schema side whenever sprout has a real signed
  update path — but don't expose `POST
  /tenants/{tenant_id}/sprouts/updates` publicly, even behind a feature
  flag, until that dependency resolves. Building the API surface now and
  the sprout-side capability later (blocked on upstream) are intentionally
  decoupled work.
