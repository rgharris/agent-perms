// Package codex provides exec policy rule generation and AGENTS.md content
// for OpenAI Codex CLI integration with agent-perms.
package codex

import (
	"fmt"
	"strings"

	"github.com/rgharris/agent-perms/internal/classify"
)

// ProfileNames returns the list of available profile names.
func ProfileNames() []string {
	return []string{"write-local", "read", "full-write"}
}

// GenerateExecPolicy returns Starlark prefix_rule() entries for the given profile.
// The output is a complete .rules file suitable for ~/.codex/rules/agent-perms.rules.
func GenerateExecPolicy(profile string) (string, error) {
	switch profile {
	case "read":
		return generateReadOnly(), nil
	case "write-local":
		return generateLocalDev(), nil
	case "full-write":
		return generateFullWrite(), nil
	default:
		return "", fmt.Errorf("unknown profile %q. Available profiles: %s", profile, strings.Join(ProfileNames(), ", "))
	}
}

// GenerateAGENTSMD returns AGENTS.md content with agent-perms usage instructions
// for all supported CLIs.
func GenerateAGENTSMD() string {
	return agentsMDContent
}

// ValidateExecPolicy checks a .rules file for common agent-perms issues.
// Returns a list of diagnostic messages (empty if no issues found).
func ValidateExecPolicy(content string) []Diagnostic {
	var diags []Diagnostic

	lines := strings.Split(content, "\n")
	hasPrefixRule := false
	hasAgentPerms := false
	hasDirectCLIDeny := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.Contains(trimmed, "prefix_rule(") {
			hasPrefixRule = true
		}
		if strings.Contains(trimmed, "agent-perms") {
			hasAgentPerms = true
		}

		// Check for common mistakes: pattern with agent-perms but missing exec.
		if strings.Contains(trimmed, "agent-perms") &&
			strings.Contains(trimmed, "pattern") &&
			!strings.Contains(trimmed, "exec") {
			diags = append(diags, Diagnostic{
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("line %d: agent-perms pattern missing 'exec' — commands won't be classified", i+1),
			})
		}
	}

	// Check for rules that block direct CLI access.
	// Pattern and decision are on separate lines in Starlark, so scan
	// the full content for each CLI's pattern+forbidden combination.
	for _, cli := range classify.SupportedCLIs() {
		snippet := fmt.Sprintf(`pattern = ["%s"]`, cli)
		idx := strings.Index(content, snippet)
		if idx >= 0 {
			// Look within a reasonable window after the pattern line.
			end := idx + len(snippet) + 200
			if end > len(content) {
				end = len(content)
			}
			window := content[idx:end]
			if strings.Contains(window, `decision = "forbidden"`) {
				hasDirectCLIDeny = true
			}
		}
	}

	if hasPrefixRule && hasAgentPerms && !hasDirectCLIDeny {
		diags = append(diags, Diagnostic{
			Severity:   SeverityWarning,
			Message:    "no rules to block direct CLI access (bypassing agent-perms)",
			Suggestion: "Add prefix_rule(pattern=[\"<cli>\"], decision=\"forbidden\") for each supported CLI",
		})
	}

	if !hasPrefixRule {
		diags = append(diags, Diagnostic{
			Severity: SeverityWarning,
			Message:  "no prefix_rule() entries found",
		})
	}

	return diags
}

// Diagnostic represents a single validation issue.
type Diagnostic struct {
	Severity   Severity `json:"severity"`
	Message    string   `json:"message"`
	Suggestion string   `json:"suggestion,omitempty"`
}

// Severity indicates how serious a diagnostic is.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// generateReadOnly generates exec policy rules for the read-only profile.
// Allows: exec read local, exec read remote
// Prompts: exec write, exec read-sensitive
// Denies: exec admin, direct CLI access
func generateReadOnly() string {
	return buildRulesFile("read", []ruleSpec{
		{comment: "Allow all read operations through agent-perms"},
		{pattern: []string{"agent-perms", "exec", "read", "local", "--"}, decision: "allow"},
		{pattern: []string{"agent-perms", "exec", "read", "remote", "--"}, decision: "allow"},
		{comment: "Prompt for sensitive read operations (commands that expose secrets)"},
		{pattern: []string{"agent-perms", "exec", "read-sensitive"}, decision: "prompt"},
		{comment: "Prompt for write operations"},
		{pattern: []string{"agent-perms", "exec", "write"}, decision: "prompt"},
		{comment: "Block admin operations"},
		{pattern: []string{"agent-perms", "exec", "admin"}, decision: "forbidden", justification: "admin operations require human approval outside agent-perms"},
		{comment: "Allow explain and version (informational, always safe)"},
		{pattern: []string{"agent-perms", "explain"}, decision: "allow"},
		{pattern: []string{"agent-perms", "version"}, decision: "allow"},
	})
}

// generateLocalDev generates exec policy rules for the local-dev profile.
// Allows: exec read local/remote, exec write local
// Prompts: exec write remote, exec read-sensitive
// Denies: exec admin, direct CLI access
func generateLocalDev() string {
	return buildRulesFile("write-local", []ruleSpec{
		{comment: "Allow all read operations"},
		{pattern: []string{"agent-perms", "exec", "read", "local", "--"}, decision: "allow"},
		{pattern: []string{"agent-perms", "exec", "read", "remote", "--"}, decision: "allow"},
		{comment: "Prompt for sensitive read operations (commands that expose secrets)"},
		{pattern: []string{"agent-perms", "exec", "read-sensitive"}, decision: "prompt"},
		{comment: "Allow local write operations (git commit, go fmt, pulumi config, etc.)"},
		{pattern: []string{"agent-perms", "exec", "write", "local", "--"}, decision: "allow"},
		{comment: "Prompt for remote write operations (git push, gh pr create, pulumi up, etc.)"},
		{pattern: []string{"agent-perms", "exec", "write", "remote", "--"}, decision: "prompt"},
		{comment: "Block admin operations"},
		{pattern: []string{"agent-perms", "exec", "admin"}, decision: "forbidden", justification: "admin operations require human approval outside agent-perms"},
		{comment: "Allow explain and version (informational, always safe)"},
		{pattern: []string{"agent-perms", "explain"}, decision: "allow"},
		{pattern: []string{"agent-perms", "version"}, decision: "allow"},
	})
}

// generateFullWrite generates exec policy rules for the full-write profile.
// Allows: exec read local/remote, exec write (local and remote)
// Denies: exec admin, direct CLI access
func generateFullWrite() string {
	return buildRulesFile("full-write", []ruleSpec{
		{comment: "Allow all read operations"},
		{pattern: []string{"agent-perms", "exec", "read", "local", "--"}, decision: "allow"},
		{pattern: []string{"agent-perms", "exec", "read", "remote", "--"}, decision: "allow"},
		{comment: "Prompt for sensitive read operations (commands that expose secrets)"},
		{pattern: []string{"agent-perms", "exec", "read-sensitive"}, decision: "prompt"},
		{comment: "Allow all write operations (local and remote)"},
		{pattern: []string{"agent-perms", "exec", "write", "local", "--"}, decision: "allow"},
		{pattern: []string{"agent-perms", "exec", "write", "remote", "--"}, decision: "allow"},
		{comment: "Block admin operations"},
		{pattern: []string{"agent-perms", "exec", "admin"}, decision: "forbidden", justification: "admin operations require human approval outside agent-perms"},
		{comment: "Allow explain and version (informational, always safe)"},
		{pattern: []string{"agent-perms", "explain"}, decision: "allow"},
		{pattern: []string{"agent-perms", "version"}, decision: "allow"},
	})
}

// ruleSpec describes a single prefix_rule or comment to emit.
type ruleSpec struct {
	pattern       []string
	decision      string
	justification string
	comment       string // if non-empty, emits a comment line instead of a rule
}

// buildRulesFile assembles a complete .rules file from specs.
func buildRulesFile(profileName string, specs []ruleSpec) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# agent-perms exec policy rules — profile: %s\n", profileName))
	b.WriteString("# Generated by: agent-perms codex init\n")
	b.WriteString("# Place in: ~/.codex/rules/agent-perms.rules\n\n")

	// Emit agent-perms rules.
	for _, s := range specs {
		if s.pattern == nil {
			b.WriteString(fmt.Sprintf("# %s\n", s.comment))
			continue
		}
		writeRule(&b, s)
	}

	// Emit deny rules for direct CLI access.
	b.WriteString("\n# Block direct CLI access — force all commands through agent-perms\n")
	for _, cli := range classify.SupportedCLIs() {
		b.WriteString(fmt.Sprintf("prefix_rule(\n    pattern = [\"%s\"],\n    decision = \"forbidden\",\n    justification = \"use agent-perms exec instead of %s directly\",\n)\n\n", cli, cli))
	}

	return b.String()
}

// writeRule emits a single prefix_rule() call.
func writeRule(b *strings.Builder, s ruleSpec) {
	// Format the pattern list.
	patternParts := make([]string, len(s.pattern))
	for i, p := range s.pattern {
		patternParts[i] = fmt.Sprintf("%q", p)
	}

	b.WriteString("prefix_rule(\n")
	b.WriteString(fmt.Sprintf("    pattern = [%s],\n", strings.Join(patternParts, ", ")))
	b.WriteString(fmt.Sprintf("    decision = %q,\n", s.decision))
	if s.justification != "" {
		b.WriteString(fmt.Sprintf("    justification = %q,\n", s.justification))
	}
	b.WriteString(")\n\n")
}

// agentsMDContent is the AGENTS.md snippet for Codex, parallel to the
// CLAUDE.md content generated by cmdClaudeMD.
var agentsMDContent = `# agent-perms

Run CLI commands through agent-perms to enforce permission tiers. Declare the
action and scope upfront — agent-perms classifies the command and runs it only
if the claimed tier matches exactly. On denial it prints the required tier so
you can adjust.

    agent-perms exec <action> <scope> -- <cli> <subcommand> [args...]

You MUST wrap the following CLIs with agent-perms: esc, gh, git, go, kubectl, pulumi.
The user's permission rules will likely deny these commands when run directly.

The claimed tier must exactly match what the command requires. There is no
hierarchy — ` + "`write`" + ` does not cover ` + "`read`" + `, and ` + "`read`" + ` does not cover
` + "`read-sensitive`" + `. Each tier is independent. Use
` + "`agent-perms explain <cli> <subcommand>`" + ` to check the exact tier required.
Only skip this if you have already run explain for the same command in this
session or if the tier is shown in the examples below.

## gh (GitHub CLI)

Actions: read, read-sensitive, write, admin
Scopes: remote (all gh ops contact the GitHub API)

    agent-perms exec read remote -- gh pr list
    agent-perms exec read remote -- gh issue list --repo owner/repo
    agent-perms exec read-sensitive remote -- gh auth token
    agent-perms exec write remote -- gh pr create --title "fix" --body ""
    agent-perms exec write remote -- gh issue create --title "bug"
    agent-perms exec admin remote -- gh repo delete my-repo

## esc (Pulumi ESC CLI)

Actions: read, read-sensitive, write, admin
Scopes: local, remote

    agent-perms exec read remote -- esc env ls
    agent-perms exec read-sensitive remote -- esc env open myorg/prod
    agent-perms exec write remote -- esc env edit myorg/dev
    agent-perms exec admin remote -- esc env rm myorg/old-env

For "esc run", the tier is the maximum of esc run (read-sensitive remote,
since secrets are injected) and the inner command's tier:

    agent-perms exec read-sensitive remote -- esc run myorg/dev -- kubectl get pods
    agent-perms exec write remote -- esc run myorg/dev -- kubectl apply -f manifest.yaml
    agent-perms exec admin remote -- esc run myorg/dev -- kubectl delete pod my-pod

## git

Actions: read, write, admin
Scopes: local, remote

    agent-perms exec read local -- git log --oneline -10
    agent-perms exec read local -- git status
    agent-perms exec read remote -- git fetch origin
    agent-perms exec write local -- git commit -F /tmp/agent-perms-commit-msg.txt
    agent-perms exec write local -- git add -p
    agent-perms exec write remote -- git push origin main
    agent-perms exec admin local -- git reset --hard HEAD~1
    agent-perms exec admin remote -- git push --force

## pulumi

Actions: read, read-sensitive, write, admin
Scopes: local, remote

    agent-perms exec read remote -- pulumi preview
    agent-perms exec read remote -- pulumi stack ls
    agent-perms exec read local -- pulumi config get aws:region
    agent-perms exec read-sensitive remote -- pulumi env open myorg/prod
    agent-perms exec read-sensitive remote -- pulumi env run myorg/prod -- printenv
    agent-perms exec write local -- pulumi stack select dev
    agent-perms exec write local -- pulumi config set key value
    agent-perms exec write remote -- pulumi up
    agent-perms exec admin local -- pulumi state delete <urn>
    agent-perms exec admin remote -- pulumi destroy

## go (Go toolchain)

Actions: read, write, admin
Scopes: local (all go ops are local)

    agent-perms exec read local -- go version
    agent-perms exec write local -- go test ./...
    agent-perms exec read local -- go vet ./...
    agent-perms exec read local -- go build ./...
    agent-perms exec write local -- go build -o mybinary
    agent-perms exec write local -- go fmt ./...
    agent-perms exec write local -- go mod tidy
    agent-perms exec write local -- go get github.com/foo/bar@latest
    agent-perms exec admin local -- go clean -modcache

## kubectl (Kubernetes CLI)

Actions: read, read-sensitive, write, admin
Scopes: local, remote

    agent-perms exec read remote -- kubectl get pods
    agent-perms exec read remote -- kubectl describe deploy my-app
    agent-perms exec read remote -- kubectl logs my-pod
    agent-perms exec read-sensitive remote -- kubectl get secret my-secret -o yaml
    agent-perms exec read-sensitive remote -- kubectl port-forward pod/my-pod 8080:80
    agent-perms exec read local -- kubectl config view
    agent-perms exec write local -- kubectl config use-context prod
    agent-perms exec write remote -- kubectl apply -f manifest.yaml
    agent-perms exec write remote -- kubectl scale deploy/my-app --replicas=3
    agent-perms exec write remote -- kubectl exec -it my-pod -- bash
    agent-perms exec admin remote -- kubectl delete pod my-pod
    agent-perms exec admin remote -- kubectl drain node1
    agent-perms exec admin local -- kubectl config delete-context old-ctx
`
