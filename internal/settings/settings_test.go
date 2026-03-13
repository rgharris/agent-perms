package settings

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGetProfile(t *testing.T) {
	for _, name := range ProfileNames() {
		t.Run(name, func(t *testing.T) {
			p, err := GetProfile(name)
			if err != nil {
				t.Fatalf("GetProfile(%q) error: %v", name, err)
			}
			if p.Name != name {
				t.Errorf("profile Name = %q, want %q", p.Name, name)
			}
			if len(p.Allow) == 0 {
				t.Error("profile has no allow rules")
			}
			if len(p.Deny) == 0 {
				t.Error("profile has no deny rules")
			}
			// All exec allow rules should contain --
			for _, r := range p.Allow {
				if strings.Contains(r, "exec") && !strings.Contains(r, "--") {
					t.Errorf("exec allow rule missing '--': %s", r)
				}
			}
		})
	}
}

func TestGetProfileUnknown(t *testing.T) {
	_, err := GetProfile("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

func TestReadOnlyProfile(t *testing.T) {
	p, _ := GetProfile("read")

	// Exec allow rules should only be read (read local or read remote)
	for _, r := range p.Allow {
		if strings.Contains(r, "exec") && !strings.Contains(r, "exec read ") {
			t.Errorf("read profile has non-read exec allow: %s", r)
		}
	}
	// Should include explain and version allows
	hasExplain := false
	hasVersion := false
	for _, r := range p.Allow {
		if strings.Contains(r, "explain") {
			hasExplain = true
		}
		if strings.Contains(r, "version") {
			hasVersion = true
		}
	}
	if !hasExplain {
		t.Error("read profile missing explain allow rule")
	}
	if !hasVersion {
		t.Error("read profile missing version allow rule")
	}
	// Should deny direct CLI access and admin
	hasAdminDeny := false
	for _, r := range p.Deny {
		if strings.Contains(r, "exec admin") {
			hasAdminDeny = true
		}
	}
	if !hasAdminDeny {
		t.Error("read profile missing admin deny rule")
	}
}

func TestLocalDevProfile(t *testing.T) {
	p, _ := GetProfile("write-local")

	hasWriteLocal := false
	for _, r := range p.Allow {
		if strings.Contains(r, "write local --") {
			hasWriteLocal = true
		}
	}
	if !hasWriteLocal {
		t.Error("write-local missing write local rule")
	}
	// Should NOT have unscoped write rule
	for _, r := range p.Allow {
		if strings.Contains(r, "exec write --") {
			t.Errorf("write-local should not have unscoped write rule, got: %s", r)
		}
	}
	// Exec rules should use wildcards, not per-CLI rules
	for _, r := range p.Allow {
		if strings.Contains(r, "exec") && !strings.Contains(r, "-- *)") {
			t.Errorf("write-local exec allow rule should use wildcard CLI, got: %s", r)
		}
	}
}

func TestFullWriteProfile(t *testing.T) {
	p, _ := GetProfile("full-write")

	hasWriteRemote := false
	for _, r := range p.Allow {
		if strings.Contains(r, "write remote --") {
			hasWriteRemote = true
		}
	}
	if !hasWriteRemote {
		t.Error("full-write missing write remote rule")
	}
	// Exec rules should use wildcards, not per-CLI rules
	for _, r := range p.Allow {
		if strings.Contains(r, "exec") && !strings.Contains(r, "-- *)") {
			t.Errorf("full-write exec allow rule should use wildcard CLI, got: %s", r)
		}
	}
}

func TestGenerateSettings(t *testing.T) {
	s, err := GenerateSettings("read")
	if err != nil {
		t.Fatal(err)
	}
	if s.Permissions == nil {
		t.Fatal("settings has no permissions")
	}
	if s.Hooks == nil {
		t.Fatal("settings has no hooks")
	}
	if len(s.Hooks.SessionStart) == 0 {
		t.Fatal("settings has no SessionStart hooks")
	}

	// Should serialize to valid JSON
	data, err := MarshalJSON(s)
	if err != nil {
		t.Fatal(err)
	}
	var check map[string]any
	if err := json.Unmarshal(data, &check); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
}

func TestValidateMissingSeparator(t *testing.T) {
	settings := `{
		"permissions": {
			"allow": ["Bash(agent-perms exec read gh *)"],
			"deny": ["Bash(gh *)"]
		}
	}`
	diags := Validate([]byte(settings))
	if len(diags) == 0 {
		t.Fatal("expected diagnostic for missing separator")
	}
	found := false
	for _, d := range diags {
		if d.Severity == SeverityError && strings.Contains(d.Message, "missing '--'") {
			found = true
		}
	}
	if !found {
		t.Error("expected error about missing '--' separator")
	}
}

func TestValidateEmptyExecRuleDoesNotPanic(t *testing.T) {
	settings := `{
		"permissions": {
			"allow": ["Bash(agent-perms exec    )"],
			"deny": ["Bash(gh *)"]
		}
	}`
	diags := Validate([]byte(settings))
	if len(diags) == 0 {
		t.Fatal("expected diagnostic for empty exec rule")
	}
	found := false
	for _, d := range diags {
		if d.Severity == SeverityError && strings.Contains(d.Message, "missing action before '--'") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected missing action diagnostic, got: %+v", diags)
	}
}

func TestValidateMalformedExecRuleShape(t *testing.T) {
	settings := `{
		"permissions": {
			"allow": ["Bash(agent-perms exec)"],
			"deny": ["Bash(gh *)"]
		}
	}`
	diags := Validate([]byte(settings))
	if len(diags) == 0 {
		t.Fatal("expected diagnostics for malformed exec rule shape")
	}
	found := false
	for _, d := range diags {
		if d.Severity == SeverityError && strings.Contains(d.Message, "malformed agent-perms exec rule") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected malformed-rule diagnostic, got: %+v", diags)
	}
}

func TestValidateAllowWithoutDeny(t *testing.T) {
	settings := `{
		"permissions": {
			"allow": ["Bash(agent-perms exec read -- gh *)"],
			"deny": []
		}
	}`
	diags := Validate([]byte(settings))
	found := false
	for _, d := range diags {
		if d.Severity == SeverityWarning && strings.Contains(d.Message, "no deny rule") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about missing deny bypass rule")
	}
}

func TestValidateUnsupportedCLI(t *testing.T) {
	settings := `{
		"permissions": {
			"allow": ["Bash(agent-perms exec read -- docker *)"],
			"deny": ["Bash(docker *)"]
		}
	}`
	diags := Validate([]byte(settings))
	found := false
	for _, d := range diags {
		if d.Severity == SeverityError && strings.Contains(d.Message, "unsupported CLI") {
			found = true
		}
	}
	if !found {
		t.Error("expected error about unsupported CLI")
	}
}

func TestValidateInvalidAction(t *testing.T) {
	settings := `{
		"permissions": {
			"allow": ["Bash(agent-perms exec readwrite -- gh *)"],
			"deny": ["Bash(gh *)"]
		}
	}`
	diags := Validate([]byte(settings))
	found := false
	for _, d := range diags {
		if d.Severity == SeverityError && strings.Contains(d.Message, "invalid action") {
			found = true
		}
	}
	if !found {
		t.Error("expected error about invalid action")
	}
}

func TestValidateInvalidScope(t *testing.T) {
	settings := `{
		"permissions": {
			"allow": ["Bash(agent-perms exec write foo -- gh *)"],
			"deny": ["Bash(gh *)"]
		}
	}`
	diags := Validate([]byte(settings))
	found := false
	for _, d := range diags {
		if d.Severity == SeverityError && strings.Contains(d.Message, "invalid action/scope") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about invalid scope, got: %+v", diags)
	}
}

func TestValidateAdminInAllow(t *testing.T) {
	settings := `{
		"permissions": {
			"allow": ["Bash(agent-perms exec admin -- gh *)"],
			"deny": ["Bash(gh *)"]
		}
	}`
	diags := Validate([]byte(settings))
	found := false
	for _, d := range diags {
		if d.Severity == SeverityWarning && strings.Contains(d.Message, "admin rule in allow") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about admin in allow")
	}
}

func TestValidateClean(t *testing.T) {
	// Wildcard allow rules with per-CLI deny rules.
	settings := `{
		"permissions": {
			"allow": [
				"Bash(agent-perms exec read -- *)"
			],
			"deny": [
				"Bash(gh *)",
				"Bash(git *)",
				"Bash(go *)",
				"Bash(kubectl *)",
				"Bash(pulumi *)",
				"Bash(agent-perms exec admin *)"
			]
		}
	}`
	diags := Validate([]byte(settings))
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics, got: %+v", diags)
	}
}

func TestValidateCleanPerCLI(t *testing.T) {
	// Per-CLI allow rules still work fine.
	settings := `{
		"permissions": {
			"allow": [
				"Bash(agent-perms exec read -- gh *)",
				"Bash(agent-perms exec read -- git *)",
				"Bash(agent-perms exec write local -- git *)"
			],
			"deny": [
				"Bash(gh *)",
				"Bash(git *)",
				"Bash(agent-perms exec admin *)"
			]
		}
	}`
	diags := Validate([]byte(settings))
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics, got: %+v", diags)
	}
}

func TestValidateNoPermissions(t *testing.T) {
	settings := `{"mcpServers": {}}`
	diags := Validate([]byte(settings))
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for settings without permissions, got: %+v", diags)
	}
}

func TestValidateInvalidJSON(t *testing.T) {
	diags := Validate([]byte("not json"))
	if len(diags) == 0 {
		t.Fatal("expected diagnostic for invalid JSON")
	}
	if diags[0].Severity != SeverityError {
		t.Error("expected error severity")
	}
}

func TestMerge(t *testing.T) {
	existing := `{
  "mcpServers": {"foo": {}},
  "permissions": {
    "allow": [
      "Bash(some-other-tool *)",
      "Bash(agent-perms exec read gh *)"
    ],
    "deny": [
      "Bash(something-else *)"
    ]
  }
}`
	result, err := Merge([]byte(existing), "read")
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("merged output is not valid JSON: %v", err)
	}

	// mcpServers should be preserved
	if _, ok := parsed["mcpServers"]; !ok {
		t.Error("mcpServers not preserved in merge")
	}

	// Old agent-perms rules should be removed
	out := string(result)
	if strings.Contains(out, "exec read gh") {
		t.Error("old agent-perms rule was not removed during merge")
	}

	// Non-agent-perms rules should be preserved
	if !strings.Contains(out, "some-other-tool") {
		t.Error("non-agent-perms allow rule was not preserved")
	}
	if !strings.Contains(out, "something-else") {
		t.Error("non-agent-perms deny rule was not preserved")
	}

	// New rules should be present
	if !strings.Contains(out, "exec read local -- *") {
		t.Error("new agent-perms read local rule not in merged output")
	}
	if !strings.Contains(out, "exec read remote -- *") {
		t.Error("new agent-perms read remote rule not in merged output")
	}
}

func TestMergeAddsHooks(t *testing.T) {
	existing := `{"permissions": {"allow": [], "deny": []}}`
	result, err := Merge([]byte(existing), "read")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), "agent-perms claude md") {
		t.Error("merge did not add SessionStart hook")
	}
}

func TestMergePreservesExistingHooks(t *testing.T) {
	existing := `{
  "permissions": {"allow": [], "deny": []},
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "echo hi"}]}]
  }
}`
	result, err := Merge([]byte(existing), "read")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), "echo hi") {
		t.Error("merge did not preserve existing SessionStart hook")
	}
}

func TestFilterNonAgentPerms(t *testing.T) {
	rules := []string{
		"Bash(some-tool *)",
		"Bash(agent-perms exec read -- gh *)",
		"Bash(another *)",
	}
	got := filterNonAgentPerms(rules)
	if len(got) != 2 {
		t.Fatalf("expected 2 rules, got %d: %v", len(got), got)
	}
	if got[0] != "Bash(some-tool *)" || got[1] != "Bash(another *)" {
		t.Errorf("unexpected filtered rules: %v", got)
	}
}

func TestValidateWildcardAllowMissingDeny(t *testing.T) {
	// Wildcard allow with no deny rules should warn for all supported CLIs.
	settings := `{
		"permissions": {
			"allow": ["Bash(agent-perms exec read -- *)"],
			"deny": []
		}
	}`
	diags := Validate([]byte(settings))
	warnCount := 0
	for _, d := range diags {
		if d.Severity == SeverityWarning && strings.Contains(d.Message, "no deny rule") {
			warnCount++
		}
	}
	if warnCount == 0 {
		t.Error("expected warnings about missing deny rules for wildcard allow")
	}
}

func TestValidateExternalDenySuppressesWarning(t *testing.T) {
	// Local file has allow rules but no deny — normally warns.
	// With external deny rules provided, the warning is suppressed.
	localSettings := `{
		"permissions": {
			"allow": ["Bash(agent-perms exec read -- gh *)"],
			"deny": []
		}
	}`
	externalDeny := []string{"Bash(gh *)"}
	diags := Validate([]byte(localSettings), externalDeny)
	for _, d := range diags {
		if d.Severity == SeverityWarning && strings.Contains(d.Message, "no deny rule") {
			t.Error("expected no 'allow without deny' warning when external deny covers the CLI")
		}
	}
}

func TestValidateExternalDenyStillWarnsWhenMissing(t *testing.T) {
	// Local file has allow for gh, external deny only covers git — should still warn.
	localSettings := `{
		"permissions": {
			"allow": ["Bash(agent-perms exec read -- gh *)"],
			"deny": []
		}
	}`
	externalDeny := []string{"Bash(git *)"}
	diags := Validate([]byte(localSettings), externalDeny)
	found := false
	for _, d := range diags {
		if d.Severity == SeverityWarning && strings.Contains(d.Message, "no deny rule") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about missing deny bypass rule when external deny doesn't cover the CLI")
	}
}

func TestValidateReadSensitive(t *testing.T) {
	// read-sensitive should be valid
	settings := `{
		"permissions": {
			"allow": ["Bash(agent-perms exec read-sensitive -- pulumi *)"],
			"deny": ["Bash(pulumi *)"]
		}
	}`
	diags := Validate([]byte(settings))
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for read-sensitive, got: %+v", diags)
	}
}

func TestValidateLegacyReadSensitiveRejected(t *testing.T) {
	settings := `{
		"permissions": {
			"allow": ["Bash(agent-perms exec read sensitive -- pulumi *)"],
			"deny": ["Bash(pulumi *)"]
		}
	}`
	diags := Validate([]byte(settings))
	found := false
	for _, d := range diags {
		if d.Severity == SeverityError && strings.Contains(d.Message, "read-sensitive") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about using read-sensitive, got: %+v", diags)
	}
}

func TestValidateTooManyTokens(t *testing.T) {
	settings := `{
		"permissions": {
			"allow": ["Bash(agent-perms exec read write local -- gh *)"],
			"deny": ["Bash(gh *)"]
		}
	}`
	diags := Validate([]byte(settings))
	found := false
	for _, d := range diags {
		if d.Severity == SeverityError && strings.Contains(d.Message, "too many tokens") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about too many tokens, got: %+v", diags)
	}
}
