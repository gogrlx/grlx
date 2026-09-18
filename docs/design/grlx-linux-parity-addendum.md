# Addendum to grlx-fork-roadmap.md: Linux Module Parity (Salt Gap Closure)

Companion to `grlx-windows-parity-addendum.md`, same method: compare against
Salt's own module set and what each depends on. The Linux picture is
structurally different from Windows, though, and worth stating plainly
before the table:

**On Windows, "native vs. shell-out" was mostly a Salt-imposed distinction**
— `win_service`/`win_dacl`/`win_task` use `pywin32`/COM because the
operation genuinely has no CLI equivalent. **On Linux, several Salt modules
don't use a native binding either** — `salt.modules.mount` and
`salt.modules.selinux` carry no `:depends:` entry at all; they shell out to
`mount`/`umount` and `semanage`/`setsebool`/`getenforce` themselves, same as
grlx's `cmd` ingredient could. So the real question for each Linux gap isn't
"does Salt use a library" (mostly no) — it's **"does going native (syscall
or netlink) actually buy something a shell-out can't"**: atomic operations,
structured errors, no fragile text-parsing of CLI output. Only the ones
where that answer is genuinely yes are listed below as dedicated
ingredients. Everything else (LVM/blockdev provisioning, `ipset`, `archive`,
`git`, `x509` certs, ini/config editing, `/etc/hosts`) is already well
served by the existing `cmd` and `file`/`file.line` ingredients — building
a dedicated wrapper would just be argument templating around the same CLI
tool Salt itself shells out to, so it's not worth the ingredient-count
overhead.

---

### H.1 Network interface / route management *(net-new)*
| | |
|---|---|
| **Change** | Interface, IP address, and route configuration matching `network`/`linux_ip`/`debian_ip`'s surface. |
| **Salt's own approach** | Shells to `ip addr`/`ip route`, then parses the text output — or hand-writes distro-specific config files (`/etc/network/interfaces`, `ifcfg-*`). Fragile in both directions: text-parsing `ip` output breaks on format changes across `iproute2` versions, and distro-config-file writing means maintaining N different file formats. |
| **Library** | **`vishvananda/netlink`** (Apache-2.0) — pure Go, talks directly to the kernel over netlink sockets, no shell-out, no text parsing. The same library Docker, Kubernetes, and most CNI plugins use for exactly this. Structured Go types for links/addresses/routes instead of scraped CLI text. |
| **Effort** | ~1–1.5 weeks — link up/down, address add/remove, route add/remove/list, mapped onto a `network` ingredient's desired-state shape. |
| **Claude Code fit** | 🟢 High — `netlink`'s API is well-documented and idiomatic Go; mechanical once the ingredient's schema is fixed. |
| **Risk** | 🟡 Medium — misconfiguring a route or interface state can cut off the sprout's own connectivity to farmer, which is a self-inflicted outage class worth a deliberate "verify connectivity survives the change, or roll back" pattern in the ingredient itself, not just a code-review concern. |

### H.2 Firewall (nftables / iptables) *(net-new)*
| | |
|---|---|
| **Change** | Firewall rule management matching `iptables`/`firewalld`'s surface — add/delete/check rules, chains, tables. |
| **Salt's own approach** | Shells to `iptables`/`ip6tables`/`ipset`, or talks to firewalld over D-Bus. The `iptables` module in particular parses `iptables-save`/`iptables -L` text output to check rule existence — exactly the fragile-parsing pattern worth avoiding. |
| **Library** | **`google/nftables`** (Apache-2.0) — pure Go, netlink-based, structured rule/chain/table objects instead of parsed CLI text. Given modern RHEL8+/Debian10+ (the likely baseline for CERT-In-scoped Indian enterprise/BFSI/govt deployments) default to nftables as the backend anyway, this is also the more future-proof choice over iptables-legacy. Add **`coreos/go-iptables`** (Apache-2.0) only if genuine iptables-legacy compat is a real customer requirement — it shells to the `iptables` binary itself, so it's a compatibility fallback, not the primary path. |
| **Effort** | ~1.5–2 weeks for nftables (rule/chain/table CRUD, matching Salt's add_rule/delete_rule/rule_exists shape). +2–3 days if iptables-legacy compat is added. |
| **Claude Code fit** | 🟡 Medium — nftables' rule-expression building (matching a rule spec like `firewalld`'s to `nftables`' expr objects) has enough of a translation layer to need care, though it's a bounded, well-specified problem once the mapping is written. |
| **Risk** | 🔴 High if rushed — a firewall ingredient is squarely in "can lock yourself out or open something you didn't mean to" territory. Treat this with the same security-review discipline the main roadmap already applies to auth-adjacent work (workstream B/H), not as routine ingredient work. |

### H.3 SELinux *(net-new — compliance-relevant)*
| | |
|---|---|
| **Change** | Enforcement mode, booleans, file/process contexts, matching `selinux`'s get/set surface. |
| **Salt's own approach** | Shells to `semanage`, `setsebool`, `getenforce`, `semodule` — no native binding, despite `libselinux` having a C API. |
| **Library** | **`opencontainers/selinux`** (Apache-2.0) — pure Go, no cgo, reads `/sys/fs/selinux` and SELinux config directly rather than shelling to `getenforce`/`setsebool` and parsing their output. Same library used across the container ecosystem (Docker, runc, Kubernetes), so it's a well-trusted dependency, not a niche pick. |
| **Effort** | ~4–6 days — enforcement mode get/set, boolean get/set, basic context operations. Full policy-module install/list (`semodule`) is a smaller addition on top if needed. |
| **Claude Code fit** | 🟢 High — the library's API surface maps cleanly onto Salt's own function names. |
| **Risk** | 🟡 Medium — worth flagging as CERT-In/DPDP-relevant given your BFSI/government customer base: getting SELinux state management right (vs. silently degrading to permissive) is a compliance-visible correctness property, not just a feature. |

### H.4 Mount / fstab management *(net-new)*
| | |
|---|---|
| **Change** | Mount/unmount + `/etc/fstab` management matching `mount`'s surface. |
| **Salt's own approach** | Shells to `mount`/`umount`, parses `/etc/fstab` and `/proc/mounts` as flat text. |
| **Library** | `golang.org/x/sys/unix` exposes the `Mount`/`Unmount` syscalls directly — no shell-out, structured `errno`-level errors instead of parsed stderr. `/etc/fstab` itself is a simple whitespace-delimited format, straightforward to parse/write with the stdlib, no third-party library needed. |
| **Effort** | ~4–6 days. |
| **Claude Code fit** | 🟢 High — small, well-bounded syscall wrapper plus a simple flat-file parser. |
| **Risk** | 🟢 Low for local filesystem mounts (ext4/xfs/etc.) — the realistic case for bare-metal/GPU storage provisioning. **Worth an explicit caveat, though:** the `mount` *command* also dispatches to filesystem-specific mount helpers (`mount.nfs`, `mount.cifs`, etc.) for network filesystems, which the raw `mount(2)` syscall does not do. If NFS/CIFS mounts are ever in scope, keep a `cmd`-based fallback to the real `mount` binary for those filesystem types rather than assuming the syscall covers everything Salt's `mount` module does. |

### H.5 Cron *(net-new — cheap, high recipe-frequency)*
| | |
|---|---|
| **Change** | Per-user crontab management matching `cron`'s list/set/remove surface. |
| **Salt's own approach** | Reads/writes `crontab -l`/`crontab <file>` output — a well-documented flat text format, no library. |
| **Library** | None needed — direct read/write of the crontab format via the stdlib, or shelling to the `crontab` binary itself (matches Salt's own approach exactly, and avoids needing to handle every historical crontab quirk yourself). |
| **Effort** | ~3–4 days — this is genuinely one of the cheapest items on this list; called out separately from the "just use `cmd`" bucket only because cron entries are common enough in real recipes that a proper desired-state ingredient (idempotent add/remove-by-identifier, not just "run this command") is worth the small investment. |
| **Claude Code fit** | 🟢 Very high. |
| **Risk** | 🟢 Low. |

---

## What's deliberately left as `cmd`/`file` work, not a dedicated ingredient

| Salt module | Why it's fine via `cmd`/`file.line` |
|---|---|
| `linux_lvm`, `blockdev`, `extfs`, `disk` | No mature Go LVM binding exists, and Salt itself just shells to `lvm`/`pvcreate`/`mkfs.*` too — going native buys nothing here that a `cmd` wrapper doesn't already give you. |
| `kmod` | **Correction from first pass:** I initially assumed `x/sys/unix`'s `InitModule`/`DeleteModule` syscalls would be a clean native win here, same pattern as `mount`. They're not — those raw syscalls don't resolve module dependencies the way `modprobe` does via `modules.dep` (built by `depmod`). Reimplementing that resolution logic is real, unnecessary work; shelling to `modprobe`/`rmmod` (Salt's own approach) is actually the *correct* choice here, not just the cheap one. |
| `ipset` | Thin enough to fold into the H.2 firewall ingredient later if needed, not worth a separate item now. |
| `archive` | Go stdlib `archive/tar`/`archive/zip` cover this without any third-party dependency — trivial either way, doesn't need a design discussion. |
| `git` | `go-git/go-git` (Apache-2.0) is a genuinely nice-to-have (recipes wouldn't need `git` installed on the sprout), but it's a convenience, not a gap — `cmd` + the system `git` binary works today with zero new code. |
| `x509` | Go stdlib `crypto/x509` is already stronger than Salt's Python `cryptography` dependency; only worth a dedicated ingredient if a recipe-level need shows up, not speculatively. |
| `ini_manage`, `augeas_cfg` | `ini_manage`'s realistic use cases are covered by the already-planned `file.line` ingredient (workstream L). `augeas_cfg` has no clean pure-Go binding — `libaugeas` would need cgo, which conflicts with the CGO-free posture the rest of this plan follows — recommend explicitly not porting it rather than compromising on that. |
| `hosts` | Flat-file edit, `file`/`file.line` already covers it. |

---

## Revised total for this addendum

| Sub-item | Effort | Risk |
|---|---|---|
| H.1 Network (netlink) | ~1–1.5 weeks | 🟡 |
| H.2 Firewall (nftables) | ~1.5–2 weeks | 🔴 |
| H.3 SELinux | ~4–6 days | 🟡 |
| H.4 Mount/fstab | ~4–6 days | 🟢 |
| H.5 Cron | ~3–4 days | 🟢 |
| **Total** | **~4.5–6 weeks** | — |

Notably smaller and lower-risk in aggregate than the Windows addendum
(~14–17 weeks), for two reasons: Linux's syscall/netlink surface in Go is
more mature than Windows' COM/Win32 surface, and — per the correction above
— being disciplined about *which* gaps actually deserve a native binding
(rather than assuming "native beats shell-out" as a blanket rule) trims real
scope. **H.2 (firewall) is the one item here that deserves the same
security-review weight as the Windows DACL/user-group items** — everything
else is comparatively low-stakes, mechanical work.

All five items are sprout-side, fully parallel with the farmer/core
workstreams (A–F, H–M) and with the Windows addendum (G.1–G.9) — no
cross-dependency between the two platforms' ingredient work.
