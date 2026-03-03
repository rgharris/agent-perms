package exec

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/rgharris/agent-perms/internal/types"
)

// captureStderr runs fn while capturing os.Stderr output.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStderr := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = origStderr }()

	fn()

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestDenialHumanOutput(t *testing.T) {
	output := captureStderr(t, func() {
		code := Run(types.TierReadRemote, []string{"gh", "repo", "delete", "myrepo"}, Options{})
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
	})

	if !strings.Contains(output, "denied") {
		t.Errorf("output missing 'denied': %s", output)
	}
	if !strings.Contains(output, "suggestion: agent-perms exec admin remote -- gh repo delete myrepo") {
		t.Errorf("output missing suggestion: %s", output)
	}
}

func TestDenialJSONOutput(t *testing.T) {
	output := captureStderr(t, func() {
		code := Run(types.TierReadRemote, []string{"gh", "repo", "delete", "myrepo"}, Options{JSON: true})
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
	})

	var d DenialJSON
	if err := json.Unmarshal([]byte(output), &d); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, output)
	}
	if d.Error != "denied" {
		t.Errorf("error = %q, want %q", d.Error, "denied")
	}
	if d.Required != "admin remote" {
		t.Errorf("required = %q, want %q", d.Required, "admin remote")
	}
	if d.Claimed != "read remote" {
		t.Errorf("claimed = %q, want %q", d.Claimed, "read remote")
	}
	if d.Suggestion != "agent-perms exec admin remote -- gh repo delete myrepo" {
		t.Errorf("suggestion = %q, want %q", d.Suggestion, "agent-perms exec admin remote -- gh repo delete myrepo")
	}
}

func TestDenialJSONIsIndented(t *testing.T) {
	output := captureStderr(t, func() {
		Run(types.TierReadRemote, []string{"gh", "repo", "delete", "myrepo"}, Options{JSON: true})
	})

	if !strings.Contains(output, "\n  ") {
		t.Errorf("JSON output is not indented:\n%s", output)
	}
}

func TestUnknownCommandDeny(t *testing.T) {
	output := captureStderr(t, func() {
		code := Run(types.TierReadRemote, []string{"docker", "ps"}, Options{OnUnknown: OnUnknownDeny})
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
	})

	if !strings.Contains(output, "unknown command") {
		t.Errorf("output missing 'unknown command': %s", output)
	}
}

func TestUnknownCommandDenyJSON(t *testing.T) {
	output := captureStderr(t, func() {
		code := Run(types.TierReadRemote, []string{"docker", "ps"}, Options{OnUnknown: OnUnknownDeny, JSON: true})
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
	})

	var d DenialJSON
	if err := json.Unmarshal([]byte(output), &d); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, output)
	}
	if d.Error != "unknown" {
		t.Errorf("error = %q, want %q", d.Error, "unknown")
	}
}

func TestSuggestionFormat(t *testing.T) {
	tests := []struct {
		name       string
		claimed    types.Tier
		cmd        []string
		wantSuffix string
	}{
		{
			name:       "gh admin",
			claimed:    types.TierReadRemote,
			cmd:        []string{"gh", "repo", "delete", "myrepo"},
			wantSuffix: "agent-perms exec admin remote -- gh repo delete myrepo",
		},
		{
			name:       "gh write",
			claimed:    types.TierReadRemote,
			cmd:        []string{"gh", "pr", "create", "--title", "fix"},
			wantSuffix: "agent-perms exec write remote -- gh pr create --title fix",
		},
		{
			name:       "git write remote",
			claimed:    types.TierReadRemote,
			cmd:        []string{"git", "push", "origin", "main"},
			wantSuffix: "agent-perms exec write remote -- git push origin main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureStderr(t, func() {
				Run(tt.claimed, tt.cmd, Options{})
			})
			if !strings.Contains(output, tt.wantSuffix) {
				t.Errorf("output missing suggestion %q:\n%s", tt.wantSuffix, output)
			}
		})
	}
}
