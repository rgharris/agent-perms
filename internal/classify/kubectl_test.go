package classify

import (
	"testing"

	"github.com/rgharris/agent-perms/internal/types"
)

func TestClassifyKubectl(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    types.Tier
		unknown bool
	}{
		// No args
		{name: "no args", args: []string{"kubectl"}, want: types.TierUnknown, unknown: true},

		// Help flags
		{name: "help flag", args: []string{"kubectl", "--help"}, want: types.TierReadLocal},
		{name: "help subcommand", args: []string{"kubectl", "get", "--help"}, want: types.TierReadLocal},
		{name: "-h flag", args: []string{"kubectl", "delete", "-h"}, want: types.TierReadLocal},

		// Read local
		{name: "completion", args: []string{"kubectl", "completion", "bash"}, want: types.TierReadLocal},
		{name: "kustomize", args: []string{"kubectl", "kustomize", "."}, want: types.TierReadLocal},
		{name: "options", args: []string{"kubectl", "options"}, want: types.TierReadLocal},
		{name: "plugin list", args: []string{"kubectl", "plugin", "list"}, want: types.TierReadLocal},
		{name: "plugin bare", args: []string{"kubectl", "plugin"}, want: types.TierReadLocal},
		{name: "version --client", args: []string{"kubectl", "version", "--client"}, want: types.TierReadLocal},

		// Config — read local
		{name: "config view", args: []string{"kubectl", "config", "view"}, want: types.TierReadLocal},
		{name: "config bare", args: []string{"kubectl", "config"}, want: types.TierReadLocal},
		{name: "config get-contexts", args: []string{"kubectl", "config", "get-contexts"}, want: types.TierReadLocal},
		{name: "config get-clusters", args: []string{"kubectl", "config", "get-clusters"}, want: types.TierReadLocal},
		{name: "config get-users", args: []string{"kubectl", "config", "get-users"}, want: types.TierReadLocal},
		{name: "config current-context", args: []string{"kubectl", "config", "current-context"}, want: types.TierReadLocal},

		// Config — write local
		{name: "config set-context", args: []string{"kubectl", "config", "set-context", "my-ctx"}, want: types.TierWriteLocal},
		{name: "config set-cluster", args: []string{"kubectl", "config", "set-cluster", "my-cluster"}, want: types.TierWriteLocal},
		{name: "config set-credentials", args: []string{"kubectl", "config", "set-credentials", "admin"}, want: types.TierWriteLocal},
		{name: "config use-context", args: []string{"kubectl", "config", "use-context", "prod"}, want: types.TierWriteLocal},
		{name: "config set", args: []string{"kubectl", "config", "set", "key", "value"}, want: types.TierWriteLocal},
		{name: "config rename-context", args: []string{"kubectl", "config", "rename-context", "old", "new"}, want: types.TierWriteLocal},

		// Config — admin local
		{name: "config delete-context", args: []string{"kubectl", "config", "delete-context", "old-ctx"}, want: types.TierAdminLocal},
		{name: "config delete-cluster", args: []string{"kubectl", "config", "delete-cluster", "old-cluster"}, want: types.TierAdminLocal},
		{name: "config delete-user", args: []string{"kubectl", "config", "delete-user", "old-user"}, want: types.TierAdminLocal},
		{name: "config unset", args: []string{"kubectl", "config", "unset", "key"}, want: types.TierAdminLocal},

		// Read remote
		{name: "get pods", args: []string{"kubectl", "get", "pods"}, want: types.TierReadRemote},
		{name: "get deployments", args: []string{"kubectl", "get", "deployments", "-n", "kube-system"}, want: types.TierReadRemote},
		{name: "get pod/name", args: []string{"kubectl", "get", "pod/my-pod"}, want: types.TierReadRemote},
		{name: "describe pod", args: []string{"kubectl", "describe", "pod", "my-pod"}, want: types.TierReadRemote},
		{name: "describe node", args: []string{"kubectl", "describe", "node", "node-1"}, want: types.TierReadRemote},
		{name: "logs", args: []string{"kubectl", "logs", "my-pod"}, want: types.TierReadRemote},
		{name: "logs -f", args: []string{"kubectl", "logs", "-f", "my-pod"}, want: types.TierReadRemote},
		{name: "top", args: []string{"kubectl", "top", "pods"}, want: types.TierReadRemote},
		{name: "cluster-info", args: []string{"kubectl", "cluster-info"}, want: types.TierReadRemote},
		{name: "api-resources", args: []string{"kubectl", "api-resources"}, want: types.TierReadRemote},
		{name: "api-versions", args: []string{"kubectl", "api-versions"}, want: types.TierReadRemote},
		{name: "explain", args: []string{"kubectl", "explain", "pods"}, want: types.TierReadRemote},
		{name: "version", args: []string{"kubectl", "version"}, want: types.TierReadRemote},
		{name: "diff", args: []string{"kubectl", "diff", "-f", "manifest.yaml"}, want: types.TierReadRemote},
		{name: "wait", args: []string{"kubectl", "wait", "--for=condition=ready", "pod/my-pod"}, want: types.TierReadRemote},
		{name: "events", args: []string{"kubectl", "events"}, want: types.TierReadRemote},
		{name: "auth can-i", args: []string{"kubectl", "auth", "can-i", "get", "pods"}, want: types.TierReadRemote},
		{name: "auth whoami", args: []string{"kubectl", "auth", "whoami"}, want: types.TierReadRemote},
		{name: "rollout status", args: []string{"kubectl", "rollout", "status", "deploy/my-app"}, want: types.TierReadRemote},
		{name: "rollout history", args: []string{"kubectl", "rollout", "history", "deploy/my-app"}, want: types.TierReadRemote},

		// Read-sensitive remote
		{name: "get secret", args: []string{"kubectl", "get", "secret", "my-secret"}, want: types.TierReadSensitiveRemote},
		{name: "get secrets", args: []string{"kubectl", "get", "secrets"}, want: types.TierReadSensitiveRemote},
		{name: "get secret -o yaml", args: []string{"kubectl", "get", "secret", "my-secret", "-o", "yaml"}, want: types.TierReadSensitiveRemote},
		{name: "get secret/name", args: []string{"kubectl", "get", "secret/my-secret"}, want: types.TierReadSensitiveRemote},
		{name: "describe secret", args: []string{"kubectl", "describe", "secret", "my-secret"}, want: types.TierReadSensitiveRemote},
		{name: "describe secrets", args: []string{"kubectl", "describe", "secrets", "my-secret"}, want: types.TierReadSensitiveRemote},
		{name: "cluster-info dump", args: []string{"kubectl", "cluster-info", "dump"}, want: types.TierReadSensitiveRemote},
		{name: "proxy", args: []string{"kubectl", "proxy"}, want: types.TierReadSensitiveRemote},
		{name: "port-forward", args: []string{"kubectl", "port-forward", "pod/my-pod", "8080:80"}, want: types.TierReadSensitiveRemote},

		// Write remote
		{name: "create", args: []string{"kubectl", "create", "-f", "pod.yaml"}, want: types.TierWriteRemote},
		{name: "apply", args: []string{"kubectl", "apply", "-f", "manifest.yaml"}, want: types.TierWriteRemote},
		{name: "edit", args: []string{"kubectl", "edit", "deploy/my-app"}, want: types.TierWriteRemote},
		{name: "patch", args: []string{"kubectl", "patch", "deploy", "my-app", "-p", "{}"}, want: types.TierWriteRemote},
		{name: "replace", args: []string{"kubectl", "replace", "-f", "pod.yaml"}, want: types.TierWriteRemote},
		{name: "set", args: []string{"kubectl", "set", "image", "deploy/my-app", "app=img:v2"}, want: types.TierWriteRemote},
		{name: "scale", args: []string{"kubectl", "scale", "deploy/my-app", "--replicas=3"}, want: types.TierWriteRemote},
		{name: "autoscale", args: []string{"kubectl", "autoscale", "deploy/my-app", "--min=1", "--max=5"}, want: types.TierWriteRemote},
		{name: "expose", args: []string{"kubectl", "expose", "deploy/my-app", "--port=80"}, want: types.TierWriteRemote},
		{name: "run", args: []string{"kubectl", "run", "my-pod", "--image=nginx"}, want: types.TierWriteRemote},
		{name: "label", args: []string{"kubectl", "label", "pod", "my-pod", "env=prod"}, want: types.TierWriteRemote},
		{name: "annotate", args: []string{"kubectl", "annotate", "pod", "my-pod", "key=val"}, want: types.TierWriteRemote},
		{name: "cp", args: []string{"kubectl", "cp", "file.txt", "my-pod:/tmp/file.txt"}, want: types.TierWriteRemote},
		{name: "exec", args: []string{"kubectl", "exec", "-it", "my-pod", "--", "bash"}, want: types.TierWriteRemote},
		{name: "attach", args: []string{"kubectl", "attach", "my-pod"}, want: types.TierWriteRemote},
		{name: "taint", args: []string{"kubectl", "taint", "node", "node1", "key=val:NoSchedule"}, want: types.TierWriteRemote},
		{name: "cordon", args: []string{"kubectl", "cordon", "node1"}, want: types.TierWriteRemote},
		{name: "uncordon", args: []string{"kubectl", "uncordon", "node1"}, want: types.TierWriteRemote},
		{name: "debug", args: []string{"kubectl", "debug", "my-pod", "--image=busybox"}, want: types.TierWriteRemote},
		{name: "rollout restart", args: []string{"kubectl", "rollout", "restart", "deploy/my-app"}, want: types.TierWriteRemote},
		{name: "rollout undo", args: []string{"kubectl", "rollout", "undo", "deploy/my-app"}, want: types.TierWriteRemote},
		{name: "rollout resume", args: []string{"kubectl", "rollout", "resume", "deploy/my-app"}, want: types.TierWriteRemote},
		{name: "rollout pause", args: []string{"kubectl", "rollout", "pause", "deploy/my-app"}, want: types.TierWriteRemote},
		{name: "auth reconcile", args: []string{"kubectl", "auth", "reconcile", "-f", "rbac.yaml"}, want: types.TierWriteRemote},
		{name: "certificate approve", args: []string{"kubectl", "certificate", "approve", "csr-name"}, want: types.TierWriteRemote},

		// Admin remote
		{name: "delete", args: []string{"kubectl", "delete", "pod", "my-pod"}, want: types.TierAdminRemote},
		{name: "delete -f", args: []string{"kubectl", "delete", "-f", "manifest.yaml"}, want: types.TierAdminRemote},
		{name: "drain", args: []string{"kubectl", "drain", "node1"}, want: types.TierAdminRemote},
		{name: "replace --force", args: []string{"kubectl", "replace", "--force", "-f", "pod.yaml"}, want: types.TierAdminRemote},
		{name: "certificate deny", args: []string{"kubectl", "certificate", "deny", "csr-name"}, want: types.TierAdminRemote},

		// Global flag skipping
		{name: "get with namespace", args: []string{"kubectl", "--namespace", "prod", "get", "pods"}, want: types.TierReadRemote},
		{name: "get with -n", args: []string{"kubectl", "-n", "prod", "get", "pods"}, want: types.TierReadRemote},
		{name: "get with context", args: []string{"kubectl", "--context", "staging", "get", "pods"}, want: types.TierReadRemote},
		{name: "get with namespace=", args: []string{"kubectl", "--namespace=prod", "get", "pods"}, want: types.TierReadRemote},
		{name: "delete with namespace", args: []string{"kubectl", "-n", "prod", "delete", "pod", "my-pod"}, want: types.TierAdminRemote},

		// Unknown subcommands
		{name: "unknown subcommand", args: []string{"kubectl", "foobar"}, want: types.TierUnknown, unknown: true},
		{name: "unknown rollout sub", args: []string{"kubectl", "rollout", "foobar"}, want: types.TierUnknown, unknown: true},
		{name: "unknown config sub", args: []string{"kubectl", "config", "foobar"}, want: types.TierUnknown, unknown: true},
		{name: "unknown auth sub", args: []string{"kubectl", "auth"}, want: types.TierUnknown, unknown: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.args)
			if got.Tier != tt.want {
				t.Errorf("Classify(%v) tier = %v, want %v (note: %s)", tt.args, got.Tier, tt.want, got.BaseTierNote)
			}
			if got.Unknown != tt.unknown {
				t.Errorf("Classify(%v) unknown = %v, want %v", tt.args, got.Unknown, tt.unknown)
			}
		})
	}
}

func TestClassifyKubectlFlagEffects(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantEffects int
	}{
		{name: "get pods no effects", args: []string{"kubectl", "get", "pods"}, wantEffects: 0},
		{name: "get secret has effect", args: []string{"kubectl", "get", "secret", "foo"}, wantEffects: 1},
		{name: "describe secret has effect", args: []string{"kubectl", "describe", "secret", "foo"}, wantEffects: 1},
		{name: "cluster-info dump has effect", args: []string{"kubectl", "cluster-info", "dump"}, wantEffects: 1},
		{name: "replace --force has effect", args: []string{"kubectl", "replace", "--force", "-f", "x.yaml"}, wantEffects: 1},
		{name: "version --client has effect", args: []string{"kubectl", "version", "--client"}, wantEffects: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.args)
			if len(got.FlagEffects) != tt.wantEffects {
				t.Errorf("Classify(%v) FlagEffects count = %d, want %d (effects: %v)", tt.args, len(got.FlagEffects), tt.wantEffects, got.FlagEffects)
			}
		})
	}
}
