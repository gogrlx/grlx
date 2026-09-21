# grlx: Envoy Gateway & Enrollment Subsystem

Companion to `grlx-nats-jwt-auth-design.md`. Covers Phase 0 of `grlx-master-plan.md` — the part of the plan called out as the single biggest structural dependency, since every later phase assumes the identity this subsystem issues is trustworthy.

## Implementation notes: Go libraries

**Gateway JWT construction + JWKS serving:** `github.com/lestrrat-go/jwx/v2` (MIT). Its `jws` package supports detached signing — build the signing input, get a signature from wherever (here, OpenBao Transit), assemble the compact token — rather than insisting on holding a local private key, which fits the Transit-based custody model directly. Its `jwk` package builds a correct OKP/Ed25519 JWK (`jwk.FromRaw()`) for the JWKS response without hand-rolling base64url encoding.

**OpenBao client:** `github.com/openbao/openbao/api` (MPL-2.0, same accepted licensing exception already carved out for OpenBao itself) — handles token renewal, retries, and TLS against OpenBao's Transit `sign` endpoint, rather than hand-written HTTP calls.

**Libraries evaluated and rejected for this role:**
- `golang-jwt/jwt/v5` (MIT) — fine for parsing/verifying, but its `SigningMethodEdDSA` type-asserts on a concrete local `ed25519.PrivateKey`; no detached-signing path, so it fights the Transit custody model rather than fitting it.
- `gourdiantoken` (MIT) — a comprehensive access/refresh-token library (rotation, revocation, multi-tenant bulk revocation), but built around holding `PrivateKeyPEM` directly in its config and signing locally; no external-signer hook. Better fit for the SaaS API's own human-user login session layer (§1.7's open item) than for this gateway/NATS problem — separate decision, not pursued further here. Also young (single maintainer, several breaking changes within `v2.x` by the project's own admission) — worth weighing that against a well-established primitive if adopted for the human-auth case.
- `infamousjoeg/jwt-service` — confirms the two-endpoint (`generate-jwt` + `/.well-known/jwks.json`) shape is a reasonable pattern, but not adopted directly: built on the archived, unmaintained `dgrijalva/jwt-go`; RSA-only (no Ed25519, breaking consistency with NKeys/gateway-key/X25519 elsewhere in this design); and manages its own key generation/rotation in-process rather than deferring to an external KMS/HSM — the opposite of the Transit-backed custody model here. Its own README candidly notes thin documentation and test coverage.
- OpenBao's Identity/OIDC "identity token" feature (native ID-token issuance with an automatic JWKS and built-in rotation) was also considered and set aside for this specific role: it issues tokens about the *caller's own authenticated OpenBao entity*, which would require modeling one OpenBao entity per sprout — a second, duplicated identity store at odds with `farmer.sprouts`/PXC already being the source of truth, and not a scale shape OpenBao's identity store is built for at 1M sprouts. Worth keeping in mind for the SaaS API's own future human-user SSO/OIDC need (§1.7), a materially better-matched use case.

## Two gates, checking different things, not redundant with each other

Requirement set specified: Envoy in front of NATS doing JWT validation, each sprout holding a JWT to connect, and recipe download authenticating with the same JWT. **Revised from the original single-JWT framing** — that's not achievable as literally specified, because the two validators speak incompatible JWT profiles (see "Why one JWT can't serve both gates" below). What ships instead is one signing identity, expressed as two paired tokens, three surfaces:

1. **Envoy** — terminates the sprout's `wss://` connection at the DMZ edge, validates the **gateway JWT**'s signature/claims/expiry via a `jwt_authn` filter against a JWKS trust configuration, **before the connection ever reaches nats-server**.
2. **nats-server itself** — once past Envoy, the sprout presents its **NATS User JWT** for NATS's own native Account/User decentralized-auth model (see the companion doc), which enforces ongoing, per-subject publish/subscribe permissions for the life of the connection.
3. **Recipe HTTP endpoint** — the **gateway JWT** again, as a bearer token, validated by the same Envoy instance on a second route, which proxies the authenticated request through to a small non-DMZ recipe-service (never lets the DMZ side hold direct object-storage credentials).

These are complementary, not duplicated: Envoy's check is a one-time gate at connection establishment; NATS's own model enforces fine-grained, ongoing subject permissions Envoy has no visibility into at all. A security-relevant side effect worth being explicit about: Envoy rejecting malformed or unauthenticated connections before they reach nats-server directly mitigates the 2026 pre-auth websocket CVE class (memory-exhaustion DoS) discussed earlier — a compromised or malicious pre-auth payload targeting that class never touches nats-server's websocket listener at all.

## Why one JWT can't serve both gates

NATS's decentralized-auth JWTs (`nats-io/jwt` v2) use a non-standard header: `{"typ":"JWT","alg":"ed25519-nkey"}`. That `alg` value isn't a JOSE-registered algorithm (RFC 7518/8037 define `RS256`, `ES256`, `EdDSA`, etc.) — Envoy's `jwt_authn` filter is a generic JOSE validator with no NATS awareness, and rejects the `alg` field outright before it ever reaches signature or JWKS matching. So the NATS User JWT cannot be the token Envoy validates, as originally scoped.

**Resolution: mint two tokens from the same identity, at the same time.**
- **NATS User JWT** — unchanged from the companion doc's design: `ed25519-nkey` alg, Account-signed, consumed only by nats-server.
- **Gateway JWT** — a standard RFC 8037-compliant token (`alg: EdDSA`, key type `OKP`), signed by a **single, dedicated gateway signing key** (see below), carrying enough claims (tenant ID, sprout ID, expiry) for Envoy's `jwt_authn` filter to validate and for routing decisions, but doing no NATS-side authorization itself — that stays entirely nats-server's job via the paired NATS JWT.

## Gateway signing key — one key for the whole platform, not per-tenant

Unlike the per-tenant Account signing keys, the gateway JWT is signed by **one Ed25519 keypair shared across every tenant**, delegated from the Operator the same way Account signing keys are, OpenBao-custodied, rotated on its own schedule (current + previous key both valid during the overlap window).

This is deliberate, not a shortcut: Envoy's check never needs to distinguish tenants to do its job — it only decides "was this connection allowed to open," with tenant-scoped authorization enforced entirely afterward by NATS Accounts. Reusing per-tenant Account keys for the gateway JWT would tie Envoy's JWKS to tenant-onboarding and per-tenant-rotation cadence for no benefit, and would blur two credentials that should have separate blast radii: compromise of a tenant's Account key threatens only that tenant's NATS-side authorization; compromise of the gateway key threatens the connection-admission gate for every tenant, and should get Operator-adjacent custody treatment accordingly.

**Consequence for the JWKS:** it stays small and effectively static — one entry (two during the gateway key's own rotation), never growing with tenant count, never touched by tenant onboarding/offboarding or by any tenant's own Account-key rotation.

## JWKS endpoint

- **Owner:** farmer. It already fetches signing-key material from OpenBao when minting; converting the gateway public key to JWK format (`kty: OKP, crv: Ed25519, x: <base64url pubkey>, kid: <stable key id>, use: sig`) is pure data transformation, no new secret access.
- **Shape:** a plain, unauthenticated `{"keys": [...]}` document (public keys only, same trust model as any standard `/.well-known/jwks.json`) — reachable from Envoy's DMZ side, not behind the `jwt_authn` filter itself.
- **Envoy config:** `remote_jwks` with a periodic refresh interval (e.g. 5–10 min), not `local_jwks` — a static inline JWKS would need an Envoy config reload on every gateway-key rotation.
- **Rotation:** during the gateway key's overlap window, the JWKS carries both the outgoing and incoming public keys, distinguished by `kid`, exactly matching the grace-period pattern already used elsewhere in this plan (payload-encryption key rotation, PKI accept/deny/revoke lifecycle).

## Enrollment: the chicken-and-egg problem this subsystem exists to solve

Every credential elsewhere in this design (the JWT, the X25519 keypairs) assumes a sprout already has an identity to present. At the exact moment a fresh host runs its Ansible playbook, it doesn't. This needs its own answer, separate from the JWT-authenticated Envoy routes above.

**Design, borrowed deliberately from `kubeadm`'s join-token model** — the same problem, solved the same way, by a project with a lot of production hardening behind it:

- **Short-lived** (expires in hours, not indefinitely valid).
- **Tenant-scoped**, not global — a leaked key only threatens one customer's enrollment window.
- **Usage-capped**, ideally matched to the actual fleet size being rolled out in one Ansible run, rather than unlimited use.
- Own PXC table: `tenant_id`, key hash (never the raw key), expiry, max/used count, revoked flag.
- Own endpoint, deliberately **not** behind Envoy's `jwt_authn` filter — a sprout enrolling has no JWT yet, so this route validates the presented registration key by direct lookup against the PXC table above (constant-time hash comparison, check expiry/usage, decrement/mark used), not via Envoy's JWT machinery.
- **Response, in one round trip:** the sprout's signed NATS User JWT (from the workstream B model), its paired gateway JWT (see below, for Envoy's `jwt_authn` on the websocket and recipe routes), its NKey identity, and the tenant's X25519 public key (from workstream J's payload encryption bootstrap) — everything the sprout needs for every subsequent interaction with the platform, issued atomically at enrollment rather than across several separate exchanges.

## Why this is safe even crossing the DMZ bus before any payload encryption exists

The only things transiting the bus during this exchange are the sprout's own generated **public** keys (NKey public key, X25519 public key) — not secret even if a compromised bus observes them. The sprout's private key material never leaves the sprout; the tenant's private key never leaves OpenBao custody. No bootstrapping-before-security-exists problem here, by construction.

## What still needs deciding at implementation time

- Exact revocation semantics for a spent or expired registration key — should a spent key's row be deleted, or retained with `used = true` for audit purposes? Given CERT-In/DPDP audit expectations, retention with a clear revoked/used state is likely the safer default, but worth an explicit decision rather than defaulting silently either way.
- Rate-limiting the enrollment endpoint itself — since it's deliberately not JWT-gated, it's the one DMZ-facing surface without that layer of protection, and deserves its own abuse-resistance treatment (e.g., IP-based rate limiting at Envoy, even though the route itself skips JWT validation).
