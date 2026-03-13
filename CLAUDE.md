# agent-perms Development Guide

## Adding a New CLI Classifier

When you add a new CLI classifier (a new `internal/classify/<cli>.go` file), update all of the following:

1. **`internal/classify/classify.go`** — add a `case "<cli>":` branch in the `Classify` switch
2. **`cmd/agent-perms/main.go`** — add the CLI's tiers and example commands to the `cmdClaudeMD()` output
3. **`internal/codex/codex.go`** — add the CLI's examples to `agentsMDContent` (the `codex md` output)
4. **`README.md`** — add the CLI to the settings example (both `allow` and `deny` arrays)
5. **`examples/claude-settings.md`** — add example profiles for the new CLI
6. **`examples/codex-settings.md`** — add example profiles for the new CLI
7. **`docs/new-clis-to-add.md`** — remove the CLI from the candidates list if it was listed there
8. **`internal/settings/settings_test.go`** — add `"Bash(<cli> *)"` to the deny array in `TestValidateClean`

## Auditing an Existing CLI Classifier

When updating a classifier to add missing subcommands, follow this process:

1. **Capture the full command reference** — Run the CLI's help command to get every subcommand:
   - `gh help reference` (or `gh help -a`)
   - `git help -a`
   - `go help` + `go help mod` / `go help work` for sub-groups
   - `pulumi --help` + `pulumi <group> --help` for each subcommand group
   - `kubectl --help` + `kubectl <subcommand> --help` for each subcommand
2. **Extract all subcommand paths** — List every valid `<group> <sub>` combination from the help output
3. **Diff against the classifier** — Compare against the keys in the `*Tiers` map and any special-case `switch` handlers in `internal/classify/<cli>.go`
4. **Classify each gap** — Determine the correct tier (read/write/admin × local/remote) for each missing subcommand
5. **Check key depth** — Verify the classifier function supports the required key depth (e.g., `gh` supports 3-token keys like `repo autolink list`)
6. **Check flag bypass gaps** — For each flag-dependent handler, verify:
   - Combined short flags (e.g., `-fd` containing `-f` for force)
   - Embedded value forms (e.g., `--force-with-lease=ref`)
   - Prefix/alias forms (e.g., `+refspec` for git force push)
7. **Build, test, install** — `go build ./...`, `go test ./...`, `go install ./cmd/agent-perms`
8. **Verify with explain** — Run `agent-perms explain <cli> <subcommand>` for new entries

## Classification Decisions

Use conservative defaults when a command executes arbitrary user-controlled code
or can hide write/destructive behavior behind a helper interface.

- `git submodule foreach` is `admin local` because it executes arbitrary shell
  in each submodule.
- `git gui` is `write local` because the UI supports staging/committing and other
  state mutations.
- `go test` is `write local` because tests execute project code and can mutate
  local state.
- `kubectl exec` is `write remote` because it runs arbitrary commands in
  containers — commonly used for debugging, similar reasoning to `go test`.
- `kubectl get secret` / `kubectl describe secret` escalate to
  `read-sensitive remote` because output can expose secret data.
- `kubectl drain` is `admin remote` because it evicts all pods from a node,
  risking service disruption.
- `kubectl proxy` and `kubectl port-forward` are `read-sensitive remote`
  because they open privileged network access without mutating state.
- Default/recommended profile is `write-local` for practical day-to-day usage;
  `read` remains available for stricter environments.

## Maintaining This File

Update CLAUDE.md whenever you encounter a recurring pattern, non-obvious decision,
or process that should be followed consistently. If you find yourself explaining
something that would help future sessions, add it here. Examples:

- New classification rationale → add to "Classification Decisions"
- New checklist step discovered during a task → add to the relevant checklist
- A mistake that a rule here would have prevented → add the rule

## Commit Practices

### During Development

- **Commit as you go.** Don't batch all changes into a single commit at the end. Commit meaningful units of work as they're completed.
- Write clear, descriptive commit messages that explain the "why" not just the "what."

### Attribution & Audit Trail

- **Sign commits** (GPG or Git signing) when possible to cryptographically verify authorship
- **Never push force-push to shared branches** (main, master, or long-lived feature branches)
