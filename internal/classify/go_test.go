package classify

import (
	"testing"

	"github.com/rgharris/agent-perms/internal/types"
)

func TestClassifyGo(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    types.Tier
		unknown bool
	}{
		// Read — info and verification commands
		{name: "go version", args: []string{"go", "version"}, want: types.TierReadLocal},
		{name: "go list", args: []string{"go", "list", "./..."}, want: types.TierReadLocal},
		{name: "go doc", args: []string{"go", "doc", "fmt.Println"}, want: types.TierReadLocal},
		{name: "go vet", args: []string{"go", "vet", "./..."}, want: types.TierReadLocal},
		{name: "go tool", args: []string{"go", "tool", "pprof"}, want: types.TierWriteLocal},
		{name: "go test", args: []string{"go", "test", "./..."}, want: types.TierWriteLocal},
		{name: "go test (flags)", args: []string{"go", "test", "-v", "-run", "TestFoo", "./..."}, want: types.TierWriteLocal},

		// Read — env (no flags)
		{name: "go env (no flags)", args: []string{"go", "env"}, want: types.TierReadLocal},
		{name: "go env GOPATH", args: []string{"go", "env", "GOPATH"}, want: types.TierReadLocal},

		// Read — mod graph/why/verify
		{name: "go mod graph", args: []string{"go", "mod", "graph"}, want: types.TierReadLocal},
		{name: "go mod why", args: []string{"go", "mod", "why", "github.com/foo/bar"}, want: types.TierReadLocal},
		{name: "go mod verify", args: []string{"go", "mod", "verify"}, want: types.TierReadLocal},

		// Read — build without -o
		{name: "go build (no -o)", args: []string{"go", "build", "./..."}, want: types.TierReadLocal},
		{name: "go build (package)", args: []string{"go", "build", "."}, want: types.TierReadLocal},

		// Write — build with -o
		{name: "go build -o mybinary", args: []string{"go", "build", "-o", "mybinary", "."}, want: types.TierWriteLocal},
		{name: "go build -o=mybinary", args: []string{"go", "build", "-o=mybinary", "."}, want: types.TierWriteLocal},

		// Write — simple subcommands
		{name: "go run", args: []string{"go", "run", "main.go"}, want: types.TierWriteLocal},
		{name: "go fmt", args: []string{"go", "fmt", "./..."}, want: types.TierWriteLocal},
		{name: "go generate", args: []string{"go", "generate", "./..."}, want: types.TierWriteLocal},
		{name: "go install", args: []string{"go", "install", "github.com/foo/bar@latest"}, want: types.TierWriteLocal},
		{name: "go get", args: []string{"go", "get", "./..."}, want: types.TierWriteLocal},
		{name: "go get (versioned)", args: []string{"go", "get", "github.com/foo/bar@v1.2.3"}, want: types.TierWriteLocal},

		// Write — env with -w or -u
		{name: "go env -w", args: []string{"go", "env", "-w", "GOPATH=/tmp"}, want: types.TierWriteLocal},
		{name: "go env -u", args: []string{"go", "env", "-u", "GOPATH"}, want: types.TierWriteLocal},

		// Write — mod subcommands that mutate go.mod / module cache
		{name: "go mod tidy", args: []string{"go", "mod", "tidy"}, want: types.TierWriteLocal},
		{name: "go mod init", args: []string{"go", "mod", "init", "example.com/mymod"}, want: types.TierWriteLocal},
		{name: "go mod edit", args: []string{"go", "mod", "edit", "-require", "github.com/foo/bar@v1.0.0"}, want: types.TierWriteLocal},
		{name: "go mod vendor", args: []string{"go", "mod", "vendor"}, want: types.TierWriteLocal},
		{name: "go mod download", args: []string{"go", "mod", "download"}, want: types.TierWriteLocal},

		// Write — work subcommands
		{name: "go work init", args: []string{"go", "work", "init"}, want: types.TierWriteLocal},
		{name: "go work use", args: []string{"go", "work", "use", "./sub"}, want: types.TierWriteLocal},
		{name: "go work edit", args: []string{"go", "work", "edit", "-go=1.21"}, want: types.TierWriteLocal},
		{name: "go work sync", args: []string{"go", "work", "sync"}, want: types.TierWriteLocal},

		// Write — clean without destructive flags
		{name: "go clean (no flags)", args: []string{"go", "clean"}, want: types.TierWriteLocal},
		{name: "go clean (package)", args: []string{"go", "clean", "."}, want: types.TierWriteLocal},

		// Admin — clean -cache or -modcache
		{name: "go clean -cache", args: []string{"go", "clean", "-cache"}, want: types.TierAdminLocal},
		{name: "go clean -modcache", args: []string{"go", "clean", "-modcache"}, want: types.TierAdminLocal},

		// Help flags → read local
		{name: "go --help", args: []string{"go", "--help"}, want: types.TierReadLocal},
		{name: "go -h", args: []string{"go", "-h"}, want: types.TierReadLocal},
		{name: "go help", args: []string{"go", "help"}, want: types.TierReadLocal},
		{name: "go test --help", args: []string{"go", "test", "--help"}, want: types.TierReadLocal},
		{name: "go build --help", args: []string{"go", "build", "--help"}, want: types.TierReadLocal},

		// Unknown
		{name: "go unknown subcommand", args: []string{"go", "foobar"}, want: types.TierUnknown, unknown: true},
		{name: "go with no args", args: []string{"go"}, want: types.TierUnknown, unknown: true},
		{name: "go mod (no sub)", args: []string{"go", "mod"}, want: types.TierUnknown, unknown: true},
		{name: "go work (no sub)", args: []string{"go", "work"}, want: types.TierUnknown, unknown: true},
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
