// Package classify — kubectl (Kubernetes CLI) classifier.
//
// # Tier model
//
// kubectl operations are split between local (kubeconfig manipulation) and
// remote (Kubernetes API server). Almost everything is remote; the main
// exceptions are `config` subcommands and a few informational commands.
//
// # Why "kubectl exec" is write remote
//
// exec runs arbitrary commands inside a container. This is analogous to
// "go test" executing arbitrary project code — commonly used for debugging,
// well-understood, but capable of mutation. admin would be too restrictive
// for standard debugging workflows.
//
// # Why "kubectl get secret" is read-sensitive remote
//
// "kubectl get" is normally read remote, but when the resource type is
// "secret" or "secrets", the output can expose secret data (especially with
// -o yaml/json). We detect the resource type argument and escalate.
//
// # Why "kubectl drain" is admin remote
//
// drain evicts all pods from a node, which can cause service disruption.
// This is significantly more destructive than cordon (which only prevents
// new scheduling) and warrants admin tier.
//
// # Why "kubectl proxy" and "kubectl port-forward" are read-sensitive remote
//
// These commands open network access to cluster resources without modifying
// state, but they bypass normal access boundaries — proxy exposes the full
// API server, port-forward gives direct pod access. The "sensitive" tier
// signals this elevated access level.
package classify

import (
	"fmt"
	"strings"

	"github.com/rgharris/agent-perms/internal/types"
)

// kubectlSimpleTiers maps flag-independent kubectl subcommands to their tier.
var kubectlSimpleTiers = map[string]types.Tier{
	// Read remote: query cluster state via API server
	"get":           types.TierReadRemote,
	"describe":      types.TierReadRemote,
	"logs":          types.TierReadRemote,
	"top":           types.TierReadRemote,
	"cluster-info":  types.TierReadRemote,
	"api-resources": types.TierReadRemote,
	"api-versions":  types.TierReadRemote,
	"explain":       types.TierReadRemote,
	"version":       types.TierReadRemote,
	"diff":          types.TierReadRemote,
	"wait":          types.TierReadRemote,
	"events":        types.TierReadRemote,

	// Read-sensitive remote: expose secrets or open privileged network access
	"proxy":        types.TierReadSensitiveRemote, // opens proxy to full API server
	"port-forward": types.TierReadSensitiveRemote, // tunnels to pod ports

	// Read local: informational, no cluster contact
	"completion": types.TierReadLocal,
	"kustomize":  types.TierReadLocal, // renders templates locally
	"options":    types.TierReadLocal,

	// Write remote: mutate cluster state
	"create":    types.TierWriteRemote,
	"apply":     types.TierWriteRemote,
	"edit":      types.TierWriteRemote,
	"patch":     types.TierWriteRemote,
	"set":       types.TierWriteRemote,
	"scale":     types.TierWriteRemote,
	"autoscale": types.TierWriteRemote,
	"expose":    types.TierWriteRemote,
	"run":       types.TierWriteRemote,
	"label":     types.TierWriteRemote,
	"annotate":  types.TierWriteRemote,
	"cp":        types.TierWriteRemote, // copies files to/from containers
	"exec":      types.TierWriteRemote, // runs commands in containers
	"attach":    types.TierWriteRemote,
	"taint":     types.TierWriteRemote,
	"cordon":    types.TierWriteRemote, // marks node unschedulable
	"uncordon":  types.TierWriteRemote, // allows node scheduling
	"debug":     types.TierWriteRemote,

	// Admin remote: destructive or highly disruptive
	"delete": types.TierAdminRemote,
	"drain":  types.TierAdminRemote, // evicts all pods from a node; can disrupt services
}

// classifyKubectl classifies a kubectl command given args after "kubectl".
func classifyKubectl(args []string) Result {
	if len(args) == 0 {
		return Result{
			CLI:          "kubectl",
			Tier:         types.TierUnknown,
			Unknown:      true,
			BaseTierNote: "no kubectl subcommand provided",
		}
	}

	if hasHelpFlag(args) {
		sub, _ := kubectlSubcommand(args)
		desc := "kubectl --help"
		if sub != "" && sub != "help" {
			desc = fmt.Sprintf("kubectl %s --help", sub)
		}
		return Result{
			CLI: "kubectl", Subcommand: sub,
			Tier: types.TierReadLocal, BaseTier: types.TierReadLocal,
			BaseTierNote: desc + " (help output; read-only)",
		}
	}

	sub, rest := kubectlSubcommand(args)
	if sub == "" {
		return Result{
			CLI:          "kubectl",
			Tier:         types.TierUnknown,
			Unknown:      true,
			BaseTierNote: "no kubectl subcommand found after global flags",
		}
	}

	// Commands with dedicated handlers for flag-dependent or nested classification.
	switch sub {
	case "config":
		return classifyKubectlConfig(rest)
	case "get":
		return classifyKubectlGet(rest)
	case "describe":
		return classifyKubectlDescribe(rest)
	case "cluster-info":
		return classifyKubectlClusterInfo(rest)
	case "rollout":
		return classifyKubectlRollout(rest)
	case "replace":
		return classifyKubectlReplace(rest)
	case "auth":
		return classifyKubectlAuth(rest)
	case "certificate":
		return classifyKubectlCertificate(rest)
	case "version":
		return classifyKubectlVersion(rest)
	case "plugin":
		return classifyKubectlPlugin(rest)
	}

	if tier, ok := kubectlSimpleTiers[sub]; ok {
		return Result{
			CLI:          "kubectl",
			Subcommand:   sub,
			Tier:         tier,
			BaseTier:     tier,
			BaseTierNote: "kubectl " + sub,
		}
	}

	return Result{
		CLI:          "kubectl",
		Subcommand:   sub,
		Tier:         types.TierUnknown,
		BaseTier:     types.TierUnknown,
		BaseTierNote: "kubectl " + sub + ": not in classification DB",
		Unknown:      true,
	}
}

// classifyKubectlConfig classifies "kubectl config" subcommands.
// Config commands manipulate the local kubeconfig file, so they're all local scope.
func classifyKubectlConfig(args []string) Result {
	configTiers := map[string]types.Tier{
		// Read local: inspect kubeconfig
		"view":            types.TierReadLocal,
		"get-contexts":    types.TierReadLocal,
		"get-clusters":    types.TierReadLocal,
		"get-users":       types.TierReadLocal,
		"current-context": types.TierReadLocal,

		// Write local: modify kubeconfig
		"set-context":     types.TierWriteLocal,
		"set-cluster":     types.TierWriteLocal,
		"set-credentials": types.TierWriteLocal,
		"use-context":     types.TierWriteLocal,
		"set":             types.TierWriteLocal,
		"rename-context":  types.TierWriteLocal,

		// Admin local: delete kubeconfig entries
		"delete-context": types.TierAdminLocal,
		"delete-cluster": types.TierAdminLocal,
		"delete-user":    types.TierAdminLocal,
		"unset":          types.TierAdminLocal,
	}

	sub := kubectlFirstPositional(args)
	if sub == "" {
		return Result{
			CLI: "kubectl", Subcommand: "config",
			Tier: types.TierReadLocal, BaseTier: types.TierReadLocal,
			BaseTierNote: "kubectl config (view by default)",
		}
	}

	if tier, ok := configTiers[sub]; ok {
		return Result{
			CLI:          "kubectl",
			Subcommand:   "config " + sub,
			Tier:         tier,
			BaseTier:     tier,
			BaseTierNote: "kubectl config " + sub,
		}
	}

	return Result{
		CLI:          "kubectl",
		Subcommand:   "config " + sub,
		Tier:         types.TierUnknown,
		BaseTier:     types.TierUnknown,
		BaseTierNote: "kubectl config " + sub + ": not in classification DB",
		Unknown:      true,
	}
}

// classifyKubectlGet classifies "kubectl get" commands.
// Most resources are read remote, but secrets escalate to read-sensitive remote.
func classifyKubectlGet(args []string) Result {
	resource := kubectlResourceType(args)

	if resource == "secret" || resource == "secrets" {
		return Result{
			CLI:          "kubectl",
			Subcommand:   "get",
			Tier:         types.TierReadSensitiveRemote,
			BaseTier:     types.TierReadRemote,
			BaseTierNote: "kubectl get (read remote by default)",
			FlagEffects:  []string{"resource type 'secret' → read-sensitive remote"},
		}
	}

	return Result{
		CLI:          "kubectl",
		Subcommand:   "get",
		Tier:         types.TierReadRemote,
		BaseTier:     types.TierReadRemote,
		BaseTierNote: "kubectl get",
	}
}

// classifyKubectlDescribe classifies "kubectl describe" commands.
// Like get, secrets escalate to read-sensitive remote.
func classifyKubectlDescribe(args []string) Result {
	resource := kubectlResourceType(args)

	if resource == "secret" || resource == "secrets" {
		return Result{
			CLI:          "kubectl",
			Subcommand:   "describe",
			Tier:         types.TierReadSensitiveRemote,
			BaseTier:     types.TierReadRemote,
			BaseTierNote: "kubectl describe (read remote by default)",
			FlagEffects:  []string{"resource type 'secret' → read-sensitive remote"},
		}
	}

	return Result{
		CLI:          "kubectl",
		Subcommand:   "describe",
		Tier:         types.TierReadRemote,
		BaseTier:     types.TierReadRemote,
		BaseTierNote: "kubectl describe",
	}
}

// classifyKubectlClusterInfo classifies "kubectl cluster-info" commands.
// "cluster-info dump" produces extensive cluster state → read-sensitive remote.
func classifyKubectlClusterInfo(args []string) Result {
	sub := kubectlFirstPositional(args)

	if sub == "dump" {
		return Result{
			CLI:          "kubectl",
			Subcommand:   "cluster-info dump",
			Tier:         types.TierReadSensitiveRemote,
			BaseTier:     types.TierReadRemote,
			BaseTierNote: "kubectl cluster-info (read remote by default)",
			FlagEffects:  []string{"dump → read-sensitive remote (exposes extensive cluster state)"},
		}
	}

	return Result{
		CLI:          "kubectl",
		Subcommand:   "cluster-info",
		Tier:         types.TierReadRemote,
		BaseTier:     types.TierReadRemote,
		BaseTierNote: "kubectl cluster-info",
	}
}

// classifyKubectlRollout classifies "kubectl rollout" subcommands.
func classifyKubectlRollout(args []string) Result {
	rolloutTiers := map[string]types.Tier{
		"status":  types.TierReadRemote,
		"history": types.TierReadRemote,
		"restart": types.TierWriteRemote,
		"undo":    types.TierWriteRemote,
		"resume":  types.TierWriteRemote,
		"pause":   types.TierWriteRemote,
	}

	sub := kubectlFirstPositional(args)
	if sub == "" {
		return Result{
			CLI: "kubectl", Subcommand: "rollout",
			Tier: types.TierUnknown, BaseTier: types.TierUnknown,
			BaseTierNote: "kubectl rollout: no subcommand provided",
			Unknown:      true,
		}
	}

	if tier, ok := rolloutTiers[sub]; ok {
		return Result{
			CLI:          "kubectl",
			Subcommand:   "rollout " + sub,
			Tier:         tier,
			BaseTier:     tier,
			BaseTierNote: "kubectl rollout " + sub,
		}
	}

	return Result{
		CLI:          "kubectl",
		Subcommand:   "rollout " + sub,
		Tier:         types.TierUnknown,
		BaseTier:     types.TierUnknown,
		BaseTierNote: "kubectl rollout " + sub + ": not in classification DB",
		Unknown:      true,
	}
}

// classifyKubectlReplace classifies "kubectl replace" commands.
// --force deletes and recreates the resource → admin remote.
func classifyKubectlReplace(args []string) Result {
	for _, arg := range args {
		if arg == "--force" {
			return Result{
				CLI:          "kubectl",
				Subcommand:   "replace",
				Tier:         types.TierAdminRemote,
				BaseTier:     types.TierWriteRemote,
				BaseTierNote: "kubectl replace (write remote by default)",
				FlagEffects:  []string{"--force → admin remote (deletes and recreates resource)"},
			}
		}
		if arg == "--" {
			break
		}
	}

	return Result{
		CLI:          "kubectl",
		Subcommand:   "replace",
		Tier:         types.TierWriteRemote,
		BaseTier:     types.TierWriteRemote,
		BaseTierNote: "kubectl replace",
	}
}

// classifyKubectlAuth classifies "kubectl auth" subcommands.
func classifyKubectlAuth(args []string) Result {
	authTiers := map[string]types.Tier{
		"can-i":     types.TierReadRemote,
		"whoami":    types.TierReadRemote,
		"reconcile": types.TierWriteRemote,
	}

	sub := kubectlFirstPositional(args)
	if sub == "" {
		return Result{
			CLI: "kubectl", Subcommand: "auth",
			Tier: types.TierUnknown, BaseTier: types.TierUnknown,
			BaseTierNote: "kubectl auth: no subcommand provided",
			Unknown:      true,
		}
	}

	if tier, ok := authTiers[sub]; ok {
		return Result{
			CLI:          "kubectl",
			Subcommand:   "auth " + sub,
			Tier:         tier,
			BaseTier:     tier,
			BaseTierNote: "kubectl auth " + sub,
		}
	}

	return Result{
		CLI:          "kubectl",
		Subcommand:   "auth " + sub,
		Tier:         types.TierUnknown,
		BaseTier:     types.TierUnknown,
		BaseTierNote: "kubectl auth " + sub + ": not in classification DB",
		Unknown:      true,
	}
}

// classifyKubectlCertificate classifies "kubectl certificate" subcommands.
func classifyKubectlCertificate(args []string) Result {
	sub := kubectlFirstPositional(args)

	switch sub {
	case "approve":
		return Result{
			CLI: "kubectl", Subcommand: "certificate approve",
			Tier: types.TierWriteRemote, BaseTier: types.TierWriteRemote,
			BaseTierNote: "kubectl certificate approve",
		}
	case "deny":
		return Result{
			CLI: "kubectl", Subcommand: "certificate deny",
			Tier: types.TierAdminRemote, BaseTier: types.TierAdminRemote,
			BaseTierNote: "kubectl certificate deny (blocks certificate issuance)",
		}
	}

	return Result{
		CLI: "kubectl", Subcommand: "certificate",
		Tier: types.TierUnknown, BaseTier: types.TierUnknown,
		BaseTierNote: "kubectl certificate: no subcommand provided",
		Unknown:      true,
	}
}

// classifyKubectlVersion classifies "kubectl version" commands.
// --client restricts to local-only output (no API server contact).
func classifyKubectlVersion(args []string) Result {
	for _, arg := range args {
		if arg == "--client" {
			return Result{
				CLI:          "kubectl",
				Subcommand:   "version",
				Tier:         types.TierReadLocal,
				BaseTier:     types.TierReadRemote,
				BaseTierNote: "kubectl version (read remote by default; contacts API server)",
				FlagEffects:  []string{"--client → read local (client version only; no server contact)"},
			}
		}
	}

	return Result{
		CLI:          "kubectl",
		Subcommand:   "version",
		Tier:         types.TierReadRemote,
		BaseTier:     types.TierReadRemote,
		BaseTierNote: "kubectl version (contacts API server)",
	}
}

// classifyKubectlPlugin classifies "kubectl plugin" subcommands.
func classifyKubectlPlugin(args []string) Result {
	sub := kubectlFirstPositional(args)

	if sub == "list" || sub == "" {
		return Result{
			CLI: "kubectl", Subcommand: "plugin list",
			Tier: types.TierReadLocal, BaseTier: types.TierReadLocal,
			BaseTierNote: "kubectl plugin list (lists locally installed plugins)",
		}
	}

	return Result{
		CLI:          "kubectl",
		Subcommand:   "plugin " + sub,
		Tier:         types.TierUnknown,
		BaseTier:     types.TierUnknown,
		BaseTierNote: "kubectl plugin " + sub + ": not in classification DB",
		Unknown:      true,
	}
}

// kubectlSubcommand scans args for the first non-flag token, skipping kubectl
// global flags (e.g., --namespace <name>, --context <name>, -n <name>).
func kubectlSubcommand(args []string) (string, []string) {
	flagsWithValue := map[string]bool{
		"--namespace": true, "-n": true,
		"--context":    true,
		"--kubeconfig": true,
		"--cluster":    true,
		"--user":       true,
		"--server":     true, "-s": true,
		"--output":         true, "-o": true,
		"--selector":       true, "-l": true,
		"--field-selector": true,
		"--token":          true,
		"--as":             true,
		"--as-group":       true,
		"--as-uid":         true,
		"--cache-dir":      true,
		"--certificate-authority": true,
		"--client-certificate":    true,
		"--client-key":            true,
		"--tls-server-name":       true,
		"--request-timeout":       true,
		"-v": true, "--v": true,
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsWithValue[arg] {
			i++ // skip value
			continue
		}
		// Embedded-value forms: --namespace=foo, --context=bar, etc.
		if strings.Contains(arg, "=") && strings.HasPrefix(arg, "--") {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue // standalone boolean flag
		}
		return arg, args[i+1:]
	}
	return "", nil
}

// kubectlFirstPositional returns the first non-flag token from args.
func kubectlFirstPositional(args []string) string {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			return arg
		}
	}
	return ""
}

// kubectlResourceType extracts the resource type from kubectl get/describe args.
// The resource type is the first positional argument (non-flag token).
// Handles both "kubectl get secret" and "kubectl get secret/my-secret" forms.
func kubectlResourceType(args []string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		// Handle "type/name" form: "secret/my-secret" → "secret"
		if idx := strings.Index(arg, "/"); idx > 0 {
			return strings.ToLower(arg[:idx])
		}
		return strings.ToLower(arg)
	}
	return ""
}
