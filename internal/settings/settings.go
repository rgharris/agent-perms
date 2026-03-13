// Package settings provides profile generation, validation, and merge logic
// for Claude Code settings.json files that use agent-perms rules.
package settings

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/rgharris/agent-perms/internal/classify"
	"github.com/rgharris/agent-perms/internal/types"
)

// Profile represents a named permission preset.
type Profile struct {
	Name        string
	Description string
	Allow       []string
	Deny        []string
}

// Settings represents the permissions and hooks section of a Claude settings.json.
type Settings struct {
	Permissions *Permissions `json:"permissions,omitempty"`
	Hooks       *Hooks       `json:"hooks,omitempty"`
}

// Permissions holds allow/deny rule arrays.
type Permissions struct {
	Allow []string `json:"allow"`
	Deny  []string `json:"deny"`
}

// Hooks holds hook definitions.
type Hooks struct {
	SessionStart []HookGroup `json:"SessionStart,omitempty"`
}

// HookGroup holds a group of hooks.
type HookGroup struct {
	Hooks []Hook `json:"hooks"`
}

// Hook represents a single hook entry.
type Hook struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// Severity indicates how serious a diagnostic is.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Diagnostic represents a single validation issue.
type Diagnostic struct {
	Severity   Severity `json:"severity"`
	Message    string   `json:"message"`
	Suggestion string   `json:"suggestion,omitempty"`
}

// ProfileNames returns the list of available profile names.
func ProfileNames() []string {
	return []string{"write-local", "read", "full-write"}
}

// ProfileDescriptions returns a map of profile name to human-readable description.
func ProfileDescriptions() map[string]string {
	return map[string]string{
		"read":        "Read all CLIs (not sensitive), writes prompt, deny admin",
		"write-local": "Read + local writes (git commit, go fmt, etc.), remote writes prompt, deny admin, sensitive reads prompt",
		"full-write":  "Read + write all CLIs (local and remote), deny admin, sensitive reads prompt",
	}
}

// GetProfile returns the profile with the given name, or an error if not found.
func GetProfile(name string) (Profile, error) {
	supportedCLIs := classify.SupportedCLIs()

	switch name {
	case "read":
		return buildReadOnly(supportedCLIs), nil
	case "write-local":
		return buildLocalDev(supportedCLIs), nil
	case "full-write":
		return buildFullWrite(supportedCLIs), nil
	default:
		return Profile{}, fmt.Errorf("unknown profile %q. Available profiles: %s", name, strings.Join(ProfileNames(), ", "))
	}
}

func buildReadOnly(clis []string) Profile {
	allow := []string{
		"Bash(agent-perms exec read local -- *)",
		"Bash(agent-perms exec read remote -- *)",
		"Bash(agent-perms explain *)",
		"Bash(agent-perms version)",
		"Read(" + commitMsgFile + ")",
		"Write(" + commitMsgFile + ")",
	}
	var deny []string
	for _, cli := range clis {
		deny = append(deny, fmt.Sprintf("Bash(%s *)", cli))
	}
	deny = append(deny, "Bash(agent-perms exec admin *)")
	return Profile{
		Name:        "read",
		Description: "Read all CLIs, writes prompt, deny admin",
		Allow:       allow,
		Deny:        deny,
	}
}

func buildLocalDev(clis []string) Profile {
	allow := []string{
		"Bash(agent-perms exec read local -- *)",
		"Bash(agent-perms exec read remote -- *)",
		"Bash(agent-perms exec write local -- *)",
		"Bash(agent-perms explain *)",
		"Bash(agent-perms version)",
		"Read(" + commitMsgFile + ")",
		"Write(" + commitMsgFile + ")",
	}
	var deny []string
	for _, cli := range clis {
		deny = append(deny, fmt.Sprintf("Bash(%s *)", cli))
	}
	deny = append(deny, "Bash(agent-perms exec admin *)")
	return Profile{
		Name:        "write-local",
		Description: "Read + write local, remote writes prompt, deny admin",
		Allow:       allow,
		Deny:        deny,
	}
}

func buildFullWrite(clis []string) Profile {
	allow := []string{
		"Bash(agent-perms exec read local -- *)",
		"Bash(agent-perms exec read remote -- *)",
		"Bash(agent-perms exec write local -- *)",
		"Bash(agent-perms exec write remote -- *)",
		"Bash(agent-perms explain *)",
		"Bash(agent-perms version)",
		"Read(" + commitMsgFile + ")",
		"Write(" + commitMsgFile + ")",
	}
	var deny []string
	for _, cli := range clis {
		deny = append(deny, fmt.Sprintf("Bash(%s *)", cli))
	}
	deny = append(deny, "Bash(agent-perms exec admin *)")
	return Profile{
		Name:        "full-write",
		Description: "Read + write all CLIs (local and remote), deny admin",
		Allow:       allow,
		Deny:        deny,
	}
}

// GenerateSettings creates a complete Settings struct for the given profile.
func GenerateSettings(profileName string) (*Settings, error) {
	profile, err := GetProfile(profileName)
	if err != nil {
		return nil, err
	}

	return &Settings{
		Permissions: &Permissions{
			Allow: profile.Allow,
			Deny:  profile.Deny,
		},
		Hooks: &Hooks{
			SessionStart: []HookGroup{
				{
					Hooks: []Hook{
						{Type: "command", Command: "agent-perms claude md"},
					},
				},
			},
		},
	}, nil
}

// MarshalJSON serializes settings to indented JSON.
func MarshalJSON(s *Settings) ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

// Merge reads an existing settings.json, merges agent-perms rules into it
// (preserving non-agent-perms entries), and returns the merged JSON.
func Merge(existing []byte, profileName string) ([]byte, error) {
	profile, err := GetProfile(profileName)
	if err != nil {
		return nil, err
	}

	// Parse existing JSON preserving unknown fields.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(existing, &raw); err != nil {
		return nil, fmt.Errorf("parsing existing settings: %w", err)
	}

	// Parse existing permissions if present.
	var perms map[string]json.RawMessage
	if p, ok := raw["permissions"]; ok {
		if err := json.Unmarshal(p, &perms); err != nil {
			return nil, fmt.Errorf("parsing permissions: %w", err)
		}
	} else {
		perms = make(map[string]json.RawMessage)
	}

	// Parse existing allow/deny arrays.
	var existingAllow []string
	if a, ok := perms["allow"]; ok {
		if err := json.Unmarshal(a, &existingAllow); err != nil {
			return nil, fmt.Errorf("parsing allow: %w", err)
		}
	}
	var existingDeny []string
	if d, ok := perms["deny"]; ok {
		if err := json.Unmarshal(d, &existingDeny); err != nil {
			return nil, fmt.Errorf("parsing deny: %w", err)
		}
	}

	// Remove old agent-perms rules from existing arrays.
	existingAllow = filterNonAgentPerms(existingAllow)
	existingDeny = filterNonAgentPerms(existingDeny)

	// Append new profile rules.
	mergedAllow := append(existingAllow, profile.Allow...)
	mergedDeny := append(existingDeny, profile.Deny...)

	// Serialize back.
	allowJSON, _ := json.Marshal(mergedAllow)
	denyJSON, _ := json.Marshal(mergedDeny)
	perms["allow"] = allowJSON
	perms["deny"] = denyJSON
	permsJSON, _ := json.Marshal(perms)
	raw["permissions"] = permsJSON

	// Merge hooks: add or update the agent-perms SessionStart hook.
	var hooks map[string]json.RawMessage
	if h, ok := raw["hooks"]; ok {
		if err := json.Unmarshal(h, &hooks); err != nil {
			hooks = make(map[string]json.RawMessage)
		}
	} else {
		hooks = make(map[string]json.RawMessage)
	}
	agentPermsHook := Hook{Type: "command", Command: "agent-perms claude md"}
	if ssRaw, ok := hooks["SessionStart"]; ok {
		var groups []HookGroup
		if err := json.Unmarshal(ssRaw, &groups); err == nil {
			groups = upsertAgentPermsHook(groups, agentPermsHook)
			sessionJSON, _ := json.Marshal(groups)
			hooks["SessionStart"] = sessionJSON
		}
	} else {
		sessionHook := []HookGroup{{Hooks: []Hook{agentPermsHook}}}
		sessionJSON, _ := json.Marshal(sessionHook)
		hooks["SessionStart"] = sessionJSON
	}
	hooksJSON, _ := json.Marshal(hooks)
	raw["hooks"] = hooksJSON

	return json.MarshalIndent(raw, "", "  ")
}

// upsertAgentPermsHook replaces any existing agent-perms hook in the
// SessionStart groups, or appends a new group if none is found.
func upsertAgentPermsHook(groups []HookGroup, hook Hook) []HookGroup {
	for i, g := range groups {
		for j, h := range g.Hooks {
			if strings.Contains(h.Command, "agent-perms") {
				groups[i].Hooks[j] = hook
				return groups
			}
		}
	}
	return append(groups, HookGroup{Hooks: []Hook{hook}})
}

// filterNonAgentPerms removes agent-perms rules from a rule slice,
// keeping all other entries intact.
func filterNonAgentPerms(rules []string) []string {
	var kept []string
	for _, r := range rules {
		if !isAgentPermsRule(r) {
			kept = append(kept, r)
		}
	}
	return kept
}

// commitMsgFile is the temp file path used for git commit messages.
const commitMsgFile = "/tmp/agent-perms-commit-msg.txt"

// isAgentPermsRule checks if a rule was generated by agent-perms.
// This includes agent-perms exec/explain rules, CLI bypass deny rules,
// and the commit message temp file Read/Write rules.
func isAgentPermsRule(rule string) bool {
	if strings.Contains(rule, "agent-perms") {
		return true
	}
	if rule == "Read("+commitMsgFile+")" || rule == "Write("+commitMsgFile+")" {
		return true
	}
	for _, cli := range classify.SupportedCLIs() {
		if rule == fmt.Sprintf("Bash(%s *)", cli) {
			return true
		}
	}
	return false
}

// agentPermsExecRe matches agent-perms exec rules inside Bash() patterns.
var agentPermsExecRe = regexp.MustCompile(`^Bash\(agent-perms\s+exec\s+(.+)\)$`)

// Validate checks a settings.json for agent-perms rule issues.
// Optional externalDeny slices provide deny rules from other settings files
// (e.g., global ~/.claude/settings.json) so that "allow without deny" checks
// consider the full merged rule set.
func Validate(data []byte, externalDeny ...[]string) []Diagnostic {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return []Diagnostic{{
			Severity: SeverityError,
			Message:  fmt.Sprintf("invalid JSON: %v", err),
		}}
	}

	permData, ok := raw["permissions"]
	if !ok {
		return nil // no permissions section, nothing to validate
	}

	var perms struct {
		Allow []string `json:"allow"`
		Deny  []string `json:"deny"`
	}
	if err := json.Unmarshal(permData, &perms); err != nil {
		return []Diagnostic{{
			Severity: SeverityError,
			Message:  fmt.Sprintf("invalid permissions: %v", err),
		}}
	}

	var diags []Diagnostic

	allRules := make([]string, 0, len(perms.Allow)+len(perms.Deny))
	allRules = append(allRules, perms.Allow...)
	allRules = append(allRules, perms.Deny...)

	supportedCLIs := make(map[string]bool)
	for _, cli := range classify.SupportedCLIs() {
		supportedCLIs[cli] = true
	}

	// Track which CLIs have allow rules for the "allow without deny" check.
	allowedCLIs := make(map[string]bool)

	for _, rule := range allRules {
		diags = append(diags, validateRule(rule, supportedCLIs)...)
	}

	// Check allow rules specifically.
	for _, rule := range perms.Allow {
		cli := extractCLIFromRule(rule)
		if cli == "*" {
			// Wildcard allow covers all supported CLIs.
			for c := range supportedCLIs {
				allowedCLIs[c] = true
			}
		} else if cli != "" {
			allowedCLIs[cli] = true
		}

		// Check 5: admin in allow
		if isAdminAllowRule(rule) {
			diags = append(diags, Diagnostic{
				Severity:   SeverityWarning,
				Message:    fmt.Sprintf("admin rule in allow auto-approves destructive ops: %s", rule),
				Suggestion: "Consider removing from allow or adding to deny instead",
			})
		}
	}

	// Merge file-local deny rules with any external deny rules (e.g., from global settings).
	allDeny := perms.Deny
	for _, ext := range externalDeny {
		allDeny = append(allDeny, ext...)
	}

	// Check 2: allow without matching deny
	for cli := range allowedCLIs {
		if !hasDenyBypass(allDeny, cli) {
			diags = append(diags, Diagnostic{
				Severity:   SeverityWarning,
				Message:    fmt.Sprintf("allow rules for %s but no deny rule to block direct %s access", cli, cli),
				Suggestion: fmt.Sprintf("Add \"Bash(%s *)\" to deny to force all access through agent-perms", cli),
			})
		}
	}

	return diags
}

// validateRule checks a single rule for structural issues.
func validateRule(rule string, supportedCLIs map[string]bool) []Diagnostic {
	if !isAgentPermsRule(rule) {
		return nil // not an agent-perms rule, skip
	}

	var diags []Diagnostic

	m := agentPermsExecRe.FindStringSubmatch(rule)
	if m == nil {
		if strings.Contains(rule, "agent-perms exec") {
			return []Diagnostic{{
				Severity:   SeverityError,
				Message:    fmt.Sprintf("malformed agent-perms exec rule: %s", rule),
				Suggestion: "Use format: Bash(agent-perms exec <action> <scope> -- <cli> ...)",
			}}
		}
		return nil // not an exec rule pattern
	}

	inner := m[1] // everything after "exec " inside Bash()
	parts := strings.Fields(inner)
	if len(parts) == 0 {
		return []Diagnostic{{
			Severity:   SeverityError,
			Message:    fmt.Sprintf("missing action before '--' in rule: %s", rule),
			Suggestion: "Add an action/scope and command, e.g. Bash(agent-perms exec read local -- gh pr list)",
		}}
	}

	// Check 1: missing -- separator.
	// Rules like "exec admin *" are valid deny globs (they catch all admin
	// exec attempts). Skip if the last token is "*" and there are ≤2
	// non-separator tokens — it's a catch-all glob, not a malformed exec rule.
	hasSep := false
	for _, p := range parts {
		if p == "--" {
			hasSep = true
			break
		}
	}
	if !hasSep {
		// Check if it looks like a catch-all deny glob: "<action> *" or
		// "<action> <scope> *" — i.e., only 2-3 parts where the last is *.
		if parts[len(parts)-1] == "*" && len(parts) <= 3 {
			allTierTokens := true
			for _, p := range parts[:len(parts)-1] {
				_, isAction := types.ParseAction(p)
				_, isScope := types.ParseScope(p)
				if !isAction && !isScope {
					allTierTokens = false
					break
				}
			}
			if allTierTokens {
				return nil // valid deny glob pattern
			}
		}
		diags = append(diags, Diagnostic{
			Severity:   SeverityError,
			Message:    fmt.Sprintf("missing '--' separator in rule: %s", rule),
			Suggestion: "Add '--' between action/scope and the CLI command",
		})
		return diags // can't reliably parse further
	}

	// Split on --
	sepIdx := -1
	for i, p := range parts {
		if p == "--" {
			sepIdx = i
			break
		}
	}
	tierTokens := parts[:sepIdx]
	cmdTokens := parts[sepIdx+1:]

	// Check 4: invalid action/scope
	diags = append(diags, validateTierTokens(tierTokens, rule)...)

	// Check 3: unsupported CLI (skip wildcard patterns like "-- *")
	if len(cmdTokens) > 0 {
		cli := cmdTokens[0]
		if cli != "*" && !supportedCLIs[cli] {
			diags = append(diags, Diagnostic{
				Severity:   SeverityError,
				Message:    fmt.Sprintf("unsupported CLI %q in rule: %s", cli, rule),
				Suggestion: fmt.Sprintf("Supported CLIs: %s", strings.Join(classify.SupportedCLIs(), ", ")),
			})
		}
	}

	return diags
}

// validateTierTokens checks that the action/scope tokens are valid.
func validateTierTokens(tokens []string, rule string) []Diagnostic {
	if len(tokens) == 0 {
		return []Diagnostic{{
			Severity:   SeverityError,
			Message:    fmt.Sprintf("missing action before '--' in rule: %s", rule),
			Suggestion: "Add an action (read, read-sensitive, write, admin) before '--'",
		}}
	}
	if len(tokens) > 2 {
		return []Diagnostic{{
			Severity:   SeverityError,
			Message:    fmt.Sprintf("too many tokens before '--' in rule: %s", rule),
			Suggestion: "Expected <action> or <action> <scope> before '--'",
		}}
	}

	if len(tokens) == 1 {
		if _, ok := types.ParseAction(tokens[0]); !ok {
			return []Diagnostic{{
				Severity:   SeverityError,
				Message:    fmt.Sprintf("invalid action %q in rule: %s", tokens[0], rule),
				Suggestion: "Valid actions: read, read-sensitive, write, admin",
			}}
		}
		return nil
	}

	// 2 tokens: one must be action, one scope (either order)
	_, aOk0 := types.ParseAction(tokens[0])
	_, sOk0 := types.ParseScope(tokens[0])
	_, aOk1 := types.ParseAction(tokens[1])
	_, sOk1 := types.ParseScope(tokens[1])

	// Check for legacy "read sensitive" form
	if (tokens[0] == "read" && tokens[1] == "sensitive") || (tokens[0] == "sensitive" && tokens[1] == "read") {
		return []Diagnostic{{
			Severity:   SeverityError,
			Message:    fmt.Sprintf("use 'read-sensitive' instead of 'read sensitive' in rule: %s", rule),
			Suggestion: "Replace 'read sensitive' with 'read-sensitive' (single hyphenated token)",
		}}
	}

	// action + scope or scope + action
	if (aOk0 && sOk1) || (sOk0 && aOk1) {
		return nil
	}

	return []Diagnostic{{
		Severity:   SeverityError,
		Message:    fmt.Sprintf("invalid action/scope %q %q in rule: %s", tokens[0], tokens[1], rule),
		Suggestion: "Expected <action> <scope>. Valid actions: read, read-sensitive, write, admin. Valid scopes: local, remote",
	}}
}

// extractCLIFromRule extracts the CLI name from an agent-perms exec rule.
func extractCLIFromRule(rule string) string {
	m := agentPermsExecRe.FindStringSubmatch(rule)
	if m == nil {
		return ""
	}
	inner := m[1]
	parts := strings.Fields(inner)
	for i, p := range parts {
		if p == "--" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// isAdminAllowRule checks if a rule auto-approves admin operations.
func isAdminAllowRule(rule string) bool {
	m := agentPermsExecRe.FindStringSubmatch(rule)
	if m == nil {
		return false
	}
	inner := m[1]
	parts := strings.Fields(inner)
	for i, p := range parts {
		if p == "--" {
			// Check tokens before --
			for _, t := range parts[:i] {
				if t == "admin" {
					return true
				}
			}
			return false
		}
	}
	return false
}

// hasDenyBypass checks if there's a deny rule blocking direct CLI access.
func hasDenyBypass(deny []string, cli string) bool {
	target := fmt.Sprintf("Bash(%s ", cli)
	for _, rule := range deny {
		if strings.HasPrefix(rule, target) {
			return true
		}
	}
	return false
}
