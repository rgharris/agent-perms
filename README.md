# agent-perms

**Fine-grained, tiered permissions for AI agent CLI access.**

`agent-perms` makes agent automation predictable and safe to scale across teams. Instead of approving brittle command patterns, you approve semantic tiers (`read`, `write`, `admin` across `local`/`remote`). Safe work runs automatically, risky work prompts, and dangerous operations stay blocked by policy.

At a glance:

- Deterministic classification in Go using CLI + subcommand + flags (for example, `git push` vs `git push --force`)
- Two-layer enforcement: agents are instructed to use `agent-perms`, and direct CLI access is denied by outer rules
- Exact tier matching: if an agent claims the wrong tier, the command is denied with the required tier
- One-time setup for Claude Code and Codex (`agent-perms <platform> init`)

AI coding agents let you allowlist shell commands so they run without prompting. Those allowlists are hard to maintain and encourage permissive wildcards like `gh *`, even though agents frequently use `gh api` for routine reads (PR comments, reviews, metadata) and the same command can also mutate or delete data based on flags. They also cannot distinguish `git reset --soft` from `git reset --hard`, or a safe `gh api` GET from a destructive DELETE.

`agent-perms` adds a semantic layer between your agent and your CLIs. You define which _tiers_ run automatically, and the agent declares its intent upfront. One rule replaces dozens of individual command allowlists.

```sh
# Classified as "read remote" — auto-approved
agent-perms exec read remote -- gh pr list

# Classified as "read-sensitive remote" — prompts (exposes secrets)
agent-perms exec read-sensitive remote -- pulumi env open myorg/prod

# Classified as "admin remote" — prompts you
agent-perms exec admin remote -- gh api --method DELETE /repos/OWNER/REPO

# Claimed tier must match exactly — wrong tier is denied:
# ERROR: denied. 'gh api --method DELETE /repos/OWNER/REPO' requires 'admin remote', claimed 'read remote'.
agent-perms exec read remote -- gh api --method DELETE /repos/OWNER/REPO
```

### How agents know to use agent-perms

Agents do not discover `agent-perms` on their own. The `init` commands for each platform inject instructions that tell the agent to wrap CLI commands with `agent-perms exec`:

- **Claude Code**: `agent-perms claude init` adds a `SessionStart` hook that runs `agent-perms claude md`, injecting usage instructions (the `agent-perms exec` syntax and examples for each CLI) at session start.
- **Codex CLI**: `agent-perms codex init` writes an `AGENTS.md` file (loaded automatically by Codex) with the same instructions.

In both cases, the agent sees instructions like "run CLI commands through `agent-perms exec <action> <scope> -- <cli> <subcommand>`" and follows them. Permission rules then ensure the agent _cannot_ bypass `agent-perms` by calling CLIs directly; those commands are denied.

### Why agent allowlists aren't enough

Agent allowlists match command strings; they have no visibility into flags. This makes common cases impossible to handle safely:

| Command                                     | Problem                                                                                            |
| ------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| `git reset --hard`                          | Can't distinguish `--soft` (safe) from `--hard` (destroys uncommitted work)                        |
| `git push --force`                          | Can't distinguish a normal push from one that rewrites remote history                              |
| `gh api /repos/.../pulls/.../comments`      | Common read path for PR context, but command-string allowlists cannot separate it from write/delete API calls |
| `gh api --method DELETE /repos/…`           | Same `gh api` subcommand, but method/path can permanently delete data                              |
| `git config --get` vs `git config --global` | Same subcommand, opposite risk: one reads a value, the other mutates global config                  |

`agent-perms` classifies commands with full flag awareness, so `git reset` and `git reset --hard` land in different tiers. Your agent's allowlist rules stay simple; the semantic work happens inside `agent-perms`.

---

## Install

```sh
go install github.com/rgharris/agent-perms/cmd/agent-perms@main
```

Make sure `$(go env GOPATH)/bin` is on your `PATH`.

---

## Quick Setup

### Claude Code

One command generates a `settings.json` with permissions, deny rules, and a SessionStart hook:

```console
$ agent-perms claude init
```

You'll be prompted to choose a profile. If you already have a `~/.claude/settings.json`, rules are merged automatically. See [Claude Code setup](#claude-code) for details.

### Codex CLI

One command generates exec policy rules and `AGENTS.md`:

```console
$ agent-perms codex init
```

You'll be prompted to choose a profile and confirm writing. This creates `~/.codex/rules/agent-perms.rules` and `~/.codex/AGENTS.md`. See [Codex CLI setup](#codex-cli) for details.

---

## Checking Tiers

Use `agent-perms explain` to see how a command is classified:

```console
$ agent-perms explain gh secret set TOKEN
cli:        gh
command:    secret set
base_tier:  admin remote (gh secret set)
result:     admin remote

$ agent-perms explain git push --force
cli:        git
command:    push
base_tier:  write remote (git push)
flags:      --force → admin remote
result:     admin remote
```

---

## Profiles

Both Claude Code and Codex use the same three profiles:

| Profile        | Description                                                                        |
| -------------- | ---------------------------------------------------------------------------------- |
| `read`         | Read access for all CLIs (non-sensitive); writes prompt; admin denied                |
| `write-local`  | Read + local writes (git commit, go fmt, etc.); remote writes prompt; sensitive prompts; admin denied |
| `full-write`   | Read + write for all CLIs (including remote); sensitive prompts; admin denied        |

`write-local` is the recommended default profile for day-to-day development.

---

## Claude Code

### 1. Generate settings

```console
$ agent-perms claude init
```

If `~/.claude/settings.json` exists, rules are merged into it automatically. The generated settings include:

- **Permissions**: allow/deny rules that auto-approve the right tiers and block direct CLI access
- **SessionStart hook**: runs `agent-perms claude md` to inject usage instructions into every session

To merge into a specific file:

```console
$ agent-perms claude init --merge=~/.claude/settings.json
```

### 2. Validate

```console
$ agent-perms claude validate
```

### How it works

Claude Code's glob match (`Bash(agent-perms exec read remote -- gh *)`) is the outer gate. If Claude tries to skip `agent-perms` and run `gh api --method DELETE ...` directly, the deny rule blocks it. `agent-perms exec` is the inner gate, enforcing tier semantics independently.

See [`examples/claude-settings.md`](examples/claude-settings.md) for granular profiles, per-CLI rules, and mixing broad and fine-grained patterns.

---

## Codex CLI

### 1. Generate rules

```console
$ agent-perms codex init
```

This creates:

- `~/.codex/rules/agent-perms.rules` — Starlark `prefix_rule()` entries
- `~/.codex/AGENTS.md` — usage instructions for all supported CLIs

### 2. Enable network access

The `workspace-write` sandbox blocks network by default. Many commands need it (`gh`, `git push`, `pulumi up`). Add to `~/.codex/config.toml`:

```toml
[sandbox_workspace_write]
network_access = true
```

### 3. Validate

```console
$ agent-perms codex validate
```

### How it works

Codex applies the most restrictive decision when multiple rules match (`forbidden > prompt > allow`). The generated rules:

1. **Allow** agent-perms exec commands at the profile's tier level
2. **Forbid** direct CLI access (`gh`, `git`, `go`, `pulumi` without agent-perms)
3. **Forbid** admin operations in all profiles

The exec policy is the outer gate; `agent-perms exec` is the inner gate.

See [`examples/codex-settings.md`](examples/codex-settings.md) for profile details, sandbox mode interaction, and rule precedence.

---

## Docs

- [Claude Code settings examples](examples/claude-settings.md)
- [Codex CLI settings examples](examples/codex-settings.md)
- [Concept & future direction](docs/agent-perms-concept.md)
