# grlx: Sprout Orchestration & Spot-Derived Capabilities

Design decision, with a capability map, for extending sprout's recipe/cook engine. Supersedes the "probe as a workflow-running ingredient" framing from earlier in the plan.

## The principle

**One orchestrator per sprout: the existing recipe/cook engine (`internal/cook`). Everything else is an atomic ingredient it sequences.**

Before this decision, three separate things could each have ended up implementing their own notion of step ordering, conditionals, and variable passing: the recipe engine itself, a probe-internal workflow (if probe were built as a self-contained multi-step runner), and a ported version of Spot's playbook/task engine. That's redundant and a real source of divergent bugs — three places that could each get "run this only if that succeeded" or "pass this value to the next step" subtly wrong in different ways. Collapsing to one orchestrator means every capability borrowed from Spot lands in exactly one of two buckets: something the recipe engine can *do* (an orchestration primitive), or something the recipe engine can *invoke* (an ingredient). Nothing gets its own competing sequencing logic.

**Farmer-level fleet orchestration (rolling/staged rollout across many sprouts) is not a third thing to reconcile with this — it's a genuinely different layer, operating at a different scope, and stays separate.** The recipe engine sequences steps *within* one sprout's execution of one recipe. Farmer's job dispatch sequences *which sprouts* get told to start cooking, and when, across a fleet. Confirmed against the actual code: cooking happens entirely in `internal/cook/sproutcook.go`; farmer's own cook-related code (`farmercook.go`) only resolves and serves recipe content, never executes anything. Keeping these distinct matters — conflating them would either bloat the sprout-level engine with fleet concepts it has no business knowing about, or bloat farmer with per-step execution logic it structurally can't run.

## New atomic ingredients (things that *do* something)

| Ingredient | Source idea | Notes |
|---|---|---|
| `probe.http` | Spot's ad-hoc HTTP checks, generalized | Single HTTP call + response validation. No internal sequencing — chaining across probes happens via the recipe engine's own steps. |
| `probe.database` | Same | Single query + validation, same scoping. |
| `wait` | Spot's `wait: {cmd, timeout, interval}` | Poll a command/condition until success or timeout — an atomic action in its own right (retries internally), not a sequencer of other ingredients. |
| `file.sync` | Spot's `sync` | Directory sync with optional delete-orphans and exclude patterns — check whether the existing `file` ingredient already covers this before treating it as net-new. |
| `file.line` | Spot's `line` | Regex-based delete/replace/append of a line in a file — check against existing `file` ingredient coverage first; Salt has `file.line`/`file.replace` equivalents this may already mirror. |
| `file.copy` enrichment | Spot's `copy` (push/pull direction, glob, exclude, mkdir, chmod+x) | Enrichment of the existing ingredient rather than a new one, if the current implementation doesn't already cover bidirectional transfer and glob/exclude. |

**Explicitly not an ingredient:** anything resembling Spot's `script` command is already covered by the existing `cmd` ingredient — no new capability needed there.

## New recipe/cook-engine orchestration primitives (things the engine can do, not things it invokes)

| Primitive | Source idea | Applies to |
|---|---|---|
| Conditional step execution (`cond`, with negation) | Spot's `cond` | Any ingredient — a step runs only if a shell test passes/fails. Confirm this doesn't already exist in some form before building it as new. |
| Deferred/`on_exit` actions | Spot's `on_exit` | Any ingredient — cleanup that runs regardless of the step's outcome (e.g., always remove a temp file). |
| Variable registration & passing between steps | Spot's `register` + auto-exported shell vars | The general mechanism. Probe's sensitive-value chaining (already designed) becomes a *property* of a registered variable (`sensitive: true`) rather than a probe-specific bolt-on — one mechanism, marked per-value, used by any ingredient that produces or consumes step output. |
| Runtime context variables | Spot's `{SPOT_REMOTE_HOST}` family | `{GRLX_SPROUT_ID}`, `{GRLX_TENANT_ID}`, etc. — auto-populated, available in any recipe step. |
| Secret injection into step input | Spot's `options.secrets: [name]` pattern | Not a new mechanism — wired directly to the already-designed `sdb://` SecretProvider interface. Reference by name, resolve at execution time, never write the literal value into the recipe. Redaction extends beyond persisted job logs to any verbose/debug/diagnostic output, since Spot's own docs disclose a gap here (its masking only covers `options.secrets`, not its separate `register` mechanism) — worth deliberately covering both paths rather than repeating that gap. |

## Explicit non-goals

- No probe-internal workflow engine — probe is atomic ingredients, full stop.
- No ported Spot playbook/task engine — its useful ideas are absorbed as recipe-engine primitives above, not as a parallel orchestration system.
- No separate probe-only secrets mechanism — routed through SDB via the same primitive every other ingredient uses.
- Farmer-level rolling rollout is tracked separately (fleet-wide dispatch sequencing) and is not part of this document's scope.

## Open item carried from earlier discussion

Farmer-side rolling/staged rollout with health-gating (Spot's `--concurrent` + `wait` combination, translated to fleet scope) remains a real, separate gap worth its own design pass — dispatch to a batch of sprouts, gate on a probe/job-status success signal, proceed to the next batch. Natural to sequence alongside this work since it consumes probe's results, but it is farmer logic, not a sprout-side ingredient or recipe-engine primitive.
