package classify

import (
	"testing"

	"github.com/rgharris/agent-perms/internal/types"
)

func TestClassifyEsc(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    types.Tier
		unknown bool
	}{
		// No args
		{name: "no args", args: []string{"esc"}, want: types.TierUnknown, unknown: true},

		// Help
		{name: "help flag", args: []string{"esc", "--help"}, want: types.TierReadLocal},
		{name: "env help", args: []string{"esc", "env", "--help"}, want: types.TierReadLocal},

		// Read local
		{name: "version", args: []string{"esc", "version"}, want: types.TierReadLocal},

		// Read remote
		{name: "env bare", args: []string{"esc", "env"}, want: types.TierReadRemote},
		{name: "env ls", args: []string{"esc", "env", "ls"}, want: types.TierReadRemote},
		{name: "env list", args: []string{"esc", "env", "list"}, want: types.TierReadRemote},
		{name: "env get", args: []string{"esc", "env", "get", "myorg/myenv"}, want: types.TierReadRemote},
		{name: "env diff", args: []string{"esc", "env", "diff", "myorg/a", "myorg/b"}, want: types.TierReadRemote},
		{name: "whoami", args: []string{"esc", "whoami"}, want: types.TierReadRemote},
		{name: "env tag get", args: []string{"esc", "env", "tag", "get", "myorg/myenv"}, want: types.TierReadRemote},
		{name: "env tag ls", args: []string{"esc", "env", "tag", "ls"}, want: types.TierReadRemote},
		{name: "env version", args: []string{"esc", "env", "version"}, want: types.TierReadRemote},
		{name: "env version history", args: []string{"esc", "env", "version", "history", "myorg/myenv"}, want: types.TierReadRemote},
		{name: "env version tag ls", args: []string{"esc", "env", "version", "tag", "ls"}, want: types.TierReadRemote},

		// Read-sensitive remote
		{name: "env open", args: []string{"esc", "env", "open", "myorg/myenv"}, want: types.TierReadSensitiveRemote},

		// Write local
		{name: "login", args: []string{"esc", "login"}, want: types.TierWriteLocal},
		{name: "logout", args: []string{"esc", "logout"}, want: types.TierWriteLocal},

		// Write remote
		{name: "env init", args: []string{"esc", "env", "init", "myorg/myenv"}, want: types.TierWriteRemote},
		{name: "env edit", args: []string{"esc", "env", "edit", "myorg/myenv"}, want: types.TierWriteRemote},
		{name: "env set", args: []string{"esc", "env", "set", "myorg/myenv", "key", "value"}, want: types.TierWriteRemote},
		{name: "env tag mv", args: []string{"esc", "env", "tag", "mv", "old", "new"}, want: types.TierWriteRemote},
		{name: "env version rollback", args: []string{"esc", "env", "version", "rollback", "myorg/myenv"}, want: types.TierWriteRemote},

		// Admin remote
		{name: "env rm", args: []string{"esc", "env", "rm", "myorg/myenv"}, want: types.TierAdminRemote},
		{name: "env remove", args: []string{"esc", "env", "remove", "myorg/myenv"}, want: types.TierAdminRemote},
		{name: "env version retract", args: []string{"esc", "env", "version", "retract", "myorg/myenv"}, want: types.TierAdminRemote},

		// Unknown
		{name: "unknown sub", args: []string{"esc", "foobar"}, want: types.TierUnknown, unknown: true},

		// esc run — nested command classification
		// Inner: kubectl get pods (read remote) → max with read-sensitive = read-sensitive remote
		{name: "run kubectl get pods",
			args: []string{"esc", "run", "myorg/myenv", "--", "kubectl", "get", "pods"},
			want: types.TierReadSensitiveRemote},

		// Inner: kubectl get secret (read-sensitive remote) → max = read-sensitive remote
		{name: "run kubectl get secret",
			args: []string{"esc", "run", "myorg/myenv", "--", "kubectl", "get", "secret", "foo"},
			want: types.TierReadSensitiveRemote},

		// Inner: kubectl apply (write remote) → max = write remote (escalated)
		{name: "run kubectl apply",
			args: []string{"esc", "run", "myorg/myenv", "--", "kubectl", "apply", "-f", "manifest.yaml"},
			want: types.TierWriteRemote},

		// Inner: kubectl delete (admin remote) → max = admin remote (escalated)
		{name: "run kubectl delete",
			args: []string{"esc", "run", "myorg/myenv", "--", "kubectl", "delete", "pod", "my-pod"},
			want: types.TierAdminRemote},

		// Inner: git status (read local) → max = read-sensitive remote
		{name: "run git status",
			args: []string{"esc", "run", "myorg/myenv", "--", "git", "status"},
			want: types.TierReadSensitiveRemote},

		// Inner: unknown command → whole thing unknown
		{name: "run unknown command",
			args: []string{"esc", "run", "myorg/myenv", "--", "foobar", "baz"},
			want: types.TierUnknown, unknown: true},

		// No inner command
		{name: "run no inner",
			args: []string{"esc", "run", "myorg/myenv"},
			want: types.TierReadSensitiveRemote},

		// No "--" separator (positional form)
		{name: "run positional kubectl get",
			args: []string{"esc", "run", "myorg/myenv", "kubectl", "get", "pods"},
			want: types.TierReadSensitiveRemote},
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

func TestClassifyEscRunInnerResult(t *testing.T) {
	// Verify InnerResult is populated for nested commands.
	result := Classify([]string{"esc", "run", "myorg/myenv", "--", "kubectl", "delete", "pod", "x"})
	if result.InnerResult == nil {
		t.Fatal("expected InnerResult to be set for esc run with inner command")
	}
	if result.InnerResult.CLI != "kubectl" {
		t.Errorf("InnerResult.CLI = %q, want %q", result.InnerResult.CLI, "kubectl")
	}
	if result.InnerResult.Tier != types.TierAdminRemote {
		t.Errorf("InnerResult.Tier = %v, want %v", result.InnerResult.Tier, types.TierAdminRemote)
	}
	if result.Tier != types.TierAdminRemote {
		t.Errorf("resolved tier = %v, want %v (admin from inner should dominate)", result.Tier, types.TierAdminRemote)
	}
	if len(result.FlagEffects) == 0 {
		t.Error("expected FlagEffects to note the inner command escalation")
	}
}

func TestClassifyEscRunNoEscalation(t *testing.T) {
	// When inner tier <= outer tier, no FlagEffects needed.
	result := Classify([]string{"esc", "run", "myorg/myenv", "--", "kubectl", "get", "pods"})
	if len(result.FlagEffects) != 0 {
		t.Errorf("expected no FlagEffects when inner doesn't escalate, got: %v", result.FlagEffects)
	}
	if result.InnerResult == nil {
		t.Fatal("expected InnerResult to be set")
	}
	if result.InnerResult.Tier != types.TierReadRemote {
		t.Errorf("InnerResult.Tier = %v, want %v", result.InnerResult.Tier, types.TierReadRemote)
	}
}
