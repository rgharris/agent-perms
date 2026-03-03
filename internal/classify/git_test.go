package classify

import (
	"testing"

	"github.com/rgharris/agent-perms/internal/types"
)

func TestClassifyGit(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    types.Tier
		unknown bool
	}{
		// Read local — simple subcommands
		{name: "git status", args: []string{"git", "status"}, want: types.TierReadLocal},
		{name: "git log", args: []string{"git", "log"}, want: types.TierReadLocal},
		{name: "git diff", args: []string{"git", "diff"}, want: types.TierReadLocal},
		{name: "git show", args: []string{"git", "show", "HEAD"}, want: types.TierReadLocal},
		{name: "git blame", args: []string{"git", "blame", "main.go"}, want: types.TierReadLocal},
		{name: "git grep", args: []string{"git", "grep", "TODO"}, want: types.TierReadLocal},
		{name: "git describe", args: []string{"git", "describe", "--tags"}, want: types.TierReadLocal},
		{name: "git shortlog", args: []string{"git", "shortlog", "-s"}, want: types.TierReadLocal},
		{name: "git rev-parse", args: []string{"git", "rev-parse", "HEAD"}, want: types.TierReadLocal},
		{name: "git rev-list", args: []string{"git", "rev-list", "HEAD"}, want: types.TierReadLocal},
		{name: "git cat-file", args: []string{"git", "cat-file", "-t", "HEAD"}, want: types.TierReadLocal},
		{name: "git ls-tree", args: []string{"git", "ls-tree", "HEAD"}, want: types.TierReadLocal},
		{name: "git ls-files", args: []string{"git", "ls-files"}, want: types.TierReadLocal},

		// Read remote — contacts remote server
		{name: "git fetch", args: []string{"git", "fetch", "origin"}, want: types.TierReadRemote},

		// WriteLocal — simple subcommands
		{name: "git add", args: []string{"git", "add", "."}, want: types.TierWriteLocal},
		{name: "git commit", args: []string{"git", "commit", "-m", "fix"}, want: types.TierWriteLocal},
		{name: "git rm", args: []string{"git", "rm", "file.go"}, want: types.TierWriteLocal},
		{name: "git mv", args: []string{"git", "mv", "old.go", "new.go"}, want: types.TierWriteLocal},
		{name: "git merge", args: []string{"git", "merge", "feature"}, want: types.TierWriteLocal},
		{name: "git rebase", args: []string{"git", "rebase", "main"}, want: types.TierWriteLocal},
		{name: "git cherry-pick", args: []string{"git", "cherry-pick", "abc123"}, want: types.TierWriteLocal},
		{name: "git apply", args: []string{"git", "apply", "fix.patch"}, want: types.TierWriteLocal},
		{name: "git revert", args: []string{"git", "revert", "HEAD"}, want: types.TierWriteLocal},
		{name: "git init", args: []string{"git", "init"}, want: types.TierWriteLocal},
		{name: "git clone", args: []string{"git", "clone", "https://github.com/x/y"}, want: types.TierWriteLocal},
		{name: "git pull", args: []string{"git", "pull"}, want: types.TierWriteLocal},
		{name: "git checkout", args: []string{"git", "checkout", "main"}, want: types.TierWriteLocal},
		{name: "git switch", args: []string{"git", "switch", "main"}, want: types.TierWriteLocal},
		{name: "git restore", args: []string{"git", "restore", "file.go"}, want: types.TierWriteLocal},

		// AdminLocal — destructive local
		{name: "git gc", args: []string{"git", "gc"}, want: types.TierAdminLocal},
		{name: "git filter-branch", args: []string{"git", "filter-branch", "--tree-filter", "rm -f secret"}, want: types.TierAdminLocal},

		// git push — remote tiers
		{name: "git push (no flags)", args: []string{"git", "push", "origin", "main"}, want: types.TierWriteRemote},
		{name: "git push (bare)", args: []string{"git", "push"}, want: types.TierWriteRemote},
		{name: "git push --force", args: []string{"git", "push", "--force"}, want: types.TierAdminRemote},
		{name: "git push -f", args: []string{"git", "push", "-f"}, want: types.TierAdminRemote},
		{name: "git push --force-with-lease", args: []string{"git", "push", "--force-with-lease"}, want: types.TierAdminRemote},
		{name: "git push --force-if-includes", args: []string{"git", "push", "--force-if-includes"}, want: types.TierAdminRemote},
		{name: "git push --delete", args: []string{"git", "push", "--delete", "origin", "branch"}, want: types.TierAdminRemote},
		{name: "git push -d", args: []string{"git", "push", "-d", "origin", "branch"}, want: types.TierAdminRemote},
		{name: "git push origin :branch (colon-ref)", args: []string{"git", "push", "origin", ":branch"}, want: types.TierAdminRemote},

		// git branch
		{name: "git branch (list)", args: []string{"git", "branch"}, want: types.TierReadLocal},
		{name: "git branch -a (list all)", args: []string{"git", "branch", "-a"}, want: types.TierReadLocal},
		{name: "git branch -r (list remote)", args: []string{"git", "branch", "-r"}, want: types.TierReadLocal},
		{name: "git branch <name> (create)", args: []string{"git", "branch", "feature"}, want: types.TierWriteLocal},
		{name: "git branch -d (delete)", args: []string{"git", "branch", "-d", "feature"}, want: types.TierWriteLocal},
		{name: "git branch -D (force delete)", args: []string{"git", "branch", "-D", "feature"}, want: types.TierWriteLocal},
		{name: "git branch -m (rename)", args: []string{"git", "branch", "-m", "old", "new"}, want: types.TierWriteLocal},
		{name: "git branch --move (rename)", args: []string{"git", "branch", "--move", "old", "new"}, want: types.TierWriteLocal},
		{name: "git branch -u (set upstream)", args: []string{"git", "branch", "-u", "origin/main"}, want: types.TierWriteLocal},
		{name: "git branch --set-upstream-to=", args: []string{"git", "branch", "--set-upstream-to=origin/main"}, want: types.TierWriteLocal},
		{name: "git branch --unset-upstream", args: []string{"git", "branch", "--unset-upstream"}, want: types.TierWriteLocal},

		// git tag
		{name: "git tag (list)", args: []string{"git", "tag"}, want: types.TierReadLocal},
		{name: "git tag -l (list)", args: []string{"git", "tag", "-l"}, want: types.TierReadLocal},
		{name: "git tag --list (list)", args: []string{"git", "tag", "--list"}, want: types.TierReadLocal},
		{name: "git tag <name> (create)", args: []string{"git", "tag", "v1.0"}, want: types.TierWriteLocal},
		{name: "git tag -a (annotated)", args: []string{"git", "tag", "-a", "v1.0", "-m", "release"}, want: types.TierWriteLocal},
		{name: "git tag -d (delete)", args: []string{"git", "tag", "-d", "v1.0"}, want: types.TierWriteLocal},
		{name: "git tag --delete (delete)", args: []string{"git", "tag", "--delete", "v1.0"}, want: types.TierWriteLocal},

		// git stash
		{name: "git stash (push)", args: []string{"git", "stash"}, want: types.TierWriteLocal},
		{name: "git stash push", args: []string{"git", "stash", "push"}, want: types.TierWriteLocal},
		{name: "git stash pop", args: []string{"git", "stash", "pop"}, want: types.TierWriteLocal},
		{name: "git stash apply", args: []string{"git", "stash", "apply"}, want: types.TierWriteLocal},
		{name: "git stash drop", args: []string{"git", "stash", "drop"}, want: types.TierWriteLocal},
		{name: "git stash clear", args: []string{"git", "stash", "clear"}, want: types.TierWriteLocal},
		{name: "git stash list", args: []string{"git", "stash", "list"}, want: types.TierReadLocal},
		{name: "git stash show", args: []string{"git", "stash", "show"}, want: types.TierReadLocal},

		// git remote
		{name: "git remote (list)", args: []string{"git", "remote"}, want: types.TierReadLocal},
		{name: "git remote -v (list verbose)", args: []string{"git", "remote", "-v"}, want: types.TierReadLocal},
		{name: "git remote show", args: []string{"git", "remote", "show", "origin"}, want: types.TierReadLocal},
		{name: "git remote get-url", args: []string{"git", "remote", "get-url", "origin"}, want: types.TierReadLocal},
		{name: "git remote add", args: []string{"git", "remote", "add", "origin", "url"}, want: types.TierWriteLocal},
		{name: "git remote remove", args: []string{"git", "remote", "remove", "origin"}, want: types.TierWriteLocal},
		{name: "git remote rename", args: []string{"git", "remote", "rename", "old", "new"}, want: types.TierWriteLocal},
		{name: "git remote set-url", args: []string{"git", "remote", "set-url", "origin", "url"}, want: types.TierWriteLocal},
		{name: "git remote prune", args: []string{"git", "remote", "prune", "origin"}, want: types.TierWriteLocal},

		// git clean
		{name: "git clean -n (dry-run)", args: []string{"git", "clean", "-n"}, want: types.TierReadLocal},
		{name: "git clean --dry-run", args: []string{"git", "clean", "--dry-run"}, want: types.TierReadLocal},
		{name: "git clean -f (force)", args: []string{"git", "clean", "-f"}, want: types.TierAdminLocal},
		{name: "git clean -fd", args: []string{"git", "clean", "-fd"}, want: types.TierAdminLocal},
		{name: "git clean -fx", args: []string{"git", "clean", "-fx"}, want: types.TierAdminLocal},

		// git config
		{name: "git config --list", args: []string{"git", "config", "--list"}, want: types.TierReadLocal},
		{name: "git config -l", args: []string{"git", "config", "-l"}, want: types.TierReadLocal},
		{name: "git config --get", args: []string{"git", "config", "--get", "user.email"}, want: types.TierReadLocal},
		{name: "git config --get-all", args: []string{"git", "config", "--get-all", "remote.origin.url"}, want: types.TierReadLocal},
		{name: "git config --get-regexp", args: []string{"git", "config", "--get-regexp", "remote.*"}, want: types.TierReadLocal},
		{name: "git config --global --list", args: []string{"git", "config", "--global", "--list"}, want: types.TierReadLocal},
		{name: "git config local set", args: []string{"git", "config", "core.autocrlf", "false"}, want: types.TierWriteLocal},
		{name: "git config --global set", args: []string{"git", "config", "--global", "user.email", "x@y.com"}, want: types.TierAdminLocal},
		{name: "git config --system set", args: []string{"git", "config", "--system", "core.autocrlf", "false"}, want: types.TierAdminLocal},

		// git reset
		{name: "git reset (default mixed)", args: []string{"git", "reset", "HEAD"}, want: types.TierWriteLocal},
		{name: "git reset --soft", args: []string{"git", "reset", "--soft", "HEAD~1"}, want: types.TierWriteLocal},
		{name: "git reset --mixed", args: []string{"git", "reset", "--mixed", "HEAD~1"}, want: types.TierWriteLocal},
		{name: "git reset --hard", args: []string{"git", "reset", "--hard", "HEAD"}, want: types.TierAdminLocal},
		{name: "git reset --hard origin/main", args: []string{"git", "reset", "--hard", "origin/main"}, want: types.TierAdminLocal},

		// Global flags before subcommand
		{name: "git -C dir status", args: []string{"git", "-C", "/tmp/repo", "status"}, want: types.TierReadLocal},
		{name: "git --no-pager log", args: []string{"git", "--no-pager", "log"}, want: types.TierReadLocal},
		{name: "git -c key=val commit", args: []string{"git", "-c", "core.autocrlf=false", "commit", "-m", "x"}, want: types.TierWriteLocal},

		// Unknown
		{name: "git unknown subcommand", args: []string{"git", "foobar"}, want: types.TierUnknown, unknown: true},
		{name: "git with no args", args: []string{"git"}, want: types.TierUnknown, unknown: true},
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
