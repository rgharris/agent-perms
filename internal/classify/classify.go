// Package classify provides semantic permission tier classification for CLI commands.
package classify

import (
	"fmt"

	"github.com/rgharris/agent-perms/internal/types"
)

// Result holds the full classification output for a command.
type Result struct {
	CLI          string     // the CLI binary name (e.g., "gh")
	Subcommand   string     // the classified subcommand key (e.g., "pr list" or "api")
	Tier         types.Tier // final required permission tier
	BaseTier     types.Tier // tier from DB before any flag escalation
	BaseTierNote string     // human-readable note on the base tier
	FlagEffects  []string   // flag escalations applied (e.g., "--method DELETE → admin")
	Unknown      bool       // true if the subcommand was not found in the classification DB
}

// hasHelpFlag returns true if any arg is "--help" or "-h".
// Commands with help flags just print usage text and are always read-only.
func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" || arg == "help" {
			return true
		}
		if arg == "--" {
			return false // stop at separator
		}
	}
	return false
}

// SupportedCLIs returns the list of CLI names that agent-perms can classify.
func SupportedCLIs() []string {
	return []string{"gh", "git", "go", "pulumi"}
}

// Classify classifies a command specified as a slice of tokens.
// The first token must be the CLI name (e.g., "gh").
// Returns a Result describing the required permission tier.
func Classify(args []string) Result {
	if len(args) == 0 {
		return Result{
			Tier:         types.TierUnknown,
			Unknown:      true,
			BaseTierNote: "no command provided",
		}
	}

	cli := args[0]
	switch cli {
	case "gh":
		return classifyGH(args[1:])
	case "git":
		return classifyGit(args[1:])
	case "pulumi":
		return classifyPulumi(args[1:])
	case "go":
		return classifyGo(args[1:])
	default:
		return Result{
			CLI:          cli,
			Tier:         types.TierUnknown,
			Unknown:      true,
			BaseTierNote: fmt.Sprintf("unsupported CLI: %s", cli),
		}
	}
}
