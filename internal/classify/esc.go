// Package classify — esc (Pulumi ESC standalone CLI) classifier.
//
// # Tier model
//
// The esc CLI manages Pulumi ESC environments (secrets and configuration).
// Most operations contact Pulumi Cloud (remote scope). The command tree
// mirrors "pulumi env" closely.
//
// # Nested command classification for "esc run"
//
// "esc run <env> -- <command...>" injects resolved secrets into the child
// process environment, then executes <command>. The resolved tier is the
// maximum of the outer tier (read-sensitive remote, since secrets are
// exposed) and the inner command's classified tier.
//
// If the inner command is from a supported CLI (kubectl, git, etc.), it is
// recursively classified. If unknown, the whole command is treated as
// unknown — we cannot assess risk we do not understand.
//
// Example escalations:
//   - esc run env -- kubectl get pods      → max(read-sensitive, read) = read-sensitive remote
//   - esc run env -- kubectl apply -f x    → max(read-sensitive, write) = write remote
//   - esc run env -- kubectl delete pod x  → max(read-sensitive, admin) = admin remote
package classify

import (
	"fmt"
	"strings"

	"github.com/rgharris/agent-perms/internal/types"
)

// escTiers maps esc subcommands to their permission tier.
var escTiers = map[string]types.Tier{
	// Read remote: query ESC environments in Pulumi Cloud
	"env":      types.TierReadRemote, // bare "esc env" lists environments
	"env ls":   types.TierReadRemote,
	"env list": types.TierReadRemote,
	"env get":  types.TierReadRemote,
	"env diff": types.TierReadRemote,

	// Read-sensitive remote: resolves and exposes secret values
	"env open": types.TierReadSensitiveRemote,

	// Write remote: modifies ESC environment data in Pulumi Cloud
	"env init":   types.TierWriteRemote,
	"env edit":   types.TierWriteRemote,
	"env set":    types.TierWriteRemote,
	"env clone":  types.TierWriteRemote,
	"env rotate": types.TierWriteRemote,

	// Admin remote: deletes an environment; irreversible
	"env rm":     types.TierAdminRemote,
	"env remove": types.TierAdminRemote,

	// Nested: env tag/version handled by pulumi's shared helpers if needed,
	// but for the standalone esc CLI we keep them in the map for simplicity.
	"env tag":     types.TierWriteRemote,
	"env tag get": types.TierReadRemote,
	"env tag ls":  types.TierReadRemote,
	"env tag mv":  types.TierWriteRemote,
	"env tag rm":  types.TierWriteRemote,

	"env version":          types.TierReadRemote,
	"env version history":  types.TierReadRemote,
	"env version retract":  types.TierAdminRemote,
	"env version rollback": types.TierWriteRemote,
	"env version tag":      types.TierWriteRemote,
	"env version tag ls":   types.TierReadRemote,
	"env version tag rm":   types.TierWriteRemote,

	// Auth: modifies local credentials
	"login":  types.TierWriteLocal,
	"logout": types.TierWriteLocal,

	// Informational
	"version":        types.TierReadLocal,
	"help":           types.TierReadLocal,
	"gen-completion": types.TierReadLocal,
	"whoami":         types.TierReadRemote,
}

// classifyEsc classifies an esc command given args after "esc".
func classifyEsc(args []string) Result {
	if len(args) == 0 {
		return Result{
			CLI:          "esc",
			Tier:         types.TierUnknown,
			Unknown:      true,
			BaseTierNote: "no esc subcommand provided",
		}
	}

	if hasHelpFlag(args) {
		sub := escFirstPositional(args)
		desc := "esc --help"
		if sub != "" && sub != "help" {
			desc = fmt.Sprintf("esc %s --help", sub)
		}
		return Result{
			CLI: "esc", Subcommand: sub,
			Tier: types.TierReadLocal, BaseTier: types.TierReadLocal,
			BaseTierNote: desc + " (help output; read-only)",
		}
	}

	sub := escFirstPositional(args)
	if sub == "" {
		return Result{
			CLI:          "esc",
			Tier:         types.TierUnknown,
			Unknown:      true,
			BaseTierNote: "no esc subcommand found",
		}
	}

	// "esc run" has special handling: nested command classification.
	if sub == "run" {
		return classifyEscRun(args)
	}

	// Try 4-token key first (env version tag ls/rm), then 3, 2, 1.
	tokens := escCollectPositionals(args, 4)
	for depth := len(tokens); depth >= 1; depth-- {
		key := strings.Join(tokens[:depth], " ")
		if tier, ok := escTiers[key]; ok {
			return Result{
				CLI:          "esc",
				Subcommand:   key,
				Tier:         tier,
				BaseTier:     tier,
				BaseTierNote: "esc " + key,
			}
		}
	}

	return Result{
		CLI:          "esc",
		Subcommand:   sub,
		Tier:         types.TierUnknown,
		BaseTier:     types.TierUnknown,
		BaseTierNote: "esc " + sub + ": not in classification DB",
		Unknown:      true,
	}
}

// classifyEscRun classifies "esc run <env> [--] <command...>".
// The outer tier is always read-sensitive remote (secrets are injected).
// The inner command is recursively classified and the resolved tier is
// the maximum of the two. If the inner command is unknown, the whole
// result is unknown.
func classifyEscRun(args []string) Result {
	outerTier := types.TierReadSensitiveRemote

	// Find the inner command. "esc run" takes positional args:
	//   esc run [flags] <env> [--] <command> [args...]
	// We look for "--" first. If not found, the inner command starts
	// after the environment name (first positional after "run").
	innerArgs := escFindInnerCommand(args)

	if len(innerArgs) == 0 {
		// No inner command — just "esc run <env>" with no command to execute.
		// Still read-sensitive because it resolves and displays secrets.
		return Result{
			CLI:          "esc",
			Subcommand:   "run",
			Tier:         outerTier,
			BaseTier:     outerTier,
			BaseTierNote: "esc run (resolves and exposes secret values)",
		}
	}

	innerResult := Classify(innerArgs)

	if innerResult.Unknown {
		return Result{
			CLI:          "esc",
			Subcommand:   "run",
			Tier:         types.TierUnknown,
			BaseTier:     outerTier,
			BaseTierNote: "esc run (resolves and exposes secret values)",
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
		CLI:          "esc",
		Subcommand:   "run",
		Tier:         resolved,
		BaseTier:     outerTier,
		BaseTierNote: "esc run (resolves and exposes secret values)",
		FlagEffects:  effects,
		InnerResult:  &innerResult,
	}
}

// escFindInnerCommand extracts the inner command tokens from "esc run" args.
// Handles both "esc run <env> -- <cmd...>" and "esc run <env> <cmd...>" forms.
func escFindInnerCommand(args []string) []string {
	// Skip past "run" to find the rest.
	foundRun := false
	startIdx := 0
	for i, arg := range args {
		if !foundRun {
			if arg == "run" {
				foundRun = true
				startIdx = i + 1
			}
			continue
		}
	}
	if !foundRun {
		return nil
	}
	rest := args[startIdx:]

	// Look for explicit "--" separator.
	for i, arg := range rest {
		if arg == "--" {
			if i+1 < len(rest) {
				return rest[i+1:]
			}
			return nil
		}
	}

	// No "--". Skip flags and the environment name, then the rest is the inner command.
	// esc run [flags] <env> <command> [args...]
	escRunFlagsWithValue := map[string]bool{
		"--env":     true,
		"--format":  true,
		"--lifetime": true,
		"-l":        true,
		"-f":        true,
	}

	positionals := 0
	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		if escRunFlagsWithValue[arg] {
			i++ // skip value
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		positionals++
		if positionals == 1 {
			// This is the environment name; everything after is the inner command.
			if i+1 < len(rest) {
				return rest[i+1:]
			}
			return nil
		}
	}

	return nil
}

// escFirstPositional returns the first non-flag token from args.
func escFirstPositional(args []string) string {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			return arg
		}
	}
	return ""
}

// escCollectPositionals returns up to maxN non-flag positional tokens from args.
func escCollectPositionals(args []string, maxN int) []string {
	var result []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		result = append(result, arg)
		if len(result) >= maxN {
			break
		}
	}
	return result
}
