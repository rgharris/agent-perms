package codex

import (
	"strings"
	"testing"
)

func TestGenerateExecPolicy_Profiles(t *testing.T) {
	tests := []struct {
		name         string
		profile      string
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:    "read allows read, prompts write, forbids admin",
			profile: "read",
			wantContains: []string{
				`"agent-perms", "exec", "read", "local", "--"`,
				`"agent-perms", "exec", "read", "remote", "--"`,
				`"agent-perms", "exec", "read-sensitive"`,
				`decision = "allow"`,
				`decision = "prompt"`,
				`decision = "forbidden"`,
				`"agent-perms", "exec", "write"`,
				`"agent-perms", "exec", "admin"`,
				// Direct CLI deny rules
				`pattern = ["gh"]`,
				`pattern = ["git"]`,
				`pattern = ["go"]`,
				`pattern = ["pulumi"]`,
				"profile: read",
			},
			wantAbsent: []string{
				`"agent-perms", "exec", "read-sensitive", "--"`,
			},
		},
		{
			name:    "write-local allows read + write local, prompts write remote",
			profile: "write-local",
			wantContains: []string{
				`"agent-perms", "exec", "read", "local", "--"`,
				`"agent-perms", "exec", "read", "remote", "--"`,
				`"agent-perms", "exec", "write", "local", "--"`,
				`"agent-perms", "exec", "write", "remote", "--"`,
				"profile: write-local",
			},
		},
		{
			name:    "full-write allows all writes",
			profile: "full-write",
			wantContains: []string{
				`"agent-perms", "exec", "read", "local", "--"`,
				`"agent-perms", "exec", "read", "remote", "--"`,
				`"agent-perms", "exec", "write", "local", "--"`,
				`"agent-perms", "exec", "write", "remote", "--"`,
				"profile: full-write",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := GenerateExecPolicy(tt.profile)
			if err != nil {
				t.Fatalf("GenerateExecPolicy(%q) error: %v", tt.profile, err)
			}
			for _, s := range tt.wantContains {
				if !strings.Contains(output, s) {
					t.Errorf("output missing %q:\n%s", s, output)
				}
			}
			for _, s := range tt.wantAbsent {
				if strings.Contains(output, s) {
					t.Errorf("output unexpectedly contains %q:\n%s", s, output)
				}
			}
		})
	}
}

func TestGenerateExecPolicy_UnknownProfile(t *testing.T) {
	_, err := GenerateExecPolicy("nonexistent")
	if err == nil {
		t.Error("expected error for unknown profile")
	}
}

func TestGenerateExecPolicy_DirectCLIDeny(t *testing.T) {
	// All profiles should deny direct CLI access.
	for _, profile := range ProfileNames() {
		t.Run(profile, func(t *testing.T) {
			output, err := GenerateExecPolicy(profile)
			if err != nil {
				t.Fatalf("GenerateExecPolicy(%q) error: %v", profile, err)
			}
			for _, cli := range []string{"gh", "git", "go", "pulumi"} {
				deny := `pattern = ["` + cli + `"]`
				if !strings.Contains(output, deny) {
					t.Errorf("profile %q missing direct deny for %s", profile, cli)
				}
				// Check that the deny rule for this CLI uses "forbidden".
				// The pattern and decision are on adjacent lines in the output.
				snippet := deny + ",\n    decision = \"forbidden\""
				if !strings.Contains(output, snippet) {
					t.Errorf("profile %q: direct %s deny should use 'forbidden' decision", profile, cli)
				}
			}
		})
	}
}

func TestGenerateExecPolicy_AdminForbidden(t *testing.T) {
	// All profiles should forbid admin.
	for _, profile := range ProfileNames() {
		t.Run(profile, func(t *testing.T) {
			output, err := GenerateExecPolicy(profile)
			if err != nil {
				t.Fatalf("GenerateExecPolicy(%q) error: %v", profile, err)
			}
			if !strings.Contains(output, `"admin"`) {
				t.Errorf("profile %q missing admin rule", profile)
			}
		})
	}
}

func TestGenerateAGENTSMD(t *testing.T) {
	output := GenerateAGENTSMD()

	wantContains := []string{
		"# agent-perms",
		"agent-perms exec <action>",
		"agent-perms explain",
		"## gh",
		"## git",
		"## pulumi",
		"## go",
		"exec read remote -- gh pr list",
		"exec write local -- git commit",
		"exec write remote -- pulumi up",
		"exec write local -- go mod tidy",
	}

	for _, s := range wantContains {
		if !strings.Contains(output, s) {
			t.Errorf("AGENTS.md missing %q", s)
		}
	}
}

func TestValidateExecPolicy_Good(t *testing.T) {
	good := `# agent-perms rules
prefix_rule(
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
	diags := ValidateExecPolicy(good)
	if len(diags) > 0 {
		t.Errorf("expected no diagnostics for valid rules, got: %v", diags)
	}
}

func TestValidateExecPolicy_MissingDirectDeny(t *testing.T) {
	bad := `prefix_rule(
    pattern = ["agent-perms", "exec", "read", "--"],
    decision = "allow",
)
`
	diags := ValidateExecPolicy(bad)
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "direct CLI access") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about missing direct CLI deny rules")
	}
}

func TestValidateExecPolicy_NoPrefixRules(t *testing.T) {
	diags := ValidateExecPolicy("# just a comment\n")
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "no prefix_rule()") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about no prefix_rule entries")
	}
}

func TestProfileNames(t *testing.T) {
	names := ProfileNames()
	if len(names) != 3 {
		t.Errorf("expected 3 profiles, got %d", len(names))
	}
	expected := map[string]bool{"read": true, "write-local": true, "full-write": true}
	for _, n := range names {
		if !expected[n] {
			t.Errorf("unexpected profile name: %s", n)
		}
	}
}
