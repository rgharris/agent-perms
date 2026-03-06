package classify

import (
	"testing"

	"github.com/rgharris/agent-perms/internal/types"
)

func TestClassifyPulumi(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    types.Tier
		unknown bool
	}{
		// Read remote — dry-run and cloud-querying commands
		{name: "pulumi preview", args: []string{"pulumi", "preview"}, want: types.TierReadRemote},
		{name: "pulumi preview (flags)", args: []string{"pulumi", "preview", "--diff"}, want: types.TierReadRemote},
		{name: "pulumi pre (alias)", args: []string{"pulumi", "pre"}, want: types.TierReadRemote},
		{name: "pulumi logs", args: []string{"pulumi", "logs"}, want: types.TierReadRemote},
		{name: "pulumi whoami", args: []string{"pulumi", "whoami"}, want: types.TierReadRemote},

		// Read local — local info commands
		{name: "pulumi about", args: []string{"pulumi", "about"}, want: types.TierReadLocal},
		{name: "pulumi version", args: []string{"pulumi", "version"}, want: types.TierReadLocal},

		// Read remote — stack info (queries Pulumi Cloud)
		{name: "pulumi stack (bare)", args: []string{"pulumi", "stack"}, want: types.TierReadRemote},
		{name: "pulumi stack ls", args: []string{"pulumi", "stack", "ls"}, want: types.TierReadRemote},
		{name: "pulumi stack list", args: []string{"pulumi", "stack", "list"}, want: types.TierReadRemote},
		{name: "pulumi stack output", args: []string{"pulumi", "stack", "output"}, want: types.TierReadRemote},
		{name: "pulumi stack output (named)", args: []string{"pulumi", "stack", "output", "bucketName"}, want: types.TierReadRemote},
		{name: "pulumi stack history", args: []string{"pulumi", "stack", "history"}, want: types.TierReadRemote},
		{name: "pulumi stack graph", args: []string{"pulumi", "stack", "graph", "graph.dot"}, want: types.TierReadRemote},

		// Read local — config (reads local Pulumi config files)
		{name: "pulumi config (bare)", args: []string{"pulumi", "config"}, want: types.TierReadLocal},
		{name: "pulumi config get", args: []string{"pulumi", "config", "get", "aws:region"}, want: types.TierReadLocal},

		// Read remote — state (queries backend, typically Pulumi Cloud)
		{name: "pulumi state export", args: []string{"pulumi", "state", "export"}, want: types.TierReadRemote},

		// Read local — plugin (checks locally installed plugins)
		{name: "pulumi plugin (bare)", args: []string{"pulumi", "plugin"}, want: types.TierReadLocal},
		{name: "pulumi plugin ls", args: []string{"pulumi", "plugin", "ls"}, want: types.TierReadLocal},
		{name: "pulumi plugin list", args: []string{"pulumi", "plugin", "list"}, want: types.TierReadLocal},

		// Read local — schema / package (reads from local plugin binaries)
		{name: "pulumi schema (bare)", args: []string{"pulumi", "schema"}, want: types.TierReadLocal},
		{name: "pulumi schema get", args: []string{"pulumi", "schema", "get", "aws"}, want: types.TierReadLocal},
		{name: "pulumi package (bare)", args: []string{"pulumi", "package"}, want: types.TierReadLocal},
		{name: "pulumi package get-schema", args: []string{"pulumi", "package", "get-schema", "aws"}, want: types.TierReadLocal},

		// WriteLocal — config mutation
		{name: "pulumi config set", args: []string{"pulumi", "config", "set", "aws:region", "us-east-1"}, want: types.TierWriteLocal},
		{name: "pulumi config rm", args: []string{"pulumi", "config", "rm", "aws:region"}, want: types.TierWriteLocal},
		{name: "pulumi config remove", args: []string{"pulumi", "config", "remove", "aws:region"}, want: types.TierWriteLocal},
		{name: "pulumi config cp", args: []string{"pulumi", "config", "cp", "--dest", "prod"}, want: types.TierWriteLocal},
		{name: "pulumi config env", args: []string{"pulumi", "config", "env", "add", "myenv"}, want: types.TierWriteLocal},
		{name: "pulumi config refresh", args: []string{"pulumi", "config", "refresh"}, want: types.TierWriteLocal},

		// WriteLocal — stack mutation (local only)
		{name: "pulumi stack init", args: []string{"pulumi", "stack", "init", "dev"}, want: types.TierWriteLocal},
		{name: "pulumi stack select", args: []string{"pulumi", "stack", "select", "prod"}, want: types.TierWriteLocal},
		{name: "pulumi stack rename", args: []string{"pulumi", "stack", "rename", "new-name"}, want: types.TierWriteLocal},
		{name: "pulumi stack tag set", args: []string{"pulumi", "stack", "tag", "set", "env", "prod"}, want: types.TierWriteRemote},
		{name: "pulumi stack change-secrets-provider", args: []string{"pulumi", "stack", "change-secrets-provider", "awskms://..."}, want: types.TierWriteLocal},

		// WriteLocal — plugin install
		{name: "pulumi plugin install", args: []string{"pulumi", "plugin", "install", "resource", "aws"}, want: types.TierWriteLocal},
		{name: "pulumi plugin add", args: []string{"pulumi", "plugin", "add", "resource", "aws"}, want: types.TierWriteLocal},
		{name: "pulumi plugin rm", args: []string{"pulumi", "plugin", "rm", "resource", "aws"}, want: types.TierWriteLocal},
		{name: "pulumi plugin remove", args: []string{"pulumi", "plugin", "remove", "resource", "aws"}, want: types.TierWriteLocal},

		// WriteLocal — state edits that don't destroy infra
		{name: "pulumi state import", args: []string{"pulumi", "state", "import"}, want: types.TierWriteLocal},
		{name: "pulumi state move", args: []string{"pulumi", "state", "move", "src", "dst"}, want: types.TierWriteLocal},
		{name: "pulumi state rename", args: []string{"pulumi", "state", "rename", "old", "new"}, want: types.TierWriteLocal},
		{name: "pulumi state protect", args: []string{"pulumi", "state", "protect", "urn:..."}, want: types.TierWriteLocal},
		{name: "pulumi state unprotect", args: []string{"pulumi", "state", "unprotect", "urn:..."}, want: types.TierWriteLocal},

		// WriteLocal — package add
		{name: "pulumi package add", args: []string{"pulumi", "package", "add", "aws"}, want: types.TierWriteLocal},

		// WriteLocal — auth / project
		{name: "pulumi login", args: []string{"pulumi", "login"}, want: types.TierWriteLocal},
		{name: "pulumi logout", args: []string{"pulumi", "logout"}, want: types.TierWriteLocal},
		{name: "pulumi new", args: []string{"pulumi", "new", "aws-typescript"}, want: types.TierWriteLocal},

		// WriteLocal — import and refresh (no infra change)
		{name: "pulumi import", args: []string{"pulumi", "import", "aws:s3/bucket:Bucket", "my-bucket", "my-bucket-id"}, want: types.TierWriteLocal},
		{name: "pulumi refresh", args: []string{"pulumi", "refresh"}, want: types.TierWriteLocal},
		{name: "pulumi refresh (--yes)", args: []string{"pulumi", "refresh", "--yes"}, want: types.TierWriteLocal},

		// WriteRemote — deploy to cloud
		{name: "pulumi up", args: []string{"pulumi", "up"}, want: types.TierWriteRemote},
		{name: "pulumi up (--yes)", args: []string{"pulumi", "up", "--yes", "--skip-preview"}, want: types.TierWriteRemote},
		{name: "pulumi up (--target)", args: []string{"pulumi", "up", "--target", "urn:..."}, want: types.TierWriteRemote},
		{name: "pulumi update (alias)", args: []string{"pulumi", "update"}, want: types.TierWriteRemote},
		{name: "pulumi watch", args: []string{"pulumi", "watch"}, want: types.TierWriteRemote},

		// AdminLocal — destructive local state ops
		{name: "pulumi stack rm", args: []string{"pulumi", "stack", "rm", "old-stack"}, want: types.TierAdminLocal},
		{name: "pulumi stack remove", args: []string{"pulumi", "stack", "remove", "old-stack"}, want: types.TierAdminLocal},
		{name: "pulumi state delete", args: []string{"pulumi", "state", "delete", "urn:..."}, want: types.TierAdminLocal},
		{name: "pulumi state edit", args: []string{"pulumi", "state", "edit"}, want: types.TierAdminLocal},

		// AdminRemote — destroy or disrupt cloud resources
		{name: "pulumi destroy", args: []string{"pulumi", "destroy"}, want: types.TierAdminRemote},
		{name: "pulumi destroy (--yes)", args: []string{"pulumi", "destroy", "--yes"}, want: types.TierAdminRemote},
		{name: "pulumi destroy (--target)", args: []string{"pulumi", "destroy", "--target", "urn:..."}, want: types.TierAdminRemote},
		{name: "pulumi cancel", args: []string{"pulumi", "cancel"}, want: types.TierAdminRemote},

		// Global flags before subcommand
		{name: "pulumi --stack dev up", args: []string{"pulumi", "--stack", "dev", "up"}, want: types.TierWriteRemote},
		{name: "pulumi -s prod destroy", args: []string{"pulumi", "-s", "prod", "destroy"}, want: types.TierAdminRemote},
		{name: "pulumi --cwd /app preview", args: []string{"pulumi", "--cwd", "/app", "preview"}, want: types.TierReadRemote},
		{name: "pulumi --non-interactive up", args: []string{"pulumi", "--non-interactive", "up"}, want: types.TierWriteRemote},

		// pulumi env / esc (Pulumi ESC — environments stored in Pulumi Cloud)

		// Read remote
		{name: "pulumi env (bare)", args: []string{"pulumi", "env"}, want: types.TierReadRemote},
		{name: "pulumi esc (alias)", args: []string{"pulumi", "esc"}, want: types.TierReadRemote},
		{name: "pulumi env ls", args: []string{"pulumi", "env", "ls"}, want: types.TierReadRemote},
		{name: "pulumi env list", args: []string{"pulumi", "env", "list"}, want: types.TierReadRemote},
		{name: "pulumi env get", args: []string{"pulumi", "env", "get", "myorg/myenv", "aws.region"}, want: types.TierReadRemote},
		{name: "pulumi env diff", args: []string{"pulumi", "env", "diff", "myorg/myenv"}, want: types.TierReadRemote},
		{name: "pulumi env open", args: []string{"pulumi", "env", "open", "myorg/myenv"}, want: types.TierReadSensitiveRemote},
		{name: "pulumi env run", args: []string{"pulumi", "env", "run", "myorg/myenv", "--", "bash"}, want: types.TierReadSensitiveRemote},
		{name: "pulumi env tag get", args: []string{"pulumi", "env", "tag", "get", "myorg/myenv", "team"}, want: types.TierReadRemote},
		{name: "pulumi env tag ls", args: []string{"pulumi", "env", "tag", "ls", "myorg/myenv"}, want: types.TierReadRemote},
		{name: "pulumi env tag list", args: []string{"pulumi", "env", "tag", "list", "myorg/myenv"}, want: types.TierReadRemote},
		{name: "pulumi env version (bare)", args: []string{"pulumi", "env", "version", "myorg/myenv@1"}, want: types.TierReadRemote},
		{name: "pulumi env version history", args: []string{"pulumi", "env", "version", "history", "myorg/myenv"}, want: types.TierReadRemote},
		{name: "pulumi env version tag ls", args: []string{"pulumi", "env", "version", "tag", "ls", "myorg/myenv"}, want: types.TierReadRemote},
		{name: "pulumi env version tag list", args: []string{"pulumi", "env", "version", "tag", "list", "myorg/myenv"}, want: types.TierReadRemote},

		// WriteRemote — create or modify ESC environment data in Pulumi Cloud
		{name: "pulumi env init", args: []string{"pulumi", "env", "init", "myorg/myenv"}, want: types.TierWriteRemote},
		{name: "pulumi env edit", args: []string{"pulumi", "env", "edit", "myorg/myenv"}, want: types.TierWriteRemote},
		{name: "pulumi env set", args: []string{"pulumi", "env", "set", "myorg/myenv", "key", "val"}, want: types.TierWriteRemote},
		{name: "pulumi env clone", args: []string{"pulumi", "env", "clone", "myorg/src", "--target", "myorg/dst"}, want: types.TierWriteRemote},
		{name: "pulumi env rotate", args: []string{"pulumi", "env", "rotate", "myorg/myenv"}, want: types.TierWriteRemote},
		{name: "pulumi env tag (bare set)", args: []string{"pulumi", "env", "tag", "myorg/myenv", "team", "platform"}, want: types.TierWriteRemote},
		{name: "pulumi env tag mv", args: []string{"pulumi", "env", "tag", "mv", "myorg/myenv", "old", "new"}, want: types.TierWriteRemote},
		{name: "pulumi env tag rm", args: []string{"pulumi", "env", "tag", "rm", "myorg/myenv", "team"}, want: types.TierWriteRemote},
		{name: "pulumi env version rollback", args: []string{"pulumi", "env", "version", "rollback", "myorg/myenv", "5"}, want: types.TierWriteRemote},
		{name: "pulumi env version tag (bare)", args: []string{"pulumi", "env", "version", "tag", "myorg/myenv@5", "stable"}, want: types.TierWriteRemote},
		{name: "pulumi env version tag rm", args: []string{"pulumi", "env", "version", "tag", "rm", "myorg/myenv", "stable"}, want: types.TierWriteRemote},

		// AdminRemote — destructive / irreversible remote ops
		{name: "pulumi env rm", args: []string{"pulumi", "env", "rm", "myorg/myenv"}, want: types.TierAdminRemote},
		{name: "pulumi env remove", args: []string{"pulumi", "env", "remove", "myorg/myenv"}, want: types.TierAdminRemote},
		{name: "pulumi env version retract", args: []string{"pulumi", "env", "version", "retract", "myorg/myenv", "3"}, want: types.TierAdminRemote},

		// esc alias works the same as env
		{name: "pulumi esc init", args: []string{"pulumi", "esc", "init", "myorg/myenv"}, want: types.TierWriteRemote},
		{name: "pulumi esc rm", args: []string{"pulumi", "esc", "rm", "myorg/myenv"}, want: types.TierAdminRemote},

		// Unknown
		{name: "pulumi unknown subcommand", args: []string{"pulumi", "foobar"}, want: types.TierUnknown, unknown: true},
		{name: "pulumi with no args", args: []string{"pulumi"}, want: types.TierUnknown, unknown: true},
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
