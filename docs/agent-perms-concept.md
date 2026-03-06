# agent-perms

## A Semantic Permission Layer for CLI Agents

**Product Concept & Research Summary** | February 2026

**Status:** MVP implemented. Core binary supports `gh`, `git`, `go`, and `pulumi` classifiers with `exec`, `explain`, `claude` (md/init/validate), and `codex` (md/init/validate) commands. Three built-in profiles (`read`, `write-local`, `full-write`) for both Claude Code and Codex CLI. See README.md for current usage.

---

## Reading Guide

- **Evaluating the concept?** Read Sections 1–7. (~15 min)
- **Integrating with an agent platform?** Focus on Sections 4–5 and [Appendix C](#appendix-c-agent-platform-deep-dives).
- **Concerned about security?** See Section 7 and [Appendix B](#appendix-b-security-deep-dive).
- **Setting up or configuring?** See Section 5 and [Appendix C](#appendix-c-agent-platform-deep-dives).

---

## Table of Contents

**Core Document**

1. [Executive Summary](#1-executive-summary)
2. [The Problem](#2-the-problem)
3. [The Solution](#3-the-solution)
4. [How Agents Integrate](#4-how-agents-integrate)
5. [Setup & Configuration](#5-setup--configuration)
6. [Prior Art & Competitive Landscape](#6-prior-art--competitive-landscape)
7. [Strengths, Weaknesses & Security Posture](#7-strengths-weaknesses--security-posture)
8. [Project Strategy](#8-project-strategy)
9. [FAQ](#9-faq)
10. [Conclusion](#10-conclusion)

**Appendices**

- [A. Technical Specification](#appendix-a-technical-specification)
- [B. Security Deep Dive](#appendix-b-security-deep-dive)
- [C. Agent Platform Deep Dives](#appendix-c-agent-platform-deep-dives)
- [D. Extensibility & Future Roadmap](#appendix-d-extensibility--future-roadmap)

---

## 1. Executive Summary

Every major AI coding agent—Claude Code, OpenAI Codex, Gemini CLI, Cursor, Windsurf, Kiro, and Cline—requires users to manage CLI permissions. The typical solution is either overly permissive allowlists (e.g., allowing all git commands when you only want reads) or a tedious allowlist of every safe subcommand. Both approaches scale poorly: allowlists become unmaintainable as CLIs evolve, and the cognitive overhead leads developers to approve everything without scrutiny.

> **agent-perms** is a developer ergonomics tool that dramatically simplifies CLI permission management. It classifies commands by intent (read, write, admin) and acts as a transparent execution proxy. Instead of allowlisting dozens of individual subcommands, you write a single rule like `Bash(agent-perms exec read -- gh *)` and get precise read-only access to the entire GitHub CLI. **One rule replaces fifty.**

agent-perms is not an OS-level security boundary. It is a deterministic, fast, agent-agnostic classification layer that makes permission policies easy to express, audit, and maintain. It complements existing sandboxes and permission configs by providing semantic vocabulary those mechanisms lack.

---

## 2. The Problem

### 2.1 The Classification Gap

Agent platforms are converging on coarse-grained safety mechanisms — sandboxes reduce prompt volume (Claude Code's sandbox cuts prompts by ~84%), exec policies gate command prefixes, and hooks allow custom logic. But none of these mechanisms can classify CLI commands by _intent_. No sandbox, glob pattern, or prefix rule can distinguish `git reset --soft` from `git reset --hard`, or a `gh api` GET from a `gh api --method DELETE`. This is the classification gap.

The symptoms of this gap manifest as permission fatigue — where users reflexively approve everything because prompts lack semantic precision — and as overly permissive policies where teams give up on granularity entirely:

- **Claude Code:** Users report that even simple wildcard patterns don't reliably match, forcing repeated approvals. The community has built third-party hooks in Rust and Python just to work around this. Deny rules have had repeated reliability issues (CVE-2026-25724, Issues #6699, #12918), and sub-agents via the Task tool can bypass deny rules entirely (Issue #25000).
- **Gemini CLI:** A GitHub issue describes how constant prompting for safe commands causes "click fatigue" leading developers to "habitually approve without scrutiny." The `tools.core` restriction only validates the first command in a pipe chain (Issue #11510, fixed July 2025).
- **OpenAI Codex:** Blog posts describe the experience as "a very smart intern locked in a glass room with a whiteboard and no Wi-Fi" until you manually unlock access paths. Hooks are still in development (PR #11067), leaving enforcement instruction-based only.
- **Cursor:** Published a detailed engineering blog on agent sandboxing, noting that approval-based security degrades because users "approve without reading." Their sandbox-first pivot reduced developer interruptions by 40% — but at the cost of command-level granularity.
- **Kiro (AWS):** A December 2025 incident demonstrated the consequences of coarse permission grants: an agent with inherited operator-level access deleted and recreated a live environment, causing a 13-hour AWS outage.

### 2.2 The Granularity Gap

The core problem is a mismatch between the granularity that CLIs offer and the granularity that agents need. Consider the GitHub CLI:

- `gh pr list` is read-only. Completely safe.
- `gh pr create` is a write operation. Needs oversight.
- `gh repo delete` is destructive. Should require explicit approval.

But agent permission systems only see the binary: either you allow all of `gh` or you enumerate every safe subcommand. A typical allowlist for even moderate gh usage looks like this:

```
"Bash(gh pr list *)",
"Bash(gh pr view *)",
"Bash(gh pr checks *)",
"Bash(gh issue list *)",
"Bash(gh issue view *)",
"Bash(gh gist list *)",
"Bash(gh repo view *)",
"Bash(gh run list *)",
"Bash(gh run view *)",
... dozens more
```

This is brittle, incomplete, and unmaintainable. When `gh` ships new read-only subcommands, your allowlist is stale.

### 2.3 Existing Approaches Fall Short

| Approach | How It Works | Limitation |
|----------|-------------|------------|
| Glob allowlists | Pattern matching on command strings | No semantic awareness; must enumerate every safe subcommand |
| OS sandboxing | Seatbelt/Landlock/bubblewrap restrict filesystem and network | Operates at syscall level, not CLI-command level; too coarse for read vs. write |
| PreToolUse hooks | Custom scripts that approve/deny per tool call | Requires each user to write and maintain regex-based classification logic |
| LLM-based hooks | A second LLM classifies the command at runtime | Adds 5–25 seconds latency per command; non-deterministic |

### 2.4 Who Feels This Pain

**The Security-Conscious Developer** — Uses Claude Code daily. Reads every permission prompt but gets 50+ approvals per session. Ends up clicking "allow all" from fatigue. Wants: fewer, more meaningful prompts. Would switch if setup takes <10 minutes and reduces prompts by 80%.

**The Enterprise Security Lead** — Governs agent policies for 200+ developers. Needs audit logs, consistent policy enforcement, and compliance evidence. Current allowlists are unmaintainable. Wants: centralized policy as code, structured logs, and an SLA on classification updates.

**The Platform PM** — A product manager building agent infrastructure. Permission prompts are the #1 usability complaint. Evaluates agent-perms as "build vs. buy" — if the taxonomy is good and the DB is maintained, embedding it is cheaper than building from scratch.

---

## 3. The Solution: agent-perms

### 3.1 Core Concept

agent-perms is a standalone CLI tool that sits between the agent and any supported CLI. It classifies every subcommand into a **permission tier**:

| Tier | Intent | Examples |
|------|--------|----------|
| **read** | Queries, listings, status checks | `gh pr list`, `git log`, `go vet`, `pulumi preview` |
| **write** | Creates, updates, modifies state | `gh pr create`, `git commit`, `go fmt`, `pulumi up` |
| **admin** | Destructive or irreversible actions | `gh repo delete`, `git reset --hard`, `pulumi destroy` |

Git and Pulumi additionally support **sub-tiers** (`write:local`, `write:remote`, `admin:local`, `admin:remote`) to distinguish local from remote operations. See [Appendix A.1](#a1-permission-tiers) for details.

Classification decisions come from **Go classifiers** bundled in the binary, mapping each known CLI subcommand to a tier. These ship with the binary and are updated with each release — no network calls required.

agent-perms provides two operations:

**Explain** — query the required permission tier for a command:
```console
$ agent-perms explain gh pr view 123
cli:        gh
command:    pr view
base_tier:  read (gh pr view)
result:     read

$ agent-perms explain gh repo delete myrepo
cli:        gh
command:    repo delete
base_tier:  admin (gh repo delete)
result:     admin
```

This is the primary way agents discover the correct tier *before* executing. An LLM cannot reliably guess whether `gh api` is read or admin, or whether `git stash drop` differs from `git stash list`. `explain` answers that question deterministically. See [Section 4.1](#41-integration-pattern) for the recommended explain-then-exec flow.

**Exec** — validate and execute with transparent passthrough:
```console
$ agent-perms exec read -- gh pr view 123
# Checks that 'gh pr view' is 'read', then executes transparently

$ agent-perms exec read -- gh pr create --title 'Fix bug'
# ERROR: 'gh pr create' requires 'write', not 'read'. Exits non-zero.
```

The `exec` subcommand classifies the command internally. If the claimed tier exactly matches the required tier, it executes the command transparently—stdin, stdout, stderr, and exit codes all pass through. If the tiers don't match, it refuses with an actionable error showing the correct tier. There is no hierarchy — each tier (including `read sensitive` for commands that expose secrets) is independent and must be claimed exactly.

### 3.2 The Before/After

**Without agent-perms** — enumerate every safe subcommand (as in [Section 2.2](#22-the-granularity-gap)):
```
Bash(gh pr list *)
Bash(gh pr view *)
Bash(gh pr checks *)
... 20+ more rules
```

**With agent-perms** — one rule covers all read-only `gh` commands:
```
Bash(agent-perms exec read -- gh *)
```

The classification database stays up to date as CLIs evolve, so new read-only subcommands are covered automatically.

### 3.3 Composable Permission Rules

Because the permission tier is an explicit argument to `exec`, you get fine-grained control using existing wildcard syntax:

**Allow all reads across all supported CLIs:**
```
Bash(agent-perms exec read -- *)
```

**Git with local/remote split** (uses git-specific sub-tiers):
```
Bash(agent-perms exec read -- *)
Bash(agent-perms exec write local -- git *)
Bash(agent-perms exec write remote -- git *)
Bash(agent-perms exec write -- gh *)
# admin:local, admin:remote: agent must ask for approval
```

> `write local` and `write remote` are **sub-tiers** (used by git and pulumi) that distinguish local operations (commit, checkout) from remote ones (push, fetch). `go` and `gh` use flat `read`/`write`/`admin` tiers.

**Allow reads for gh, writes for gists only:**
```
Bash(agent-perms exec read -- gh *)
Bash(agent-perms exec write -- gh gist *)
```

Teams express nuanced policies in 2–5 lines that would otherwise require 50+ individual allowlists.

### 3.4 Command Ergonomics

The full command `agent-perms exec read -- gh pr list` is verbose. For convenience:

*Note: The `ap` short alias and default subcommand described here are not yet implemented. Currently use `agent-perms exec` and `agent-perms explain` explicitly.*

### 3.5 Unknown Commands

If `exec` encounters an unsupported CLI, it returns `unknown`. Fallback is configurable:

- `--on-unknown=deny` — refuse and exit non-zero (**default** — fail closed)
- `--on-unknown=allow` — run the command directly (opt-in for convenience)

*Note: `--on-unknown=ask` (interactive prompting) is not implemented.*

---

## 4. How Agents Integrate

### 4.1 Integration Pattern

The agent is instructed (via CLAUDE.md, AGENTS.md, or equivalent) to use the **explain-then-exec** flow:

1. **Agent wants to run:** `gh pr list --state open`
2. **Agent checks the tier:** `agent-perms explain gh pr list --state open` → `result=read`
3. **Agent executes with the correct tier:** `agent-perms exec read -- gh pr list --state open`
4. **Output:** The PR list, exactly as if `gh pr list` was called directly

**Why explain first?** An LLM cannot reliably predict permission tiers. Commands like `git stash list` (read) vs `git stash drop` (admin), or `gh api` (admin due to raw HTTP), or flag-modified tiers (`git push --force` escalates write:remote→admin) are non-obvious. The `explain` command gives the agent a deterministic answer in one fast call, avoiding trial-and-error.

**Shortcut for simple, base-form commands:** For base-form subcommands with no flags (e.g., `gh pr list`, `git log`, `git status`), the agent may skip `explain` and call `exec` directly. Always use `explain` when flags are present or the subcommand is unfamiliar — flag-modified tiers (`git push --force` → admin) and ambiguous commands (`gh api` → admin) are precisely the cases where guessing fails. If the tier is wrong, the denial message includes the correct tier and a suggestion command, so the agent can retry:

```json
{"error":"denied","claimed":"read","required":"write","command":"gh pr create","suggestion":"agent-perms exec write -- gh pr create"}
```

Note: `explain` is an ergonomic convenience, not a security control. The security enforcement is always in `exec`, which independently verifies the tier regardless of whether `explain` was called first.

**Recommended setup:** Run `agent-perms claude init` to generate settings with permissions, deny rules, and a SessionStart hook that runs `agent-perms claude md` to inject tier instructions into every session. For Codex CLI, run `agent-perms codex init`. See README.md for details.

### 4.2 Defense-in-Depth

1. **Layer 1 (instruction):** CLAUDE.md / AGENTS.md tells the agent to use agent-perms.
2. **Layer 2 (allowlist):** Permission rules only allow `agent-perms exec` commands at approved tiers.
3. **Layer 3 (deny/hook):** Deny rules block direct CLI invocation (e.g., `gh *`, `git *` without `agent-perms` prefix).
4. **Layer 4 (sandbox):** OS sandbox restricts filesystem and network regardless.

No single layer is a guarantee. But together, they make accidental or injection-driven bypass significantly harder. Even Layer 2 alone — allowlisting `agent-perms exec <tier> --` patterns — provides the rule-reduction benefit that is agent-perms' primary value.

### 4.3 Agent Compatibility Matrix

All major platforms can run agent-perms. Enforcement strength varies — Claude Code and Gemini CLI can require agent-perms prefixing; others rely on instruction + optional hooks.

| Capability | Claude Code | Codex | Gemini CLI | Cursor |
|-----------|------------|-------|-----------|--------|
| Command-level allowlists | Full globs | Starlark prefix_rule | Prefix match | No |
| Hooks for enforcement | PreToolUse | In development (PR #11067) | Not yet | beforeShellExecution |
| Enterprise lockdown | Managed settings | requirements.toml | Enterprise config | Admin controls |
| Instruction file | CLAUDE.md | AGENTS.md | GEMINI.md | .cursor/rules/ |
| One-command setup | `claude init` | `codex init` | Manual | Manual |
| Can enforce agent-perms-only | Via deny + hooks | Via execpolicy | Via core tool restriction | Via hooks only |

**Claude Code** has the best fit — glob-based `Bash()` rules are purpose-built for this pattern. **Gemini CLI** can restrict ALL shell commands to agent-perms-prefixed commands via `tools.core`, the strongest enforcement in any current platform. See [Appendix C](#appendix-c-agent-platform-deep-dives) for per-agent deep dives.

---

## 5. Setup & Configuration

### 5.1 Installation

Single static Go binary, no runtime dependencies.

```bash
go install github.com/rgharris/agent-perms/cmd/agent-perms@main
```

Make sure `$(go env GOPATH)/bin` is on your `PATH`.

*Note: Package manager distribution (brew, etc.) and signed releases are not yet available.*

### 5.2 One-Command Setup

Setup is implemented for both Claude Code and Codex CLI:

**Claude Code:**
```console
$ agent-perms claude init                        # interactive profile selection
$ agent-perms claude init --profile=write-local  # scripted
```

This generates `~/.claude/settings.json` with allow/deny rules and a SessionStart hook. If the file already exists, rules are merged automatically.

**Codex CLI:**
```console
$ agent-perms codex init                             # interactive profile selection + write
$ agent-perms codex init --profile=write-local       # scripted
```

This creates `~/.codex/rules/agent-perms.rules` and `~/.codex/AGENTS.md`.

Both support three profiles: `read`, `write-local`, and `full-write`. Validation is available via `agent-perms claude validate` and `agent-perms codex validate`.

### 5.3 Project Configuration: `agent-perms.toml` *(not yet implemented)*

```toml
[project]
name = "my-app"

[policy]
on_unknown = "deny"           # deny | allow | ask
clean_env = false              # true = strip dangerous env vars (see Appendix A.3)
audit_log = ".agent-perms/audit.jsonl"

[tiers.allow]
read = true
"write:local" = true

[tiers.ask]
write = true
"write:remote" = true

[tiers.deny]
"admin:local" = true
"admin:remote" = true

[overrides.gh]
"pr merge" = "admin"           # our team requires extra approval for merges

[overrides.docker]
"push" = "admin"               # registry pushes need approval
```

**Tier actions:** `[tiers.allow]` auto-approves commands at those tiers. `[tiers.ask]` requires user approval per command. `[tiers.deny]` blocks commands at those tiers entirely.

**How `agent-perms.toml` relates to agent platform config:**

| Config file | Controls |
|---|---|
| `agent-perms.toml` | Tier policy — which tiers to allow/ask/deny, plus per-command overrides |
| Agent platform config (e.g., `settings.json`) | Approval rules — which `agent-perms exec <tier> --` patterns are auto-approved |

They work together: `agent-perms.toml` controls classification, the agent platform controls approval.

### 5.4 Override Safety Rules *(not yet implemented)*

Overrides follow an **asymmetric trust model** to prevent repository poisoning:

1. **Project-local overrides can only elevate tiers.** A committed `agent-perms.toml` can make things stricter (write→admin) but never looser (admin→read).
2. **User-local overrides** (`~/.config/agent-perms/overrides.toml`) can elevate or demote. Only the individual user can relax their own safety policy.
3. **The MVP flag denylist is non-overridable.** No override can disable `--upload-pack` or `--exec`.

**Precedence:** project overrides (elevate only) → user overrides → classification DB → CLI flags

**Override CLI shortcut:**
```bash
agent-perms override gh "pr merge" admin          # edits project agent-perms.toml
agent-perms override --user npm install admin      # edits user-local overrides
```

### 5.5 Verification and Debugging *(not yet implemented)*

```console
$ agent-perms doctor
Binary:  ✓ agent-perms v0.3.0 at /usr/local/bin/agent-perms
         ✓ Classification DB: 4 CLIs, 267 commands
CLIs:    ✓ gh v2.62.0 (87 commands)  ✓ git v2.43.0 (64 commands)
Agents:  ✓ Claude Code: allow rules, hook installed  ⚠ No deny rules

$ agent-perms explain --config gh pr merge
cli:        gh
command:    pr merge
base_tier:  write (gh pr merge)
override:   admin (from agent-perms.toml, line 3)
result:     admin

$ agent-perms exec --dry-run read -- gh pr create --title "Fix bug"
[DRY RUN] DENY: 'gh pr create' requires 'write', claimed 'read'
```

### 5.6 Migration from Existing Allowlists *(not yet implemented)*

```bash
$ agent-perms migrate --from claude-code
Found 23 Bash() allow rules:
  Bash(gh pr list *)      → covered by: agent-perms exec read -- gh *
  Bash(git status)        → covered by: agent-perms exec read -- git *
  ...
Proposed replacement (4 rules replace 23). Apply? [y/N/diff]
```

Teams can adopt incrementally — supported CLIs go through agent-perms while unsupported CLIs keep existing rules.

---

## 6. Prior Art & Competitive Landscape

### 6.1 Community Tools

**claude-code-permissions-hook (Korny Sietsma)** — A Rust-based PreToolUse hook with regex-based allow/deny rules. Provides *better syntax* for the same kind of rules; agent-perms provides *semantic classification* that eliminates per-subcommand rules entirely. Complementary. A Windows port shows cross-platform demand.

**Dyad's Permission Hooks** — The most sophisticated existing implementation: 627 lines of Python regex for `gh`, plus an LLM fallback for unknown commands (~25s, ~$1/5K decisions). agent-perms extracts this into a standalone binary covering multiple CLIs. Key insight from Dyad: "Rule-based hooks are better when you can write them. They are fast, deterministic, and testable."

**Trail of Bits' Claude Code Config** — A PostToolUse hook that classifies CLI commands as read or write using verb pattern lists. Validates the pattern is already used by security-focused teams, but in a one-off fashion that doesn't scale.

### 6.2 Platform Permission Systems

No agent platform has built or announced semantic CLI command classification. All rely on pattern matching, binary toggles, or sandbox isolation:

| Platform | Mechanism | Granularity | Semantic? |
|----------|-----------|-------------|-----------|
| Claude Code | Glob `Bash()` rules + hooks | Per-command patterns | No |
| Codex | `execpolicy` Starlark rules | Per-command patterns | No |
| Gemini CLI | `tools.core` prefix restriction | Per-binary prefix | No |
| Cursor | Sandbox + `beforeShellExecution` hooks | Binary (sandbox on/off) | No |
| Cline | Per-capability auto-approve toggles | Binary (all commands or none) | No |
| Windsurf | Workspace-level restrictions | Low | No |
| Kiro | Spec-driven + AWS IAM | Task-oriented | No |

Permission fatigue is the most commonly reported usability issue across all platforms. Documented complaints span every major tool:

| Platform | Complaint | Source |
|----------|-----------|--------|
| Gemini CLI | "Currently prompts for approval before executing any shell command... creates significant prompt fatigue" | Issue #5256 |
| Codex | "Repeatedly asks for command execution approval despite auto-approve settings" | Issue #10187 |
| Cursor | "Agent Mode keeps asking approval for changes" | Cursor Forum |
| Claude Code | Sub-agents bypass deny rules and run 22+ commands without approval | Issue #25000 |
| Aider | `--yes-always` doesn't actually auto-run shell commands | Issue #3903 |

The community response has been consistent: developers build one-off classification hooks (regex, LLM-based, or verb-pattern-based) because no shared solution exists.

### 6.3 Infrastructure & Standards Validation

**GitHub Agentic Workflows** (Feb 2026, technical preview) — Multi-layer architecture: a read-only agent step analyzes code and generates outputs, then a separate "safe outputs" job with write permissions validates and applies changes through an untrusted-data pipeline. Network firewalls restrict agent egress. This independently validates agent-perms' core thesis: separating read from write intent is the right abstraction at scale. GitHub enforces at infrastructure level for CI/CD; agent-perms provides the same abstraction at CLI command level for local development.

**OWASP Top 10 for Agentic Applications** (Dec 2025) — agent-perms maps to multiple risk categories, not just tool misuse:

| OWASP Risk | How agent-perms helps |
|------------|----------------------|
| **ASI02 — Tool Misuse** | Tier-based classification enables least-privilege tool access without per-subcommand enumeration |
| **ASI03 — Privilege Escalation** | Tier boundaries (read→write→admin) and flag denylist prevent escalation via dangerous flags |
| **ASI07 — Improper Output Handling** | `exec` validates command classification before execution, acting as a validation layer for agent-generated CLI commands |
| **ASI09 — Logging Deficiencies** | Structured JSON audit logs of all classification and execution decisions |

**NIST AI Agent Standards Initiative** (Feb 2026) — NIST [announced](https://www.nist.gov/news-events/news/2026/02/announcing-ai-agent-standards-initiative-interoperable-and-secure) a standards initiative emphasizing human oversight for consequential actions and scoped permissions. The Federal Register [RFI on AI agent security](https://www.federalregister.gov/documents/2026/01/08/2026-00206/request-for-information-regarding-security-considerations-for-artificial-intelligence-agents) (Jan 2026) calls for auditable, deterministic permission enforcement — aligning directly with agent-perms' structured classification model. For enterprise teams, regulatory compliance is an increasingly practical motivation for adopting semantic command classification over ad-hoc allowlists.

**AWS Kiro Incident** (Dec 2025) — An AI coding agent (Kiro) was granted an operator's full access level. The agent determined the best action was to "delete and recreate the environment," causing a 13-hour outage of AWS Cost Explorer in a mainland China region. A two-human sign-off safeguard failed because it was implemented as a permission (something that could be misconfigured or inherited) rather than an architectural gate external to the agent. This is the exact failure mode agent-perms is designed to prevent: the agent had write+admin access when it only needed read+write. Tier-based classification with an explicit admin gate would have required separate approval for the destructive operation.

**Academic Validation** — Multiple recent papers confirm the need for tool-level least privilege in agent systems:

- **Progent** (April 2025) — A domain-specific policy language for tool-call-level privilege control. Reduced attack success rate from 41.2% to 2.2%, demonstrating that deterministic permission enforcement dramatically outperforms relying on model behavior alone.
- **MiniScope** (Dec 2025, UC Berkeley) — Permission hierarchies over tool calls solved via integer linear programming. Achieved 1-6% latency overhead, validating that fine-grained permission enforcement can be fast.
- **IEEE SAGAI** (Dec 2025) — "Systems Security Foundations for Agentic Computing" identifies tool-level least privilege as a critical **open research problem** and explicitly calls for deterministic enforcement and domain-specific policy languages — precisely what agent-perms provides.
- **IEEE S&P 2026** — Found that user permission preferences for data access are consistent and predictable (85.1% accuracy), supporting the feasibility of a pre-built classification database.

**Capability-Based Security Heritage** — agent-perms' tier+CLI model borrows from decades of capability-based security research (Dennis & Van Horn 1966, Capsicum, WASI). Each `exec <tier> -- <cli> *` pattern functions as a logical capability grant: the tier specifies what operations are permitted and the CLI name scopes the capability. Capsicum's incremental adoption model — layering capability-based security on UNIX without requiring a rewrite — directly mirrors agent-perms' approach of layering semantic classification on existing agent permission systems. Unlike true capability systems (which require OS-level token enforcement), agent-perms creates *logical* capabilities that the agent platform enforces through its allowlist. The tier model also maps to familiar RBAC concepts (Conservative/Developer/Power User profiles as roles), reducing adoption friction for enterprise security teams.

### 6.4 Adjacent Tools (Complementary, Not Competitive)

OS-level sandboxing is commoditizing — all major agents now ship one (Claude Code uses Seatbelt/bubblewrap, Codex uses Landlock/seccomp, Cursor uses Seatbelt/Landlock). Claude Code's sandbox reduces permission prompts by ~84%. The remaining ~16% of prompts are about **command intent** — exactly the gap agent-perms addresses. The broader agent security ecosystem operates at different layers, all complementary:

| Layer | Tools | What they do | Relation to agent-perms |
|-------|-------|-------------|------------------------|
| **OS/Kernel** | Seatbelt, Landlock, seccomp, bubblewrap | Syscall/filesystem/network restrictions | Different layer entirely; can't distinguish `gh pr view` from `gh pr create` |
| **Container** | Docker profiles, Codex sandbox | Isolated execution environments | Provides isolation but no command semantics |
| **Network/API** | Lasso Security, Pillar Security, MCP gateways | MCP protocol proxying, access control | Operates on MCP tool calls, not shell CLI commands |
| **Pattern matching** | Codex execpolicy, Claude Code globs | String-based command allowlists | What agent-perms replaces/enhances |
| **Policy engines** | OPA/Rego, Cedar | General-purpose authorization | Could consume agent-perms classifications as input data |
| **Agent frameworks** | AutoGen, Semantic Kernel | Function-level plugin permissions | Same concept at API/function layer, not CLI layer |
| **Security scanning** | MCP Scan (Invariant Labs), Prompt Armor | Vulnerability detection, prompt injection defense | Different problem space (detection vs. classification) |

### 6.5 Competitive Position

No tool currently provides deterministic, per-subcommand read/write/admin classification as a standalone binary. agent-perms occupies an uncontested niche: **semantic CLI command classification**. Every adjacent tool either operates at a different layer (OS, network, MCP) or lacks semantic awareness (pattern matching). The consistent independent arrival at read/write separation — by GitHub (infrastructure), OWASP (standards), community hook authors (tools), and Anthropic's model spec (behavior) — confirms this is the right abstraction.

**Cross-platform policy portability** is a unique advantage no competing tool or platform feature offers. Teams using both Claude Code and Codex — or migrating between agents — define one set of semantic tiers (`read`/`write`/`admin` × `local`/`remote`) that agent-perms enforces identically on both platforms. The same classification of `gh api --method DELETE` as `admin remote` applies whether the command runs through Claude Code's `Bash()` glob rules or Codex's Starlark `prefix_rule()` entries. As agent diversity increases (Cursor, Windsurf, Gemini CLI, Kiro), this portability becomes a stronger differentiator.

---

## 7. Strengths, Weaknesses & Security Posture

### 7.1 Strengths

- **Dramatic rule reduction:** One rule replaces dozens. Easier to audit, harder to misconfigure.
- **Agent-agnostic:** Works with any agent that can run shell commands.
- **Deterministic:** No LLM in the loop. Fast (~20-80ms cold start), predictable, auditable.
- **Transparent:** Output identical to direct CLI execution.
- **Composable:** Leverages existing glob-based permission systems.
- **Community-extensible:** Adding a CLI is a self-contained Go classifier with a standard pattern.

### 7.2 Weaknesses

- **Classifier maintenance** is the primary ongoing cost. Requires governance and update cadence as CLIs evolve.
- **Not OS-level enforcement.** Relies on agent following instructions. Hooks and deny rules help but aren't guarantees.
- **Shell wrapper bypass.** `sh -c 'gh repo delete foo'` invokes the CLI indirectly. Mitigated by classifying common wrappers as `admin`.
- **Flag and environment variable injection.** Mitigated by MVP flag denylist and `--clean-env` (see [Appendix A](#appendix-a-technical-specification)).
- **Cold-start latency.** ~20-80ms per command.

### 7.3 Honest Security Posture

agent-perms is an ergonomics tool that provides defense-in-depth benefits when layered with other controls. It is not a security boundary.

**What it defends against:** Honest agents that want to follow policy. Accidental over-permissioning. Maintaining consistent policies across teams. In OWASP's Top 10 for Agentic Applications (Dec 2025), this maps to ASI02 (Tool Misuse), ASI03 (Privilege Escalation), ASI07 (Improper Output Handling), and ASI09 (Logging Deficiencies). See [Section 6.3](#63-infrastructure--standards-validation) for the full mapping.

**What it does NOT defend against:** A determined adversary with shell access, sophisticated prompt injection, or OS-level attacks. Use sandboxes for those.

See [Appendix B](#appendix-b-security-deep-dive) for the full threat model, bypass vectors, and hardening options.

### 7.4 Key Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| **Platform builds native solution** | High | High | Position as open standard; plan for graceful sunset |
| **Classification DB has critical error** | Medium | Critical | Tier-down protection; mandatory review; CI validation |
| **Maintainer burnout** | High | High | Recruit second maintainer; automate DB updates via CI |
| **Adoption stalls** | Medium | High | Focus on Claude Code; ship migration tooling |

### 7.5 Comparison with Alternatives

| Factor | agent-perms | Glob Allowlists | OS Sandbox | Dyad Hooks |
|--------|------------|----------------|------------|------------|
| Semantic awareness | Yes | No | No | Yes (per-CLI) |
| Config complexity | Low (1–5 rules) | High (50+) | Medium | Medium |
| Enforcement level | Instruction (+hooks) | Config-level | OS kernel | Hook-level |
| Cross-agent | Yes | Per-agent | Per-platform | Claude Code only |
| Latency | 20-80ms cold | None | Minimal | ms / 5-25s (LLM) |
| Maintenance | Central DB (monthly) | Per-user per-CLI | Low | Per-CLI scripts |

---

## 8. Project Strategy

### 8.1 License & Open Source

**Apache 2.0** for the binary and classification DB. The classification DB is the primary asset — open-source and forkable. The goal is to be the upstream that forks track.

### 8.2 Sustainability

agent-perms is an **infrastructure commons** project, not a commercial product. Sustainability models:

1. **Platform adoption (primary goal):** If a platform integrates classification natively, the project becomes the upstream DB or sunsets gracefully.
2. **Foundation sponsorship:** OpenSSF or Linux Foundation if adoption warrants.
3. **Corporate sponsorship:** GitHub Sponsors / Open Collective for maintenance.

No paid tier, no enterprise edition, no feature gating.

### 8.3 Competitive Response

- **If a platform builds native support:** Validates the thesis. Focus on remaining platforms and cross-platform consistency.
- **If platforms build incompatible systems:** agent-perms becomes the Rosetta Stone.
- **If no platform builds it:** agent-perms remains the only option.

In all scenarios, the classification DB retains value as a community-maintained taxonomy of command intent.

---

## 9. FAQ

**Q: Is agent-perms a security tool?**
A: No. It's an ergonomics tool that *complements* actual security boundaries. It does not prevent a determined adversary from bypassing it.

**Q: What happens if a command is misclassified?**
A: False denials produce an actionable error with the correct tier. False permits are backstopped by sandbox and permission layers. Critical fixes ship as patch releases.

**Q: Can I add support for a new CLI?**
A: Yes — add a Go classifier in `internal/classify/` and follow the checklist in CLAUDE.md.

**Q: What if I disagree with a classification?**
A: Use `[overrides]` in `agent-perms.toml`. If the canonical classification is wrong, open an issue.

**Q: Does it phone home?**
A: No. Fully offline. No telemetry, no network calls except explicit `agent-perms update`.

**Q: How is this different from hooks?**
A: Hooks require you to write and maintain classification logic per CLI. agent-perms packages that as a shared, tested, versioned database.

**Q: Why does the agent need to call `explain` before `exec`? Can't it just guess?**
A: LLMs cannot reliably predict tiers for ambiguous commands (`gh api` → admin), flag-modified tiers (`git push --force` → admin), or subcommand variants (`git stash list` → read vs `git stash drop` → admin). `explain` gives a deterministic answer in one fast call. Without it, the agent guesses, gets denied, parses the error, and retries — wasting an LLM planning cycle that costs far more than the `explain` call. For simple commands like `gh pr list` or `git log`, the agent can skip `explain` and call `exec` directly.

**Q: Will it slow down my agent?**
A: ~20-80ms per invocation. With the recommended explain-then-exec flow, that's two calls per command (~40-160ms), or ~1.6-6.4s over a 40-command session. This is still well below the cost of a single LLM re-planning cycle when a wrong-tier guess triggers a denial and retry.

**Q: What if the agent skips the prefix?**
A: Defense-in-depth: hooks catch unmediated calls, deny rules block direct invocation, OS sandboxes provide a backstop.

---

## 10. Conclusion

The agent permissions problem is universal, growing, and currently addressed by ad-hoc solutions. Every platform is independently reinventing command classification—in prompt rules, regex hooks, LLM classifiers, or sandbox policies.

agent-perms proposes a clean extraction: package the semantic knowledge of what CLI commands do as a shared, deterministic, fast utility any agent can use. It doesn't replace OS sandboxing or agent configs. It gives those systems a higher-level vocabulary: instead of matching command strings, match permission intents.

The ecosystem evidence in Section 6 — from GitHub's infrastructure-level enforcement to community-built classification hooks — confirms that read/write/admin is the right abstraction. agent-perms makes it reusable, portable, and maintainable.

> **Current state:** MVP is built with `gh`, `git`, `go`, and `pulumi` classifiers. One-command setup via `agent-perms claude init` and `agent-perms codex init` with three built-in profiles (`read`, `write-local`, `full-write`). See README.md for setup and usage.

---

# Appendices

---

## Appendix A: Technical Specification

### A.1 Permission Tiers

#### Default Tiers (most CLIs)

The core `read`/`write`/`admin` tiers are defined in [Section 3.1](#31-core-concept). In addition:

| Tier | Intent | Examples |
|------|--------|----------|
| **read sensitive** | Read that exposes secrets | `pulumi env open`, `gh auth token` |
| **unknown** | CLI not supported | Any unsupported tool |

Tiers use exact match — the claimed tier must match the required tier exactly. There is no hierarchy: `write` does not allow `read`, and `read` does not allow `read sensitive`. Each tier is independent.

#### Git-Specific Tiers

Git and Pulumi have a clear distinction between local and remote operations. agent-perms introduces sub-tiers:

| Tier | Intent | Git Examples |
|------|--------|-------------|
| **read** | Inspect state, no mutations | `status`, `log`, `diff`, `branch --list`, `show` |
| **write:local** | Mutate local repo only | `add`, `commit`, `checkout`, `merge`, `rebase`, `stash`, `tag` (local) |
| **write:remote** | Modify remote repositories | `push`, `remote add/remove/set-url`, `fetch`, `pull`, `clone` |
| **admin:local** | Destructive local operations | `reset --hard`, `clean -fd` |
| **admin:remote** | Destructive remote operations | `push --force`, `filter-branch` + push |

**Why `push --force` is `admin:remote`:** A regular `push` is `write:remote`. Flag-aware classification elevates `push --force` to `admin:remote`.

### A.2 Classification Database

For each supported CLI, agent-perms maintains command→tier mappings and optional sub-tier definitions. Classifiers are implemented as Go functions in `internal/classify/` and compiled into the binary.

**Integrity constraints:**
- Releases signed with Sigstore/cosign. `doctor` verifies integrity.
- DB-only updates signed against a public key embedded in the binary at build time.
- **Tier-down protection:** Lowering a command's tier requires explicit maintainer approval.
- **Rollback protection:** Monotonic version counter. Binary refuses older DB versions.
- **Transparency log:** Updates recorded in Sigstore's Rekor log.

### A.3 Exec Passthrough

The `exec` command is transparent: it connects stdin/stdout/stderr directly to the child process, propagates the child's exit code, and adds no buffering or transformation.

#### Critical: `execvp()` Semantics (No Shell)

**`exec` MUST invoke the target CLI via `execvp()` (direct argv invocation), NOT via `sh -c`.** This protects the boundary between agent-perms and the target CLI. With `execvp()`, arguments are passed as an argv array, so shell metacharacters within arguments are treated as literal strings.

**Important caveat:** `execvp()` protects against injection *within* the arguments passed to agent-perms. It does NOT protect against shell-level command composition *around* agent-perms. When Claude Code runs `bash -c "agent-perms exec read -- gh pr list ; gh repo delete myrepo"`, the semicolon is parsed by bash *before* agent-perms runs, resulting in two separate commands. The hook enforcement (Appendix B) addresses this outer boundary.

#### Argument Delimiter (`--`)

`execvp()` does not prevent **argument injection** — the target CLI's own argument parser still interprets flags. The flag denylist (Section A.6) catches known-dangerous flags. As defense-in-depth, agent-perms inserts `--` before user-supplied positional arguments where the target CLI supports it:

```
execvp("git", ["git", "clone", "--", "https://repo.url", "/path"])
```

The `--` delimiter primarily prevents positional arguments from being interpreted as flags (e.g., a repo named `--exec`). The classification DB records per-command `--` support.

#### Concurrency Safety

- **Audit log:** Append-only writes with `O_APPEND` flag, atomic for writes under `PIPE_BUF`.
- **DB updates:** Atomic file replacement (write to tempfile, `rename(2)`).
- **Config writes:** `agent-perms init` uses atomic file replacement.

#### Environment Variables

Certain environment variables can turn "read" commands into code execution:

| Variable | Risk |
|----------|------|
| `LD_PRELOAD` | Loads arbitrary shared libraries into any process |
| `GIT_SSH_COMMAND` | Executes arbitrary code on any git remote operation |
| `GIT_EXTERNAL_DIFF` | Executes on `git diff` (classified as read) |
| `PAGER` / `GIT_PAGER` | Executes when git pages output |

**Mitigation:** `--clean-env` restricts the child to safe variables (`PATH`, `HOME`, `USER`, `TERM`, `LANG`, `LC_*`, `SSH_AUTH_SOCK`, `DISPLAY`, `COLORTERM`).

### A.4 Error Handling and Exit Codes

| Exit Code | Meaning | Stderr Output |
|-----------|---------|---------------|
| 0 | Success | *(none)* |
| 1 | Permission denied | JSON: `{"error":"denied","claimed":"read","required":"write","command":"gh pr create","suggestion":"agent-perms exec write -- gh pr create"}` |
| 2 | Unknown CLI | JSON: `{"error":"unknown","cli":"sometool"}` |
| 3 | Internal error | JSON: `{"error":"internal","message":"..."}` |
| 10+ | Child exit code + 10 | *(child's stderr passes through)* |

Denial messages include the **correct tier** and a **suggestion** command so the agent immediately knows how to proceed.

**Exit code 10+ offset:** The offset avoids collision between agent-perms' own error codes (1-3) and child process exit codes. On POSIX systems, exit codes are modulo 256 — child exit codes >245 will wrap. For precise child exit code reporting in edge cases, use the audit log or stderr JSON.

### A.5 Known Classification Challenges

| Command | Challenge | MVP Classification | Rationale |
|---------|-----------|-------------------|-----------|
| `gh api` | Raw HTTP client | **admin** | Cannot classify without parsing HTTP method |
| `docker exec` | Arbitrary commands in container | **admin** | Equivalent to arbitrary code execution |
| `kubectl exec` | Same as docker exec | **admin** | Same rationale |
| `git stash` | `list` is read, `drop` is destructive | **write:local** (`drop` → admin:local) | Requires second-level parsing |
| `git config` | `--get` is read, `--global` mutates | **write:local** (`--get` → read) | Flag-aware in MVP |
| `npm install` | Runs arbitrary postinstall scripts | **write** | Common but genuinely mutates |

### A.6 MVP Flag Denylist

| CLI | Flag | Effect | MVP Behavior |
|-----|------|--------|-------------|
| git | `--hard` (on reset) | Destroys uncommitted work | Elevates to **admin** |
| git | `--force` / `-f` (on push) | Overwrites remote history | Elevates to **admin** |
| git | `--upload-pack` (on fetch/clone) | Arbitrary code execution (CVE-2025-21613) | **Denied** |
| git | `--receive-pack` (on push) | Same class as `--upload-pack` | **Denied** |
| git | `-c` / `--config` (global) | Many config keys execute external programs | **Denied** unconditionally |
| git | `--exec` | Executes commands during rebase | **Denied** |
| git | `--output` (on log/format-patch) | Writes to arbitrary file paths | Elevates to **write:local** |
| git | `--work-tree` | Redirects to arbitrary directory | Elevates to **admin** |
| git | `--git-dir` | Points at arbitrary .git directory | Elevates to **admin** |
| gh | `--dry-run` (on pr merge) | Makes write commands read-only | Demotes to **read** |
| kubectl | `--dry-run=client` | Makes write commands read-only | Demotes to **read** |
| kubectl | `--kubeconfig` | Arbitrary kubeconfig | Elevates to **admin** |
| kubectl | `-f` with URL | Remote manifest fetch | Elevates to **admin** |
| docker | `--privileged` (on run) | Full host access | Elevates to **admin** |
| docker | `-v` / `--volume` (on run) | Mounts arbitrary host paths | Elevates to **admin** |

Design principle: **"when in doubt, deny."**

### A.7 Audit Logging *(not yet implemented)*

Opt-in via `audit_log` in `agent-perms.toml` (Section 5.3) or environment variable (overrides config file):

```bash
export AGENT_PERMS_AUDIT_LOG=/var/log/agent-perms.jsonl
```

```json
{"ts":"2026-02-27T10:15:32Z","action":"exec","cli":"gh","command":"pr list","claimed":"read","actual":"read","result":"allowed","pid":12345,"cwd":"/home/user/myproject"}
```

Fields: `ts`, `action`, `cli`, `command`, `argv`, `claimed`, `actual`, `result`, `pid`, `cwd`. Command arguments are JSON-escaped to prevent log injection.

---

## Appendix B: Security Deep Dive

### B.1 Bypass Vectors

| Vector | How It Works | Mitigation |
|--------|-------------|------------|
| **Direct CLI invocation** | Agent calls `gh` directly | Hooks + deny rules (B.2–B.3) |
| **Shell wrapper commands** | `sh -c "gh repo delete"` | Classify wrappers as `admin` |
| **Alias/function shadowing** | `alias agent-perms=true` | Hooks use absolute path |
| **PATH manipulation** | Fake `agent-perms` in PATH | Hooks use absolute path |
| **Environment variable injection** | `LD_PRELOAD`, `GIT_SSH_COMMAND` | `--clean-env` flag |
| **Symlink attacks** | Unrecognized CLI → fallback | `--on-unknown=deny` |
| **Target CLI PATH shadowing** | Trojan `gh` earlier in PATH | `--clean-env` with restricted PATH |

### B.2 PreToolUse Hook Enforcement (Claude Code)

```bash
#!/bin/bash
# .claude/hooks/enforce-agent-perms.sh
json_input=$(cat)
command=$(echo "$json_input" | jq -r '.tool_input.command // empty')

if [[ "$command" == agent-perms\ * ]]; then
  exit 0  # passthrough
fi

for cli in gh git docker kubectl; do
  if [[ "$command" == ${cli}\ * ]]; then
    echo '{"permissionDecision":"deny","permissionDecisionReason":"Use agent-perms exec <tier> -- '"$command"'"}'
    exit 0
  fi
done

exit 0
```

**Caveats:** Hooks are not a security boundary — prompt injection can work around them. For enterprise, distribute via managed settings (`allowManagedHooksOnly`). CVE-2025-59536 demonstrated hooks from untrusted repos can execute automatically.

### B.3 Deny Rules

```json
{
  "permissions": {
    "deny": ["Bash(gh *)", "Bash(git push *)", "Bash(docker push *)"],
    "allow": ["Bash(agent-perms explain *)", "Bash(agent-perms exec read -- *)", "Bash(agent-perms exec write local -- git *)"]
  }
}
```

**Important caveats:**
- Deny rule enforcement has had vulnerabilities: CVE-2026-25724 (symlink bypass), Issue #6699 (non-functional in v1.0.93).
- Sub-agents via Task tool may bypass deny rules (Issue #25000).
- **Hooks provide a more reliable enforcement path.** For enterprise, use `managed-settings.json`.

### B.4 MCP Security Requirements *(not yet implemented)*

If agent-perms ships as an MCP server:

- **Transport:** Unix socket or stdio only. No TCP (DNS rebinding risk).
- **Authentication:** Per-session token, no long-lived API keys.
- **Tool descriptions:** Static and reviewed. Never dynamically generated from DB content.
- **Input handling:** Arguments passed as individual argv elements to `execvp()`, never concatenated (CVE-2025-53107).
- **Cross-server isolation:** Document shadowing risk from malicious co-resident MCP servers.

### B.5 Update Security

- **Signatures:** Verified against public key embedded in binary. Never fetched from update server.
- **Transparency log:** DB updates in Sigstore's Rekor log. Never skip tlog verification (CVE-2026-24122).
- **Rollback protection:** Monotonic version counter.

---

## Appendix C: Agent Platform Deep Dives

### C.1 Claude Code

**Integration quality: Excellent.** Glob-based `Bash()` rules are purpose-built for this pattern.

**Permission model (current as of March 2026):** Claude Code uses a three-tier rule system (`allow`, `ask`, `deny`) with `deny -> ask -> allow` precedence. It supports five permission modes: `default` (prompts on first use), `acceptEdits` (auto-approves file edits), `plan` (read-only analysis), `dontAsk` (auto-denies unless pre-approved — most complementary with agent-perms), and `bypassPermissions` (skips all checks, isolated environments only). Tool-specific rules cover Bash, Read, Edit, WebFetch, MCP, and Agent subagents individually. OS-level sandboxing (Seatbelt on macOS, bubblewrap on Linux) complements permissions as a defense-in-depth layer. PreToolUse hooks can approve, deny, or modify tool calls based on custom logic — agent-perms can serve as the classification engine behind these hooks.

**Setup:** Run `agent-perms claude init` to generate settings with allow/deny rules and a SessionStart hook:

```console
$ agent-perms claude init                        # interactive profile selection
$ agent-perms claude init --profile=write-local  # scripted
```

The generated settings include allow rules for `agent-perms exec` at the profile's tier level, deny rules blocking direct CLI access (`gh *`, `git *`, etc.), and a SessionStart hook that runs `agent-perms claude md` to inject usage instructions. If `~/.claude/settings.json` exists, rules are merged automatically.

Validate with `agent-perms claude validate`.

**Allowlist note:** The `explain` subcommand is read-only and safe to auto-approve.

**Settings precedence:** Managed > local > project > user. Organizations can enforce agent-perms at managed level. `*` wildcard only matches at the end — fine for agent-perms since variable parts are always rightmost.

#### Alternative: Exec-First Pattern

Instead of always calling `explain` before `exec`, the agent can attempt `exec` directly with a guessed tier and fall back on denial:

```markdown
## CLI Commands (exec-first variant)
When running CLI commands, use agent-perms exec directly:
1. Guess the most likely tier (e.g., `read` for queries, `write local` for mutations)
2. Run `agent-perms exec <guessed-tier> -- <command>`
3. If exec returns a tier mismatch error, read the correct tier from stderr and retry
```

**Pros:**
- **Faster on the happy path.** One LLM round-trip per command instead of two. Over a session with many commands, this saves meaningful wall-clock time (each avoided round-trip saves ~1-5s of model inference).
- **Simpler agent instructions.** No two-step dance; the agent just runs commands.
- **Self-correcting.** Denial stderr includes the correct tier, so the retry is deterministic.

**Cons:**
- **Wasted exec on wrong guesses.** A misclassified `exec` triggers a denial + retry, costing more than the `explain` call would have. If the agent guesses wrong >~10% of the time, the latency savings disappear.
- **Noisier audit logs.** Denied-then-retried commands appear as two entries (one denial, one success), complicating log analysis.
- **Requires robust error parsing.** The agent must reliably parse stderr to extract the correct tier. `explain` returns clean structured output purpose-built for this.
- **Allowlist complexity.** The agent platform sees the guessed tier in the command string. A wrong guess that hits a `deny` rule (e.g., guessing `admin` when the command is actually `write:local`) could produce confusing denials at the platform level, not just the agent-perms level.

**Recommendation:** Use the two-step explain-then-exec pattern as the default. The `explain` call is cheap and auto-approved, and it guarantees correctness on every call. The exec-first pattern is worth evaluating for latency-sensitive scenarios where the agent has high confidence in tier guesses (e.g., a small set of well-known commands). Both patterns can coexist — an agent could skip `explain` for commands it knows well and use it for unfamiliar or flagged commands.

### C.2 OpenAI Codex

**Integration quality: Good, with caveats.** Three-mode system (Read Only, Auto, Full Access). Integration via AGENTS.md and execpolicy Starlark rules (`~/.codex/rules/`).

**Setup:** Run `agent-perms codex init` to generate both rule and instruction files:

```console
$ agent-perms codex init                             # interactive profile selection + write
$ agent-perms codex init --profile=write-local       # scripted
```

This creates `~/.codex/rules/agent-perms.rules` (Starlark `prefix_rule()` entries) and `~/.codex/AGENTS.md` (usage instructions). The generated rules allow `agent-perms exec` at the profile's tier level and forbid direct CLI access. Codex applies the most restrictive decision when multiple rules match (`forbidden > prompt > allow`).

Validate with `agent-perms codex validate`. Print rules to stdout (without writing) by omitting `--write`.

Codex intelligently splits compound commands (`&&`, `||`, `;`, `|`) and evaluates each part separately, preventing bypass via chaining. Smart approvals (enabled by default) propose `prefix_rule` entries during escalation, so rules accumulate as users approve commands.

**Hooks progress:** A comprehensive lifecycle hooks system is under active development (PR #11067). Currently shipped: `prepare-commit-msg` hook only. Enforcement remains instruction-based for now.

### C.3 Gemini CLI

**Integration quality: Good.** Can restrict all shell commands to agent-perms via `tools.core`:

```json
{ "tools": { "core": ["ReadFileTool", "GlobTool", "ShellTool(agent-perms)"] } }
```

Strongest enforcement in any current platform.

### C.4 Cursor

**Integration quality: Moderate.** Sandbox-first model. No command-level allowlists. Integration via `.cursor/rules/` and `beforeShellExecution` hooks.

### C.5 CI/CD Usage

```yaml
- name: Run agent with classification enforcement
  env:
    AGENT_PERMS_ON_UNKNOWN: deny
    AGENT_PERMS_AUDIT_LOG: ${{ runner.temp }}/agent-perms.jsonl
    AGENT_PERMS_CLEAN_ENV: "true"
  run: |
    claude -p "Fix the failing test" \
      --allowedTools "Bash(agent-perms explain *)" \
                     "Bash(agent-perms exec read -- *)" \
                     "Bash(agent-perms exec write local -- git *)"
```

**Recommended CI defaults:** `--on-unknown=deny`, `--clean-env`, audit logging enabled. `agent-perms init --profile ci` generates these.

---

## Appendix D: Extensibility & Future Roadmap

### D.1 Supported CLIs: Expansion Path

| Priority | CLI | Notes |
|----------|-----|-------|
| ~~P0~~ | ~~gh, git~~ | **Implemented.** Git has local/remote sub-tiers |
| ~~P0~~ | ~~go, pulumi~~ | **Implemented.** Go uses flat tiers; Pulumi has local/remote sub-tiers |
| P1 | docker/compose, kubectl | push/pull have remote implications |
| P2 | aws/gcloud/az, npm/yarn/pnpm | Massive surface area |

**Shell wrappers** (`sh`, `bash`, `env`, `python`, `node`, `ssh`, `find` with `-exec`) classified as `admin`.

### D.2 MCP Server Mode *(not yet implemented)*

MCP would enable agent auto-discovery without CLAUDE.md. The MCP ecosystem has 5,800+ servers, 300+ clients, and 97M monthly SDK downloads.

### D.3 Community Registry *(not yet implemented)*

Public GitHub repo of classification manifests per CLI.

### D.4 Future Configuration (Post-MVP)

Per-CLI overrides are the only committed customization feature. Everything below is gated by user demand:

- **CLI Groups** — Named groups as init-time shorthand. Config-generation convenience, not runtime concept.
- **Sub-Tiers for Docker/kubectl** — Probably unnecessary if simple overrides work.
- **Named Profiles** — Flat presets only, no inheritance.
- **Context-Aware Overrides** — Flag-presence-based tier elevation. Highest complexity, lowest priority.

**Deliberate exclusions (permanent):** No runtime conditionals. No LLM fallback. No per-directory policies. No profile inheritance. No policy language. agent-perms classifies commands — the agent platform enforces policy.

### D.5 PreToolUse Hook Integration *(not yet implemented)*

Claude Code's [PreToolUse hooks](https://code.claude.com/docs/en/hooks) can use
agent-perms as a classification engine — an alternative to glob-pattern rules
where the hook calls agent-perms and approves or blocks based on the classified
tier.

Planned interface:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "agent-perms hook --profile=write-local"
          }
        ]
      }
    ]
  }
}
```

The `agent-perms hook` subcommand would classify the command's tier and compare
it against the profile. Commands within the profile's allowed tiers pass through;
commands outside the profile are blocked with an explanation of the required tier.

Advantages over glob patterns:
- **Flag-aware classification** — `git push` and `git push --force` are distinguished automatically
- **No per-subcommand rules** — the classification DB handles all supported CLIs
- **Simpler config** — one hook entry replaces dozens of `Bash()` allow/deny rules

This positions agent-perms as the classification engine *behind* hooks rather
than a replacement for allowlists. A short hook is simpler and more maintainable
than custom regex classifiers (cf. Dyad's 627-line Python hook, kornysietsma's
Rust hook).

### D.6 Governance & Maintenance

- **Two maintainers minimum** (bus factor > 1)
- **PR-based contribution** with CI validation against CLI help output
- **48-hour review SLA**
- **Tier-down protection:** Two-maintainer approval
- **Monthly releases**
- **Semantic versioning:** 0.x during validation, 1.0 when gh + git are stable

---

## Appendix E: Project Planning

*Removed — the original phased timeline is superseded by the working MVP. See `docs/new-clis-to-add.md` for planned CLI additions.*

### E.2 Success Metrics

**Alpha:** >98% classification accuracy. <100ms p99 latency. <5 min setup. 0 false denials.

**Beta:** >80% allowlist rule reduction. >50 weekly active users. >5 external classification PRs.

**1.0:** >10 supported CLIs. >1 platform integration. >1,000 monthly downloads.
