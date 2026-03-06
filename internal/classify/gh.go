package classify

import (
	"fmt"
	"strings"

	"github.com/rgharris/agent-perms/internal/types"
)

// ghTiers maps gh subcommand strings to their permission tier.
// Keys are "subcommand sub" for two-level commands, or "subcommand" for single-level.
// All gh commands contact the GitHub API, so all reads are remote.
var ghTiers = map[string]types.Tier{
	// PRs
	"pr list":     types.TierReadRemote,
	"pr view":     types.TierReadRemote,
	"pr status":   types.TierReadRemote,
	"pr checks":   types.TierReadRemote,
	"pr diff":     types.TierReadRemote,
	"pr checkout": types.TierReadRemote, // local branch switch only
	"pr create":   types.TierWriteRemote,
	"pr edit":     types.TierWriteRemote,
	"pr comment":  types.TierWriteRemote,
	"pr review":   types.TierWriteRemote,
	"pr merge":    types.TierWriteRemote,
	"pr close":    types.TierWriteRemote,
	"pr reopen":   types.TierWriteRemote,
	"pr ready":    types.TierWriteRemote,
	"pr lock":     types.TierAdminRemote, // moderation action
	"pr unlock":   types.TierAdminRemote,

	// Issues
	"issue list":     types.TierReadRemote,
	"issue view":     types.TierReadRemote,
	"issue status":   types.TierReadRemote,
	"issue create":   types.TierWriteRemote,
	"issue edit":     types.TierWriteRemote,
	"issue comment":  types.TierWriteRemote,
	"issue close":    types.TierWriteRemote,
	"issue reopen":   types.TierWriteRemote,
	"issue pin":      types.TierWriteRemote,
	"issue unpin":    types.TierWriteRemote,
	"issue delete":   types.TierAdminRemote, // irreversible
	"issue transfer": types.TierAdminRemote, // cross-repo
	"issue lock":     types.TierAdminRemote,

	// Repos
	"repo list":    types.TierReadRemote,
	"repo view":    types.TierReadRemote,
	"repo clone":   types.TierReadRemote,
	"repo create":  types.TierWriteRemote,
	"repo fork":    types.TierWriteRemote,
	"repo edit":    types.TierWriteRemote,
	"repo sync":    types.TierWriteRemote,
	"repo delete":  types.TierAdminRemote, // irreversible
	"repo rename":  types.TierAdminRemote, // breaks existing URLs
	"repo archive": types.TierAdminRemote, // irreversible

	// Releases
	"release list":     types.TierReadRemote,
	"release view":     types.TierReadRemote,
	"release download": types.TierReadRemote,
	"release create":   types.TierWriteRemote,
	"release edit":     types.TierWriteRemote,
	"release upload":   types.TierWriteRemote,
	"release delete":   types.TierAdminRemote, // irreversible

	// Workflows
	"workflow list":    types.TierReadRemote,
	"workflow view":    types.TierReadRemote,
	"workflow run":     types.TierWriteRemote,
	"workflow enable":  types.TierAdminRemote, // blocks/unblocks CI
	"workflow disable": types.TierAdminRemote,

	// Runs
	"run list":     types.TierReadRemote,
	"run view":     types.TierReadRemote,
	"run watch":    types.TierReadRemote,
	"run download": types.TierReadRemote,
	"run rerun":    types.TierWriteRemote,
	"run cancel":   types.TierWriteRemote,
	"run delete":   types.TierAdminRemote, // irreversible

	// Auth
	"auth status":  types.TierReadRemote,
	"auth token":   types.TierReadSensitiveRemote,
	"auth login":   types.TierAdminRemote, // security-critical
	"auth logout":  types.TierAdminRemote,
	"auth refresh": types.TierAdminRemote,
	"auth switch":  types.TierAdminRemote,

	// SSH & GPG Keys
	"ssh-key list":   types.TierReadRemote,
	"ssh-key add":    types.TierAdminRemote, // security-critical
	"ssh-key delete": types.TierAdminRemote,
	"gpg-key list":   types.TierReadRemote,
	"gpg-key add":    types.TierAdminRemote, // security-critical
	"gpg-key delete": types.TierAdminRemote,

	// Secrets & Variables
	"secret list":    types.TierReadRemote,
	"secret set":     types.TierAdminRemote, // security-critical
	"secret delete":  types.TierAdminRemote,
	"variable list":  types.TierReadRemote,
	"variable get":   types.TierReadRemote,
	"variable set":   types.TierWriteRemote,
	"variable delete": types.TierAdminRemote, // irreversible; can break CI/CD pipelines

	// Labels
	"label list":   types.TierReadRemote,
	"label create": types.TierWriteRemote,
	"label edit":   types.TierWriteRemote,
	"label clone":  types.TierWriteRemote,
	"label delete": types.TierAdminRemote, // irreversible

	// Gists
	"gist list":   types.TierReadRemote,
	"gist view":   types.TierReadRemote,
	"gist clone":  types.TierReadRemote,
	"gist create": types.TierWriteRemote,
	"gist edit":   types.TierWriteRemote,
	"gist rename": types.TierWriteRemote,
	"gist delete": types.TierAdminRemote, // irreversible

	// Projects (GitHub Projects v2)
	"project list":         types.TierReadRemote,
	"project view":         types.TierReadRemote,
	"project field-list":   types.TierReadRemote,
	"project item-list":    types.TierReadRemote,
	"project create":       types.TierWriteRemote,
	"project edit":         types.TierWriteRemote,
	"project copy":         types.TierWriteRemote,
	"project close":        types.TierWriteRemote,
	"project link":         types.TierWriteRemote,
	"project unlink":       types.TierWriteRemote,
	"project mark-template": types.TierWriteRemote,
	"project field-create": types.TierWriteRemote,
	"project item-add":     types.TierWriteRemote,
	"project item-create":  types.TierWriteRemote,
	"project item-edit":    types.TierWriteRemote,
	"project item-archive": types.TierWriteRemote,
	"project field-delete": types.TierAdminRemote, // irreversible
	"project item-delete":  types.TierAdminRemote, // irreversible
	"project delete":       types.TierAdminRemote, // irreversible

	// Rulesets
	"ruleset list":  types.TierReadRemote,
	"ruleset view":  types.TierReadRemote,
	"ruleset check": types.TierReadRemote,

	// Cache (GitHub Actions caches)
	"cache list":   types.TierReadRemote,
	"cache delete": types.TierAdminRemote, // irreversible

	// Codespaces
	"codespace list":    types.TierReadRemote,
	"codespace view":    types.TierReadRemote,
	"codespace logs":    types.TierReadRemote,
	"codespace ports":   types.TierReadRemote,
	"codespace ssh":     types.TierWriteRemote,
	"codespace code":    types.TierWriteRemote,
	"codespace jupyter": types.TierWriteRemote,
	"codespace cp":      types.TierWriteRemote,
	"codespace create":  types.TierWriteRemote,
	"codespace edit":    types.TierWriteRemote,
	"codespace rebuild": types.TierWriteRemote,
	"codespace stop":    types.TierWriteRemote,
	"codespace delete":  types.TierAdminRemote, // irreversible

	// Attestations
	"attestation download":     types.TierReadRemote,
	"attestation verify":       types.TierReadRemote,
	"attestation trusted-root": types.TierReadRemote,

	// Agent Tasks (preview)
	"agent-task list":   types.TierReadRemote,
	"agent-task view":   types.TierReadRemote,
	"agent-task create": types.TierWriteRemote,

	// Repo subresources (3-level: "repo autolink list" etc.)
	"repo autolink list":     types.TierReadRemote,
	"repo autolink view":     types.TierReadRemote,
	"repo autolink create":   types.TierWriteRemote,
	"repo autolink delete":   types.TierAdminRemote, // irreversible
	"repo deploy-key list":   types.TierReadRemote,
	"repo deploy-key add":    types.TierAdminRemote, // security-critical
	"repo deploy-key delete": types.TierAdminRemote, // security-critical
	"repo gitignore list":    types.TierReadRemote,
	"repo gitignore view":    types.TierReadRemote,
	"repo license list":      types.TierReadRemote,
	"repo license view":      types.TierReadRemote,
	"repo set-default":       types.TierWriteRemote,
	"repo unarchive":         types.TierAdminRemote, // reverses archive, changes repo state

	// Additional PR subcommands
	"pr update-branch": types.TierWriteRemote,
	"pr revert":        types.TierWriteRemote,

	// Additional Issue subcommands
	"issue develop": types.TierWriteRemote,
	"issue unlock":  types.TierAdminRemote,

	// Additional Release subcommands
	"release delete-asset": types.TierAdminRemote, // irreversible
	"release verify":       types.TierReadRemote,
	"release verify-asset": types.TierReadRemote,

	// Config
	"config set":         types.TierWriteRemote,
	"config clear-cache": types.TierWriteRemote,

	// Alias management
	"alias set":    types.TierWriteRemote,
	"alias delete": types.TierWriteRemote,
	"alias import": types.TierWriteRemote,

	// Extension management
	"extension browse":  types.TierReadRemote,
	"extension search":  types.TierReadRemote,
	"extension create":  types.TierWriteRemote,
	"extension install": types.TierWriteRemote,
	"extension upgrade": types.TierWriteRemote,
	"extension exec":    types.TierWriteRemote,
	"extension remove":  types.TierWriteRemote,

	// Auth
	"auth setup-git": types.TierWriteRemote,

	// Codespace ports (3-level)
	"codespace ports forward":    types.TierWriteRemote,
	"codespace ports visibility": types.TierWriteRemote,

	// Preview (debug/test commands)
	"preview prompter": types.TierReadRemote,

	// Search & Misc (all read remote — queries GitHub API)
	"search code":    types.TierReadRemote,
	"search commits": types.TierReadRemote,
	"search issues":  types.TierReadRemote,
	"search prs":     types.TierReadRemote,
	"search repos":   types.TierReadRemote,
	"status":         types.TierReadRemote,
	"browse":         types.TierReadRemote,
	"completion":     types.TierReadRemote,
	"org list":       types.TierReadRemote,
	"config list":    types.TierReadRemote,
	"config get":     types.TierReadRemote,
	"alias list":     types.TierReadRemote,
	"extension list": types.TierReadRemote,
}

// classifyGH classifies a gh command given args after "gh".
// e.g., for "gh pr list --state open", args = ["pr", "list", "--state", "open"].
func classifyGH(args []string) Result {
	if len(args) == 0 {
		return Result{CLI: "gh", Tier: types.TierUnknown, Unknown: true,
			BaseTierNote: "no gh subcommand provided"}
	}

	sub := args[0]

	// Special case: gh api has its own method-level classification.
	if sub == "api" {
		return classifyGHAPI(args[1:])
	}

	// Try three-token key first (e.g., "repo autolink list", "repo deploy-key add").
	if len(args) >= 3 && !strings.HasPrefix(args[1], "-") && !strings.HasPrefix(args[2], "-") {
		key := sub + " " + args[1] + " " + args[2]
		if tier, ok := ghTiers[key]; ok {
			return Result{
				CLI:          "gh",
				Subcommand:   key,
				Tier:         tier,
				BaseTier:     tier,
				BaseTierNote: fmt.Sprintf("gh %s", key),
			}
		}
	}

	// Try two-token key (e.g., "pr list", "issue create", "repo set-default").
	// Only combine if args[1] exists and is not a flag.
	if len(args) >= 2 && !strings.HasPrefix(args[1], "-") {
		key := sub + " " + args[1]
		if tier, ok := ghTiers[key]; ok {
			return Result{
				CLI:          "gh",
				Subcommand:   key,
				Tier:         tier,
				BaseTier:     tier,
				BaseTierNote: fmt.Sprintf("gh %s", key),
			}
		}
	}

	// Fall back to single-token key (e.g., "status", "browse").
	key := sub
	if tier, ok := ghTiers[key]; ok {
		return Result{
			CLI:          "gh",
			Subcommand:   key,
			Tier:         tier,
			BaseTier:     tier,
			BaseTierNote: fmt.Sprintf("gh %s", key),
		}
	}

	return Result{
		CLI:          "gh",
		Subcommand:   sub,
		Tier:         types.TierUnknown,
		BaseTier:     types.TierUnknown,
		BaseTierNote: fmt.Sprintf("gh %s: not in classification DB", sub),
		Unknown:      true,
	}
}

// classifyGHAPI classifies "gh api" commands by HTTP method.
// args is everything after "gh api".
//
// Method detection rules:
//   - --method <METHOD> or -X <METHOD> or -X<METHOD>: explicit method
//   - -f or -F flags with no explicit method: implicit POST (gh default)
//   - No method flag, no -f/-F: defaults to GET
//   - graphql endpoint with mutation in query: write
//   - graphql endpoint without mutation: read
func classifyGHAPI(args []string) Result {
	method := ""   // empty until explicitly set
	hasFFlags := false
	isGraphQL := false
	var fValues []string
	var flagEffects []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--method" || arg == "-X":
			if i+1 < len(args) {
				method = strings.ToUpper(args[i+1])
				i++
			}
		case strings.HasPrefix(arg, "--method="):
			method = strings.ToUpper(strings.TrimPrefix(arg, "--method="))
		case len(arg) > 2 && strings.HasPrefix(arg, "-X"):
			method = strings.ToUpper(arg[2:])
		case arg == "-f" || arg == "-F" || arg == "--raw-field" || arg == "--field":
			hasFFlags = true
			if i+1 < len(args) {
				fValues = append(fValues, args[i+1])
				i++
			}
		case arg == "--input":
			hasFFlags = true
			if i+1 < len(args) {
				i++ // consume the file path value
			}
		case strings.HasPrefix(arg, "-f=") || strings.HasPrefix(arg, "-F="):
			hasFFlags = true
			fValues = append(fValues, arg[strings.Index(arg, "=")+1:])
		// Detect graphql endpoint (positional arg, not a flag)
		case !strings.HasPrefix(arg, "-") && (arg == "graphql" || arg == "/graphql" || strings.HasSuffix(arg, "/graphql")):
			isGraphQL = true
		}
	}

	// GraphQL endpoint: classify by presence of "mutation" in query.
	if isGraphQL {
		for _, v := range fValues {
			if strings.Contains(v, "mutation") {
				flagEffects = append(flagEffects, "graphql mutation detected → write")
				return Result{
					CLI:          "gh",
					Subcommand:   "api",
					Tier:         types.TierWriteRemote,
					BaseTier:     types.TierReadRemote,
					BaseTierNote: "gh api graphql (read by default)",
					FlagEffects:  flagEffects,
				}
			}
		}
		return Result{
			CLI:          "gh",
			Subcommand:   "api",
			Tier:         types.TierReadRemote,
			BaseTier:     types.TierReadRemote,
			BaseTierNote: "gh api graphql query",
		}
	}

	// Determine effective HTTP method and build a single flag note.
	effectiveMethod := method
	if effectiveMethod == "" {
		if hasFFlags {
			// gh implicitly uses POST when -f/-F fields are provided.
			effectiveMethod = "POST"
			flagEffects = append(flagEffects, "-f/-F fields present → implicit POST → write remote")
		} else {
			effectiveMethod = "GET"
		}
	} else {
		// Build the escalation note for the explicit --method flag.
		switch strings.ToUpper(effectiveMethod) {
		case "GET":
			flagEffects = append(flagEffects, fmt.Sprintf("--method %s (explicit read)", effectiveMethod))
		case "POST", "PATCH", "PUT":
			flagEffects = append(flagEffects, fmt.Sprintf("--method %s → write remote", effectiveMethod))
		default:
			flagEffects = append(flagEffects, fmt.Sprintf("--method %s → admin remote", effectiveMethod))
		}
	}

	baseTierNote := "gh api (GET default)"

	switch strings.ToUpper(effectiveMethod) {
	case "GET":
		return Result{
			CLI:          "gh",
			Subcommand:   "api",
			Tier:         types.TierReadRemote,
			BaseTier:     types.TierReadRemote,
			BaseTierNote: baseTierNote,
			FlagEffects:  flagEffects,
		}
	case "POST", "PATCH", "PUT":
		return Result{
			CLI:          "gh",
			Subcommand:   "api",
			Tier:         types.TierWriteRemote,
			BaseTier:     types.TierReadRemote,
			BaseTierNote: baseTierNote,
			FlagEffects:  flagEffects,
		}
	case "DELETE":
		return Result{
			CLI:          "gh",
			Subcommand:   "api",
			Tier:         types.TierAdminRemote,
			BaseTier:     types.TierReadRemote,
			BaseTierNote: baseTierNote,
			FlagEffects:  flagEffects,
		}
	default:
		return Result{
			CLI:          "gh",
			Subcommand:   "api",
			Tier:         types.TierAdminRemote,
			BaseTier:     types.TierReadRemote,
			BaseTierNote: baseTierNote,
			FlagEffects:  append(flagEffects, fmt.Sprintf("unknown method %s → admin remote", effectiveMethod)),
		}
	}
}
