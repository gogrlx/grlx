# grlx SaaS Machine Manager — Claude Code Parallel Build Plan

This is a working playbook for building the CloudXP machine-manager fork of
`gogrlx/grlx` using Claude Code cloud agents (`claude --cloud`), running
workstreams in parallel where the roadmap allows it and gating the rest on
the dependencies already identified in the design docs.

---

## 0. Prerequisites (do this once)

1. **Repo on GitHub**, public (open source). Install the Claude GitHub App
   on it, or run `/web-setup` from a terminal with `gh` already
   authenticated against it.
2. **Commit the design docs into the repo** under `docs/design/` — cloud
   sessions only see what's in git, not this chat. Copy in:
   - `grlx-master-plan.md`
   - `grlx-fork-roadmap.md`
   - `grlx-nats-jwt-auth-design.md`
   - `grlx-envoy-enrollment-design.md`
   - `grlx-payload-encryption-design.md`
   - `grlx-sdb-secrets-design.md`
   - `grlx-1m-scale-plan.md`
   - `grlx-windows-parity-addendum.md`
   - `grlx-linux-parity-addendum.md`
   - `grlx-tls-entropy-caution.md`
   - `cloudxp-machine-manager-api-design.md`
   - `requirements.md` (the original numbered requirements list)
3. **Add a root `CLAUDE.md`** (template below) so every cloud session
   inherits the same ground rules without you repeating them in every
   prompt.
4. If your org account: confirm `allow_remote_sessions` is enabled
   (Owner sets this at claude.ai/admin-settings/claude-code).
5. Decide now whether this repo runs on Anthropic-hosted cloud sessions or
   a self-hosted environment — revisit if CERT-In/DPDP scope ever pulls
   this fork inside CloudXP proper rather than staying upstream OSS.

### Root `CLAUDE.md` template

```markdown
# grlx SaaS Machine Manager — repo conventions

Fork of gogrlx/grlx (github.com/gogrlx/grlx). See docs/design/ for the
full architecture — read grlx-master-plan.md and grlx-fork-roadmap.md
first, then the specific design doc named in your task.

## Constraints (non-negotiable)
- Licensing: Apache-2.0 / MIT dependencies only. Flag anything else
  before adding it as a dependency, don't just add it.
- Go, existing module layout. Don't introduce a second language/runtime
  without flagging it first.
- No CGO (blocks a CGO-free build target elsewhere in the plan).
- Follow the existing self-registering ingredient plugin pattern
  (see internal/ingredients/*) for any new ingredient.

## Workflow
- Work only within the file scope stated in your task brief. If you need
  to touch a file outside that scope, stop and say why instead of doing it.
- Write unit tests for new logic. `go test ./...` must pass before you
  consider the task done.
- Commit messages: `<workstream-id>: <what>` (e.g. `B: mint sprout JWT
  via nats-io/jwt/v2`).
- Open a PR when done. In the PR description, state what you built,
  what you deliberately left out or deferred, and any open question.

## Security review flag
If your task brief includes the line "FLAG FOR SECURITY REVIEW", say so
explicitly at the top of the PR description, and do not describe the
work as "done" or "safe to merge" — only as "ready for review."
```

---

## 1. Wave 0 — launch immediately, no dependencies

Run each of these as its own `claude --cloud "..."` call (or paste them
into a Claude Code **Project** so one lead conversation dispatches and
tracks all nine).

**1. Workstream B — NATS JWT auth**
```
claude --cloud "Implement workstream B from docs/design/grlx-fork-roadmap.md:
replace grlx's custom NKey allow-list with NATS decentralized JWT auth
(Operator -> Account-per-tenant -> User-per-sprout). Full design in
docs/design/grlx-nats-jwt-auth-design.md. Build: Operator/SystemAccount
keypair bootstrap, 'full' resolver config, JWT minting via nats-io/jwt/v2
+ the existing nkeys dependency, and a push-to-resolver mechanism over a
NATS system-account connection that replaces pki.ReloadNKeys()'s
in-process ReloadOptions() call. Preserve today's accept/deny/reject
sprout lifecycle, remapped onto JWT issuance/revocation. Scope: internal/pki/
only. Do not touch internal/natsapi/subjects.go. Write tests against a
local nats-server. FLAG FOR SECURITY REVIEW in the PR description — this
is the trust-chain root."
```

**2. Workstream D — queue-group fix**
```
claude --cloud "Implement workstream D from docs/design/grlx-fork-roadmap.md:
change nc.Subscribe(...) to nc.QueueSubscribe(subject, \"grlx-core\", ...)
in internal/natsapi/router.go. Also review internal/jobs/listener.go and
internal/facts/listener.go (currently plain Subscribe) and decide, with
reasoning in the PR, whether they should be queue-grouped too or whether
their fan-out is intentional. Small, mechanical change; add/adjust tests
covering the subscription pattern."
```

**3. Workstream F — OpenBao TLS**
```
claude --cloud "Implement workstream F from docs/design/grlx-fork-roadmap.md:
replace internal/certs/tls.go's self-signed local CA (genCACert, GenCert,
RotateTLSCerts, forceRegenCert) with calls to OpenBao's PKI secrets
engine, writing results to the same CertFile/KeyFile/RootCA paths that
internal/pki/nats.go and the API server already consume. Rotation should
use OpenBao lease renewal instead of a self-managed timer. Scope:
internal/certs/ only. Write tests against a local OpenBao dev server.
FLAG FOR SECURITY REVIEW — this is certificate/key-handling code."
```

**4. Workstream G.1 + G.3 — Windows service provider + registry ingredient**
```
claude --cloud "Implement G.1 and G.3 from docs/design/grlx-windows-parity-addendum.md:
(1) a Windows SCM-backed service provider matching the create/start/
stop/delete/status surface of the existing systemd/openrc/rcd providers
in internal/ingredients/service/, using golang.org/x/sys/windows/svc/mgr.
(2) a new registry ingredient from scratch using
golang.org/x/sys/windows/registry, following the self-registering
ingredient pattern used elsewhere in internal/ingredients/. Cross-compile
with GOOS=windows to confirm it builds; note in the PR that this cannot
be runtime-validated outside a real Windows host."
```

**5. Workstream G.5 + G.8 + G.9 — Windows facts + PowerShell/CLI module batch**
```
claude --cloud "Implement G.5, G.8, and G.9 from docs/design/grlx-windows-parity-addendum.md:
(1) hardware/BIOS facts via the SMBIOS route described (prefer
digitalocean/go-smbios or jaypipes/ghw's SMBIOS reader over WMI/COM for
this subset), wired into the existing facts collection path. (2) the
PowerShell-only module batch (win_servermanager, win_dsc, win_psget,
win_iis, win_pki, win_snmp, win_smtp_server, win_appx) as
exec.Command('powershell.exe', ...) wrappers with ConvertTo-Json output
parsing, reusing the existing cmd ingredient execution path — build the
first one, get the pattern right, then repeat it for the rest. (3) the
CLI-tool-only batch (win_firewall via netsh, win_dns_client, win_auditpol,
win_powercfg, win_certutil) as plain-text-parsing wrappers. Cross-compile
GOOS=windows; flag that none of this is runtime-verified."
```

**6. Workstream H.4 + H.5 — Linux mount/fstab + cron ingredients**
```
claude --cloud "Implement H.4 and H.5 from docs/design/grlx-linux-parity-addendum.md:
(1) a mount/fstab ingredient using golang.org/x/sys/unix's Mount/Unmount
syscalls plus a plain parser/writer for /etc/fstab — keep a cmd-based
fallback path for NFS/CIFS mount types, since raw mount(2) doesn't
dispatch to mount.nfs/mount.cifs the way the mount binary does. (2) a
per-user crontab ingredient (list/set/remove, idempotent add/remove-by-
identifier) reading/writing the crontab format directly or shelling to
the crontab binary. Follow the self-registering ingredient pattern.
Write tests."
```

**7. Workstream L — sprout ingredients + cook-engine primitives**
```
claude --cloud "Implement workstream L from docs/design/grlx-sprout-orchestration.md:
new atomic ingredients probe.http, probe.database, wait, file.sync,
file.line (check existing file ingredient coverage first before treating
these as net-new), and a file.copy enrichment (bidirectional push/pull,
glob, exclude, mkdir, chmod+x). Separately, cook-engine orchestration
primitives that apply to any ingredient: conditional step execution
(cond, with negation — confirm nothing equivalent exists first),
deferred/on_exit actions, variable registration/passing between steps
with a sensitive:true flag, and runtime context variables
({GRLX_SPROUT_ID}, {GRLX_TENANT_ID}). Do not build a probe-internal
workflow engine or a separate probe-only secrets mechanism — probe is
atomic ingredients only, and secret injection should reference the
sdb:// interface by name (stub the interface if workstream K isn't
merged yet). Everything sensitive-marked must be redacted from the
persisted job log AND from verbose/debug output, never just the log —
write a test that specifically exercises the debug-output path. FLAG
FOR SECURITY REVIEW on the sensitive-value redaction path only."
```

**8. Workstream K — SDB secret resolution (v1 tier)**
```
claude --cloud "Implement the v1 tier of docs/design/grlx-sdb-secrets-design.md:
a sprout-side SecretProvider interface (Get(ctx, ref) (value, error))
behind the sdb:// URI scheme, with self-registering implementations for
OpenBao/customer Vault (certificate-based auth, file-watched/hot-reloaded
so a rotated customer cert doesn't need a sprout restart), and Azure/
AWS/GCP via managed identity only (no JWT federation in this pass).
Follow the same self-registering plugin pattern as other ingredients.
Write tests with a mocked OpenBao client at minimum."
```

**9. SaaS API scaffold — tenants, enrollment-keys, schema**
```
claude --cloud "Scaffold the new SaaS API service described in
docs/design/cloudxp-machine-manager-api-design.md, sections 1.1, 1.2,
and 5. Standard Go REST service with GORM against the 'saas' schema in
the shared PXC cluster (do not touch the 'farmer' schema). Implement:
POST/GET/PATCH/DELETE /tenants and GET /tenants/{tenant_id}/status
(async provisioning per the doc's pending/active/failed states, backed
by a provisioning_jobs outbox table), and POST/GET/DELETE
/tenants/{tenant_id}/enrollment-keys per section 1.2/3.1's key_id.secret
split (key_id plaintext indexed lookup, secret's SHA-256 stored as
key_hash, never the raw secret). Do not implement the actual farmer-side
NATS calls yet (internal.tenant.provision etc.) — stub them with a TODO
referencing section 2.2, since that depends on workstream B/H landing
first. Write tests. FLAG FOR SECURITY REVIEW on the enrollment-key
hashing/lookup logic specifically."
```

---

## 2. Wave 1 — hold until B (and ideally A) are merged to `main`

**Workstream A — PXC/Valkey/object storage** (can actually start in Wave 0
in parallel with B, since it touches different files — include it there if
you have capacity):
```
claude --cloud "Implement workstream A from docs/design/grlx-fork-roadmap.md
and grlx-master-plan.md Phase 1: move PKI, props/facts, and RBAC to
Percona XtraDB Cluster with read-through and no in-memory cache (fixes
the cross-replica divergence bug in props/store.go directly). Move
connection-state/heartbeat to Valkey TTL keys driven by NATS's own
$SYS.ACCOUNT.*.CONNECT/DISCONNECT events, not an app-level heartbeat.
Move recipe storage off farmer's local-disk basepath
(internal/cook/farmercook.go's ResolveRecipeFilePath/os.ReadFile) to
object storage (S3/MinIO), git remaining the source of truth synced on
merge, with a new authenticated HTTP endpoint serving the read path
(reuse the pattern in internal/ingredients/file/http/provider.go). Every
query against the PXC-backed tables must scope by tenant_id in the same
WHERE clause. FLAG FOR SECURITY REVIEW on the tenant-scoping discipline
specifically (workstream A.1)."
```

**Workstream C — DMZ split** (needs B settled):
```
claude --cloud "Implement workstream C from docs/design/grlx-fork-roadmap.md:
split cmd/farmer/main.go into two deployables — a bus process
(RunNATSServer() + TLS/NKey config) for the DMZ, and a core process
(ConnectFarmer() + all registered subscribers) for non-DMZ, outbound-only
to the bus. Wire however workstream B's JWT push mechanism delivers
updates to the DMZ-side bus. Separate lifecycle/shutdown handling for
each binary."
```

**Workstream H — Envoy gateway + enrollment subsystem** (needs B):
```
claude --cloud "Implement workstream H from docs/design/grlx-fork-roadmap.md
and docs/design/grlx-envoy-enrollment-design.md: Envoy config in front of
NATS's websocket listener validating each sprout's JWT via a jwt_authn
filter against JWKS before the connection reaches nats-server, plus a
second Envoy route proxying authenticated recipe-download requests to
non-DMZ. Build the enrollment endpoint per section 3 of
cloudxp-machine-manager-api-design.md: token format {key_id}.{secret},
constant-time hash comparison against saas.enrollment_keys, atomic
redemption via the UPDATE ... WHERE used_count < max_uses guard shown in
that doc, idempotency check on nkey_pub first, generic
'enrollment_failed' response for every failure case (never distinguish
unknown/expired/revoked/exhausted in the response). Response returns the
sprout's JWT + NKey identity + tenant X25519 public key in one round
trip. FLAG FOR SECURITY REVIEW — this is the literal front door of the
trust chain."
```

---

## 3. Wave 2 — hold until A + H are merged

**Workstream E — multi-tenancy:**
```
claude --cloud "Implement workstream E from docs/design/grlx-fork-roadmap.md,
using the simplification from grlx-nats-jwt-auth-design.md: one NATS
Account per tenant means internal/natsapi/subjects.go's subject strings
can stay unchanged, since isolation is enforced by which Account a
connection authenticated into. Re-key internal/pki/pki.go's storage by
(tenant_id, sprout_id) instead of sprout_id alone. Add a tenant field to
internal/rbac's cohort/role maps. Make FarmerOrganization dynamic —
one Account created/pushed per tenant at onboarding instead of one
static config-loaded string. FLAG FOR SECURITY REVIEW — tenant isolation
correctness."
```

**Workstream I — recipe storage migration** (mostly done as part of A above
if you sequenced it that way; otherwise the HTTP endpoint half, gated on H's
Envoy route):
```
claude --cloud "Finish workstream I: confirm the recipe HTTP endpoint from
workstream A is served behind workstream H's Envoy JWT-gated route, and
that internal/natsapi/recipes.go's old NATS-based recipe delivery is
removed in favor of it."
```

**Workstream J — payload encryption + rotation:**
```
claude --cloud "Implement workstream J from docs/design/grlx-fork-roadmap.md
and docs/design/grlx-payload-encryption-design.md: NaCl box (X25519)
encryption of NATS payloads. One tenant keypair (farmer-side,
OpenBao-custodied private key, per workstream F), one sprout keypair
(generated locally at enrollment, private key never transmitted — not
even encrypted). Bootstrap both public keys through workstream H's
enrollment response. Build a shared encrypt/decrypt request/response
helper in internal/natsapi so this is transparent to individual handlers
rather than opt-in per handler. Key rotation is sprout-initiated only:
sprout generates a new keypair locally, sends only the new public key,
farmer may trigger rotation but never generates or holds a sprout's
private key. Grace-period overlap reusing the existing PKI accept/deny/
revoke lifecycle. FLAG FOR SECURITY REVIEW — this is cryptographic code
defending against a compromised DMZ bus."
```

---

## 4. Ongoing / fully parallel, no gating — launch whenever you have capacity

```
claude --cloud "Implement G.2 (Windows user/group provider) from
docs/design/grlx-windows-parity-addendum.md using
deploymenttheory/go-bindings-win32's netmanagement package. FLAG FOR
SECURITY REVIEW — user/group creation, and the dependency itself is
young (v0.2.x) so note that in the PR."

claude --cloud "Implement G.4 (Windows DACL/ACL ingredient) from
docs/design/grlx-windows-parity-addendum.md using hectane/go-acl for
file ACLs first, then extend to registry-key ACLs (SE_REGISTRY_KEY).
FLAG FOR SECURITY REVIEW — propagation/inheritance semantics are a
security-relevant bug class, not just functional."

claude --cloud "Implement G.6 (Task Scheduler, Windows Update, Shortcut
COM ingredients) from docs/design/grlx-windows-parity-addendum.md using
go-ole/go-ole. Start with the Shortcut ingredient (IShellLink, smallest
scope) to prove the COM lifecycle pattern before Task Scheduler and WUA.
FLAG FOR SECURITY REVIEW — COM lifecycle bugs (missed Release, wrong
apartment threading) are easy to miss in review."

claude --cloud "Scope and implement a v1 subset of G.7 (LGPO) from
docs/design/grlx-windows-parity-addendum.md — parse registry.pol
(encoding/binary) and ADMX/ADML (encoding/xml), and propose in the PR
description which policy subset to cover for a first pass rather than
attempting full Salt win_lgpo parity."

claude --cloud "Implement H.1 (network/route management) from
docs/design/grlx-linux-parity-addendum.md using vishvananda/netlink.
Include a 'verify connectivity survives the change, or roll back'
pattern in the ingredient itself, since a bad route/interface change can
cut off the sprout's own connectivity to farmer."

claude --cloud "Implement H.2 (nftables firewall ingredient) from
docs/design/grlx-linux-parity-addendum.md using google/nftables.
FLAG FOR SECURITY REVIEW — a firewall ingredient can lock out or expose
a host; treat with the same discipline as the auth workstreams."

claude --cloud "Implement H.3 (SELinux ingredient) from
docs/design/grlx-linux-parity-addendum.md using opencontainers/selinux.
FLAG FOR SECURITY REVIEW — CERT-In/DPDP-relevant: silently degrading to
permissive is a compliance-visible failure, not just a bug."
```

---

## 5. Orchestrator prompt — paste into one lead Claude Code session

Use this if you'd rather have Claude dispatch and track Wave 0 for you
instead of running the nine commands above by hand. Run it as a **local**
`claude` session at the repo root (it needs Bash to shell out to
`claude --cloud`), or as a Claude Code **Project**.

```
You are coordinating the Wave 0 buildout of the grlx SaaS Machine Manager
fork. Read docs/design/grlx-master-plan.md and
docs/design/grlx-fork-roadmap.md for context first.

Create docs/BUILD-STATUS.md with a table of these nine workstreams:
B, D, F, G.1+G.3, G.5+G.8+G.9, H.4+H.5, L, K, SaaS-API-scaffold.
Columns: workstream | one-line description | cloud session ID | status
(dispatched / in review / merged) | needs security review (y/n).

For each workstream, run `claude --cloud "<task brief>"` using the exact
task briefs from claude-code-parallel-build-plan.md sections 1.1 through
1.9 in this repo (copy them verbatim — do not paraphrase or shorten
them). Record each returned session ID into BUILD-STATUS.md.

Do not dispatch Wave 1 or Wave 2 workstreams (A, C, H, E, I, J) yourself
— those depend on B and A being merged first. Stop after dispatching
Wave 0 and tell me to review the nine PRs. When I tell you B (and A, if
you've also started it) are merged, come back and I'll ask you to
dispatch Wave 1.

For any workstream whose brief says "FLAG FOR SECURITY REVIEW," mark
that in BUILD-STATUS.md and do not represent it as ready to merge, only
ready for review, even after its tests pass.
```

---

## 6. Recap of the gating logic (why the waves are ordered this way)

- **B gates C and H** — both touch the auth/trust-chain plumbing B builds.
- **A gates E and I, and effectively D's "real" horizontal scaling** —
  tenant-scoping and recipe storage both assume shared PXC/object storage
  exists first.
- **H gates J** — payload encryption bootstraps its keys through H's
  enrollment response.
- **G (Windows), H.1–H.5 (Linux ingredients), L, K, and the SaaS API
  scaffold** have no dependency on the messaging/storage/auth work and are
  safe to run in Wave 0 regardless of what else is in flight.
- Workstreams flagged 🔴/🟡 in the roadmap docs (auth, crypto, tenant
  isolation, firewall, SELinux, user/group creation) are fine as
  unattended first drafts but should never be treated as merge-ready on
  green tests alone — that's what the "FLAG FOR SECURITY REVIEW" line in
  each prompt is for.
