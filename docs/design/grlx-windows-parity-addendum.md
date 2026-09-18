# Addendum to grlx-fork-roadmap.md: Windows Module Parity (Salt Gap Closure)

Expands workstream G ("Windows sprout support") from a single estimate into
discrete sub-items, based on a module-by-module comparison against Salt's
`win_*` execution modules and what Python library (if any) each depends on.
Splitting matters because the gaps fall into three genuinely different
difficulty classes:

- **G.1–G.7**: native Win32/COM/WMI capabilities — Salt itself uses
  `pywin32`/`wmi` here, not a shell-out, because the operation needs the API.
  These need a real Go binding and carry the same "implemented ≠ verified on
  Windows" constraint as the rest of workstream G.
- **G.8**: PowerShell-only modules — Salt shells out to `powershell.exe`
  itself. Porting means an `os/exec` call and output parsing, not a new
  binding.
- **G.9**: CLI-tool-only modules — same shape as G.8, one level simpler
  (`netsh`, `auditpol.exe`, etc., no PowerShell involved at all).

Original workstream G's Windows-ingredient estimate (~4–6 weeks) implicitly
assumed something closer to G.1–G.3 below. G.4–G.9 are net-new scope this
addendum surfaces, mostly *because* Salt's own module set is broader than
what the original grlx fork target covered.

---

### G.1 Windows service provider *(supersedes original G's service-provider line — unchanged effort, now confirmed against Salt's own approach)*
| | |
|---|---|
| **Change** | SCM-backed service provider matching `win_service`'s create/start/stop/delete/status surface. Salt's own module was rewritten in 2016.11.0 specifically to use PyWin32 instead of shelling out — confirms the API-native approach is the right one, not a shortcut. |
| **Library** | `golang.org/x/sys/windows/svc/mgr` (stdlib-adjacent, zero extra dependency) or `kardianos/service` (MIT) if a higher-level cross-platform interface is preferred over hand-rolling the OS-detection layer. |
| **Effort** | ~1 week. |
| **Claude Code fit** | 🟢 High — mechanical once the target interface (grlx's existing `service` ingredient shape) is fixed. |
| **Risk** | 🟢 Low — mature, well-trodden API on both the Salt and Go sides. |

### G.2 Windows user/group provider *(supersedes original G's user/group line)*
| | |
|---|---|
| **Change** | Replace the Linux-only `useradd`/`usermod`/`groupadd` shell-outs with a real provider matching `win_useradd`/`win_groupadd`'s add/delete/list/list_groups surface. |
| **Library** | `deploymenttheory/go-bindings-win32`'s `netmanagement` package — generated bindings for `NetUserAdd`, `NetUserDel`, `NetLocalGroupAdd`, `NetLocalGroupAddMembers`, etc., the same `netapi32.dll` surface Salt's `win32net`/`win32netcon` wrap. |
| **Effort** | ~1–1.5 weeks (unchanged from original estimate). |
| **Claude Code fit** | 🟢 High for writing against the generated bindings — the example `localaccount` program in that repo is close to a working template already. |
| **Risk** | 🟡 Medium — the library itself is young (v0.2.x, generated code, first released mid-2026). Treat this as the roadmap's existing caution for workstream G applies doubly here: a first draft is fine, but user/group creation is exactly the "auth-adjacent, needs human security review" category, and the dependency's own maturity is part of what to review. |

### G.3 Registry ingredient *(unchanged from original — confirmed, not superseded)*
| | |
|---|---|
| **Change** | New ingredient type from scratch — no equivalent exists in grlx today. |
| **Library** | `golang.org/x/sys/windows/registry` — official, stable; nothing in the ecosystem beats it (the one alternative found, `prcoito/registry`, is for offline registry-*file* forensics, not live management). |
| **Effort** | ~1–1.5 weeks (unchanged). |
| **Claude Code fit** | 🟢 High — follows the existing self-registering ingredient plugin pattern directly. |
| **Risk** | 🟢 Low. |

### G.4 DACL/ACL ingredient *(net-new — not in original roadmap)*
| | |
|---|---|
| **Change** | File and registry-key ACL management matching `win_dacl`'s add_ace/check_ace/check_inheritance surface. No grlx equivalent exists today, on either OS. |
| **Library** | `hectane/go-acl` (MIT) — wraps `GetNamedSecurityInfo`/`SetNamedSecurityInfo`/`SetEntriesInAcl`, the same three Win32 calls Salt's `win32security`-based module uses. Ships file-object (`SE_FILE_OBJECT`) support directly; registry-key ACLs (`SE_REGISTRY_KEY`) need the same calls with a different object-type constant — an extension of the library's exposed `api` package, not a new dependency. |
| **Effort** | ~1 week for file ACLs, +3–5 days to extend to registry-key ACLs. |
| **Claude Code fit** | 🟡 Medium — mechanical once the ingredient's desired-state shape (grant/deny/propagation, matching Salt's ALLOW/DENY + KEY/KEY&SUBKEYS/SUBKEYS propagation model) is settled, but permission/security-descriptor code is worth a human review pass same as any ACL-touching change. |
| **Risk** | 🟡 Medium — getting propagation/inheritance semantics subtly wrong is a security-relevant bug class (over-broad or under-broad grants), not just a functional one. |

### G.5 Hardware/BIOS facts *(net-new — strengthens existing facts/probe subsystem)*
| | |
|---|---|
| **Change** | Hardware/BIOS/disk facts matching `win_smbios`/`win_disk`/parts of `win_system`, which Salt gathers via WQL queries against the Python `wmi` library. Also directly useful as backing for `probe` ingredient result enrichment (Phase 5 of the master plan). |
| **Library (Route A — WMI)** | `yusufpapurcu/wmi` (MIT) — actively maintained fork of the now-archived `StackExchange/wmi`, built on `go-ole/go-ole`. Proven in production via the Prometheus `windows_exporter`, not a toy dependency. Matches Salt's own approach exactly (WQL queries), so it's the lower-risk, most-direct port. |
| **Library (Route B — direct SMBIOS, no COM)** | A Go equivalent of Rust's `smbios-lib` — e.g. `digitalocean/go-smbios` or the SMBIOS reader inside `jaypipes/ghw` — parses SMBIOS tables directly (`GetSystemFirmwareTable` on Windows, `/sys/firmware/dmi/tables` on Linux) instead of going through WMI/COM at all. **Surfaced by Tencent's `tat-agent`** (a production Rust cross-platform fleet agent solving the same problem), which uses `smbios-lib` for exactly this rather than a WMI-equivalent. For the specific subset of facts `win_smbios`/`win_disk` actually needs (BIOS vendor/version, serial number, disk enumeration), this sidesteps COM entirely — meaning it doesn't depend on G.6's COM investment (`go-ole`) at all, and could ship independently and earlier. |
| **Recommendation** | Prefer Route B for the BIOS/hardware-identity subset (cheaper, no COM dependency, cross-platform code reuse with the Linux facts path) and fall back to Route A (WMI) only for facts that genuinely have no SMBIOS equivalent (live disk I/O counters, OS-level state `win_disk` also exposes). Worth a short spike comparing actual field coverage before committing to one route exclusively. |
| **Effort** | Route A: ~3–5 days for the initial set of WQL queries (`Win32_BIOS`, `Win32_DiskDrive`, `Win32_ComputerSystem`, etc.) wired into the existing facts collection path. Route B: ~2–4 days — parsing is simpler than wiring a WMI/COM client, and the same library/approach can back the Linux facts path too (one code path instead of two). |
| **Claude Code fit** | 🟢 High for either route — repetitive, well-precedented work once the first query/parse is wired end-to-end. |
| **Risk** | 🟢 Low for either — read-only, no state-changing risk. Route B is arguably lower risk overall since it has one fewer moving part (no COM lifecycle to get wrong). |

### G.6 COM-based ingredients: Task Scheduler, Windows Update, Shortcuts *(net-new)*
| | |
|---|---|
| **Change** | Three Salt modules (`win_task`, `win_wua`, `win_shortcut`) that Salt implements via COM objects (`win32com`), not a flat API call — Task Scheduler's `ITaskService`/`ITaskDefinition`, WUA's update-search/install COM interface, and Shell's `IShellLink` for `.lnk` creation. No grlx equivalent for any of the three today. |
| **Library** | `go-ole/go-ole` (MIT) directly for COM lifecycle (`CoInitialize`, `IDispatch` method calls) — the same foundation `yusufpapurcu/wmi` above is built on, so adopting it here shares infrastructure with G.5. |
| **Effort** | Task Scheduler: ~1–1.5 weeks (triggers/actions/XML task definitions are the most involved of the three, matching Salt's own ~2,200-line module). WUA: ~1 week (search/download/install state machine). Shortcut: ~2–3 days (`IShellLink` is a small, well-documented interface). |
| **Claude Code fit** | 🟡 Medium — COM method-call code is mechanical once the object model is understood, but COM lifecycle bugs (missed `Release`, wrong apartment threading) fail in ways that are easy to miss in a quick review and worth deliberate testing, not just passing unit tests. |
| **Risk** | 🟡 Medium — same "cannot validate in a Linux sandbox" constraint as the rest of workstream G, compounded by COM being a less forgiving failure mode than a flat syscall. |

### G.7 Local Group Policy (LGPO) *(net-new — the one item with no library shortcut)*
| | |
|---|---|
| **Change** | Matching `win_lgpo`'s policy get/set surface. Unlike G.1–G.6, there's no off-the-shelf Go library to adopt — Salt's own module leans on `pywin32` *and* `lxml` because the real work is parsing the binary `registry.pol` format and the XML-based ADMX/ADML policy template files, then mapping human-readable policy names to the underlying registry keys they control. |
| **Library** | None found. `registry.pol` is a simple, documented binary format (tractable with stdlib `encoding/binary`); ADMX/ADML are plain XML (stdlib `encoding/xml` suffices). This is a parser-and-lookup-table build, not a missing-dependency problem. |
| **Effort** | ~2–3 weeks — by far the largest single item in this addendum, and Salt's own docs flag known limitations even after years of development (start/shutdown scripts policies aren't fully configurable, not all Security Settings policies are mapped) — worth setting expectations for partial coverage rather than full LGPO parity on a first pass. |
| **Claude Code fit** | 🟡 Medium — the binary/XML parsing itself is mechanical, but building and validating the policy-name-to-registry-key mapping table against real ADMX templates is exactly the kind of long-tail correctness work that benefits from human spot-checking, not just tests passing. |
| **Risk** | 🔴 High if scoped as "full parity" — recommend explicitly scoping a v1 subset (the policies your actual customer base needs) rather than treating this as a complete Salt LGPO port, given even Salt's mature implementation has documented gaps. |

### G.8 PowerShell-only modules — batch port *(net-new, but cheap)*
| | |
|---|---|
| **Change** | `win_servermanager`, `win_dsc`/`win_dsc_resource`, `win_psget`, `win_iis`, `win_pki`, `win_snmp`, `win_smtp_server`, `win_appx` — Salt itself shells out to `powershell.exe` for every one of these (ServerManager module, DSC engine, PowerShellGet, WebAdministration, Pki module, etc.). No Win32 binding needed at all; this is `exec.Command("powershell.exe", "-Command", ...)` plus `ConvertTo-Json` output parsing, reusing grlx's existing `cmd` ingredient execution path. |
| **Effort** | ~2–3 days per module once the first one establishes the pattern (PowerShell invocation wrapper + JSON-output parsing convention) — call it ~3 weeks for all eight, highly parallelizable across engineers or agent sessions since each is independent. |
| **Claude Code fit** | 🟢 Very high — the most mechanical, lowest-ambiguity item in this entire addendum. Good candidate for unattended batch delegation once the first module is reviewed and the pattern approved. |
| **Risk** | 🟢 Low individually. 🟡 Aggregate risk is just the standard workstream-G constraint: none of these eight can be exercised outside a real Windows host, so "written" and "verified" diverge the same way they do for the rest of Windows sprout support. |

### G.9 CLI-tool-only modules — batch port *(net-new, cheaper still)*
| | |
|---|---|
| **Change** | `win_firewall` (`netsh advfirewall`), `win_dns_client`/parts of `win_ip`/`win_network` (`netsh interface`), `win_auditpol` (`auditpol.exe`), `win_powercfg` (`powercfg.exe`), `win_certutil` (`certutil.exe`) — no PowerShell even involved, just native CLI tools every Windows install ships with. |
| **Effort** | ~1–2 days per module — simpler than G.8 since there's no PowerShell JSON-conversion step, just plain-text output parsing per tool. ~1.5 weeks total for all five. |
| **Claude Code fit** | 🟢 Very high — same shape as G.8, one notch simpler. |
| **Risk** | 🟢 Low. |

---

## Revised workstream G total

| Sub-item | Effort | Risk |
|---|---|---|
| G.1 Service provider | ~1 week | 🟢 |
| G.2 User/group provider | ~1–1.5 weeks | 🟡 |
| G.3 Registry ingredient | ~1–1.5 weeks | 🟢 |
| G.4 DACL/ACL ingredient | ~1–1.5 weeks | 🟡 |
| G.5 Hardware/BIOS facts (SMBIOS or WMI) | ~2–5 days | 🟢 |
| G.6 COM ingredients (task/WUA/shortcut) | ~2.5–3 weeks | 🟡 |
| G.7 LGPO | ~2–3 weeks | 🔴 |
| G.8 PowerShell-module batch | ~3 weeks | 🟢 (🟡 aggregate) |
| G.9 CLI-tool-module batch | ~1.5 weeks | 🟢 |
| **Total** | **~14–17 weeks** | — |

This roughly triples the original ~4–6 week estimate for workstream G — the
increase is almost entirely G.6/G.7/G.8 (COM ingredients, LGPO, and the
PowerShell-module batch), none of which were in scope when the original
roadmap was written against a narrower "file/service/package/registry +
run commands" target. All nine sub-items remain **fully parallel with every
farmer/core workstream (A–F, H–M)** — sprout-side, no dependency on the
messaging/storage/auth work.

**Sequencing recommendation within G:** G.1–G.3 first (they unblock basic
Windows sprout viability and were already committed in the original plan),
G.5 and G.9 next (cheap, low-risk, no new infra), G.8 in parallel once the
PowerShell-invocation pattern is proven on one module, then G.4 and G.6
(both need the security/COM review discipline the rest of the plan applies
to auth-adjacent work), with G.7 (LGPO) deliberately last and explicitly
scoped to a v1 subset rather than full parity — it's the one item here
without a library shortcut and the one Salt itself still hasn't fully solved.

**Same constraint as the rest of workstream G applies to all nine
sub-items:** every one of them can be written and cross-compiled
(`GOOS=windows`) in a Linux sandbox, but none can be *validated* there — a
real Windows CI runner or VM remains a hard prerequisite before trusting
any of this in production, not an afterthought once the code looks done.
