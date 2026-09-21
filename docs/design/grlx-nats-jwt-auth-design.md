# grlx Auth Model: Decentralized JWT (replacing custom NKey allow-lists)

Companion to `grlx-fork-roadmap.md`, fleshing out workstream B (option 2) and its knock-on effect on workstream E (multi-tenancy).

**Related docs, covering credentials this doc doesn't:** `grlx-envoy-enrollment-design.md` covers how this JWT gets presented to Envoy (a separate check from NATS's own validation below) and how a sprout obtains its first JWT at all (the enrollment subsystem). `grlx-payload-encryption-design.md` covers a *different* keypair — X25519, for NaCl `box` payload encryption — bootstrapped in the same enrollment exchange as this JWT but serving an entirely separate purpose (this doc's JWT is for authentication/authorization; the encryption keypair is for confidentiality of message content). Easy to conflate since both are issued together — worth keeping distinct: one proves who a sprout is and what it may do, the other protects what it sends.

## The model

NATS's decentralized auth is a three-level trust chain, all built on the same NKey (Ed25519) primitives grlx already uses:

```
Operator (1, root trust anchor)
  └─ signs → Account JWTs   (1 per tenant)
       └─ signs → User JWTs  (1 per sprout, 1 per human user)
```

- **Operator** — one keypair for the whole SaaS. Its public key is baked into every bus node's config as the trust anchor. This is the single most sensitive secret in the system.
- **Account** — an isolated subject namespace. Two accounts share nothing by default — no cross-account pub/sub — unless you explicitly configure exports/imports.
- **User** — belongs to exactly one account, carries subject permissions (the same allow-list shape grlx already builds today).

## The key simplification this brings to multi-tenancy

**Map one NATS Account per tenant, and the subject-prefixing plan from workstream E mostly goes away.** Today's plan was to rewrite `internal/natsapi/subjects.go` to emit `grlx.tenants.<id>.api.*` / `grlx.tenants.<id>.sprouts.<id>.*` and thread tenant IDs through `pki.go`'s subject-permission logic by hand — correctness resting on every code path getting the prefix right.

With accounts as the tenant boundary, isolation is enforced by the server itself: tenant A's sprouts, connected under tenant A's account, structurally cannot see or publish to tenant B's subjects, even if both use the exact same subject strings (`grlx.sprouts.<sproutID>.>`) unchanged from today. You likely keep `subjects.go` as-is and get tenant isolation as a property of which account a connection authenticated into, not a string convention every handler has to respect. This is a real reduction in workstream E's scope, not just a different way to do the same thing.

## Solving the DMZ + clustering reload problem

This was flagged as the open question in both the DMZ split and clustering work: today, `pki.ReloadNKeys()` mutates an in-process `*nats_server.Server` handle via `ReloadOptions()` — which only works if core and the bus share a process.

The JWT model's recommended **"full" resolver** replaces this cleanly:
- Each bus node keeps its own **local** (not shared, no NFS involved) directory of account JWTs.
- New/updated JWTs are **pushed to a running server over an ordinary NATS connection**, authenticated as a designated SYSTEM account — no restart, no in-process handle needed.
- Multiple bus nodes (DMZ replicas, or real cluster members) sync JWT updates **among themselves automatically** via that same system-account mechanism — "eventually consistent," no shared filesystem, no NFS anti-pattern.

This means: core, running in non-DMZ, accepts a sprout exactly like today, then pushes the new sprout's User JWT to the bus over its existing outbound NATS connection — the same push works whether there's one bus node or ten, in the DMZ or in a cluster. One mechanism solves both open questions from workstreams B and C at once.

## Concrete code mapping

| Today | Becomes |
|---|---|
| `pki.ConfigureNats()` builds `Nkeys []*NkeyUser` from the PKI store | Builds `Operator`, `SystemAccount`, and `resolver: {type: full, dir: ...}` fields on `nats_server.Options` |
| `pki.ReloadNKeys()` mutates in-process `NatsServer.ReloadOptions()` | New function mints a signed User JWT (via `github.com/nats-io/jwt/v2` + the existing `nkeys` dependency for the token shape, with the actual signature produced by an OpenBao Transit `sign` call rather than a locally-held key) and pushes it to the resolver over a NATS system-account connection |
| `pki.go`'s flat files under `sprouts/<state>/<id>` (raw NKey pubkeys) | Same accept/deny/reject lifecycle, but the artifact stored per sprout is now a signed User JWT rather than a bare pubkey; revocation becomes appending to the account's revocation list rather than moving a file between directories |
| Sprout generates its own NKey keypair locally, sends only the public key (unchanged) | Identical — the sprout's private key never leaves the sprout either way. The farmer now returns a signed JWT naming that public key, instead of adding it to an in-memory allow-list |
| One `FarmerOrganization` string, static, loaded once at boot | One Account per tenant, created/pushed dynamically at tenant onboarding — this is what actually makes `FarmerOrganization` dynamic, more naturally than the config-reload approach originally scoped |

## Key custody — ties directly to workstream F (OpenBao)

The Operator key, and each tenant Account's signing key, are long-lived, extremely sensitive secrets — compromise of the Operator key compromises trust for the entire bus. NATS supports **signing keys** specifically so the root Operator/Account keys don't need to be online at runtime: you delegate a signing key per environment, keep the root key cold. This is exactly the kind of material that should live in OpenBao rather than a flat file — and specifically, **held via OpenBao's Transit secrets engine, never fetched as raw key material into farmer's process at all**: farmer builds the JWT signing input and calls Transit's `sign` operation (`POST /v1/transit/sign/<key-name>`), getting back a signature to assemble into the final token. The private key stays in OpenBao for its entire lifetime — not even transiently in farmer's memory, which is a stronger property than "OpenBao-issued/stored, fetched by core" implies.
- Operator root key: cold storage, effectively never touched after initial setup.
- Per-tenant account signing keys: Ed25519 keys held in OpenBao Transit (`type: "ed25519"`), signed via Transit's `sign` operation when minting sprout User JWTs — farmer never holds the raw key.
- SYS account user credentials (used to push to the resolver): OpenBao-issued.
- **Gateway signing key (new, distinct from the above):** one Ed25519 keypair for the *entire platform*, not per tenant — used to sign the separate, standard-EdDSA "gateway JWT" that Envoy's `jwt_authn` filter validates (NATS's own `ed25519-nkey`-alg User JWT isn't a JOSE-compliant token Envoy can consume directly; see `grlx-envoy-enrollment-design.md` for the full rationale, JWKS design, and Go implementation notes). Delegated from the Operator the same way Account signing keys are, held in OpenBao Transit and signed the same way (never fetched raw), rotated on its own schedule via Transit's `auto_rotate_period`/`min_encryption_version` mechanism (current + previous key both valid during the overlap window). It's deliberately platform-wide rather than per-tenant: Envoy's check never needs tenant granularity, so there's no reason to tie its trust material to tenant-onboarding or per-tenant-rotation cadence the way the Account keys are. Compromise of this key threatens the connection-admission gate for every tenant at once — closer in severity to an Operator-key compromise than to a single Account-key compromise — and its custody should reflect that.

Recommend sequencing this together with workstream F rather than after it — you'd otherwise stand up JWT auth against flat-file keys and then have to migrate them into OpenBao separately.

## Revised effort estimate for this workstream

The earlier roadmap estimated 2–3 weeks for "adopt decentralized JWT auth" in isolation. With the multi-tenancy simplification now factored in, net effort across B + E together is likely *less* than the two estimated separately (2–4 weeks for E, 2–3 weeks for B ≈ 4–7 weeks combined before), because accounts-as-tenants removes most of the manual subject-namespacing work:

- Operator/SYS account bootstrap, resolver config, JWT push mechanism in `pki.go`/`pki/nats.go`: ~1.5–2 weeks.
- Per-tenant account provisioning flow (replacing static `FarmerOrganization`): ~3–5 days.
- Sprout accept/deny/reject lifecycle rewired to JWT issuance/revocation: ~1 week.
- OpenBao-backed key custody for operator/account signing keys (done alongside, per above): folds into workstream F's existing estimate.

**Revised combined estimate for B + E together: ~4–5 weeks**, versus ~7 weeks estimated separately before — a net reduction, on top of removing a genuine correctness risk (hand-rolled subject prefixing) from the plan entirely.

## Open items worth deciding early

- **Resolver type at scale**: "full" (every node holds every account JWT) is right at your likely near-term tenant count. If tenant count grows very large, NATS offers a "cache" resolver variant to avoid every node holding every JWT — not needed now, worth knowing it exists as a lever.
- **Revocation semantics**: map grlx's existing `DenyNKey`/`RejectNKey` states onto the JWT revocation-list mechanism precisely — worth a short design pass so "denied" vs "rejected" (grlx has both today) map onto distinct, intentional JWT states rather than collapsing into one.
