# grlx: SDB-Equivalent Secret Resolution

Deferred track in `grlx-master-plan.md` — sequenced after Phase 6, not blocking anything ahead of it. Documented in full now specifically because deferred work is the most likely to be picked up later by someone without this conversation's context.

## Scope decision: sprout-side resolution, not farmer-side

Recipes carry an opaque `sdb://` reference; the **sprout itself** resolves it at execution time, via a `SecretProvider` interface behind that URI scheme. Deliberately not farmer-side: secrets never transit farmer or the DMZ bus at all this way, and farmer never needs broad cross-tenant secret-read access just to render recipes centrally — a meaningfully smaller blast radius if farmer is ever compromised, at the cost of every sprout needing its own scoped credential and outbound reach to whichever secret backend a given recipe references.

## v1 tier — four providers, one shared authentication shape

**OpenBao / customer-managed HashiCorp Vault, Azure Key Vault, AWS Secrets Manager, GCP Secret Manager.**

- **Azure, AWS, GCP: managed identity only for v1** — no JWT federation initially, despite federation being technically available on all three (this was considered and explicitly deferred, not overlooked). Managed identity is simpler to ship (no OIDC trust relationship to configure between customer IAM and grlx's JWT issuer), at a real, worth-naming cost: **managed identity is inherently same-cloud-only** (IMDS/metadata endpoints are link-local, reachable only from within that cloud's own fabric). A sprout on AWS can reach AWS Secrets Manager via its instance role; that same sprout has no managed-identity path to Azure Key Vault or GCP Secret Manager, and a sprout on bare metal or on-prem has no managed-identity path to any of the three. If cross-cloud secret consumption turns out to matter (common after M&A or in multi-cloud DR setups), JWT federation is a v2 addition on the same interface, not wasted v1 work.
- **OpenBao / customer Vault: certificate-based authentication to start.** This is explicitly **the customer's own Vault, not CloudXP's** — entirely separate infrastructure, own CA, own `cert` auth role mapping, outside grlx's control or visibility. The sprout's config file for this provider references a **customer-supplied client certificate and private key** (paths to PEM files the customer places on the sprout's host) plus optionally a CA bundle to verify the customer's Vault server's own TLS identity. Whatever CA those certs chain to is the customer's CA — grlx has no opinion on it, just presents whatever the config points at.
  - **Real consequence worth being explicit about: certificate lifecycle for this credential is the customer's operational responsibility, not grlx's** — unlike every other credential in this plan (the NATS JWT, the X25519 keypairs, OpenBao-issued TLS certs), grlx has no visibility into when this cert expires and no role in reissuing it.
  - **Mitigation grlx should build regardless:** the sprout's Vault-auth provider watches the cert file for changes and reloads it without requiring a sprout restart. Since rotation is entirely the customer's process, the least grlx can do is make picking up a rotated cert painless — drop a new file, sprout picks it up on its own, no coordination needed. Otherwise every customer cert rotation becomes a fleet-wide restart operation.

## Tier 2 — CyberArk and Delinea, sequenced later, structurally different work

Neither has a maintained Go SDK — this means a hand-rolled HTTP client against their REST APIs (CyberArk's Central Credential Provider / AAM, Delinea's Secret Server REST API), more surface area to get right and more ongoing maintenance as those APIs evolve. Their trust model also has no equivalent to OIDC federation as standardized as the cloud vendors' — CyberArk's CCP typically authenticates via client certificate or a pre-registered Application ID plus IP allowlisting, not JWT-based federation. **Sprouts need a separate, bootstrap-issued credential for these two** (most naturally a client certificate, reusing the same keypair-issuance machinery already built for sprout bootstrap) rather than reusing the existing NATS JWT the way the v1 tier's cloud providers effectively can via federation (deferred) or the existing identity generally.

## Interface shape

One `SecretProvider` interface (`Get(ctx, ref) (value, error)`), six self-registering implementations selected by URI scheme (`sdb://openbao/...`, `sdb://azurekv/...`, `sdb://cyberark/...`), following the same self-registering plugin pattern as every other grlx ingredient. Backends don't ship together and shouldn't — v1's four can land independently of Tier 2's two.

## Interaction with probe-chaining — the two moments a secret can enter execution, and why they're handled differently

Worth being explicit about since it's easy to conflate: SDB resolves secrets a recipe **already knows it needs** (a stated `sdb://` reference for a known credential). It is not the mechanism for a secret **produced during execution** (a token captured from one API call's response, used in a later step) — that's `grlx-sprout-orchestration.md`'s `sensitive: true` field-marking on registered variables, a separate, general cook-engine primitive. SDB is for known, named, reusable secrets; sensitive-field handling is for ephemeral, single-run values. Using probe-chaining to shuttle a long-lived credential in place of proper SDB resolution is the anti-pattern the orchestration doc calls out — this doc's existence is part of what makes that the wrong path (SDB is the one that should be reached for, when the secret is known ahead of time).

## Recommended build order, given the real effort asymmetry

1. **OpenBao first** — the platform's own chosen secrets manager for operator/tenant key custody elsewhere in this plan, so this is substantially dogfooding an integration already needed.
2. **Azure, AWS, GCP** — same managed-identity shape, mature SDKs, incremental once the interface is proven by OpenBao.
3. **CyberArk, Delinea** — sequenced last given the bespoke REST clients and separate credential-bootstrap story, not dropped — likely a real reason some target customers (BFSI, government) would deploy this platform at all.
