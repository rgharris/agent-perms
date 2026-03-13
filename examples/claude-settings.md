# agent-perms: Claude Code Settings Examples

> **Quick start:** Run `agent-perms claude init` to generate a complete
> `settings.json` with recommended rules. You'll be prompted to choose a profile,
> or use `--profile=write-local` / `--profile=full-write` for scripting.
> Use `--merge=~/.claude/settings.json` to add agent-perms rules to an existing file.
>
> Run `agent-perms claude validate` to check your settings for common issues.

These examples show how to configure Claude Code's `settings.json` to use
agent-perms as a permission layer for `esc`, `gh`, `git`, `kubectl`, `pulumi`, and other CLIs.

---

## How It Works

Instead of allowing `gh` directly (which gives Claude access to every subcommand),
you allow `agent-perms exec <action> <scope> -- <cli> *` patterns. Claude Code
enforces the glob match — if Claude tries to run `agent-perms exec read remote -- gh repo delete`,
the pattern `Bash(agent-perms exec read remote -- gh *)` fires but `agent-perms exec` itself
enforces the tier independently as a second check.

The claimed tier must exactly match what the command requires. `read` does not
cover `read-sensitive`, and `write` does not cover `read`. Each tier is independent.

One rule replaces dozens of individual `gh` subcommand allowlists.

---

## Mixing Depth Levels

The real power of agent-perms is combining rules at different specificity levels.
A broad rule like `read remote -- gh *` covers every read subcommand in one line. A narrow rule
like `write remote -- gh pr *` scopes writes to a single subcommand group. Together they describe
a precise capability profile — without listing every subcommand individually.

### PR workflow agent

Claude can read anything from GitHub, and write only to PRs. Issues, repos, releases,
secrets, and everything else still require your approval.

```json
{
  "permissions": {
    "allow": [
      "Bash(agent-perms exec read remote -- gh *)",
      "Bash(agent-perms exec write remote -- gh pr *)"
    ]
  }
}
```

`read remote -- gh *` is broad. `write remote -- gh pr *` is narrow. The combination is precise: full
visibility, scoped action.

### Commit locally, gate pushes

Claude can read git state, make commits, and manage branches automatically.
Remote pushes (tier `write remote`) still prompt you.

```json
{
  "permissions": {
    "allow": [
      "Bash(agent-perms exec read local -- git *)",
      "Bash(agent-perms exec read remote -- git *)",
      "Bash(agent-perms exec write local -- git *)"
    ]
  }
}
```

### Go: read-only (vet + compile checks)

Claude can run `go vet` and compile checks automatically. `go test` is classified
as `write local` because tests execute arbitrary project code. Writes (format,
generate, install) and cache-clearing admin ops still require your approval.

```json
{
  "permissions": {
    "allow": [
      "Bash(agent-perms exec read local -- go *)"
    ]
  }
}
```

### Go: read + write (full local development)

Claude can run tests, build, format, and manage modules automatically. Cache-clearing
(`go clean -cache`, `go clean -modcache`) still requires your approval.

```json
{
  "permissions": {
    "allow": [
      "Bash(agent-perms exec read local -- go *)",
      "Bash(agent-perms exec write local -- go *)"
    ]
  }
}
```

### Kubernetes: read-only cluster access

Claude can inspect cluster state and read logs, but cannot modify resources.

```json
{
  "permissions": {
    "allow": [
      "Bash(agent-perms exec read remote -- kubectl *)",
      "Bash(agent-perms exec read local -- kubectl *)"
    ]
  }
}
```

### Kubernetes: read + deploy, gate deletes

Claude can read cluster state and apply manifests. Deletes and drains require approval.
Secret access is gated behind `read-sensitive`.

```json
{
  "permissions": {
    "allow": [
      "Bash(agent-perms exec read remote -- kubectl *)",
      "Bash(agent-perms exec read local -- kubectl *)",
      "Bash(agent-perms exec write remote -- kubectl *)",
      "Bash(agent-perms exec write local -- kubectl *)"
    ]
  }
}
```

### Gating sensitive reads

Some read commands expose secrets without mutating state (e.g., `pulumi env open`,
`gh auth token`). These are classified as `read-sensitive` — a separate tier from
`read`. By default, no profile auto-approves `read-sensitive`.

Allow reads but prompt for sensitive reads:

```json
{
  "permissions": {
    "allow": [
      "Bash(agent-perms exec read local -- pulumi *)",
      "Bash(agent-perms exec read remote -- pulumi *)"
    ]
  }
}
```

`pulumi env open` requires `read-sensitive remote`, which is not in allow, so Claude asks.

Explicitly allow sensitive reads too:

```json
{
  "permissions": {
    "allow": [
      "Bash(agent-perms exec read local -- pulumi *)",
      "Bash(agent-perms exec read remote -- pulumi *)",
      "Bash(agent-perms exec read-sensitive remote -- pulumi *)"
    ]
  }
}
```

---

### Infra read-only, preview changes, gate applies

Claude can inspect Pulumi stacks and run previews freely. Applies and destroys
still require your approval.

```json
{
  "permissions": {
    "allow": [
      "Bash(agent-perms exec read local -- pulumi *)",
      "Bash(agent-perms exec read remote -- pulumi *)",
      "Bash(agent-perms exec write local -- pulumi preview *)"
    ]
  }
}
```

---

## Common Profiles

### Read-only access to gh (safest)

Claude can run any `gh` read command automatically. Write and admin require approval.

```json
{
  "permissions": {
    "allow": [
      "Bash(agent-perms exec read remote -- gh *)"
    ]
  }
}
```

**Claude.md instruction to add:**
```
Use agent-perms to run gh commands. The claimed tier must match exactly.
Use agent-perms explain to check the tier before running a command.

  agent-perms explain gh <subcommand>                         — check the required tier
  agent-perms exec read remote -- gh <subcommand> [args...]          — for read-only commands
  agent-perms exec read-sensitive remote -- gh <subcommand> [args...] — for commands that expose secrets
  agent-perms exec write remote -- gh <subcommand> [args...]         — for write commands (will prompt)
  agent-perms exec admin remote -- gh <subcommand> [args...]         — for destructive commands (will prompt)

If a command is denied, agent-perms will tell you the required tier.
```

---

### Read + write access, admin requires approval

Claude can read and create/edit things automatically. Destructive operations
(delete, auth, secrets) still require your approval.

```json
{
  "permissions": {
    "allow": [
      "Bash(agent-perms exec read remote -- gh *)",
      "Bash(agent-perms exec write remote -- gh *)"
    ]
  }
}
```

---

### Scope writes to specific command groups

Allow read and PR-related writes automatically, but require approval for anything
that touches repos, releases, or admin operations.

```json
{
  "permissions": {
    "allow": [
      "Bash(agent-perms exec read remote -- gh *)",
      "Bash(agent-perms exec write remote -- gh pr *)",
      "Bash(agent-perms exec write remote -- gh issue *)"
    ]
  }
}
```

---

### Full access (read/write/admin all auto-approved)

Not recommended for production, but useful during initial exploration or when
you trust the task fully.

```json
{
  "permissions": {
    "allow": [
      "Bash(agent-perms exec read remote -- gh *)",
      "Bash(agent-perms exec write remote -- gh *)",
      "Bash(agent-perms exec admin remote -- gh *)"
    ]
  }
}
```

---

### Deny list: block dangerous operations explicitly

If you prefer to allow gh directly but still want to block admin-tier operations:

```json
{
  "permissions": {
    "allow": [
      "Bash(agent-perms exec read remote -- gh *)",
      "Bash(agent-perms exec write remote -- gh *)"
    ],
    "deny": [
      "Bash(agent-perms exec admin remote -- gh *)"
    ]
  }
}
```

---

## Settings File Locations

| Scope | Location |
|-------|----------|
| Global (all projects) | `~/.claude/settings.json` |
| Project-specific | `.claude/settings.json` in your repo |
| Project (checked in) | `.claude/settings.local.json` (gitignored by default) |

Project settings override global settings.

---

## Handling Unclassified Commands

By default, agent-perms denies any `gh` subcommand not in its classification DB
(`--on-unknown=deny`). During initial exploration you can opt into allow mode:

```json
{
  "permissions": {
    "allow": [
      "Bash(agent-perms --on-unknown=allow exec read remote -- gh *)"
    ]
  }
}
```

> **Warning:** `--on-unknown=allow` lets unrecognized subcommands run without
> tier enforcement. Only use this during evaluation, never in production or CI.
> agent-perms prints a warning to stderr when this mode is active.

---

## Verifying Classification Before Adding Rules

Use `agent-perms explain` to check what tier a command requires before deciding
which allow rule to add:

```console
$ agent-perms explain gh release create v1.0
cli:        gh
command:    release create
base_tier:  write remote (gh release create)
result:     write remote

$ agent-perms explain gh secret set TOKEN
cli:        gh
command:    secret set
base_tier:  admin remote (gh secret set)
result:     admin remote

$ agent-perms explain gh api --method DELETE /repos/owner/repo
cli:        gh
command:    api
base_tier:  read remote (gh api (GET default))
flags:      --method DELETE → admin remote
result:     admin remote
```
