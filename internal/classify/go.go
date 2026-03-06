// Package classify — go (Go toolchain) classifier.
//
// # Tier model: read local / write local / admin local
//
// All go operations work on local state. Read operations inspect or verify
// without modifying source or producing named artifacts. Write and admin
// operations modify local state (source files, go.mod, module cache).
// "go get" and "go mod download" fetch from the network but only write to
// the local filesystem. There is no remote entity that go can corrupt or
// force-overwrite, so all mutations use ScopeLocal and all reads use ScopeLocal.
//
// # Why "go clean -cache / -modcache" is admin
//
// Deleting the build cache or module cache is irreversible in the sense that
// rebuilding it requires network access and significant time. It does not
// destroy project source code, but it crosses the "notably hard to undo
// quickly" threshold that justifies admin — consistent with "git gc"
// (admin local) in the git classifier.
//
// # Why "go test" is write
//
// Tests execute arbitrary project code and can mutate local state via test
// side effects (files, sockets, subprocesses). To keep the model conservative,
// go test is treated as write local.
//
// # Why "go build" is flag-dependent
//
// "go build ./..." with no -o flag only checks compilation and writes to the
// build cache (read-equivalent side effect). "go build -o binary" produces a
// named artifact on the filesystem, which is a meaningful write.
package classify

import (
	"strings"

	"github.com/rgharris/agent-perms/internal/types"
)

// goSimpleTiers maps flag-independent go subcommands to their permission tier.
var goSimpleTiers = map[string]types.Tier{
	// Read local: inspect or verify without modifying source or producing named artifacts.
	"version": types.TierReadLocal,
	"list":    types.TierReadLocal,
	"doc":     types.TierReadLocal,
	"vet":     types.TierReadLocal,
	"test":    types.TierWriteLocal,
	"bug":     types.TierReadLocal, // opens browser with bug report template
	"tool":    types.TierWriteLocal, // runs Go tools; Go 1.24+ supports user-defined tools via go.mod

	// Write: produce artifacts, modify source, or mutate the module graph.
	"run":       types.TierWriteLocal,
	"fmt":       types.TierWriteLocal,
	"generate":  types.TierWriteLocal,
	"install":   types.TierWriteLocal,
	"get":       types.TierWriteLocal,
	"fix":       types.TierWriteLocal, // rewrites source to use new APIs
	"telemetry": types.TierWriteLocal, // manages telemetry settings
}

// classifyGo classifies a go command given args after "go".
// e.g., for "go build -o mybinary", args = ["build", "-o", "mybinary"].
func classifyGo(args []string) Result {
	if len(args) == 0 {
		return Result{
			CLI:          "go",
			Tier:         types.TierUnknown,
			Unknown:      true,
			BaseTierNote: "no go subcommand provided",
		}
	}

	sub := args[0]
	rest := args[1:]

	switch sub {
	case "build":
		return classifyGoBuild(rest)
	case "clean":
		return classifyGoClean(rest)
	case "env":
		return classifyGoEnv(rest)
	case "mod":
		return classifyGoMod(rest)
	case "work":
		return classifyGoWork(rest)
	}

	if tier, ok := goSimpleTiers[sub]; ok {
		return Result{
			CLI:          "go",
			Subcommand:   sub,
			Tier:         tier,
			BaseTier:     tier,
			BaseTierNote: "go " + sub,
		}
	}

	return Result{
		CLI:          "go",
		Subcommand:   sub,
		Tier:         types.TierUnknown,
		BaseTier:     types.TierUnknown,
		BaseTierNote: "go " + sub + ": not in classification DB",
		Unknown:      true,
	}
}

// classifyGoBuild classifies "go build" args.
// -o or -o=<file> produces a named artifact → write; otherwise → read.
func classifyGoBuild(args []string) Result {
	for _, arg := range args {
		if arg == "-o" || strings.HasPrefix(arg, "-o=") {
			return Result{
				CLI:          "go",
				Subcommand:   "build",
				Tier:         types.TierWriteLocal,
				BaseTier:     types.TierWriteLocal,
				BaseTierNote: "go build -o produces a named artifact",
			}
		}
	}
	return Result{
		CLI:          "go",
		Subcommand:   "build",
		Tier:         types.TierReadLocal,
		BaseTier:     types.TierReadLocal,
		BaseTierNote: "go build (no -o flag): compilation check only",
	}
}

// classifyGoClean classifies "go clean" args.
// -cache or -modcache deletes the build/module cache → admin; otherwise → write.
func classifyGoClean(args []string) Result {
	for _, arg := range args {
		if arg == "-cache" || arg == "-modcache" {
			return Result{
				CLI:          "go",
				Subcommand:   "clean",
				Tier:         types.TierAdminLocal,
				BaseTier:     types.TierAdminLocal,
				BaseTierNote: "go clean " + arg + ": deletes cache; requires network to rebuild",
			}
		}
	}
	return Result{
		CLI:          "go",
		Subcommand:   "clean",
		Tier:         types.TierWriteLocal,
		BaseTier:     types.TierWriteLocal,
		BaseTierNote: "go clean: removes build artifacts",
	}
}

// classifyGoEnv classifies "go env" args.
// -w or -u modifies the user environment file → write; otherwise → read.
func classifyGoEnv(args []string) Result {
	for _, arg := range args {
		if arg == "-w" || arg == "-u" {
			return Result{
				CLI:          "go",
				Subcommand:   "env",
				Tier:         types.TierWriteLocal,
				BaseTier:     types.TierWriteLocal,
				BaseTierNote: "go env " + arg + ": modifies user Go environment file",
			}
		}
	}
	return Result{
		CLI:          "go",
		Subcommand:   "env",
		Tier:         types.TierReadLocal,
		BaseTier:     types.TierReadLocal,
		BaseTierNote: "go env: reads environment variables",
	}
}

// classifyGoMod classifies "go mod <sub>" commands.
func classifyGoMod(args []string) Result {
	modTiers := map[string]types.Tier{
		"graph":    types.TierReadLocal,
		"why":      types.TierReadLocal,
		"verify":   types.TierReadLocal,
		"download": types.TierWriteLocal,
		"edit":     types.TierWriteLocal,
		"init":     types.TierWriteLocal,
		"tidy":     types.TierWriteLocal,
		"vendor":   types.TierWriteLocal,
	}

	// Find first non-flag token.
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if tier, ok := modTiers[arg]; ok {
			return Result{
				CLI:          "go",
				Subcommand:   "mod " + arg,
				Tier:         tier,
				BaseTier:     tier,
				BaseTierNote: "go mod " + arg,
			}
		}
		// Positional found but not in map.
		return Result{
			CLI:          "go",
			Subcommand:   "mod " + arg,
			Tier:         types.TierUnknown,
			BaseTier:     types.TierUnknown,
			BaseTierNote: "go mod " + arg + ": not in classification DB",
			Unknown:      true,
		}
	}

	return Result{
		CLI:          "go",
		Subcommand:   "mod",
		Tier:         types.TierUnknown,
		BaseTier:     types.TierUnknown,
		BaseTierNote: "go mod: no subcommand provided",
		Unknown:      true,
	}
}

// classifyGoWork classifies "go work <sub>" commands.
// init, use, edit, sync are all writes (modify go.work file).
func classifyGoWork(args []string) Result {
	// Find first non-flag token.
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		switch arg {
		case "init", "use", "edit", "sync", "vendor":
			return Result{
				CLI:          "go",
				Subcommand:   "work " + arg,
				Tier:         types.TierWriteLocal,
				BaseTier:     types.TierWriteLocal,
				BaseTierNote: "go work " + arg + ": modifies go.work file",
			}
		default:
			return Result{
				CLI:          "go",
				Subcommand:   "work " + arg,
				Tier:         types.TierUnknown,
				BaseTier:     types.TierUnknown,
				BaseTierNote: "go work " + arg + ": not in classification DB",
				Unknown:      true,
			}
		}
	}

	return Result{
		CLI:          "go",
		Subcommand:   "work",
		Tier:         types.TierUnknown,
		BaseTier:     types.TierUnknown,
		BaseTierNote: "go work: no subcommand provided",
		Unknown:      true,
	}
}
