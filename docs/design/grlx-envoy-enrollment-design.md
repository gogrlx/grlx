# grlx: Envoy Gateway & Enrollment Subsystem

Companion to `grlx-nats-jwt-auth-design.md`. Covers Phase 0 of `grlx-master-plan.md` — the part of the plan called out as the single biggest structural dependency, since every later phase assumes the identity this subsystem issues is trustworthy.

## Two gates, checking different things, not redundant with each other

Requirement set specified: Envoy in front of NATS doing JWT validation, each sprout holding a JWT to connect, and recipe download authenticating with the same JWT. One JWT, three surfaces:

1. **Envoy** — terminates the sprout's `wss://` connection at the DMZ edge, validates the JWT's signature/claims/expiry via a `jwt_authn` filter against a JWKS trust configuration, **before the connection ever reaches nats-server**.
2. **nats-server itself** — once past Envoy, the same JWT is presented again for NATS's own native Account/User decentralized-auth model (see the companion doc), which enforces ongoing, per-subject publish/subscribe permissions for the life of the connection.
3. **Recipe HTTP endpoint** — the same JWT again, as a bearer token, validated by the same Envoy instance on a second route, which proxies the authenticated request through to a small non-DMZ recipe-service (never lets the DMZ side hold direct object-storage credentials).

These are complementary, not duplicated: Envoy's check is a one-time gate at connection establishment; NATS's own model enforces fine-grained, ongoing subject permissions Envoy has no visibility into at all. A security-relevant side effect worth being explicit about: Envoy rejecting malformed or unauthenticated connections before they reach nats-server directly mitigates the 2026 pre-auth websocket CVE class (memory-exhaustion DoS) discussed earlier — a compromised or malicious pre-auth payload targeting that class never touches nats-server's websocket listener at all.

## Enrollment: the chicken-and-egg problem this subsystem exists to solve

Every credential elsewhere in this design (the JWT, the X25519 keypairs) assumes a sprout already has an identity to present. At the exact moment a fresh host runs its Ansible playbook, it doesn't. This needs its own answer, separate from the JWT-authenticated Envoy routes above.

**Design, borrowed deliberately from `kubeadm`'s join-token model** — the same problem, solved the same way, by a project with a lot of production hardening behind it:

- **Short-lived** (expires in hours, not indefinitely valid).
- **Tenant-scoped**, not global — a leaked key only threatens one customer's enrollment window.
- **Usage-capped**, ideally matched to the actual fleet size being rolled out in one Ansible run, rather than unlimited use.
- Own PXC table: `tenant_id`, key hash (never the raw key), expiry, max/used count, revoked flag.
- Own endpoint, deliberately **not** behind Envoy's `jwt_authn` filter — a sprout enrolling has no JWT yet, so this route validates the presented registration key by direct lookup against the PXC table above (constant-time hash comparison, check expiry/usage, decrement/mark used), not via Envoy's JWT machinery.
- **Response, in one round trip:** the sprout's signed User JWT (from the workstream B model), its NKey identity, and the tenant's X25519 public key (from workstream J's payload encryption bootstrap) — everything the sprout needs for every subsequent interaction with the platform, issued atomically at enrollment rather than across several separate exchanges.

## Why this is safe even crossing the DMZ bus before any payload encryption exists

The only things transiting the bus during this exchange are the sprout's own generated **public** keys (NKey public key, X25519 public key) — not secret even if a compromised bus observes them. The sprout's private key material never leaves the sprout; the tenant's private key never leaves OpenBao custody. No bootstrapping-before-security-exists problem here, by construction.

## What still needs deciding at implementation time

- Exact revocation semantics for a spent or expired registration key — should a spent key's row be deleted, or retained with `used = true` for audit purposes? Given CERT-In/DPDP audit expectations, retention with a clear revoked/used state is likely the safer default, but worth an explicit decision rather than defaulting silently either way.
- Rate-limiting the enrollment endpoint itself — since it's deliberately not JWT-gated, it's the one DMZ-facing surface without that layer of protection, and deserves its own abuse-resistance treatment (e.g., IP-based rate limiting at Envoy, even though the route itself skips JWT validation).
