package classify

import (
	"testing"

	"github.com/rgharris/agent-perms/internal/types"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    types.Tier
		unknown bool
	}{
		// Empty / unsupported
		{name: "empty", args: []string{}, want: types.TierUnknown, unknown: true},
		{name: "unknown CLI", args: []string{"docker", "ps"}, want: types.TierUnknown, unknown: true},

		// git routing (full coverage in git_test.go)
		{name: "git status", args: []string{"git", "status"}, want: types.TierReadLocal},
		{name: "git commit", args: []string{"git", "commit", "-m", "fix"}, want: types.TierWriteLocal},
		{name: "git push", args: []string{"git", "push"}, want: types.TierWriteRemote},

		// gh read commands (all remote)
		{name: "gh pr list", args: []string{"gh", "pr", "list"}, want: types.TierReadRemote},
		{name: "gh pr list with flags", args: []string{"gh", "pr", "list", "--state", "open"}, want: types.TierReadRemote},
		{name: "gh pr view", args: []string{"gh", "pr", "view", "123"}, want: types.TierReadRemote},
		{name: "gh pr status", args: []string{"gh", "pr", "status"}, want: types.TierReadRemote},
		{name: "gh pr checks", args: []string{"gh", "pr", "checks", "123"}, want: types.TierReadRemote},
		{name: "gh pr diff", args: []string{"gh", "pr", "diff"}, want: types.TierReadRemote},
		{name: "gh pr checkout", args: []string{"gh", "pr", "checkout", "123"}, want: types.TierReadRemote},
		{name: "gh issue list", args: []string{"gh", "issue", "list"}, want: types.TierReadRemote},
		{name: "gh issue view", args: []string{"gh", "issue", "view", "42"}, want: types.TierReadRemote},
		{name: "gh repo list", args: []string{"gh", "repo", "list"}, want: types.TierReadRemote},
		{name: "gh repo view", args: []string{"gh", "repo", "view"}, want: types.TierReadRemote},
		{name: "gh repo clone", args: []string{"gh", "repo", "clone", "owner/repo"}, want: types.TierReadRemote},
		{name: "gh release list", args: []string{"gh", "release", "list"}, want: types.TierReadRemote},
		{name: "gh run list", args: []string{"gh", "run", "list"}, want: types.TierReadRemote},
		{name: "gh run view", args: []string{"gh", "run", "view", "123"}, want: types.TierReadRemote},
		{name: "gh workflow list", args: []string{"gh", "workflow", "list"}, want: types.TierReadRemote},
		{name: "gh auth status", args: []string{"gh", "auth", "status"}, want: types.TierReadRemote},
		{name: "gh auth token", args: []string{"gh", "auth", "token"}, want: types.TierReadSensitiveRemote},
		{name: "gh ssh-key list", args: []string{"gh", "ssh-key", "list"}, want: types.TierReadRemote},
		{name: "gh secret list", args: []string{"gh", "secret", "list"}, want: types.TierReadRemote},
		{name: "gh variable list", args: []string{"gh", "variable", "list"}, want: types.TierReadRemote},
		{name: "gh variable get", args: []string{"gh", "variable", "get", "MY_VAR"}, want: types.TierReadRemote},
		{name: "gh search code", args: []string{"gh", "search", "code", "foo"}, want: types.TierReadRemote},
		{name: "gh search repos", args: []string{"gh", "search", "repos", "agent"}, want: types.TierReadRemote},
		{name: "gh status", args: []string{"gh", "status"}, want: types.TierReadRemote},
		{name: "gh browse", args: []string{"gh", "browse"}, want: types.TierReadRemote},
		{name: "gh org list", args: []string{"gh", "org", "list"}, want: types.TierReadRemote},

		// gh write commands (all remote)
		{name: "gh pr create", args: []string{"gh", "pr", "create", "--title", "fix"}, want: types.TierWriteRemote},
		{name: "gh pr edit", args: []string{"gh", "pr", "edit", "123"}, want: types.TierWriteRemote},
		{name: "gh pr comment", args: []string{"gh", "pr", "comment", "123"}, want: types.TierWriteRemote},
		{name: "gh pr merge", args: []string{"gh", "pr", "merge", "123"}, want: types.TierWriteRemote},
		{name: "gh pr close", args: []string{"gh", "pr", "close", "123"}, want: types.TierWriteRemote},
		{name: "gh pr reopen", args: []string{"gh", "pr", "reopen", "123"}, want: types.TierWriteRemote},
		{name: "gh issue create", args: []string{"gh", "issue", "create", "--title", "bug"}, want: types.TierWriteRemote},
		{name: "gh issue edit", args: []string{"gh", "issue", "edit", "42"}, want: types.TierWriteRemote},
		{name: "gh issue close", args: []string{"gh", "issue", "close", "42"}, want: types.TierWriteRemote},
		{name: "gh repo create", args: []string{"gh", "repo", "create", "newrepo"}, want: types.TierWriteRemote},
		{name: "gh repo fork", args: []string{"gh", "repo", "fork", "owner/repo"}, want: types.TierWriteRemote},
		{name: "gh release create", args: []string{"gh", "release", "create", "v1.0"}, want: types.TierWriteRemote},
		{name: "gh workflow run", args: []string{"gh", "workflow", "run", "build.yml"}, want: types.TierWriteRemote},
		{name: "gh run rerun", args: []string{"gh", "run", "rerun", "123"}, want: types.TierWriteRemote},
		{name: "gh variable set", args: []string{"gh", "variable", "set", "KEY"}, want: types.TierWriteRemote},

		// gh admin commands (all remote)
		{name: "gh pr lock", args: []string{"gh", "pr", "lock", "123"}, want: types.TierAdminRemote},
		{name: "gh issue delete", args: []string{"gh", "issue", "delete", "42"}, want: types.TierAdminRemote},
		{name: "gh issue lock", args: []string{"gh", "issue", "lock", "42"}, want: types.TierAdminRemote},
		{name: "gh repo delete", args: []string{"gh", "repo", "delete", "myrepo"}, want: types.TierAdminRemote},
		{name: "gh repo rename", args: []string{"gh", "repo", "rename", "newname"}, want: types.TierAdminRemote},
		{name: "gh repo archive", args: []string{"gh", "repo", "archive"}, want: types.TierAdminRemote},
		{name: "gh release delete", args: []string{"gh", "release", "delete", "v1.0"}, want: types.TierAdminRemote},
		{name: "gh workflow enable", args: []string{"gh", "workflow", "enable", "build.yml"}, want: types.TierAdminRemote},
		{name: "gh workflow disable", args: []string{"gh", "workflow", "disable", "build.yml"}, want: types.TierAdminRemote},
		{name: "gh run delete", args: []string{"gh", "run", "delete", "123"}, want: types.TierAdminRemote},
		{name: "gh auth login", args: []string{"gh", "auth", "login"}, want: types.TierAdminRemote},
		{name: "gh auth logout", args: []string{"gh", "auth", "logout"}, want: types.TierAdminRemote},
		{name: "gh ssh-key add", args: []string{"gh", "ssh-key", "add", "key.pub"}, want: types.TierAdminRemote},
		{name: "gh ssh-key delete", args: []string{"gh", "ssh-key", "delete", "123"}, want: types.TierAdminRemote},
		{name: "gh secret set", args: []string{"gh", "secret", "set", "TOKEN"}, want: types.TierAdminRemote},
		{name: "gh secret delete", args: []string{"gh", "secret", "delete", "TOKEN"}, want: types.TierAdminRemote},
		{name: "gh variable delete", args: []string{"gh", "variable", "delete", "MY_VAR"}, want: types.TierAdminRemote},

		// Unknown subcommand
		{name: "gh unknown", args: []string{"gh", "foobar"}, want: types.TierUnknown, unknown: true},
		{name: "gh with no args", args: []string{"gh"}, want: types.TierUnknown, unknown: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.args)
			if got.Tier != tt.want {
				t.Errorf("Classify(%v) tier = %v, want %v", tt.args, got.Tier, tt.want)
			}
			if got.Unknown != tt.unknown {
				t.Errorf("Classify(%v) unknown = %v, want %v", tt.args, got.Unknown, tt.unknown)
			}
		})
	}
}
