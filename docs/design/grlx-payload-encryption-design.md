# grlx: Payload Encryption & Key Rotation

Covers Phase 3 of `grlx-master-plan.md`. Bootstrapped by the enrollment exchange in `grlx-envoy-enrollment-design.md`.

## Why this exists: what TLS alone doesn't cover

NATS's TLS transport encrypts each hop (sprout↔bus, farmer↔bus), but nats-server necessarily decrypts the payload in memory to route it by subject — true of any subject-routing broker. Given the bus sits in the DMZ by design, and has had real disclosed pre-auth CVEs in 2026, a fully compromised bus under TLS-only protection still sees every job command and fact payload in plaintext. Payload-level encryption means a compromised bus sees only routing metadata and ciphertext, never content.

## Design: one tenant keypair, one keypair per sprout

- **Tenant keypair** (X25519): one per tenant, farmer-side, private key custodied in OpenBao (same model as the Operator/Account signing keys). Not per-sprout — a single farmer-side keypair reused across every sprout in that tenant.
- **Sprout keypair** (X25519): generated locally by the sprout at enrollment. Private key never leaves the sprout, ever — not at generation, not at rotation.
- **Why one tenant key still gives per-sprout-specific encryption:** NaCl `box`'s shared secret is derived from *both* sides' keys jointly (`DH(tenant_priv, sprout_pub)` and `DH(sprout_priv, tenant_pub)` produce the same secret). Reusing `tenant_priv` across sprouts doesn't mean sprouts share a key with each other — each (tenant, sprout) pair still gets a distinct derived secret.
- **Threat model match:** the bus never holds `tenant_priv`, `sprout_priv`, or any derived secret regardless of key-reuse scope, so this simplification costs nothing against the specific threat (a compromised DMZ bus) this feature exists to defend against.

## Bootstrap

Rides entirely on the enrollment exchange already designed — no separate protocol. Sprout generates its X25519 keypair locally, includes `sprout_pub` in its enrollment request alongside its NKey public key; farmer's enrollment response includes `tenant_pub` alongside the issued JWT. One existing round trip, two new fields.

## Key rotation — corrected from an earlier, unsafe framing

**The private key must never be transmitted, even encrypted, even over an already-encrypted channel.** An earlier version of this design considered "send the private key encrypted in NATS" — worth stating plainly why that's wrong, not just noting it was changed: it means farmer must generate and briefly hold a sprout's private key (a new exposure surface — memory, logs, crash dumps — that doesn't exist under the bootstrap model), and the sprout ends up trusting key material it didn't generate itself, with no way to verify it wasn't logged or weakly randomized somewhere in farmer's pipeline.

**Correct pattern — sprout-initiated, same shape as bootstrap:**
- Sprout generates a **new** keypair locally. New private key never leaves the sprout.
- Sprout sends only the new **public** key to farmer, re-registering it against its identity.
- Farmer may **trigger** rotation (e.g., scheduled policy, suspected exposure) by sending a command — but that command carries no key material, only an instruction.
- **Grace period:** both old and new public keys accepted for a short overlap window, so in-flight messages encrypted under the old key still decrypt correctly, then the old key is revoked once the window closes — reusing the existing PKI accept/deny/revoke lifecycle rather than a new mechanism.

## Forward secrecy — accepted tradeoff, not an oversight

Because `tenant_priv` is a single, long-lived key, extraction of it (a future OpenBao compromise, a backup leak) lets an attacker who has been passively recording bus ciphertext retroactively decrypt all of that tenant's historical traffic — the standard limitation of static-key NaCl `box` usage, versus schemes with ephemeral per-session keys (e.g., Signal's Double Ratchet) that bound exposure to the current session only.

Building genuine forward secrecy (session-level key agreement, ratcheting state) is a materially bigger lift, very likely disproportionate to what this feature needs. **Accepted mitigation: rotate the tenant keypair on a schedule** (e.g., quarterly, or immediately on suspected exposure) — this doesn't eliminate retroactive-decryption risk, but bounds its window to "since the last rotation" rather than "since the beginning of time." Worth a deliberate, documented decision given the CERT-In/DPDP context, even if the decision is "static key, rotated periodically, and this residual risk is accepted."

## Storage, consistent with decisions elsewhere in the plan

- `tenant_priv` → OpenBao, same custody model as Operator/Account signing keys.
- `sprout_pub` (per sprout) → PXC, an additional column alongside the sprout's existing identity record.
- `tenant_pub` → not sensitive, distributed to sprouts at bootstrap/rotation, no special custody needed.

## Implementation scope, worth sizing honestly

Broader-touching than most workstreams in this plan, not because any one piece is hard, but because it needs to wrap every meaningful payload boundary in `internal/natsapi`'s handlers (job dispatch, job results, facts) — best done by building it into the shared request/response helper layer so encryption is transparent to every handler, rather than opt-in per handler where it's easy to miss one.
