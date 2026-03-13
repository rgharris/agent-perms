package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rgharris/agent-perms/internal/classify"
	"github.com/rgharris/agent-perms/internal/codex"
	agentexec "github.com/rgharris/agent-perms/internal/exec"
	"github.com/rgharris/agent-perms/internal/settings"
	"github.com/rgharris/agent-perms/internal/types"
)

// Version is set via -ldflags at build time. Falls back to "dev".
var Version = "dev"

const usage = `agent-perms — semantic permission layer for CLI agents

Usage:
  agent-perms [--on-unknown=deny|allow] [--json] exec <action> <scope> -- <command...>
  agent-perms [--on-unknown=deny|allow] [--json] explain <command...>
  agent-perms claude md
  agent-perms claude init [--profile=<name>] [--merge=<path>] [--write]
  agent-perms claude validate [<path>...]
  agent-perms codex md
  agent-perms codex init [--profile=<name>] [--write]
  agent-perms codex validate [<path>...]
  agent-perms version
  agent-perms help

Commands:
  exec <action> <scope> -- <command...>
      Classify <command>, then run it if the claimed tier matches exactly.
      Actions: read, read-sensitive, write, admin
      Scopes:  local, remote (required for all actions)
      The action and scope may appear in either order.
      On denial, prints the required tier and exits non-zero.

  explain <command...>
      Show the full classification for <command> without running it.

  claude md
      Print a CLAUDE.md snippet with usage instructions for all supported CLIs.
      Pipe into your CLAUDE.md or use a SessionStart hook to load it automatically.

  claude init [--profile=<name>] [--merge=<path>] [--write]
      Generate a settings.json with recommended agent-perms rules.
      If no --profile is given, prompts interactively (use --profile for scripting).
      If ~/.claude/settings.json exists, merges into it by default.
      With --write, writes directly to ~/.claude/settings.json.
      In interactive mode (no --write), prompts before writing.
      Profiles: read, write-local, full-write

  claude validate [<path>...]
      Check settings.json files for common agent-perms rule issues.
      Defaults to ~/.claude/settings.json if no path given.

  codex md
      Print an AGENTS.md snippet with usage instructions for all supported CLIs.
      Pipe into your AGENTS.md for Codex CLI integration.

  codex init [--profile=<name>] [--write]
      Generate Starlark exec policy rules and AGENTS.md for Codex CLI.
      If no --profile is given, prompts interactively (use --profile for scripting).
      With --write, writes directly without prompting.
      In interactive mode (no --write), prompts before writing.
      Profiles: read, write-local, full-write

  codex validate [<path>...]
      Check .rules files for common agent-perms exec policy issues.
      Defaults to ~/.codex/rules/agent-perms.rules if no path given.

Flags:
  --on-unknown=deny   Deny commands not in the classification DB (default)
  --on-unknown=allow  Run unclassified commands without restriction
  --json              Output errors/results as JSON

Examples:
  agent-perms exec read remote -- gh pr list
  agent-perms exec read-sensitive remote -- gh auth token
  agent-perms exec write remote -- gh issue create --title "bug"
  agent-perms exec read local -- git log --oneline -10
  agent-perms exec write local -- git commit -F /tmp/commit-msg.txt
  agent-perms exec write remote -- git push origin main
  agent-perms exec admin remote -- git push --force
  agent-perms exec read-sensitive remote -- pulumi env open myorg/prod
  agent-perms explain git push --force
  agent-perms explain gh api --method DELETE /repos/owner/repo
  agent-perms claude init                       # interactive profile selection + write prompt
  agent-perms claude init --profile=write-local  # scripting / non-interactive
  agent-perms claude init --write                # writes directly to ~/.claude/settings.json
  agent-perms codex init                        # interactive profile selection + write prompt
  agent-perms codex init --profile=write-local  # scripting / non-interactive
  agent-perms codex init --write                # writes directly to ~/.codex/
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	// Parse global flags first.
	opts := agentexec.Options{
		OnUnknown: agentexec.OnUnknownDeny,
		JSON:      false,
	}

	// Only consume global flags that appear before the subcommand.
	// Everything after the subcommand is passed through verbatim so that
	// flags like --json meant for the underlying CLI are not swallowed.
	i := 0
	for ; i < len(args); i++ {
		switch args[i] {
		case "--on-unknown=deny":
			opts.OnUnknown = agentexec.OnUnknownDeny
		case "--on-unknown=allow":
			opts.OnUnknown = agentexec.OnUnknownAllow
		case "--json":
			opts.JSON = true
		default:
			goto done
		}
	}
done:
	args = args[i:]

	if len(args) == 0 {
		fmt.Print(usage)
		return 0
	}

	switch args[0] {
	case "exec":
		return cmdExec(args[1:], opts)
	case "explain":
		return cmdExplain(args[1:], opts)
	case "claude":
		return cmdClaude(args[1:], opts)
	case "codex":
		return cmdCodex(args[1:], opts)
	case "version", "--version":
		fmt.Println(Version)
		return 0
	case "help", "--help", "-h":
		fmt.Print(usage)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "ERROR: unknown subcommand '%s'\n\n%s", args[0], usage)
		return 1
	}
}

// cmdExec validates the claimed tier and runs the command.
// args is everything after "exec": [<action>, [<scope>], "--", <cli>, ...]
func cmdExec(args []string, opts agentexec.Options) int {
	// Find the "--" separator.
	sepIdx := -1
	for i, a := range args {
		if a == "--" {
			sepIdx = i
			break
		}
	}
	if sepIdx == -1 {
		fmt.Fprintf(os.Stderr, "ERROR: exec requires '--' before the command\nUsage: agent-perms exec <action> <scope> -- <command...>\n")
		return 1
	}

	permTokens := args[:sepIdx]
	cmd := args[sepIdx+1:]

	if len(cmd) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: no command after '--'\nUsage: agent-perms exec <action> <scope> -- <command...>\n")
		return 1
	}

	claimed, err := parseTierTokens(permTokens)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\nUsage: agent-perms exec <action> <scope> -- <command...>\n", err)
		return 1
	}

	return agentexec.Run(claimed, cmd, opts)
}

// parseTierTokens parses 2 permission tokens (action + scope) into a Tier.
// All actions require a scope (local or remote). The tokens may appear in either order.
func parseTierTokens(tokens []string) (types.Tier, error) {
	switch len(tokens) {
	case 1:
		// Check for legacy "sensitive" token.
		if tokens[0] == "sensitive" {
			return types.TierUnknown, fmt.Errorf("use 'read-sensitive <scope>' instead of 'sensitive'")
		}
		action, ok := types.ParseAction(tokens[0])
		if !ok {
			return types.TierUnknown, fmt.Errorf("unknown action '%s'. Valid actions: read, read-sensitive, write, admin", tokens[0])
		}
		return types.TierUnknown, fmt.Errorf("'%s' requires a scope (local or remote)", action)

	case 2:
		// Check for legacy "read sensitive" two-token form.
		if (tokens[0] == "read" && tokens[1] == "sensitive") || (tokens[0] == "sensitive" && tokens[1] == "read") {
			return types.TierUnknown, fmt.Errorf("use 'read-sensitive' instead of 'read sensitive'")
		}
		// Try action-first: tokens[0]=action, tokens[1]=scope
		if action, ok := types.ParseAction(tokens[0]); ok {
			if scope, ok := types.ParseScope(tokens[1]); ok {
				return types.Tier{Action: action, Scope: scope}, nil
			}
			return types.TierUnknown, fmt.Errorf("unknown scope '%s'. Valid scopes: local, remote", tokens[1])
		}
		// Try scope-first: tokens[0]=scope, tokens[1]=action
		if scope, ok := types.ParseScope(tokens[0]); ok {
			if action, ok := types.ParseAction(tokens[1]); ok {
				return types.Tier{Action: action, Scope: scope}, nil
			}
			return types.TierUnknown, fmt.Errorf("unknown action '%s'. Valid actions: read, read-sensitive, write, admin", tokens[1])
		}
		return types.TierUnknown, fmt.Errorf("invalid permission tokens '%s %s'. Expected <action> <scope>", tokens[0], tokens[1])

	case 0:
		return types.TierUnknown, fmt.Errorf("missing action before '--'. Valid actions: read, read-sensitive, write, admin")

	default:
		return types.TierUnknown, fmt.Errorf("too many permission tokens before '--' (expected 2, got %d)", len(tokens))
	}
}

// tierExecString formats a tier and command as a suggested exec invocation.
func tierExecString(tier types.Tier, cmdStr string) string {
	return fmt.Sprintf("agent-perms exec %s -- %s", tier, cmdStr)
}

// cmdClaudeMD prints a CLAUDE.md snippet with usage instructions for all supported CLIs.
func cmdClaudeMD() int {
	fmt.Print(`# agent-perms

Run CLI commands through agent-perms to enforce permission tiers. Declare the
action and scope upfront — agent-perms classifies the command and runs it only
if the claimed tier matches exactly. On denial it prints the required tier so
you can adjust.

    agent-perms exec <action> <scope> -- <cli> <subcommand> [args...]

You MUST wrap the following CLIs with agent-perms: gh, git, go, pulumi.
The user's permission rules will likely deny these commands when run directly.

The claimed tier must exactly match what the command requires. There is no
hierarchy — ` + "`write`" + ` does not cover ` + "`read`" + `, and ` + "`read`" + ` does not cover
` + "`read-sensitive`" + `. Each tier is independent. Use
` + "`agent-perms explain <cli> <subcommand>`" + ` to check the exact tier required.
Only skip this if you have already run explain for the same command in this
session or if the tier is shown in the examples below.

Keep commands simple — avoid special shell characters like ` + "`$()`" + `, pipes, and
heredocs in agent-perms arguments. Complex shell syntax triggers manual approval
prompts in the permission system. For example, pass commit messages as plain
quoted strings rather than using command substitution. For multiline git commit
messages, write the message to a temp file and use ` + "`-F`" + `:

    agent-perms exec write local -- git commit -F /tmp/commit-msg.txt

## gh (GitHub CLI)

Actions: read, read-sensitive, write, admin
Scopes: remote (all gh ops contact the GitHub API)

    agent-perms exec read remote -- gh pr list
    agent-perms exec read remote -- gh issue list --repo owner/repo
    agent-perms exec read-sensitive remote -- gh auth token
    agent-perms exec write remote -- gh pr create --title "fix" --body ""
    agent-perms exec write remote -- gh issue create --title "bug"
    agent-perms exec admin remote -- gh repo delete my-repo

## git

Actions: read, write, admin
Scopes: local, remote

    agent-perms exec read local -- git log --oneline -10
    agent-perms exec read local -- git status
    agent-perms exec read remote -- git fetch origin
    agent-perms exec write local -- git commit -F /tmp/commit-msg.txt
    agent-perms exec write local -- git add -p
    agent-perms exec write remote -- git push origin main
    agent-perms exec admin local -- git reset --hard HEAD~1
    agent-perms exec admin remote -- git push --force

## pulumi

Actions: read, read-sensitive, write, admin
Scopes: local, remote

    agent-perms exec read remote -- pulumi preview
    agent-perms exec read remote -- pulumi stack ls
    agent-perms exec read local -- pulumi config get aws:region
    agent-perms exec read-sensitive remote -- pulumi env open myorg/prod
    agent-perms exec read-sensitive remote -- pulumi env run myorg/prod -- printenv
    agent-perms exec write local -- pulumi stack select dev
    agent-perms exec write local -- pulumi config set key value
    agent-perms exec write remote -- pulumi up
    agent-perms exec admin local -- pulumi state delete <urn>
    agent-perms exec admin remote -- pulumi destroy

## go (Go toolchain)

Actions: read, write, admin
Scopes: local (all go ops are local)

    agent-perms exec read local -- go version
    agent-perms exec write local -- go test ./...
    agent-perms exec read local -- go vet ./...
    agent-perms exec read local -- go build ./...
    agent-perms exec write local -- go build -o mybinary
    agent-perms exec write local -- go fmt ./...
    agent-perms exec write local -- go mod tidy
    agent-perms exec write local -- go get github.com/foo/bar@latest
    agent-perms exec admin local -- go clean -modcache
`)
	return 0
}

// cmdExplain prints the full classification for a command without running it.
// args is everything after "explain": [<cli>, ...]
func cmdExplain(args []string, opts agentexec.Options) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: explain requires a command\nUsage: agent-perms explain <command...>\n")
		return 1
	}

	result := classify.Classify(args)

	flagEffects := result.FlagEffects
	if flagEffects == nil {
		flagEffects = []string{}
	}

	if opts.JSON {
		out := map[string]any{
			"cli":            result.CLI,
			"subcommand":     result.Subcommand,
			"tier":           result.Tier.String(),
			"base_tier":      result.BaseTier.String(),
			"base_tier_note": result.BaseTierNote,
			"flag_effects":   flagEffects,
			"sensitive":      result.Tier.Action == types.ActionReadSensitive,
			"unknown":        result.Unknown,
		}
		if !result.Unknown {
			out["exec"] = tierExecString(result.Tier, strings.Join(args, " "))
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(out)
		return 0
	}

	// Human-readable explain output.
	cmdStr := strings.Join(args, " ")
	fmt.Printf("cli:        %s\n", result.CLI)
	fmt.Printf("command:    %s\n", result.Subcommand)
	fmt.Printf("base_tier:  %s", result.BaseTier)
	if result.BaseTierNote != "" {
		fmt.Printf(" (%s)", result.BaseTierNote)
	}
	fmt.Println()
	if len(flagEffects) > 0 {
		fmt.Printf("flags:      %s\n", strings.Join(flagEffects, ", "))
	}
	fmt.Printf("result:     %s\n", result.Tier)
	if !result.Unknown {
		fmt.Printf("exec:       %s\n", tierExecString(result.Tier, cmdStr))
	}
	if result.Unknown {
		fmt.Printf("\nNOTE: '%s' is not in the classification DB\n", cmdStr)
	}

	return 0
}

// cmdClaude dispatches claude subcommands.
func cmdClaude(args []string, opts agentexec.Options) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: claude requires a subcommand (md, init, or validate)\n")
		return 1
	}
	switch args[0] {
	case "md":
		return cmdClaudeMD()
	case "init":
		return cmdClaudeInit(args[1:], opts)
	case "validate":
		return cmdClaudeValidate(args[1:], opts)
	default:
		fmt.Fprintf(os.Stderr, "ERROR: unknown claude subcommand '%s'. Use 'md', 'init', or 'validate'.\n", args[0])
		return 1
	}
}

// cmdClaudeInit generates a settings.json with recommended agent-perms rules.
func cmdClaudeInit(args []string, opts agentexec.Options) int {
	profile := ""
	mergePath := ""
	write := false

	for _, arg := range args {
		if strings.HasPrefix(arg, "--profile=") {
			profile = strings.TrimPrefix(arg, "--profile=")
		} else if strings.HasPrefix(arg, "--merge=") {
			mergePath = strings.TrimPrefix(arg, "--merge=")
		} else if arg == "--write" {
			write = true
		} else {
			fmt.Fprintf(os.Stderr, "ERROR: unknown flag '%s'\nUsage: agent-perms claude init [--profile=<name>] [--merge=<path>] [--write]\n", arg)
			return 1
		}
	}

	if profile == "" {
		p, err := promptProfile()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return 1
		}
		profile = p
	}

	// Determine the target settings path.
	settingsPath := ""
	if home, err := os.UserHomeDir(); err == nil {
		settingsPath = filepath.Join(home, ".claude", "settings.json")
	}

	// Expand ~ in --merge path.
	if strings.HasPrefix(mergePath, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			mergePath = filepath.Join(home, mergePath[2:])
		}
	}

	// Default to merging into ~/.claude/settings.json when it exists.
	if mergePath == "" {
		if settingsPath != "" {
			if _, err := os.Stat(settingsPath); err == nil {
				mergePath = settingsPath
			}
		}
	}

	// Generate the output.
	var data []byte
	if mergePath != "" {
		existing, err := os.ReadFile(mergePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: cannot read %s: %v\n", mergePath, err)
			return 1
		}
		merged, err := settings.Merge(existing, profile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return 1
		}
		data = merged
	} else {
		s, err := settings.GenerateSettings(profile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return 1
		}
		marshaled, err := settings.MarshalJSON(s)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return 1
		}
		data = marshaled
	}

	// Determine the write target path.
	writePath := settingsPath
	if mergePath != "" {
		writePath = mergePath
	}

	// If --write, write directly without prompting.
	if write {
		return writeSettingsFile(writePath, data)
	}

	// Print the generated settings.
	fmt.Println(string(data))

	// In interactive mode, prompt to write.
	if isTerminal() && writePath != "" {
		verb := "create"
		if mergePath != "" {
			verb = "overwrite"
		} else if _, err := os.Stat(writePath); err == nil {
			verb = "overwrite"
		}
		fmt.Fprintf(os.Stderr, "\n%s %s? [Y/n]: ", capitalize(verb), writePath)
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			input := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if input == "" || input == "y" || input == "yes" {
				return writeSettingsFile(writePath, data)
			}
			fmt.Fprintf(os.Stderr, "Not written. You can pipe the output above to a file or re-run with --write.\n")
		}
	}

	return 0
}

// writeSettingsFile writes settings data to the given path, creating directories as needed.
func writeSettingsFile(path string, data []byte) int {
	if path == "" {
		fmt.Fprintf(os.Stderr, "ERROR: cannot determine settings path\n")
		return 1
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot create %s: %v\n", dir, err)
		return 1
	}

	// Ensure data ends with a newline.
	if len(data) > 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot write %s: %v\n", path, err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", path)
	return 0
}

// cmdClaudeValidate checks settings files for agent-perms rule issues.
func cmdClaudeValidate(args []string, opts agentexec.Options) int {
	paths := args

	// Filter out flags.
	var filePaths []string
	jsonOutput := opts.JSON
	for _, arg := range paths {
		if arg == "--json" {
			jsonOutput = true
		} else if strings.HasPrefix(arg, "-") {
			fmt.Fprintf(os.Stderr, "ERROR: unknown flag '%s'\n", arg)
			return 1
		} else {
			filePaths = append(filePaths, arg)
		}
	}

	// Default to ~/.claude/settings.json
	if len(filePaths) == 0 {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: cannot determine home directory: %v\n", err)
			return 1
		}
		filePaths = []string{filepath.Join(home, ".claude", "settings.json")}
	}

	// Resolve the global settings path for external deny rule loading.
	globalSettingsPath := ""
	if home, err := os.UserHomeDir(); err == nil {
		globalSettingsPath = filepath.Join(home, ".claude", "settings.json")
	}

	// Load global deny rules once (used when validating non-global files).
	globalDeny := loadDenyRules(globalSettingsPath)

	hasIssues := false
	var allDiags []map[string]any

	for _, path := range filePaths {
		// Expand ~
		if strings.HasPrefix(path, "~/") {
			home, _ := os.UserHomeDir()
			path = filepath.Join(home, path[2:])
		}

		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: cannot read %s: %v\n", path, err)
			hasIssues = true
			continue
		}

		// Pass global deny rules when validating a non-global file.
		var diags []settings.Diagnostic
		absPath, _ := filepath.Abs(path)
		if absPath != globalSettingsPath && len(globalDeny) > 0 {
			diags = settings.Validate(data, globalDeny)
		} else {
			diags = settings.Validate(data)
		}
		if len(diags) > 0 {
			hasIssues = true
		}

		if jsonOutput {
			for _, d := range diags {
				allDiags = append(allDiags, map[string]any{
					"file":       path,
					"severity":   string(d.Severity),
					"message":    d.Message,
					"suggestion": d.Suggestion,
				})
			}
		} else {
			for _, d := range diags {
				prefix := "warning"
				if d.Severity == settings.SeverityError {
					prefix = "error"
				}
				fmt.Printf("%s: %s: %s\n", prefix, path, d.Message)
				if d.Suggestion != "" {
					fmt.Printf("  suggestion: %s\n", d.Suggestion)
				}
			}
		}
	}

	if jsonOutput {
		if allDiags == nil {
			allDiags = []map[string]any{}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(allDiags)
	}

	if hasIssues {
		return 1
	}
	return 0
}

// cmdCodexMD prints an AGENTS.md snippet with usage instructions for all supported CLIs.
func cmdCodexMD() int {
	fmt.Print(codex.GenerateAGENTSMD())
	return 0
}

// cmdCodex dispatches codex subcommands.
func cmdCodex(args []string, opts agentexec.Options) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: codex requires a subcommand (md, init, or validate)\n")
		return 1
	}
	switch args[0] {
	case "md":
		return cmdCodexMD()
	case "init":
		return cmdCodexInit(args[1:])
	case "validate":
		return cmdCodexValidate(args[1:], opts)
	default:
		fmt.Fprintf(os.Stderr, "ERROR: unknown codex subcommand '%s'. Use 'md', 'init', or 'validate'.\n", args[0])
		return 1
	}
}

// cmdCodexInit generates Starlark exec policy rules for the given profile.
// With --write, writes both ~/.codex/rules/agent-perms.rules and ~/.codex/AGENTS.md.
// In interactive mode (no --write), prompts before writing.
func cmdCodexInit(args []string) int {
	profile := ""
	write := false

	for _, arg := range args {
		if strings.HasPrefix(arg, "--profile=") {
			profile = strings.TrimPrefix(arg, "--profile=")
		} else if arg == "--write" {
			write = true
		} else {
			fmt.Fprintf(os.Stderr, "ERROR: unknown flag '%s'\nUsage: agent-perms codex init [--profile=<name>] [--write]\n", arg)
			return 1
		}
	}

	if profile == "" {
		p, err := promptProfile()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return 1
		}
		profile = p
	}

	rules, err := codex.GenerateExecPolicy(profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return 1
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot determine home directory: %v\n", err)
		return 1
	}

	rulesPath := filepath.Join(home, ".codex", "rules", "agent-perms.rules")
	agentsMDPath := filepath.Join(home, ".codex", "AGENTS.md")

	// If --write, write directly without prompting.
	if write {
		return writeCodexFiles(rulesPath, rules, agentsMDPath)
	}

	agentsMD := codex.GenerateAGENTSMD()

	// Print the generated rules.
	fmt.Print(rules)
	fmt.Fprintf(os.Stderr, "\n--- AGENTS.md (%d lines) ---\n", strings.Count(agentsMD, "\n"))
	fmt.Print(agentsMD)

	// In interactive mode, prompt to write.
	if isTerminal() {
		rulesVerb := "create"
		if _, err := os.Stat(rulesPath); err == nil {
			rulesVerb = "overwrite"
		}
		mdVerb := "create"
		if _, err := os.Stat(agentsMDPath); err == nil {
			mdVerb = "overwrite"
		}
		fmt.Fprintf(os.Stderr, "\n%s %s and %s %s? [Y/n]: ", capitalize(rulesVerb), rulesPath, mdVerb, agentsMDPath)
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			input := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if input == "" || input == "y" || input == "yes" {
				return writeCodexFiles(rulesPath, rules, agentsMDPath)
			}
			fmt.Fprintf(os.Stderr, "Not written. You can pipe the output above to a file or re-run with --write.\n")
		}
	}

	return 0
}

// writeCodexFiles writes the rules and AGENTS.md files for codex.
func writeCodexFiles(rulesPath, rules, agentsMDPath string) int {
	rulesDir := filepath.Dir(rulesPath)
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot create %s: %v\n", rulesDir, err)
		return 1
	}

	if err := os.WriteFile(rulesPath, []byte(rules), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot write %s: %v\n", rulesPath, err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", rulesPath)

	agentsMDDir := filepath.Dir(agentsMDPath)
	if err := os.MkdirAll(agentsMDDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot create %s: %v\n", agentsMDDir, err)
		return 1
	}

	if err := os.WriteFile(agentsMDPath, []byte(codex.GenerateAGENTSMD()), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot write %s: %v\n", agentsMDPath, err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", agentsMDPath)

	return 0
}

// cmdCodexValidate checks .rules files for agent-perms exec policy issues.
func cmdCodexValidate(args []string, opts agentexec.Options) int {
	// Filter out flags.
	var filePaths []string
	jsonOutput := opts.JSON
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
		} else if strings.HasPrefix(arg, "-") {
			fmt.Fprintf(os.Stderr, "ERROR: unknown flag '%s'\n", arg)
			return 1
		} else {
			filePaths = append(filePaths, arg)
		}
	}

	// Default to ~/.codex/rules/agent-perms.rules
	if len(filePaths) == 0 {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: cannot determine home directory: %v\n", err)
			return 1
		}
		filePaths = []string{filepath.Join(home, ".codex", "rules", "agent-perms.rules")}
	}

	hasIssues := false
	var allDiags []map[string]any

	for _, path := range filePaths {
		// Expand ~
		if strings.HasPrefix(path, "~/") {
			home, _ := os.UserHomeDir()
			path = filepath.Join(home, path[2:])
		}

		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: cannot read %s: %v\n", path, err)
			hasIssues = true
			continue
		}

		diags := codex.ValidateExecPolicy(string(data))
		if len(diags) > 0 {
			hasIssues = true
		}

		if jsonOutput {
			for _, d := range diags {
				allDiags = append(allDiags, map[string]any{
					"file":       path,
					"severity":   string(d.Severity),
					"message":    d.Message,
					"suggestion": d.Suggestion,
				})
			}
		} else {
			for _, d := range diags {
				prefix := "warning"
				if d.Severity == codex.SeverityError {
					prefix = "error"
				}
				fmt.Printf("%s: %s: %s\n", prefix, path, d.Message)
				if d.Suggestion != "" {
					fmt.Printf("  suggestion: %s\n", d.Suggestion)
				}
			}
		}
	}

	if jsonOutput {
		if allDiags == nil {
			allDiags = []map[string]any{}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(allDiags)
	}

	if hasIssues {
		return 1
	}
	return 0
}

// isTerminal reports whether stdin is a terminal.
func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// promptProfile interactively asks the user to select a profile.
// Falls back to "read" if stdin is not a terminal.
func promptProfile() (string, error) {
	profiles := settings.ProfileNames()
	descriptions := settings.ProfileDescriptions()

	if !isTerminal() {
		return profiles[0], nil
	}

	fmt.Fprintf(os.Stderr, "\nSelect a profile:\n\n")
	for i, name := range profiles {
		fmt.Fprintf(os.Stderr, "  %d. %-15s %s\n", i+1, name, descriptions[name])
	}
	fmt.Fprintf(os.Stderr, "\nProfile [1]: ")

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return profiles[0], nil
	}
	input := strings.TrimSpace(scanner.Text())

	if input == "" {
		return profiles[0], nil
	}

	// Accept number or name.
	for i, name := range profiles {
		if input == fmt.Sprintf("%d", i+1) || input == name {
			return name, nil
		}
	}

	return "", fmt.Errorf("invalid selection %q. Use 1-%d or a profile name (%s)", input, len(profiles), strings.Join(profiles, ", "))
}

// capitalize returns s with the first letter uppercased.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// loadDenyRules reads a settings file and returns its deny rules, or nil on error.
func loadDenyRules(path string) []string {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	permData, ok := raw["permissions"]
	if !ok {
		return nil
	}
	var perms struct {
		Deny []string `json:"deny"`
	}
	if err := json.Unmarshal(permData, &perms); err != nil {
		return nil
	}
	return perms.Deny
}
