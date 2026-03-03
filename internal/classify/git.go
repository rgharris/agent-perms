package classify

import (
	"fmt"
	"strings"

	"github.com/rgharris/agent-perms/internal/types"
)

// gitSimpleTiers maps git subcommands with unambiguous tiers (flag-independent).
var gitSimpleTiers = map[string]types.Tier{
	// Read local — query local state, no working tree mutations
	"status":    types.TierReadLocal,
	"log":       types.TierReadLocal,
	"diff":      types.TierReadLocal,
	"show":      types.TierReadLocal,
	"blame":     types.TierReadLocal,
	"grep":      types.TierReadLocal,
	"describe":  types.TierReadLocal,
	"shortlog":  types.TierReadLocal,
	"rev-parse": types.TierReadLocal,
	"rev-list":  types.TierReadLocal,
	"cat-file":  types.TierReadLocal,
	"ls-tree":   types.TierReadLocal,
	"ls-files":  types.TierReadLocal,

	// Read remote — contacts a remote server to download objects
	"fetch": types.TierReadRemote,

	// WriteLocal - create or modify local state
	"add":         types.TierWriteLocal,
	"commit":      types.TierWriteLocal,
	"rm":          types.TierWriteLocal,
	"mv":          types.TierWriteLocal,
	"merge":       types.TierWriteLocal,
	"rebase":      types.TierWriteLocal,
	"cherry-pick": types.TierWriteLocal,
	"apply":       types.TierWriteLocal,
	"am":          types.TierWriteLocal,
	"revert":      types.TierWriteLocal,
	"init":        types.TierWriteLocal,
	"clone":       types.TierWriteLocal,
	"pull":        types.TierWriteLocal, // fetch + merge into local branch
	"checkout":    types.TierWriteLocal, // branch switch or file restore
	"switch":      types.TierWriteLocal,
	"restore":     types.TierWriteLocal,

	// AdminLocal - destructive local ops; data loss risk
	"gc":            types.TierAdminLocal, // garbage collection; can delete loose objects
	"filter-branch": types.TierAdminLocal, // rewrites history; destructive
}

// classifyGit classifies a git command given args after "git".
// e.g., for "git commit -m 'fix'", args = ["commit", "-m", "fix"].
func classifyGit(args []string) Result {
	if len(args) == 0 {
		return Result{CLI: "git", Tier: types.TierUnknown, Unknown: true,
			BaseTierNote: "no git subcommand provided"}
	}

	// Skip git global flags (e.g., -C <dir>, --git-dir=<path>, --no-pager).
	sub, rest := gitSubcommand(args)
	if sub == "" {
		return Result{CLI: "git", Tier: types.TierUnknown, Unknown: true,
			BaseTierNote: "no git subcommand found after global flags"}
	}

	// Commands whose tier depends on flags or sub-subcommands.
	switch sub {
	case "push":
		return classifyGitPush(rest)
	case "branch":
		return classifyGitBranch(rest)
	case "tag":
		return classifyGitTag(rest)
	case "stash":
		return classifyGitStash(rest)
	case "remote":
		return classifyGitRemote(rest)
	case "clean":
		return classifyGitClean(rest)
	case "config":
		return classifyGitConfig(rest)
	case "reset":
		return classifyGitReset(rest)
	}

	if tier, ok := gitSimpleTiers[sub]; ok {
		return Result{
			CLI:          "git",
			Subcommand:   sub,
			Tier:         tier,
			BaseTier:     tier,
			BaseTierNote: fmt.Sprintf("git %s", sub),
		}
	}

	return Result{
		CLI:          "git",
		Subcommand:   sub,
		Tier:         types.TierUnknown,
		BaseTier:     types.TierUnknown,
		BaseTierNote: fmt.Sprintf("git %s: not in classification DB", sub),
		Unknown:      true,
	}
}

// gitSubcommand scans args for the first non-flag token, skipping git global
// flags (e.g., -C <dir>, --git-dir=<path>, -c key=value).
// Returns (subcommand, args-after-subcommand).
func gitSubcommand(args []string) (string, []string) {
	// Global flags that consume the next argument as a value.
	flagsWithValue := map[string]bool{
		"-C": true, "--git-dir": true, "--work-tree": true,
		"--namespace": true, "-c": true, "--exec-path": true,
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsWithValue[arg] {
			i++ // skip value
			continue
		}
		// Embedded-value forms: --git-dir=<path>, --work-tree=<path>, etc.
		if strings.HasPrefix(arg, "--git-dir=") || strings.HasPrefix(arg, "--work-tree=") ||
			strings.HasPrefix(arg, "--namespace=") {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue // standalone flag (e.g., --no-pager, -p, --paginate)
		}
		return arg, args[i+1:]
	}
	return "", nil
}

// classifyGitPush classifies "git push" based on flags and positional args.
//   - Default → write remote
//   - --force, -f, --force-with-lease, --force-if-includes, --delete, -d → admin remote
//   - Positional arg starting with ':' (colon-ref delete syntax) → admin remote
func classifyGitPush(args []string) Result {
	base := Result{
		CLI:          "git",
		Subcommand:   "push",
		BaseTier:     types.TierWriteRemote,
		BaseTierNote: "git push (writes to remote)",
	}

	adminFlags := map[string]bool{
		"--force": true, "-f": true,
		"--force-with-lease": true, "--force-if-includes": true,
		"--delete": true, "-d": true,
	}

	for _, arg := range args {
		if adminFlags[arg] {
			base.Tier = types.TierAdminRemote
			base.FlagEffects = []string{fmt.Sprintf("%s → destructive remote op → admin remote", arg)}
			return base
		}
		// Colon-ref delete syntax: git push origin :branch
		if !strings.HasPrefix(arg, "-") && strings.HasPrefix(arg, ":") {
			base.Tier = types.TierAdminRemote
			base.FlagEffects = []string{fmt.Sprintf("%s → colon-ref delete → admin remote", arg)}
			return base
		}
	}

	base.Tier = types.TierWriteRemote
	return base
}

// classifyGitBranch classifies "git branch" based on flags and positional args.
//   - Listing (no mutation flags, no branch-name arg) → read local
//   - Creating, deleting, renaming, or setting upstream → write local
func classifyGitBranch(args []string) Result {
	base := Result{
		CLI:          "git",
		Subcommand:   "branch",
		BaseTier:     types.TierReadLocal,
		BaseTierNote: "git branch (list by default)",
	}

	mutatingFlags := map[string]bool{
		"-d": true, "--delete": true,
		"-D": true,
		"-m": true, "-M": true, "--move": true,
		"-c": true, "-C": true, "--copy": true,
		"-u": true, "--set-upstream-to": true,
		"--unset-upstream": true,
	}

	hasPositional := false
	for _, arg := range args {
		if mutatingFlags[arg] || strings.HasPrefix(arg, "--set-upstream-to=") {
			base.Tier = types.TierWriteLocal
			base.FlagEffects = []string{fmt.Sprintf("%s → write local", arg)}
			return base
		}
		if !strings.HasPrefix(arg, "-") {
			hasPositional = true
		}
	}

	if hasPositional {
		base.Tier = types.TierWriteLocal
		base.FlagEffects = []string{"branch name argument → create → write local"}
		return base
	}

	base.Tier = types.TierReadLocal
	return base
}

// classifyGitTag classifies "git tag" based on flags and positional args.
//   - No args or -l/--list → read local
//   - Creating or deleting a tag → write local
func classifyGitTag(args []string) Result {
	base := Result{
		CLI:          "git",
		Subcommand:   "tag",
		BaseTier:     types.TierReadLocal,
		BaseTierNote: "git tag (list by default)",
	}

	for _, arg := range args {
		if arg == "-d" || arg == "--delete" {
			base.Tier = types.TierWriteLocal
			base.FlagEffects = []string{fmt.Sprintf("%s → write local", arg)}
			return base
		}
		if arg == "-l" || arg == "--list" {
			// Explicit list flag; remain read regardless of other args.
			base.Tier = types.TierReadLocal
			return base
		}
		if !strings.HasPrefix(arg, "-") {
			// Positional arg = tag name → creating a tag.
			base.Tier = types.TierWriteLocal
			base.FlagEffects = []string{"tag name argument → create → write local"}
			return base
		}
	}

	base.Tier = types.TierReadLocal
	return base
}

// classifyGitStash classifies "git stash" based on the sub-subcommand.
//   - list, show → read local
//   - everything else (push, pop, apply, drop, branch, clear) → write local
//   - no sub-subcommand defaults to "push" → write local
func classifyGitStash(args []string) Result {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			tier := types.TierWriteLocal
			if arg == "list" || arg == "show" {
				tier = types.TierReadLocal
			}
			return Result{
				CLI:          "git",
				Subcommand:   "stash",
				Tier:         tier,
				BaseTier:     tier,
				BaseTierNote: fmt.Sprintf("git stash %s", arg),
			}
		}
	}
	// No sub-subcommand: defaults to "git stash push".
	return Result{
		CLI:          "git",
		Subcommand:   "stash",
		Tier:         types.TierWriteLocal,
		BaseTier:     types.TierWriteLocal,
		BaseTierNote: "git stash (push by default)",
	}
}

// classifyGitRemote classifies "git remote" based on the sub-subcommand.
//   - No sub-subcommand, show, get-url → read local
//   - add, remove, rename, set-url, set-head, prune, update → write local
//     (these only modify .git/config locally, not the remote itself)
func classifyGitRemote(args []string) Result {
	base := Result{
		CLI:          "git",
		Subcommand:   "remote",
		BaseTier:     types.TierReadLocal,
		BaseTierNote: "git remote (list by default)",
	}

	writeLocalSubcmds := map[string]bool{
		"add": true, "remove": true, "rename": true,
		"set-url": true, "set-head": true, "prune": true,
		"update": true,
	}

	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			if writeLocalSubcmds[arg] {
				base.Tier = types.TierWriteLocal
				base.FlagEffects = []string{fmt.Sprintf("remote %s → modifies .git/config → write local", arg)}
			} else {
				base.Tier = types.TierReadLocal
			}
			return base
		}
	}

	base.Tier = types.TierReadLocal
	return base
}

// classifyGitClean classifies "git clean" based on flags.
//   - -n/--dry-run → read local (shows what would be deleted, no actual deletion)
//   - otherwise → admin local (deletes untracked files; irreversible)
func classifyGitClean(args []string) Result {
	for _, arg := range args {
		if arg == "-n" || arg == "--dry-run" {
			return Result{
				CLI:          "git",
				Subcommand:   "clean",
				Tier:         types.TierReadLocal,
				BaseTier:     types.TierAdminLocal,
				BaseTierNote: "git clean (deletes untracked files)",
				FlagEffects:  []string{fmt.Sprintf("%s → dry-run only → read local", arg)},
			}
		}
	}
	return Result{
		CLI:          "git",
		Subcommand:   "clean",
		Tier:         types.TierAdminLocal,
		BaseTier:     types.TierAdminLocal,
		BaseTierNote: "git clean (deletes untracked files)",
	}
}

// classifyGitConfig classifies "git config" based on flags.
//   - Explicit read flags (--list, --get, --get-all, --get-regexp) → read local
//   - --global or --system scope (without explicit read flag) → admin local
//   - otherwise (local set, unset, etc.) → write local
func classifyGitConfig(args []string) Result {
	base := Result{
		CLI:          "git",
		Subcommand:   "config",
		BaseTier:     types.TierReadLocal,
		BaseTierNote: "git config (read by default)",
	}

	isGlobal := false
	for _, arg := range args {
		switch arg {
		case "--list", "-l", "--get", "--get-all", "--get-regexp":
			// Explicit read operation; scope does not escalate.
			base.Tier = types.TierReadLocal
			return base
		case "--global", "--system":
			isGlobal = true
		}
	}

	if isGlobal {
		base.Tier = types.TierAdminLocal
		base.FlagEffects = []string{"--global/--system → modifies global config → admin local"}
		return base
	}

	// Local set or unset — modifies the repo's .git/config.
	base.Tier = types.TierWriteLocal
	base.FlagEffects = []string{"no explicit read flag → local config write → write local"}
	return base
}

// classifyGitReset classifies "git reset" based on flags.
//   - --hard → admin local (discards working tree changes; data loss risk)
//   - --soft, --mixed, or default → write local
func classifyGitReset(args []string) Result {
	for _, arg := range args {
		if arg == "--hard" {
			return Result{
				CLI:          "git",
				Subcommand:   "reset",
				Tier:         types.TierAdminLocal,
				BaseTier:     types.TierWriteLocal,
				BaseTierNote: "git reset (soft/mixed by default)",
				FlagEffects:  []string{"--hard → discards working tree changes → admin local"},
			}
		}
	}
	return Result{
		CLI:          "git",
		Subcommand:   "reset",
		Tier:         types.TierWriteLocal,
		BaseTier:     types.TierWriteLocal,
		BaseTierNote: "git reset (soft/mixed by default)",
	}
}
