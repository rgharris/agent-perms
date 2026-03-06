# agent-perms: Codex CLI Settings Examples *(experimental)*

> **Quick start:** Run `agent-perms codex init --write` to generate both
> `~/.codex/rules/agent-perms.rules` and `~/.codex/AGENTS.md` in one step.
> Use `--profile=write-local` or `--profile=full-write` for more permissive defaults.
>
> Run `agent-perms codex validate` to check your rules for common issues.

> **Experimental:** Codex support works for core classification and exec
> enforcement, but Codex's permission model differs significantly from Claude
> Code's and has not been tested as extensively. Please report issues.

These examples show how to configure Codex CLI's exec policy to use
agent-perms as a permission layer for `gh`, `git`, `pulumi`, `go`, and other CLIs.

---

## How It Works

Instead of writing individual `prefix_rule()` entries for every CLI subcommand,
you allow `agent-perms exec <action> <scope> --` patterns. Codex enforces
the exec policy match — if Codex tries to run `gh repo delete` directly, the
`forbidden` rule for `gh` blocks it. `agent-perms exec` itself enforces tier
semantics independently as a second check.

The claimed tier must exactly match what the command requires. `read` does not
cover `read-sensitive`, and `write` does not cover `read`. Each tier is independent.

One set of rules replaces dozens of individual CLI allowlists.

---

## Sandbox Mode Interaction

Codex has two security layers:

1. **Sandbox mode** (`sandbox_mode` in config.toml) — OS-level filesystem/network restrictions
2. **Exec policy** (`.rules` files) — command-level allow/prompt/forbidden decisions

agent-perms operates at the exec policy layer. The sandbox still applies independently.
For example, even if agent-perms allows `git push`, the sandbox in `read-only` mode
would still block network access.

> **Migration note:** If your config uses `approval_policy = "on-failure"`, note
> that this option is deprecated. The Codex CLI reference recommends `on-request`
> for interactive runs or `never` for non-interactive/CI runs.

Recommended sandbox modes per profile:

| agent-perms profile | Recommended sandbox_mode |
|---------------------|--------------------------|
| `read` | `read-only` |
| `write-local` | `workspace-write` |
| `full-write` | `workspace-write` |

### Enabling network access

The `workspace-write` sandbox blocks network access by default. Many agent-perms
commands need the network (`gh` API calls, `git push`, `pulumi up`), so you must
opt in. Add this to your `~/.codex/config.toml`:

```toml
[sandbox_workspace_write]
network_access = true
```

> **Note:** On macOS, `network_access = true` may be silently ignored by the
> Seatbelt sandbox ([issue #10390](https://github.com/openai/codex/issues/10390)).
> On Linux (Landlock sandbox), it works as expected. If you hit network errors
> on macOS, use `--sandbox danger-full-access` as a workaround.

---

## Common Profiles

### Read-only (safest)

Codex can run any read command automatically. Write commands prompt, admin is forbidden.

```
agent-perms codex init --profile=read > ~/.codex/rules/agent-perms.rules
```

Generated rules:

```starlark
# Allow all read operations through agent-perms
prefix_rule(
    pattern = ["agent-perms", "exec", "read", "local", "--"],
    decision = "allow",
)
prefix_rule(
    pattern = ["agent-perms", "exec", "read", "remote", "--"],
    decision = "allow",
)

# Prompt for sensitive read operations (commands that expose secrets)
prefix_rule(
    pattern = ["agent-perms", "exec", "read-sensitive"],
    decision = "prompt",
)

# Prompt for write operations
prefix_rule(
    pattern = ["agent-perms", "exec", "write"],
    decision = "prompt",
)

# Block admin operations
prefix_rule(
    pattern = ["agent-perms", "exec", "admin"],
    decision = "forbidden",
)

# Allow explain and version (informational, always safe)
prefix_rule(
    pattern = ["agent-perms", "explain"],
    decision = "allow",
)
prefix_rule(
    pattern = ["agent-perms", "version"],
    decision = "allow",
)

# Block direct CLI access
prefix_rule(pattern = ["gh"], decision = "forbidden")
prefix_rule(pattern = ["git"], decision = "forbidden")
prefix_rule(pattern = ["go"], decision = "forbidden")
prefix_rule(pattern = ["pulumi"], decision = "forbidden")
```

### Local development

Codex can read and write locally (commits, formatting). Remote operations prompt.

```
agent-perms codex init --profile=write-local > ~/.codex/rules/agent-perms.rules
```

Key differences from read:
- `exec write local` → `allow` (git commit, go fmt, pulumi config, etc.)
- `exec write remote` → `prompt` (git push, gh pr create, pulumi up, etc.)

### Full write

Codex can read and write everything (local and remote). Only admin operations are blocked.

```
agent-perms codex init --profile=full-write > ~/.codex/rules/agent-perms.rules
```

---

## Pairing with Codex Sandbox Settings

Each agent-perms profile pairs well with a specific Codex sandbox configuration:

### Balanced local development (recommended)

```toml
sandbox_mode = "workspace-write"
approval_policy = "on-request"

[sandbox_workspace_write]
network_access = true
```

Use with `agent-perms codex init --profile=write-local`.

### Strict review / planning mode

```toml
sandbox_mode = "read-only"
approval_policy = "on-request"
```

Use with `agent-perms codex init --profile=read`. In this mode, Codex can only
run read-tier commands automatically. Write operations prompt via exec policy,
and the sandbox independently blocks filesystem mutations.

> **Note:** `approval_policy = "untrusted"` restricts to safe reads without
> prompting for untrusted operations (it does not mean "prompt for everything").
> Use `on-request` if you want interactive prompts for escalation.

---

## Rule Precedence

Codex applies the most restrictive decision when multiple rules match:

    forbidden > prompt > allow

This means the `forbidden` rules for direct CLI access and admin operations
always take priority, even if a broader `allow` rule also matches.

---

## AGENTS.md Instructions

If you used `--write`, `~/.codex/AGENTS.md` is already set up. To update it
manually or write to a different location:

```
agent-perms codex md >> ~/.codex/AGENTS.md
```

This tells Codex how to route CLI commands through agent-perms with the correct
action and scope for each supported CLI.

---

## Verifying Rules

Check that your rules file has no issues:

```console
$ agent-perms codex validate ~/.codex/rules/agent-perms.rules
```

Test how Codex will classify a specific command:

```console
$ codex execpolicy check --pretty \
    --rules ~/.codex/rules/agent-perms.rules \
    -- agent-perms exec read remote -- gh pr list
```

Use `agent-perms explain` to check what tier a command requires:

```console
$ agent-perms explain gh secret set TOKEN
cli:        gh
command:    secret set
base_tier:  admin remote (gh secret set)
result:     admin remote
```

---

## File Locations

| File | Purpose |
|------|---------|
| `~/.codex/rules/agent-perms.rules` | Exec policy rules (generated by `codex init`) |
| `~/.codex/AGENTS.md` | Global agent instructions (generated by `codex init --write` or `codex md`) |
| `~/.codex/config.toml` | Codex global config (sandbox mode, model, etc.) |
