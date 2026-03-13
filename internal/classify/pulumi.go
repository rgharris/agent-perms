package classify

import (
	"fmt"
	"strings"

	"github.com/rgharris/agent-perms/internal/types"
)

// pulumiTiers maps "subcommand" or "subcommand sub" to their permission tier.
// Two-token keys are tried first; single-token keys are the fallback.
var pulumiTiers = map[string]types.Tier{
	// Top-level read commands
	"preview": types.TierReadRemote, // dry-run; contacts cloud backend for state comparison
	"pre":     types.TierReadRemote, // alias for preview
	"logs":    types.TierReadRemote, // queries cloud logs
	"about":   types.TierReadLocal,  // displays local environment info
	"version": types.TierReadLocal,
	"whoami":  types.TierReadRemote, // queries the Pulumi Cloud service

	// stack — bare shows current stack info (read); subcommands vary
	"stack":                         types.TierReadRemote,
	"stack ls":                      types.TierReadRemote,
	"stack list":                    types.TierReadRemote,
	"stack output":                  types.TierReadRemote,
	"stack history":                 types.TierReadRemote,
	"stack graph":                   types.TierReadRemote,
	"stack export":                  types.TierReadRemote,
	"stack init":                    types.TierWriteLocal,
	"stack select":                  types.TierWriteLocal,
	"stack unselect":                types.TierWriteLocal,
	"stack rename":                  types.TierWriteLocal,
	"stack import":                  types.TierWriteLocal,
	"stack change-secrets-provider": types.TierWriteLocal,
	"stack rm":                      types.TierAdminLocal, // removes stack; irreversible
	"stack remove":                  types.TierAdminLocal,

	// config — bare lists config values (read local); subcommands vary
	"config":         types.TierReadLocal,
	"config get":     types.TierReadLocal,
	"config set":     types.TierWriteLocal,
	"config set-all": types.TierWriteLocal,
	"config rm":      types.TierWriteLocal,
	"config rm-all":  types.TierWriteLocal,
	"config remove":  types.TierWriteLocal,
	"config cp":      types.TierWriteLocal,
	"config refresh": types.TierWriteLocal,

	// state
	"state export":    types.TierReadRemote,  // exports state from backend (typically Pulumi Cloud)
	"state import":    types.TierWriteLocal,  // load a state snapshot from file
	"state move":      types.TierWriteLocal,
	"state rename":    types.TierWriteLocal,
	"state protect":   types.TierWriteLocal,
	"state unprotect": types.TierWriteLocal,
	"state taint":     types.TierWriteLocal,
	"state untaint":   types.TierWriteLocal,
	"state repair":    types.TierWriteLocal,
	"state upgrade":   types.TierWriteLocal,
	"state delete":    types.TierAdminLocal, // removes resource from state; causes drift
	"state edit":      types.TierAdminLocal, // raw edit of state blob; highly destructive

	// plugin
	"plugin":         types.TierReadLocal,
	"plugin ls":      types.TierReadLocal,
	"plugin list":    types.TierReadLocal,
	"plugin install": types.TierWriteLocal,
	"plugin add":     types.TierWriteLocal,
	"plugin rm":      types.TierWriteLocal,
	"plugin remove":  types.TierWriteLocal,

	// schema
	"schema":       types.TierReadLocal,
	"schema check": types.TierReadLocal,
	"schema get":   types.TierReadLocal,

	// package
	"package":             types.TierReadLocal,
	"package get-schema":  types.TierReadLocal,
	"package get-mapping": types.TierReadLocal,
	"package info":        types.TierReadLocal,
	"package add":         types.TierWriteLocal, // installs a package dependency locally
	"package gen-sdk":     types.TierWriteLocal, // generates SDK code locally
	"package publish":     types.TierWriteRemote, // publishes to registry
	"package delete":      types.TierAdminRemote, // deletes from registry; irreversible

	// template
	"template publish": types.TierWriteRemote, // publishes to registry

	// org
	"org":             types.TierReadRemote,
	"org get-default": types.TierReadLocal,
	"org set-default": types.TierWriteLocal,
	"org search":      types.TierReadRemote,

	// project
	"project":    types.TierReadRemote,
	"project ls": types.TierReadRemote,

	// policy
	"policy ls":              types.TierReadRemote,
	"policy list":            types.TierReadRemote,
	"policy validate-config": types.TierReadLocal,
	"policy new":             types.TierWriteLocal,  // creates local policy pack
	"policy publish":         types.TierWriteRemote, // publishes to Pulumi Cloud
	"policy enable":          types.TierWriteRemote,
	"policy disable":         types.TierWriteRemote,
	"policy rm":              types.TierAdminRemote, // removes policy pack; irreversible
	"policy remove":          types.TierAdminRemote,

	// auth — modifies local credentials file
	"login":  types.TierWriteLocal,
	"logout": types.TierWriteLocal,

	// project scaffolding (creates local files only)
	"new": types.TierWriteLocal,

	// import: adds an existing cloud resource to local state; no infra change
	"import": types.TierWriteLocal,

	// install: installs packages and plugins locally
	"install": types.TierWriteLocal,

	// refresh: reconciles local state with actual cloud state; no infra change
	"refresh": types.TierWriteLocal,

	// convert: converts programs between languages (local only)
	"convert": types.TierWriteLocal,

	// misc read
	"help":           types.TierReadLocal,
	"console":        types.TierReadRemote, // opens stack in Pulumi Console
	"gen-completion": types.TierReadLocal,

	// Remote write — deploy changes to cloud infrastructure
	"up":     types.TierWriteRemote,
	"update": types.TierWriteRemote, // alias for up
	"watch":  types.TierWriteRemote, // continuous deploy on file changes

	// Remote admin — destroy or disrupt cloud resources
	"destroy": types.TierAdminRemote,
	"cancel":  types.TierAdminRemote, // cancels an in-progress update; can leave cloud in partial state
}

// classifyPulumi classifies a pulumi command given args after "pulumi".
// e.g., for "pulumi stack init dev", args = ["stack", "init", "dev"].
func classifyPulumi(args []string) Result {
	if len(args) == 0 {
		return Result{CLI: "pulumi", Tier: types.TierUnknown, Unknown: true,
			BaseTierNote: "no pulumi subcommand provided"}
	}

	// Any command with --help or -h just prints help text; always read-local.
	if hasHelpFlag(args) {
		sub, _ := pulumiSubcommand(args)
		desc := "pulumi --help"
		if sub != "" && sub != "help" {
			desc = fmt.Sprintf("pulumi %s --help", sub)
		}
		return Result{
			CLI: "pulumi", Subcommand: sub,
			Tier: types.TierReadLocal, BaseTier: types.TierReadLocal,
			BaseTierNote: desc + " (help output; read-only)",
		}
	}

	// Skip global flags to find the subcommand.
	sub, rest := pulumiSubcommand(args)
	if sub == "" {
		return Result{CLI: "pulumi", Tier: types.TierUnknown, Unknown: true,
			BaseTierNote: "no pulumi subcommand found after global flags"}
	}

	// env/esc (Pulumi ESC) has a 3–4 level command tree; use a dedicated handler.
	if sub == "env" || sub == "esc" {
		return classifyPulumiEnv(rest)
	}

	// stack tag and config env have sub-subcommands; use dedicated handlers.
	if sub == "stack" && len(rest) > 0 && rest[0] == "tag" {
		return classifyPulumiStackTag(rest[1:])
	}
	if sub == "config" && len(rest) > 0 && rest[0] == "env" {
		return classifyPulumiConfigEnv(rest[1:])
	}
	// policy group has a sub-subcommand
	if sub == "policy" && len(rest) > 0 && rest[0] == "group" {
		return classifyPulumiPolicyGroup(rest[1:])
	}

	// Try two-token key first (e.g., "stack init", "config set").
	// Only combine if the next token is not a flag.
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		key := sub + " " + rest[0]
		if tier, ok := pulumiTiers[key]; ok {
			return Result{
				CLI:          "pulumi",
				Subcommand:   key,
				Tier:         tier,
				BaseTier:     tier,
				BaseTierNote: fmt.Sprintf("pulumi %s", key),
			}
		}
	}

	// Fall back to single-token key.
	if tier, ok := pulumiTiers[sub]; ok {
		return Result{
			CLI:          "pulumi",
			Subcommand:   sub,
			Tier:         tier,
			BaseTier:     tier,
			BaseTierNote: fmt.Sprintf("pulumi %s", sub),
		}
	}

	return Result{
		CLI:          "pulumi",
		Subcommand:   sub,
		Tier:         types.TierUnknown,
		BaseTier:     types.TierUnknown,
		BaseTierNote: fmt.Sprintf("pulumi %s: not in classification DB", sub),
		Unknown:      true,
	}
}

// classifyPulumiEnv classifies "pulumi env" (aka "pulumi esc") commands.
// ESC environments are stored in Pulumi Cloud, so mutations are remote ops.
// The command tree goes up to 4 levels: env → tag/version → (sub) → (sub).
func classifyPulumiEnv(args []string) Result {
	sub, rest := pulumiFirstPositional(args)

	if sub == "" {
		// "pulumi env" bare → lists environments
		return Result{CLI: "pulumi", Subcommand: "env",
			Tier: types.TierReadRemote, BaseTier: types.TierReadRemote,
			BaseTierNote: "pulumi env (list by default)"}
	}

	switch sub {
	// Read-only (remote — queries Pulumi Cloud)
	case "ls", "list", "get", "diff":
		return Result{CLI: "pulumi", Subcommand: "env " + sub,
			Tier: types.TierReadRemote, BaseTier: types.TierReadRemote,
			BaseTierNote: fmt.Sprintf("pulumi env %s", sub)}

	// Read sensitive — resolves and exposes secret values (remote)
	case "open":
		return Result{CLI: "pulumi", Subcommand: "env open",
			Tier: types.TierReadSensitiveRemote, BaseTier: types.TierReadSensitiveRemote,
			BaseTierNote: "pulumi env open (resolves and exposes secret values)"}
	case "run":
		return classifyPulumiEnvRun(rest)

	// Write:remote — creates or modifies ESC environment data in Pulumi Cloud
	case "init", "edit", "set", "clone", "rotate":
		return Result{CLI: "pulumi", Subcommand: "env " + sub,
			Tier: types.TierWriteRemote, BaseTier: types.TierWriteRemote,
			BaseTierNote: fmt.Sprintf("pulumi env %s (modifies ESC environment in Pulumi Cloud)", sub)}

	// Admin:remote — deletes an environment; irreversible
	case "rm", "remove":
		return Result{CLI: "pulumi", Subcommand: "env " + sub,
			Tier: types.TierAdminRemote, BaseTier: types.TierAdminRemote,
			BaseTierNote: fmt.Sprintf("pulumi env %s (deletes ESC environment; irreversible)", sub)}

	// Nested: env tag → get/ls (read) vs mv/rm (write remote) vs bare (write remote)
	case "tag":
		return classifyPulumiEnvTag(rest)

	// Nested: env version → history (read) vs retract (admin remote) vs rollback/tag (write remote)
	case "version":
		return classifyPulumiEnvVersion(rest)
	}

	return Result{CLI: "pulumi", Subcommand: "env " + sub,
		Tier: types.TierUnknown, BaseTier: types.TierUnknown,
		BaseTierNote: fmt.Sprintf("pulumi env %s: not in classification DB", sub),
		Unknown:      true}
}

// classifyPulumiEnvRun classifies "pulumi env run <env> [--] <command...>".
// Like esc run, the resolved tier is the maximum of the outer tier
// (read-sensitive remote) and the recursively classified inner command.
func classifyPulumiEnvRun(args []string) Result {
	outerTier := types.TierReadSensitiveRemote

	// Find the inner command after "--".
	innerArgs := pulumiEnvRunInnerCommand(args)

	if len(innerArgs) == 0 {
		return Result{
			CLI: "pulumi", Subcommand: "env run",
			Tier: outerTier, BaseTier: outerTier,
			BaseTierNote: "pulumi env run (injects resolved secrets into command environment)",
		}
	}

	innerResult := Classify(innerArgs)

	if innerResult.Unknown {
		return Result{
			CLI: "pulumi", Subcommand: "env run",
			Tier: types.TierUnknown, BaseTier: outerTier,
			BaseTierNote: "pulumi env run (injects resolved secrets into command environment)",
			InnerResult:  &innerResult,
			Unknown:      true,
		}
	}

	resolved := types.Max(outerTier, innerResult.Tier)
	var effects []string
	if resolved != outerTier {
		effects = append(effects, fmt.Sprintf("inner command '%s %s' → %s",
			innerResult.CLI, innerResult.Subcommand, innerResult.Tier))
	}

	return Result{
		CLI: "pulumi", Subcommand: "env run",
		Tier: resolved, BaseTier: outerTier,
		BaseTierNote: "pulumi env run (injects resolved secrets into command environment)",
		FlagEffects:  effects,
		InnerResult:  &innerResult,
	}
}

// pulumiEnvRunInnerCommand extracts the inner command from "pulumi env run" args.
// args is everything after "run" (env name, flags, --, command).
func pulumiEnvRunInnerCommand(args []string) []string {
	// Look for explicit "--" separator.
	for i, arg := range args {
		if arg == "--" {
			if i+1 < len(args) {
				return args[i+1:]
			}
			return nil
		}
	}

	// No "--". Skip the environment name (first positional), then the rest
	// is the inner command.
	positionals := 0
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			continue
		}
		positionals++
		if positionals == 1 {
			// This is the environment name; everything after is the inner command.
			if i+1 < len(args) {
				return args[i+1:]
			}
			return nil
		}
	}

	return nil
}

// classifyPulumiEnvTag classifies "pulumi env tag" commands.
// Subcommands: get, ls/list (read); mv, rm/remove (write remote).
// A non-subcommand positional is an environment name — bare set → write remote.
func classifyPulumiEnvTag(args []string) Result {
	sub, _ := pulumiFirstPositional(args)

	switch sub {
	case "get", "ls", "list":
		return Result{CLI: "pulumi", Subcommand: "env tag " + sub,
			Tier: types.TierReadRemote, BaseTier: types.TierReadRemote,
			BaseTierNote: fmt.Sprintf("pulumi env tag %s", sub)}
	case "mv", "rm", "remove":
		return Result{CLI: "pulumi", Subcommand: "env tag " + sub,
			Tier: types.TierWriteRemote, BaseTier: types.TierWriteRemote,
			BaseTierNote: fmt.Sprintf("pulumi env tag %s", sub)}
	}

	// "" or an environment name like "myorg/myenv" → sets a tag
	return Result{CLI: "pulumi", Subcommand: "env tag",
		Tier: types.TierWriteRemote, BaseTier: types.TierWriteRemote,
		BaseTierNote: "pulumi env tag (creates/sets a tag on an environment)"}
}

// classifyPulumiEnvVersion classifies "pulumi env version" commands.
// Subcommands: history (read); retract (admin remote); rollback, tag (write remote).
// A non-subcommand positional is an env@version identifier → read.
func classifyPulumiEnvVersion(args []string) Result {
	sub, rest := pulumiFirstPositional(args)

	switch sub {
	case "history":
		return Result{CLI: "pulumi", Subcommand: "env version history",
			Tier: types.TierReadRemote, BaseTier: types.TierReadRemote,
			BaseTierNote: "pulumi env version history"}
	case "retract":
		return Result{CLI: "pulumi", Subcommand: "env version retract",
			Tier: types.TierAdminRemote, BaseTier: types.TierAdminRemote,
			BaseTierNote: "pulumi env version retract (retracts a revision; blocks future reads of it)"}
	case "rollback":
		return Result{CLI: "pulumi", Subcommand: "env version rollback",
			Tier: types.TierWriteRemote, BaseTier: types.TierWriteRemote,
			BaseTierNote: "pulumi env version rollback (creates a new revision restoring an old state)"}
	case "tag":
		return classifyPulumiEnvVersionTag(rest)
	}

	// "" or an env@version identifier → shows version info (read)
	return Result{CLI: "pulumi", Subcommand: "env version",
		Tier: types.TierReadRemote, BaseTier: types.TierReadRemote,
		BaseTierNote: "pulumi env version (read by default)"}
}

// classifyPulumiEnvVersionTag classifies "pulumi env version tag" commands.
// Subcommands: ls/list (read); rm/remove (write remote).
// A non-subcommand positional is an env@version identifier → creates a named tag → write remote.
func classifyPulumiEnvVersionTag(args []string) Result {
	sub, _ := pulumiFirstPositional(args)

	switch sub {
	case "ls", "list":
		return Result{CLI: "pulumi", Subcommand: "env version tag " + sub,
			Tier: types.TierReadRemote, BaseTier: types.TierReadRemote,
			BaseTierNote: fmt.Sprintf("pulumi env version tag %s", sub)}
	case "rm", "remove":
		return Result{CLI: "pulumi", Subcommand: "env version tag " + sub,
			Tier: types.TierWriteRemote, BaseTier: types.TierWriteRemote,
			BaseTierNote: fmt.Sprintf("pulumi env version tag %s", sub)}
	}

	// "" or an env@version identifier → creates/overwrites a named version tag
	return Result{CLI: "pulumi", Subcommand: "env version tag",
		Tier: types.TierWriteRemote, BaseTier: types.TierWriteRemote,
		BaseTierNote: "pulumi env version tag (creates/sets a named version tag)"}
}

// classifyPulumiStackTag classifies "pulumi stack tag" commands.
// Subcommands: get, ls (read); set, rm (write).
func classifyPulumiStackTag(args []string) Result {
	sub, _ := pulumiFirstPositional(args)

	switch sub {
	case "get", "ls", "list":
		return Result{CLI: "pulumi", Subcommand: "stack tag " + sub,
			Tier: types.TierReadRemote, BaseTier: types.TierReadRemote,
			BaseTierNote: fmt.Sprintf("pulumi stack tag %s", sub)}
	case "set", "rm", "remove":
		return Result{CLI: "pulumi", Subcommand: "stack tag " + sub,
			Tier: types.TierWriteRemote, BaseTier: types.TierWriteRemote,
			BaseTierNote: fmt.Sprintf("pulumi stack tag %s", sub)}
	}

	return Result{CLI: "pulumi", Subcommand: "stack tag",
		Tier: types.TierWriteRemote, BaseTier: types.TierWriteRemote,
		BaseTierNote: "pulumi stack tag (sets a tag by default)"}
}

// classifyPulumiConfigEnv classifies "pulumi config env" commands.
// Subcommands: ls (read); add, init, rm (write).
func classifyPulumiConfigEnv(args []string) Result {
	sub, _ := pulumiFirstPositional(args)

	switch sub {
	case "ls", "list":
		return Result{CLI: "pulumi", Subcommand: "config env " + sub,
			Tier: types.TierReadLocal, BaseTier: types.TierReadLocal,
			BaseTierNote: fmt.Sprintf("pulumi config env %s", sub)}
	case "add", "init", "rm", "remove":
		return Result{CLI: "pulumi", Subcommand: "config env " + sub,
			Tier: types.TierWriteLocal, BaseTier: types.TierWriteLocal,
			BaseTierNote: fmt.Sprintf("pulumi config env %s", sub)}
	}

	return Result{CLI: "pulumi", Subcommand: "config env",
		Tier: types.TierWriteLocal, BaseTier: types.TierWriteLocal,
		BaseTierNote: "pulumi config env (manages stack ESC environments)"}
}

// classifyPulumiPolicyGroup classifies "pulumi policy group" commands.
func classifyPulumiPolicyGroup(args []string) Result {
	sub, _ := pulumiFirstPositional(args)

	switch sub {
	case "ls", "list":
		return Result{CLI: "pulumi", Subcommand: "policy group " + sub,
			Tier: types.TierReadRemote, BaseTier: types.TierReadRemote,
			BaseTierNote: fmt.Sprintf("pulumi policy group %s", sub)}
	}

	return Result{CLI: "pulumi", Subcommand: "policy group",
		Tier: types.TierReadRemote, BaseTier: types.TierReadRemote,
		BaseTierNote: "pulumi policy group (list by default)"}
}

// pulumiFirstPositional returns the first non-flag token and the remaining args.
func pulumiFirstPositional(args []string) (string, []string) {
	for i, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			return arg, args[i+1:]
		}
	}
	return "", nil
}

// pulumiSubcommand scans args for the first non-flag token, skipping pulumi
// global flags (e.g., --stack <name>, --cwd <dir>, -v <level>).
// Returns (subcommand, args-after-subcommand).
func pulumiSubcommand(args []string) (string, []string) {
	// Global flags that consume the next argument as a value.
	flagsWithValue := map[string]bool{
		"--stack": true, "-s": true,
		"--cwd": true, "-C": true,
		"--color":       true,
		"--tracing":     true,
		"--profiling":   true,
		"--verbose":     true, "-v": true,
		"--config-file": true,
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsWithValue[arg] {
			i++ // skip value
			continue
		}
		// Embedded-value forms: --stack=<name>, --cwd=<dir>, etc.
		if strings.HasPrefix(arg, "--stack=") || strings.HasPrefix(arg, "--cwd=") ||
			strings.HasPrefix(arg, "--color=") || strings.HasPrefix(arg, "--tracing=") ||
			strings.HasPrefix(arg, "--profiling=") || strings.HasPrefix(arg, "--verbose=") ||
			strings.HasPrefix(arg, "--config-file=") {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue // standalone flag (e.g., --non-interactive, --logflow, --emoji)
		}
		return arg, args[i+1:]
	}
	return "", nil
}
