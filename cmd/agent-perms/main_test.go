package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureStdout runs fn while capturing os.Stdout output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestGlobalFlagParsing(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantJSON     bool
		wantContains string
	}{
		{
			name:         "no global flags",
			args:         []string{"explain", "gh", "pr", "list"},
			wantJSON:     false,
			wantContains: "cli:        gh",
		},
		{
			name:         "global --json before subcommand",
			args:         []string{"--json", "explain", "gh", "pr", "list"},
			wantJSON:     true,
			wantContains: `"cli": "gh"`,
		},
		{
			name:         "global --on-unknown=allow before subcommand",
			args:         []string{"--on-unknown=allow", "explain", "gh", "pr", "list"},
			wantJSON:     false,
			wantContains: "cli:        gh",
		},
		{
			name:         "multiple global flags before subcommand",
			args:         []string{"--json", "--on-unknown=allow", "explain", "gh", "pr", "list"},
			wantJSON:     true,
			wantContains: `"cli": "gh"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureStdout(t, func() {
				code := run(tt.args)
				if code != 0 {
					t.Errorf("run(%v) = %d, want 0", tt.args, code)
				}
			})

			if tt.wantJSON {
				var m map[string]any
				if err := json.Unmarshal([]byte(output), &m); err != nil {
					t.Errorf("expected JSON output, got: %s", output)
				}
			}

			if tt.wantContains != "" {
				if !bytes.Contains([]byte(output), []byte(tt.wantContains)) {
					t.Errorf("output missing %q:\n%s", tt.wantContains, output)
				}
			}
		})
	}
}

func TestJsonFlagPassthrough(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantExec string
	}{
		{
			name:     "cli --json passes through to explain",
			args:     []string{"explain", "gh", "pr", "view", "123", "--json", "title,body"},
			wantExec: "agent-perms exec read remote -- gh pr view 123 --json title,body",
		},
		{
			name:     "cli --on-unknown flag passes through",
			args:     []string{"explain", "gh", "pr", "view", "123", "--on-unknown=allow"},
			wantExec: "agent-perms exec read remote -- gh pr view 123 --on-unknown=allow",
		},
		{
			name:     "global --json consumed, cli --json passes through",
			args:     []string{"--json", "explain", "gh", "pr", "view", "123", "--json", "title,body"},
			wantExec: "agent-perms exec read remote -- gh pr view 123 --json title,body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureStdout(t, func() {
				code := run(tt.args)
				if code != 0 {
					t.Errorf("run(%v) = %d, want 0", tt.args, code)
				}
			})

			if !bytes.Contains([]byte(output), []byte(tt.wantExec)) {
				t.Errorf("expected exec line %q in output:\n%s", tt.wantExec, output)
			}
		})
	}
}

func TestExecRequiresSeparator(t *testing.T) {
	// exec without -- should fail
	code := run([]string{"exec", "read", "gh", "pr", "list"})
	if code != 1 {
		t.Errorf("exec without -- should fail, got exit code %d", code)
	}
}

func TestParseTierTokens(t *testing.T) {
	tests := []struct {
		name    string
		tokens  []string
		wantStr string
		wantErr bool
	}{
		{name: "read local", tokens: []string{"read", "local"}, wantStr: "read local"},
		{name: "read remote", tokens: []string{"read", "remote"}, wantStr: "read remote"},
		{name: "read-sensitive local", tokens: []string{"read-sensitive", "local"}, wantStr: "read-sensitive local"},
		{name: "read-sensitive remote", tokens: []string{"read-sensitive", "remote"}, wantStr: "read-sensitive remote"},
		{name: "write + scope", tokens: []string{"write", "local"}, wantStr: "write local"},
		{name: "scope + write", tokens: []string{"local", "write"}, wantStr: "write local"},
		{name: "admin remote", tokens: []string{"admin", "remote"}, wantStr: "admin remote"},
		{name: "remote admin", tokens: []string{"remote", "admin"}, wantStr: "admin remote"},
		{name: "read without scope rejected", tokens: []string{"read"}, wantErr: true},
		{name: "read-sensitive without scope rejected", tokens: []string{"read-sensitive"}, wantErr: true},
		{name: "write without scope rejected", tokens: []string{"write"}, wantErr: true},
		{name: "admin without scope rejected", tokens: []string{"admin"}, wantErr: true},
		{name: "legacy read sensitive rejected", tokens: []string{"read", "sensitive"}, wantErr: true},
		{name: "legacy sensitive read rejected", tokens: []string{"sensitive", "read"}, wantErr: true},
		{name: "empty", tokens: []string{}, wantErr: true},
		{name: "invalid", tokens: []string{"foo"}, wantErr: true},
		{name: "too many", tokens: []string{"read", "local", "extra"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier, err := parseTierTokens(tt.tokens)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseTierTokens(%v) = %v, want error", tt.tokens, tier)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTierTokens(%v) error: %v", tt.tokens, err)
			}
			if got := tier.String(); got != tt.wantStr {
				t.Errorf("parseTierTokens(%v).String() = %q, want %q", tt.tokens, got, tt.wantStr)
			}
		})
	}
}

func TestClaudeInit(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantCode     int
		wantContains []string
	}{
		{
			name:     "default profile",
			args:     []string{"claude", "init"},
			wantCode: 0,
			wantContains: []string{
				"exec read local -- *",
				"exec read remote -- *",
				"agent-perms claude md",
			},
		},
		{
			name:     "read profile",
			args:     []string{"claude", "init", "--profile=read"},
			wantCode: 0,
			wantContains: []string{
				"exec read local -- *",
				"exec read remote -- *",
			},
		},
		{
			name:     "write-local profile",
			args:     []string{"claude", "init", "--profile=write-local"},
			wantCode: 0,
			wantContains: []string{
				"write local -- *",
			},
		},
		{
			name:     "full-write profile",
			args:     []string{"claude", "init", "--profile=full-write"},
			wantCode: 0,
			wantContains: []string{
				"write remote -- *",
				"write local -- *",
			},
		},
		{
			name:     "unknown profile",
			args:     []string{"claude", "init", "--profile=nonexistent"},
			wantCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureStdout(t, func() {
				code := run(tt.args)
				if code != tt.wantCode {
					t.Errorf("run(%v) = %d, want %d", tt.args, code, tt.wantCode)
				}
			})
			for _, s := range tt.wantContains {
				if !strings.Contains(output, s) {
					t.Errorf("output missing %q:\n%s", s, output)
				}
			}
			// Verify valid JSON on success
			if tt.wantCode == 0 {
				var m map[string]any
				if err := json.Unmarshal([]byte(output), &m); err != nil {
					t.Errorf("output is not valid JSON: %v\n%s", err, output)
				}
			}
		})
	}
}

func TestClaudeInitMerge(t *testing.T) {
	// Create a temp file with existing settings.
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	existing := `{
  "mcpServers": {"my-server": {}},
  "permissions": {
    "allow": ["Bash(some-tool *)"],
    "deny": []
  }
}`
	if err := os.WriteFile(path, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		code := run([]string{"claude", "init", "--merge=" + path})
		if code != 0 {
			t.Fatalf("merge failed with exit code %d", code)
		}
	})

	if !strings.Contains(output, "my-server") {
		t.Error("merged output missing mcpServers")
	}
	if !strings.Contains(output, "some-tool") {
		t.Error("merged output missing non-agent-perms allow rule")
	}
	if !strings.Contains(output, "exec read local -- *") {
		t.Error("merged output missing new agent-perms read local rule")
	}
	if !strings.Contains(output, "exec read remote -- *") {
		t.Error("merged output missing new agent-perms read remote rule")
	}
}

func TestClaudeValidate(t *testing.T) {
	// Create a temp file with valid settings.
	dir := t.TempDir()
	goodPath := filepath.Join(dir, "good.json")
	good := `{
  "permissions": {
    "allow": ["Bash(agent-perms exec read -- gh *)"],
    "deny": ["Bash(gh *)"]
  }
}`
	os.WriteFile(goodPath, []byte(good), 0644)

	code := run([]string{"claude", "validate", goodPath})
	if code != 0 {
		t.Errorf("validate of good settings returned %d, want 0", code)
	}

	// Create a temp file with bad settings.
	badPath := filepath.Join(dir, "bad.json")
	bad := `{
  "permissions": {
    "allow": ["Bash(agent-perms exec read gh *)"],
    "deny": []
  }
}`
	os.WriteFile(badPath, []byte(bad), 0644)

	code = run([]string{"claude", "validate", badPath})
	if code != 1 {
		t.Errorf("validate of bad settings returned %d, want 1", code)
	}
}

func TestClaudeValidateJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	content := `{
  "permissions": {
    "allow": ["Bash(agent-perms exec admin -- gh *)"],
    "deny": ["Bash(gh *)"]
  }
}`
	os.WriteFile(path, []byte(content), 0644)

	output := captureStdout(t, func() {
		code := run([]string{"--json", "claude", "validate", path})
		if code != 1 {
			t.Errorf("expected exit code 1, got %d", code)
		}
	})

	var diags []map[string]any
	if err := json.Unmarshal([]byte(output), &diags); err != nil {
		t.Fatalf("JSON output invalid: %v\n%s", err, output)
	}
	if len(diags) == 0 {
		t.Error("expected diagnostics in JSON output")
	}
}

func TestClaudeNoSubcommand(t *testing.T) {
	code := run([]string{"claude"})
	if code != 1 {
		t.Errorf("claude with no subcommand returned %d, want 1", code)
	}
}

func TestClaudeUnknownSubcommand(t *testing.T) {
	code := run([]string{"claude", "unknown"})
	if code != 1 {
		t.Errorf("claude unknown returned %d, want 1", code)
	}
}

func TestCodexMD(t *testing.T) {
	output := captureStdout(t, func() {
		code := run([]string{"codex", "md"})
		if code != 0 {
			t.Errorf("codex md returned %d, want 0", code)
		}
	})

	wantContains := []string{
		"# agent-perms",
		"agent-perms exec",
		"## gh",
		"## git",
		"## pulumi",
		"## go",
	}
	for _, s := range wantContains {
		if !strings.Contains(output, s) {
			t.Errorf("codex md output missing %q", s)
		}
	}
}

func TestCodexInit(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantCode     int
		wantContains []string
	}{
		{
			name:     "default profile",
			args:     []string{"codex", "init"},
			wantCode: 0,
			wantContains: []string{
				"prefix_rule(",
				"agent-perms",
				"exec", "read",
				`decision = "allow"`,
				"profile: read",
			},
		},
		{
			name:     "read profile",
			args:     []string{"codex", "init", "--profile=read"},
			wantCode: 0,
			wantContains: []string{
				`decision = "allow"`,
				`decision = "prompt"`,
				`decision = "forbidden"`,
			},
		},
		{
			name:     "write-local profile",
			args:     []string{"codex", "init", "--profile=write-local"},
			wantCode: 0,
			wantContains: []string{
				"write", "local",
				"write", "remote",
				"profile: write-local",
			},
		},
		{
			name:     "full-write profile",
			args:     []string{"codex", "init", "--profile=full-write"},
			wantCode: 0,
			wantContains: []string{
				"write", "local",
				"write", "remote",
				"profile: full-write",
			},
		},
		{
			name:     "unknown profile",
			args:     []string{"codex", "init", "--profile=nonexistent"},
			wantCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureStdout(t, func() {
				code := run(tt.args)
				if code != tt.wantCode {
					t.Errorf("run(%v) = %d, want %d", tt.args, code, tt.wantCode)
				}
			})
			for _, s := range tt.wantContains {
				if !strings.Contains(output, s) {
					t.Errorf("output missing %q:\n%s", s, output)
				}
			}
		})
	}
}

func TestCodexValidate(t *testing.T) {
	// Create a temp file with valid rules.
	dir := t.TempDir()
	goodPath := filepath.Join(dir, "good.rules")
	good := `prefix_rule(
    pattern = ["agent-perms", "exec", "read", "--"],
    decision = "allow",
)
prefix_rule(
    pattern = ["gh"],
    decision = "forbidden",
)
prefix_rule(
    pattern = ["git"],
    decision = "forbidden",
)
prefix_rule(
    pattern = ["go"],
    decision = "forbidden",
)
prefix_rule(
    pattern = ["pulumi"],
    decision = "forbidden",
)
`
	os.WriteFile(goodPath, []byte(good), 0644)

	code := run([]string{"codex", "validate", goodPath})
	if code != 0 {
		t.Errorf("validate of good rules returned %d, want 0", code)
	}

	// Create a temp file with rules missing direct CLI deny.
	badPath := filepath.Join(dir, "bad.rules")
	bad := `prefix_rule(
    pattern = ["agent-perms", "exec", "read", "--"],
    decision = "allow",
)
`
	os.WriteFile(badPath, []byte(bad), 0644)

	code = run([]string{"codex", "validate", badPath})
	if code != 1 {
		t.Errorf("validate of bad rules returned %d, want 1", code)
	}
}

func TestCodexValidateJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.rules")
	content := `prefix_rule(
    pattern = ["agent-perms", "exec", "read", "--"],
    decision = "allow",
)
`
	os.WriteFile(path, []byte(content), 0644)

	output := captureStdout(t, func() {
		code := run([]string{"--json", "codex", "validate", path})
		if code != 1 {
			t.Errorf("expected exit code 1, got %d", code)
		}
	})

	var diags []map[string]any
	if err := json.Unmarshal([]byte(output), &diags); err != nil {
		t.Fatalf("JSON output invalid: %v\n%s", err, output)
	}
	if len(diags) == 0 {
		t.Error("expected diagnostics in JSON output")
	}
}

func TestCodexInitWrite(t *testing.T) {
	// Use a temp dir as fake HOME so --write doesn't touch the real ~/.codex.
	fakeHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", fakeHome)
	defer os.Setenv("HOME", origHome)

	code := run([]string{"codex", "init", "--write"})
	if code != 0 {
		t.Fatalf("codex init --write returned %d, want 0", code)
	}

	// Check that both files were created.
	rulesPath := filepath.Join(fakeHome, ".codex", "rules", "agent-perms.rules")
	rulesData, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("rules file not created: %v", err)
	}
	if !strings.Contains(string(rulesData), "prefix_rule(") {
		t.Error("rules file missing prefix_rule content")
	}

	agentsPath := filepath.Join(fakeHome, ".codex", "AGENTS.md")
	agentsData, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("AGENTS.md not created: %v", err)
	}
	if !strings.Contains(string(agentsData), "agent-perms") {
		t.Error("AGENTS.md missing agent-perms content")
	}
}

func TestCodexNoSubcommand(t *testing.T) {
	code := run([]string{"codex"})
	if code != 1 {
		t.Errorf("codex with no subcommand returned %d, want 1", code)
	}
}

func TestCodexUnknownSubcommand(t *testing.T) {
	code := run([]string{"codex", "unknown"})
	if code != 1 {
		t.Errorf("codex unknown returned %d, want 1", code)
	}
}
