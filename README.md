# agent-perms

**Fine-grained, tiered permissions for AI agent CLI access.**

AI coding agents let you allowlist shell commands so they run without prompting you for permission. But those allowlists are difficult to maintain, encouraging overly permissive wildcards like `git *` that give the agent the same access to `gh repo delete` as to `gh pr list`. There's no way to distinguish `git reset --soft` from `git reset --hard`, or a safe `gh api` GET from a destructive DELETE.

`agent-perms` adds a semantic layer between your agent and your CLIs. You define which _tiers_ run automatically, and the agent declares its intent upfront. One rule replaces dozens of individual command allowlists.

```sh
# Classified as "read remote" — auto-approved
agent-perms exec read remote -- gh pr list

# Classified as "read-sensitive remote" — prompts (exposes secrets)
agent-perms exec read-sensitive remote -- pulumi env open myorg/prod

# Classified as "admin remote" — prompts you
agent-perms exec admin remote -- gh repo delete my-repo

# Claimed tier must match exactly — wrong tier is denied:
# ERROR: denied. 'gh repo delete my-repo' requires 'admin remote', claimed 'read remote'.
agent-perms exec read remote -- gh repo delete my-repo
```

### How agents know to use agent-perms

Agents don't discover `agent-perms` on their own — you tell them. The `init` commands for each agent platform inject instructions into the agent's context asking it to wrap CLI commands with `agent-perms exec`:

- **Claude Code**: `agent-perms claude init` adds a `SessionStart` hook that runs `agent-perms claude md`, which injects usage instructions (the `agent-perms exec` syntax and examples for each CLI) into the session context at startup.
- **Codex CLI**: `agent-perms codex init` writes an `AGENTS.md` file (loaded automatically by Codex) with the same usage instructions.

In both cases, the agent sees instructions like "run CLI commands through `agent-perms exec <action> <scope> -- <cli> <subcommand>`" and follows them. The permission rules then ensure the agent _can't_ bypass `agent-perms` by calling CLIs directly — those commands are denied.

### Why agent allowlists aren't enough

Agent allowlists match on the command string — they have no visibility into flags. This makes several common cases impossible to handle safely:

| Command                                     | Problem                                                                                            |
| ------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| `git reset --hard`                          | Can't distinguish `--soft` (safe) from `--hard` (destroys uncommitted work)                        |
| `git push --force`                          | Can't distinguish a normal push from one that rewrites remote history                              |
| `gh api --method DELETE /repos/…`           | `gh api` is a raw HTTP client — the method flag determines whether it reads or permanently deletes |
| `git config --get` vs `git config --global` | Same subcommand, opposite risk: one reads a value, the other mutates system-wide config            |

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

One command generates exec policy rules and AGENTS.md:

```console
$ agent-perms codex init
```

You'll be prompted to choose a profile and confirm writing. This creates `~/.codex/rules/agent-perms.rules` and `~/.codex/AGENTS.md`. See [Codex CLI setup](#codex-cli) for details.

---

## Checking Tiers

Use `agent-perms explain` to see how any command is classified:

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
| `read`         | Read all CLIs (not sensitive), writes prompt, deny admin                            |
| `write-local`  | Read + local writes (git commit, go fmt, etc.), remote writes prompt, deny admin, sensitive prompts |
| `full-write`   | Read + write all CLIs (including remote), deny admin, sensitive prompts              |

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

Claude Code's glob match (`Bash(agent-perms exec read remote -- gh *)`) is the outer gate — if Claude tries to skip `agent-perms` and run `gh repo delete` directly, the deny rule blocks it. `agent-perms exec` is the inner gate, enforcing tier semantics independently.

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
