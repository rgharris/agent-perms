// Package exec validates the claimed permission tier and runs a command transparently.
package exec

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/rgharris/agent-perms/internal/classify"
	"github.com/rgharris/agent-perms/internal/types"
)

// OnUnknown controls behavior when a command is not in the classification DB.
type OnUnknown int

const (
	OnUnknownDeny  OnUnknown = iota // deny and exit non-zero (default)
	OnUnknownAllow                  // run the command without classification
)

// Options controls exec behavior.
type Options struct {
	OnUnknown OnUnknown
	JSON      bool // output denials/errors as JSON
}

// DenialJSON is the machine-readable form of a denied execution.
type DenialJSON struct {
	Error      string `json:"error"`
	Claimed    string `json:"claimed"`
	Required   string `json:"required"`
	Command    string `json:"command"`
	Suggestion string `json:"suggestion"`
}

// Run validates claimedTier against the classified tier for cmd, then executes cmd.
// cmd is the full command slice (e.g., ["gh", "pr", "list"]).
// On success, the child process output passes through and Run returns its exit code.
// On denial, Run prints an error and returns 1.
func Run(claimedTier types.Tier, cmd []string, opts Options) int {
	result := classify.Classify(cmd)

	if result.Unknown {
		switch opts.OnUnknown {
		case OnUnknownDeny:
			cmdStr := strings.Join(cmd, " ")
			if opts.JSON {
				printJSON(DenialJSON{
					Error:    "unknown",
					Command:  cmdStr,
					Required: "unknown",
					Claimed:  claimedTier.String(),
				})
			} else {
				fmt.Fprintf(os.Stderr, "ERROR: unknown command '%s'. Not in classification DB.\nrequired_permission=unknown\n", cmdStr)
			}
			return 1
		case OnUnknownAllow:
			fmt.Fprintf(os.Stderr, "WARNING: '%s' not in classification DB, running anyway (--on-unknown=allow)\n", strings.Join(cmd, " "))
			return runCommand(cmd)
		}
	}

	if !claimedTier.Allows(result.Tier) {
		cmdStr := strings.Join(cmd, " ")
		suggestion := fmt.Sprintf("agent-perms exec %s -- %s", result.Tier, cmdStr)
		if opts.JSON {
			printJSON(DenialJSON{
				Error:      "denied",
				Claimed:    claimedTier.String(),
				Required:   result.Tier.String(),
				Command:    cmdStr,
				Suggestion: suggestion,
			})
		} else {
			fmt.Fprintf(os.Stderr, "ERROR: denied. '%s' requires '%s', claimed '%s'.\nsuggestion: %s\n",
				cmdStr, result.Tier, claimedTier, suggestion)
		}
		return 1
	}

	return runCommand(cmd)
}

// runCommand executes cmd with stdin/stdout/stderr passed through.
// Returns the child process exit code.
func runCommand(cmd []string) int {
	c := exec.Command(cmd[0], cmd[1:]...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	if err := c.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "ERROR: failed to run command: %v\n", err)
		return 1
	}
	return 0
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stderr)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}
