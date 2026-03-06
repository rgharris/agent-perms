# agent-perms Development Guide

## Adding a New CLI Classifier

When you add a new CLI classifier (a new `internal/classify/<cli>.go` file), update all of the following:

1. **`internal/classify/classify.go`** — add a `case "<cli>":` branch in the `Classify` switch
2. **`cmd/agent-perms/main.go`** — add the CLI's tiers and example commands to the `cmdClaudeMD()` output
3. **`internal/codex/codex.go`** — add the CLI's examples to `agentsMDContent` (the `codex md` output)
4. **`README.md`** — add the CLI to the settings example (both `allow` and `deny` arrays)
5. **`examples/claude-settings.md`** — add example profiles for the new CLI
6. **`docs/new-clis-to-add.md`** — remove the CLI from the candidates list if it was listed there

## Auditing an Existing CLI Classifier

When updating a classifier to add missing subcommands, follow this process:

1. **Capture the full command reference** — Run the CLI's help command to get every subcommand:
   - `gh help reference` (or `gh help -a`)
   - `git help -a`
   - `go help` + `go help mod` / `go help work` for sub-groups
   - `pulumi --help` + `pulumi <group> --help` for each subcommand group
2. **Extract all subcommand paths** — List every valid `<group> <sub>` combination from the help output
3. **Diff against the classifier** — Compare against the keys in the `*Tiers` map and any special-case `switch` handlers in `internal/classify/<cli>.go`
4. **Classify each gap** — Determine the correct tier (read/write/admin × local/remote) for each missing subcommand
5. **Check key depth** — Verify the classifier function supports the required key depth (e.g., `gh` supports 3-token keys like `repo autolink list`)
6. **Build, test, install** — `go build ./...`, `go test ./...`, `go install ./cmd/agent-perms`
7. **Verify with explain** — Run `agent-perms explain <cli> <subcommand>` for new entries

## Classification Decisions

Use conservative defaults when a command executes arbitrary user-controlled code
or can hide write/destructive behavior behind a helper interface.

- `git submodule foreach` is `admin local` because it executes arbitrary shell
  in each submodule.
- `git gui` is `write local` because the UI supports staging/committing and other
  state mutations.
- `go test` is `write local` because tests execute project code and can mutate
  local state.
- Default/recommended profile is `write-local` for practical day-to-day usage;
  `read` remains available for stricter environments.

## Commit Practices

### During Development

- **Commit as you go.** Don't batch all changes into a single commit at the end. Commit meaningful units of work as they're completed.
- Write clear, descriptive commit messages that explain the "why" not just the "what."

### Attribution & Audit Trail

- **Sign commits** (GPG or Git signing) when possible to cryptographically verify authorship
- **Never push force-push to shared branches** (main, master, or long-lived feature branches)
