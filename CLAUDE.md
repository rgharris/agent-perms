# agent-perms Development Guide

## Adding a New CLI Classifier

When you add a new CLI classifier (a new `internal/classify/<cli>.go` file), update all of the following:

1. **`internal/classify/classify.go`** — add a `case "<cli>":` branch in the `Classify` switch
2. **`cmd/agent-perms/main.go`** — add the CLI's tiers and example commands to the `cmdClaudeMD()` output
3. **`internal/codex/codex.go`** — add the CLI's examples to `agentsMDContent` (the `codex md` output)
4. **`README.md`** — add the CLI to the settings example (both `allow` and `deny` arrays)
5. **`examples/claude-settings.md`** — add example profiles for the new CLI
6. **`docs/new-clis-to-add.md`** — remove the CLI from the candidates list if it was listed there

## Commit Practices

### During Development

- **Commit as you go.** Don't batch all changes into a single commit at the end. Commit meaningful units of work as they're completed.
- Write clear, descriptive commit messages that explain the "why" not just the "what."

### Attribution & Audit Trail

- **Sign commits** (GPG or Git signing) when possible to cryptographically verify authorship
- **Never push force-push to shared branches** (main, master, or long-lived feature branches)
