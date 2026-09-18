# Standing caution: rustls default crypto backend can deadlock on old-kernel KVM guests

I can't write directly into the project's existing docs (mounted read-only
in this session), so this is the exact block to paste into each location
below, plus the precise anchor text to paste it near in your actual repo.

---

## The caution block (paste verbatim wherever noted)

> **⚠️ Caution — if this path is ever implemented in Rust:** Rust's `rustls`
> defaults to the `aws-lc-rs` crypto provider, which calls `RNDGETENTCNT` to
> check kernel entropy before performing crypto operations and enters an
> infinite 250ms polling loop if `entropy_avail < 256`. On KVM guests running
> kernels older than 5.6 without `virtio-rng`, this is a **permanent
> deadlock**, not a slow start — the process hangs forever waiting for
> entropy that never arrives. Given CloudXP's BFSI/government customer base
> plausibly includes on-prem KVM stacks and older hardware, this is a real
> field risk, not a theoretical one — precisely the kind of failure that
> won't show up in dev/test on modern cloud infra and will only surface on a
> specific customer's legacy host. **Mitigation:** use `native-tls`
> (OpenSSL-backed) instead of the default `rustls`+`aws-lc-rs` stack, or
> explicitly force the `ring` crypto provider if staying on `rustls`.
> Verified as a real, previously-hit production issue (not hypothetical) by
> Tencent's `tat-agent` — a production cross-platform fleet agent — which
> carries this exact warning in its own `Cargo.toml`.

This only fires if a Rust component performs TLS operations. Today that's
none of your currently-scoped work (farmer/core/OpenBao-client are all Go,
and Go's standard `crypto/tls` doesn't have this failure mode). It becomes
relevant the moment any of the following exist: the Rust COM/WMI helper
binary discussed for the Windows ingredient gaps, a future full Rust sprout,
or any other Rust process that talks TLS to farmer, OpenBao, or the
enrollment endpoint.

---

## Exact insertion points

### 1. `grlx-master-plan.md`
Two spots, same underlying claim:
- Under **"Settled architecture, referenced throughout"** → the
  **Secrets custody** bullet ("OpenBao — operator/account signing keys, TLS
  cert issuance, per-tenant X25519 private keys.") — append the caution
  block right after this bullet.
- Under **Phase 0** → the bullet "OpenBao-backed custody for
  Operator/Account signing keys and TLS certs." — same block, or a
  one-line pointer back to the first instance to avoid duplicating a long
  callout twice in one doc.

### 2. `grlx-1m-scale-plan.md`
Under **Phase 0** → the identical bullet ("OpenBao-backed key custody for
the Operator/Account signing keys and TLS certs.") — same caution block.
This doc restates the same Phase 0 content as the master plan, so it needs
the same note for anyone reading this doc in isolation.

### 3. `grlx-fork-roadmap.md`
Under **workstream F (OpenBao TLS integration)** → in the **Risk** row,
currently "🟢 Low — clean seam, nothing downstream needs to change since it
only consumes file paths." Add a sentence: workstream F itself is Go and
unaffected, but flag that this risk applies to any future Rust component in
this trust chain (link to the caution block rather than repeating it in
full, since this table is already dense).

### 4. `grlx-nats-jwt-auth-design.md`
Under **"Key custody — ties directly to workstream F (OpenBao)"** — this
section is the actual heart of where it matters most: Operator root key,
per-tenant Account signing keys, and SYS account credentials all move
through OpenBao here. Add the full caution block at the end of this
section, since this is the highest-value place for a future implementer to
see it before choosing a TLS stack for anything that touches this flow.

### 5. `grlx-payload-encryption-design.md`
Under **"Storage, consistent with decisions elsewhere in the plan"** → the
`tenant_priv → OpenBao` bullet — add a short pointer back to the
`grlx-nats-jwt-auth-design.md` instance rather than repeating the full
block a third time, since this doc already cross-references that one for
the custody model.

---

## Why this note is structured as pointers instead of one block

The full explanation belongs in exactly one place
(`grlx-nats-jwt-auth-design.md`'s key-custody section, since that's the
document a future implementer is most likely reading right before deciding
how a component authenticates to OpenBao) — the other four locations get a
short cross-reference rather than the full paragraph again, matching the
pattern the docs themselves already use elsewhere (e.g. how
`grlx-payload-encryption-design.md` already points back to
`grlx-nats-jwt-auth-design.md` for the OpenBao custody model rather than
re-explaining it).
