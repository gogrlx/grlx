# grlx → Multi-tenant SaaS Fork: Roadmap

Based on direct inspection of the `gogrlx/grlx` source (not just docs). File/function references are exact as of the cloned commit.

---

## 1. Windows / Linux platform support — current state

**Question: does grlx support Windows and Linux today, for file/service/package/registry management and running commands/scripts?**

| Capability | Linux | Windows | Notes |
|---|---|---|---|
| Run commands / scripts | ✅ | 🟡 partial | `internal/ingredients/cmd/runas_windows.go` and `internal/shell/sprout_windows.go` already exist — run-as-user and interactive shell have Windows code paths. Not in the release build matrix, so untested in CI/release. |
| File management | ✅ | 🟡 likely mostly works | File ingredients (`fileManaged`, `fileContent`, `fileDirectory`, etc.) use Go's standard `os`/`path/filepath`, which is cross-platform. Ownership/permission semantics (Unix chmod/chown vs Windows ACLs) haven't been verified and likely need real testing/adjustment. |
| Service management | ✅ (systemd, OpenRC, rc.d) | ❌ none | `internal/ingredients/service/` has providers for `systemd`, `openrc`, `rcd` only. No Windows Service Control Manager (SCM) provider exists. |
| Package management | ✅ (apt, dnf, pacman, apk, flatpak, snap...) | 🟡 dependency ready, not wired | grlx's `pkg` ingredient delegates entirely to a separate library, `github.com/gogrlx/snack`, via `snack/detect` auto-detection. **snack already has a `winget` package manager wrapper (marked supported)** — so the underlying capability exists, but it's never been exercised through grlx's `pkg` ingredient on Windows, and Windows isn't in the release matrix. |
| User/group management | ✅ | ❌ broken as-is, not just missing | `internal/ingredients/user/userPresent.go` and `internal/ingredients/group/groupPresent.go` **hard-shell out to `useradd`, `usermod`, `groupadd`** — literal Unix commands. On Windows these binaries don't exist, so this doesn't degrade gracefully, it fails outright. A real Windows provider (`net user`/`net localgroup`, or the Win32 NetUserAdd/NetLocalGroupAdd APIs) is net-new code, not a config flag. |
| Registry management | N/A | ❌ doesn't exist | No registry ingredient anywhere in the codebase. Would be a new ingredient type from scratch. |

**Officially released builds today (from `.goreleaser.yaml`):** `farmer` and `sprout` are built **only for `linux`** (amd64/386/arm64/arm). The `grlx` CLI builds for `linux` and `darwin` — not Windows either, despite a leftover "zip format for windows archives" override in the archive config (suggests Windows was planned/attempted before, not that it's live).

**Bottom line:** grlx is a Linux-first project with a few forward-looking Windows stubs (shell, run-as-user) and a promising but unused Windows package-manager dependency. Getting to "manage files, services, packages, and registry on Windows, run commands/scripts" is a real, scoped engineering effort — not a recompile — concentrated in: a new service provider, wiring up the already-Windows-capable `snack`/winget path, a new user/group provider, and a new registry ingredient.

---

## 2. Consolidated change list, risk, and timeline

Effort estimates assume one strong Go engineer per workstream unless noted; several workstreams can run in parallel.

### A. Storage layer — PXC (metadata) + Valkey (heartbeat) + object storage (logs, recipes)
*(Supersedes the original S3-for-everything and later etcd-based framing of this workstream — settled decision per `grlx-master-plan.md` Phase 1.)*
| | |
|---|---|
| **Change** | PKI, props/facts, RBAC → Percona XtraDB Cluster (read-through, no in-memory cache — this is what actually fixes `props/store.go`'s cross-replica divergence bug, not just a backend swap). Connection-state/heartbeat → Valkey TTL keys, driven by NATS's own `$SYS.ACCOUNT.*.CONNECT`/`DISCONNECT` events rather than an app-level heartbeat protocol or a database write path. Job logs and recipes → object storage (S3/MinIO), unchanged from the original plan — both are read-mostly, blob-shaped, and were never a good fit for relational storage. |
| **Effort** | PXC swap for `pki.go`/`jobs` metadata: mechanical, same read-through seam as before. `props/store.go`: no longer needs a bespoke read-through/invalidation redesign — moving to PXC directly removes the in-memory-cache trap that made this the riskiest part of the original S3-based plan. Valkey heartbeat: new logic on both farmer (subscribe to NATS system events, set/delete TTL keys) and sprout (none — sprout does nothing extra, this is farmer-side only). Recipe storage: move off farmer's local-disk `basepath` (`internal/cook/farmercook.go`'s `ResolveRecipeFilePath`/`os.ReadFile`) to S3, since local disk doesn't survive horizontal core scaling. |
| **Timeline** | ~1.5–2 weeks for PXC-backed pki/props/RBAC. ~3–5 days for the Valkey heartbeat mechanism. ~3–5 days for recipe storage migration + the recipe-serving HTTP endpoint's read path. |
| **Claude Code fit** | 🟢 High for the PXC and object-storage swaps — clean seams, mechanical once the target schema/bucket layout is decided. 🟡 Medium for the Valkey heartbeat listener — new protocol-shaped logic (subscribing to NATS system events, TTL key lifecycle) worth testing under real connect/disconnect/restart scenarios before trusting, not just unit-tested. |
| **Risk** | 🟢 Low-medium — PXC and object storage are both mature, well-understood technology; the main risk is application-level tenant scoping (`tenant_id` on every query) needing the same discipline PXC doesn't enforce natively the way NATS Accounts do for messaging. |

### A.1 Application-level tenant isolation on PXC
| | |
|---|---|
| **Change** | `tenant_id` scoping on every query against the PXC-backed PKI/props/RBAC tables (or schema-per-tenant), since PXC gives no protocol-level isolation the way NATS Accounts do for the messaging layer. |
| **Effort** | Discipline and review work more than net-new code — a query-layer convention applied consistently, not a subsystem. |
| **Timeline** | Ongoing discipline through workstream A's implementation, not a separate phase. |
| **Claude Code fit** | 🟡 Medium — an agent can apply a stated tenant-scoping convention consistently across query sites, but a missed scoping check is a data-leak-class bug; this specific class of change deserves a dedicated security review pass, not just tests passing. |
| **Risk** | 🟡 Medium — the single most-repeated caution across the whole plan: this doesn't come for free just from picking a mature database. |



### B. NATS decentralized JWT auth model
*(Decision made: option 2 below — Operator → Account-per-tenant → User-per-sprout JWT hierarchy. Full design in `grlx-nats-jwt-auth-design.md`.)*
| | |
|---|---|
| **Change** | Stop calling `RunNATSServer()` in `cmd/farmer/main.go`; point `ConnectFarmer()`'s `config.FarmerBusURL` at an external bus. Decide the auth-injection model: (1) keep custom NKey allow-lists (`internal/pki/nats.go`'s `ReloadNKeys()`) via a sidecar that rewrites config + reloads the external node, or (2) adopt NATS's native decentralized JWT auth (operator/account keys, resolver) so any node — including future cluster members — can validate without a live call back to core. |
| **Effort** | Bus repointing itself: trivial (few lines). Auth model: **(1) is moderate** (~1 week, a config-writing + reload sidecar), **(2) is substantial** (~2–3 weeks) — effectively replacing `internal/pki/nats.go`'s auth mechanism end to end, but is the right foundation if you actually want clustering/HA on the bus later. |
| **Timeline** | 1 week (option 1) to 3 weeks (option 2, recommended if clustering is a near-term goal). |
| **Claude Code fit** | 🟡 Medium-high — once the design is settled (see companion doc), translating it into code (JWT minting via `nats-io/jwt`+`nkeys`, resolver config, the push mechanism) is executable, mechanical work an agent can iterate on against a real local `nats-server` (or several, for cluster testing) in-sandbox. The operator/account key-custody design and getting the trust chain exactly right is the part to keep under direct human review — a subtle mistake here undermines every tenant's isolation, and passing tests doesn't prove that boundary is airtight. |
| **Risk** | 🔴 High if deferred — picking option (1) first and being forced into (2) later means redoing this layer. Recommend deciding this **before** the DMZ split work below, since it determines how the DMZ-side bus gets its reloads. |

### C. DMZ / non-DMZ network split
| | |
|---|---|
| **Change** | Split `cmd/farmer/main.go` into two deployables: a bus process (`RunNATSServer()` + TLS/NKey config) for the DMZ, and a core process (`ConnectFarmer()` + all registered subscribers) for non-DMZ, outbound-only to the bus. |
| **Effort** | The two halves are already loosely coupled in the source (core dials `BusURL` like any external client) — this is decomposition, not redesign. Main work: separate lifecycle/shutdown handling, and wiring however the reload mechanism from workstream B delivers ACL changes to the DMZ-side bus. |
| **Timeline** | ~1 week, assuming workstream B's auth model is already decided. |
| **Claude Code fit** | 🟢 High — mostly extracting already-loosely-coupled code (`RunNATSServer()` / `ConnectFarmer()`) into two binaries. Well-defined, mechanical, low-risk work for an agent. |
| **Risk** | 🟢 Low, contingent on B being settled first. |

### D. Horizontal scalability of core
| | |
|---|---|
| **Change** | (1) Swap `nc.Subscribe(...)` → `nc.QueueSubscribe(subject, "grlx-core", ...)` in `internal/natsapi/router.go` (main fix); review `internal/jobs/listener.go` and `internal/facts/listener.go` (currently plain `Subscribe` too — decide if fan-out there is acceptable redundant bookkeeping or should also be queue-grouped). (2) Depends entirely on workstream A being done — without shared state, N replicas will diverge regardless of subscription pattern. |
| **Effort** | The subscribe-pattern fix is small (hours, a handful of call sites). The real horizontal-scaling enabler is workstream A. |
| **Timeline** | 1–2 days for the subscribe fix; scalability is only real once A ships. |
| **Claude Code fit** | 🟢 Very high — `nc.Subscribe` → `nc.QueueSubscribe` at a handful of call sites. Trivial, low-risk, fully mechanical; one of the best-suited changes in this whole roadmap for unattended agent work. |
| **Risk** | 🟢 Low for the fix itself; 🔴 misleading if treated as "done" before A lands — a team could ship queue groups, believe they've scaled out, and still have silent state divergence. |

### E. Multi-tenancy
| | |
|---|---|
| **Change** | Tenant-scope NATS subjects (single seam: `internal/natsapi/subjects.go`'s `SubjectPrefix`/`SproutSubjectPrefix`/`Subject()`/`SproutSubject()`), PKI store keys (`internal/pki/pki.go`, currently keyed by `SproutID` alone via `filepath.Join`), NATS permission patterns (`pki/nats.go`'s `ReloadNKeys()`), RBAC/cohorts (`internal/rbac`, flat maps with no tenant field), and `FarmerOrganization` (currently a single static string loaded once at boot via `jety`, not a per-connection value). |
| **Effort** | Subject namespacing: contained, one file. PKI re-keying: mechanical but touches ~15 functions. RBAC: additive (existing role/cohort framework, just add a tenant key). `FarmerOrganization`: the one structurally single-tenant piece — becoming dynamic touches config loading itself. |
| **Timeline** | 2–4 weeks for a working tenant-scoped core (subjects + PKI + permissions + RBAC), assuming workstream A (shared storage) lands first — tenant-scoping a local-disk store is wasted work. |
| **Claude Code fit** | 🟡 Medium-high — subject namespacing and PKI re-keying are mechanical, agent-friendly refactors. But if implemented via the JWT/Accounts-as-tenants design (see workstream B), the tenant boundary itself becomes a security property enforced by the auth model, not just application logic — same caveat as B: good for a first draft, but the isolation guarantee needs human security review before trusting it in a multi-tenant production system. |
| **Risk** | 🟡 Medium — well-bounded by the existing code structure, but should sequence *after* A, not before. |

### F. OpenBao TLS integration
| | |
|---|---|
| **Change** | Replace `internal/certs/tls.go`'s self-signed local CA (`genCACert`, `GenCert`, `RotateTLSCerts`, `forceRegenCert`) with calls to OpenBao's PKI secrets engine, writing results to the same file paths (`CertFile`/`KeyFile`/`RootCA`) that `pki/nats.go` and the API server already consume. |
| **Effort** | Contained to one file per binary (farmer + sprout each have their own cert-issuance path following the same pattern). Rotation logic needs to shift from a self-managed timer to OpenBao lease renewal. |
| **Timeline** | ~1 week. |
| **Claude Code fit** | 🟡 Medium-high — similar shape to workstream B: needs a running OpenBao dev server to iterate against (spinnable in-sandbox), and the code change itself (swap `genCACert`/`GenCert` for OpenBao API calls) is contained and mechanical once the target API behavior is understood. Certificate/key-handling code is worth a human pass before production use, same as any crypto-adjacent change. |
| **Risk** | 🟢 Low — clean seam, nothing downstream needs to change since it only consumes file paths. |

### G. Windows sprout support
| | |
|---|---|
| **Change** | (1) New Windows service provider (SCM) alongside `systemd`/`openrc`/`rcd`. (2) Wire the `pkg` ingredient's existing `snack`/winget path through for Windows and add `windows` to the release matrix for testing. (3) New Windows user/group provider — `useradd`/`usermod`/`groupadd` shelling needs a real Windows counterpart (`net user`/`net localgroup` or Win32 APIs), since it fails outright today rather than degrading. (4) New registry ingredient from scratch (e.g. via `golang.org/x/sys/windows/registry`) — the ingredient plugin pattern (self-registering `cook.RecipeCooker` implementations, seen consistently across `file`/`group`/`service`/`pkg`/`user`/`cmd`) looks well-suited to this as a new ingredient type, not a schema change. |
| **Effort** | Service provider: ~1 week. Package manager wiring + testing: 3–5 days (mostly validation, underlying capability exists). User/group provider: ~1–1.5 weeks. Registry ingredient: ~1–1.5 weeks. |
| **Timeline** | 4–6 weeks total for a genuinely usable Windows sprout across all four capabilities; can run **fully in parallel** with all farmer/core workstreams above since it's sprout-side and has no dependency on them. |
| **Claude Code fit** | 🟢 High for *writing* the code (new service provider, wiring `snack`/winget, the user/group provider, the registry ingredient, PowerShell exec wrappers) — all well within an agent's ability, and it can cross-compile with `GOOS=windows` to confirm it builds. 🔴 **Capped by one real constraint: it cannot run or validate any of it.** A coding agent's sandbox is Linux-based — `powershell.exe`, `winget`, the Win32 service/registry APIs don't exist to test against there. This is the one workstream where "implemented" and "verified" genuinely diverge: the fix isn't avoiding an agent for this work, it's making sure a real Windows CI runner or VM exists to actually exercise the result before trusting it in production. |
| **Risk** | 🟡 Medium — mostly new-code risk (untested Windows code paths, no CI today for Windows), not architectural risk. The user/group case in particular is worth flagging to whoever scopes this: it reads as "just add a build target" but is actually broken-by-design for Windows today. |

### H. Envoy gateway + enrollment subsystem
*(Full design in `grlx-envoy-enrollment-design.md`.)*
| | |
|---|---|
| **Change** | Envoy in front of NATS's websocket listener, validating each sprout's **gateway JWT** (a standard EdDSA token, distinct from and paired with the NATS User JWT — NATS's own `ed25519-nkey`-alg JWT isn't JOSE-compliant and can't be validated by Envoy's `jwt_authn` filter directly; see `grlx-envoy-enrollment-design.md`) before the connection reaches nats-server — plus a second Envoy route proxying authenticated recipe-download requests through to non-DMZ. A new, separate enrollment endpoint (not JWT-gated, since a sprout has none yet) accepting a short-lived, tenant-scoped, usage-capped one-time registration key, backed by a new PXC table, returning the sprout's paired NATS User JWT + gateway JWT + NKey identity + tenant X25519 public key in one response. |
| **Effort** | Envoy config (JWT validation filter, JWKS trust, two routes) is mostly configuration, not code. The enrollment endpoint is new: a small stateful service checking key validity/expiry/usage against PXC, then issuing credentials through the same machinery workstream B already builds for ordinary JWT issuance. |
| **Timeline** | ~1.5–2 weeks — Envoy config and the enrollment endpoint can be built in parallel by two people, but both gate Phase 0 exit criteria together. |
| **Claude Code fit** | 🟡 Medium-high — Envoy config is declarative and easy to draft and test locally. The enrollment endpoint's key-validation logic (expiry, usage caps, revocation) is exactly the kind of auth-adjacent code worth a human review pass, given a bug here means someone can mint sprout identities they shouldn't be able to. |
| **Risk** | 🔴 High if rushed — this is literally the front door to the whole trust chain; every later phase assumes whatever identity this subsystem issues is trustworthy. |

### I. Recipe storage migration
| | |
|---|---|
| **Change** | Move recipe storage off farmer's local-disk `basepath` (confirmed via `internal/cook/farmercook.go`'s `os.ReadFile`) to object storage, with git remaining the authored source of truth synced to S3 on merge. New dedicated HTTP endpoint (behind workstream H's Envoy gate) replaces the current NATS-based recipe delivery (`internal/natsapi/recipes.go`). |
| **Effort** | The storage move itself is mechanical, same shape as job logs. The new HTTP endpoint is a small, self-contained service — grlx already has a directly reusable pattern in `internal/ingredients/file/http/provider.go`'s authenticated-header download logic. |
| **Timeline** | ~1 week. |
| **Claude Code fit** | 🟢 High — well-precedented pattern already in the codebase, low ambiguity. |
| **Risk** | 🟢 Low, but real if skipped — local-disk recipe storage silently breaks the moment core is horizontally scaled (workstream D), since only the replica holding a given file on disk can serve it. |

### J. Payload encryption + key rotation
*(Full design in `grlx-payload-encryption-design.md`.)*
| | |
|---|---|
| **Change** | Application-level NaCl `box` (X25519) encryption of NATS message payloads — one keypair per tenant (farmer-side, OpenBao-custodied private key) and one per sprout (generated locally, private key never leaves the sprout), bootstrapped in the same round trip as workstream H's enrollment exchange. Sprout-initiated key rotation (never farmer-generated or farmer-transmitted private keys) with a grace-period overlap reusing the existing PKI accept/deny/revoke lifecycle. |
| **Effort** | Broad-touching rather than deep — needs wrapping every meaningful NATS payload boundary in `internal/natsapi`'s handlers, ideally via a shared request/response helper so encryption is transparent to individual handlers rather than opt-in per handler. |
| **Timeline** | ~2–2.5 weeks. |
| **Claude Code fit** | 🟡 Medium — mechanical once the shared encrypt/decrypt helper exists, but this is cryptographic code protecting the exact threat model (a compromised DMZ bus) the whole DMZ split was built around — deserves real security review, not just passing tests. |
| **Risk** | 🟡 Medium — the static long-lived tenant key means no forward secrecy; accepted tradeoff per the design doc, worth a deliberate rotation schedule rather than treating rotation as optional. |

### K. SDB secret resolution (deferred track)
*(Full design in `grlx-sdb-secrets-design.md`.)*
| | |
|---|---|
| **Change** | Sprout-side `SecretProvider` interface behind the `sdb://` scheme. v1: OpenBao/customer-managed Vault via customer-supplied client certificate (file-watched, hot-reloaded), Azure/AWS/GCP via managed identity only. Later: CyberArk and Delinea via bespoke REST clients and a separate bootstrap-issued credential. |
| **Effort** | v1 tier is moderate — four providers behind one interface, three of them following the same managed-identity shape. CyberArk/Delinea are a materially bigger lift each, given no maintained Go SDK for either. |
| **Timeline** | ~2–3 weeks for the v1 tier; CyberArk/Delinea sequenced later, ~1.5–2 weeks each. |
| **Claude Code fit** | 🟢 High for the interface and the three managed-identity providers — well-documented cloud SDKs, mechanical once the interface is settled. 🟡 Medium for CyberArk/Delinea's hand-rolled REST clients — more surface area to get right against less-standardized APIs. |
| **Risk** | 🟢 Low — explicitly deferred, no dependency blocking anything ahead of it. |

### L. Sprout ingredients, cook-engine primitives, and farmer orchestration
*(Full design in `grlx-sprout-orchestration.md`.)*
| | |
|---|---|
| **Change** | Atomic ingredients (`probe/http`, `probe/database`, `wait`, `file.sync`, `file.line`, `file.copy` enrichment) derived from `umputun/spot`, plus general cook-engine primitives (conditionals, `on_exit`, variable registration/passing with `sensitive` redaction, runtime context variables) that apply to any ingredient, not just probe. Separately, farmer-side staged rollout — grlx's equivalent of Salt's `orchestrate.sls` — dispatching a recipe to a batch of sprouts, gating on a probe/job-status signal before the next batch. |
| **Effort** | Ingredients and primitives are individually small; the set of them together is the effort. Farmer orchestration is a distinct, self-contained addition to job dispatch logic, not sprout-side work at all. |
| **Timeline** | ~1.5–2 weeks for ingredients + primitives (probe's async job pattern and sensitive-field handling included). ~1–1.5 weeks for farmer's staged rollout, sequenced after probe since it consumes probe's results as its gating signal. |
| **Claude Code fit** | 🟢 High for the atomic ingredients (small, well-precedented against the existing plugin pattern). 🟡 Medium for the sensitive-field redaction guarantee specifically — the security property (never in job logs, never in PXC/Valkey, in-memory only) is easy to state and easy to accidentally violate in a handler that logs verbosely; worth a dedicated test pass exercising exactly that. |
| **Risk** | 🟡 Medium — the sensitive-field handling is the one part of this workstream where a miss has real consequences (a leaked credential in a durable, object-storage-backed job log), not just a bug to fix later. |

### M. Windows packaging and Ansible deployment
| | |
|---|---|
| **Change** | (1) `golang.org/x/sys/windows/svc` wrapper giving the sprout binary itself Windows Service Control Manager lifecycle hooks — distinct from and more basic than workstream G's service-management *ingredient*. (2) MSI installer (WiX or `go-msi`) so Ansible's `win_package` module can drive fully silent install/uninstall/upgrade. (3) zypper validation pass on the existing rpm-based Linux packaging for SUSE-specific conventions. (4) Customer-run Ansible playbooks consuming workstream H's one-time registration key to bootstrap each host. |
| **Effort** | The Windows service wrapper and MSI packaging are the biggest pieces — both need a real Windows box to validate, not just cross-compilation. zypper validation is a testing task, not new code, given deb/rpm/apk already exist via `nfpm`. Ansible playbooks are straightforward once the installers exist. |
| **Timeline** | ~3 weeks. |
| **Claude Code fit** | 🟢 High for writing the MSI/service-wrapper code and drafting the Ansible playbooks. 🔴 Same constraint as workstream G — none of the Windows-specific pieces can be validated in a Linux-based sandbox; a real Windows CI runner is a prerequisite for trusting the result. |
| **Risk** | 🟡 Medium — mostly the same untested-Windows-path risk as workstream G, compounded by packaging/installer edge cases (silent-install flags, upgrade-in-place behavior) that are easy to get subtly wrong without real testing. |

---

## 3. Suggested sequencing

```
Week 1-3     │ B: NATS JWT auth model                        (foundational — decide/build before C, H)
Week 1-6     │ G: Windows sprout ingredients                  (fully parallel, no dependency)
Week 2-3.5   │ A: PXC + Valkey + object storage                (metadata, heartbeat, logs)
Week 2.5-4   │ H: Envoy gateway + enrollment                   (needs B's JWT model)
Week 3-4     │ F: OpenBao TLS                                  (parallel with A/B)
Week 4       │ C: DMZ split                                    (needs B settled)
Week 4-4.5   │ D: Queue groups fix                             (quick, but real scaling needs A done)
Week 4-5     │ I: Recipe storage migration                     (needs A's object storage, H's Envoy route)
Week 5-8     │ E: Multi-tenancy                                (needs A done first)
Week 6-8.5   │ J: Payload encryption + key rotation             (needs H's enrollment bootstrap)
Week 8-10    │ L: Ingredients, cook primitives, rollout         (probe async pattern, staged rollout)
Week 8-11    │ M: Windows packaging + Ansible                   (parallel with L, needs a Windows runner)
─────────────┼─────────────────────────────────────────────────────────────────────────
Deferred     │ K: SDB secret resolution                         (not blocking anything above)
```

Rough total: **~10–11 weeks** to a horizontally-scalable, DMZ-segregated, multi-tenant, payload-encrypted, OpenBao-secured farmer with Windows sprout parity — up from the original ~8-week estimate for workstreams A–G alone, reflecting the real scope added by Envoy/enrollment, payload encryption, and the ingredients/orchestration workstream. This matches the ~10–11 week range implied by the two-developer-plus-Claude-Code estimate discussed separately, once Envoy/enrollment (H) and payload encryption (J) are counted alongside the original phases.

**Biggest risk in the whole plan:** workstream H (Envoy + enrollment), not B alone anymore — it's the literal front door to the entire trust chain, and every later workstream (storage tenant-scoping, payload encryption, SDB) assumes whatever identity it issues is trustworthy. B remains a close second, since H is built on top of it.

**On delegating this to Claude Code:** the mechanical, pattern-following refactors (A, C, D, I, most of L's ingredients, most of G and M's code-writing) are where an agent needs the least oversight — the codebase's existing discipline (consistent interfaces, the `cook.RecipeCooker` pattern, existing tests) makes "follow the codebase's own conventions" a well-posed task. Workstreams touching auth, crypto, or tenant-isolation boundaries (B, E, F, H, J, A.1) are still good candidates for a first draft, but treat the output as something for security review rather than mergeable on green tests alone — the failure mode there (a tenant-isolation bug, a key-custody mistake, a forged enrollment) is worse than an ordinary bug, and tests passing doesn't prove the boundary is airtight. G and M are really infrastructure gaps as much as coding ones: the code can be written and cross-compiled in-sandbox, but nothing there can actually execute Windows-specific behavior, so a real Windows runner is a prerequisite for trusting the result, not an afterthought.
