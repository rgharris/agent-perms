package classify

import (
	"testing"

	"github.com/rgharris/agent-perms/internal/types"
)

func TestClassifyGHAPI(t *testing.T) {
	tests := []struct {
		name string
		args []string // everything after "gh api"
		want types.Tier
	}{
		// Default GET (no flags)
		{name: "no args → GET → read", args: []string{"/repos/owner/repo"}, want: types.TierReadRemote},
		{name: "endpoint only → read", args: []string{"/user"}, want: types.TierReadRemote},

		// Explicit --method
		{name: "--method GET → read", args: []string{"--method", "GET", "/repos/owner/repo"}, want: types.TierReadRemote},
		{name: "--method POST → write", args: []string{"--method", "POST", "/repos/owner/repo/issues"}, want: types.TierWriteRemote},
		{name: "--method PATCH → write", args: []string{"--method", "PATCH", "/repos/owner/repo/issues/1"}, want: types.TierWriteRemote},
		{name: "--method PUT → write", args: []string{"--method", "PUT", "/repos/owner/repo/branches/main/protection"}, want: types.TierWriteRemote},
		{name: "--method DELETE → admin", args: []string{"--method", "DELETE", "/repos/owner/repo"}, want: types.TierAdminRemote},

		// --method= form
		{name: "--method=GET → read", args: []string{"--method=GET", "/user"}, want: types.TierReadRemote},
		{name: "--method=DELETE → admin", args: []string{"--method=DELETE", "/repos/owner/repo"}, want: types.TierAdminRemote},

		// -X alias
		{name: "-X GET → read", args: []string{"-X", "GET", "/user"}, want: types.TierReadRemote},
		{name: "-X POST → write", args: []string{"-X", "POST", "/repos/owner/repo/issues"}, want: types.TierWriteRemote},
		{name: "-X DELETE → admin", args: []string{"-X", "DELETE", "/repos/owner/repo"}, want: types.TierAdminRemote},
		{name: "-XDELETE → admin", args: []string{"-XDELETE", "/repos/owner/repo"}, want: types.TierAdminRemote},
		{name: "-XPOST → write", args: []string{"-XPOST", "/repos/owner/repo/issues"}, want: types.TierWriteRemote},

		// Implicit POST from -f/-F flags
		{name: "-f field → implicit POST → write", args: []string{"/repos/owner/repo/issues", "-f", "title=Bug"}, want: types.TierWriteRemote},
		{name: "-F field → implicit POST → write", args: []string{"/repos/owner/repo/issues", "-F", "assignees[]=octocat"}, want: types.TierWriteRemote},
		{name: "-f= form → implicit POST → write", args: []string{"/repos/owner/repo", "-f=key=value"}, want: types.TierWriteRemote},
		{name: "--input → implicit POST → write", args: []string{"/repos/owner/repo/issues", "--input", "body.json"}, want: types.TierWriteRemote},

		// GraphQL queries (read)
		{name: "graphql query → read", args: []string{"graphql", "-f", "query=query { viewer { login } }"}, want: types.TierReadRemote},
		{name: "/graphql query → read", args: []string{"/graphql", "-f", "query=query { viewer { login } }"}, want: types.TierReadRemote},

		// GraphQL mutations (write)
		{name: "graphql mutation → write", args: []string{"graphql", "-f", "query=mutation { createIssue(input: {}) { issue { number } } }"}, want: types.TierWriteRemote},
		{name: "graphql mutation uppercase → write", args: []string{"graphql", "-f", "query=mutation AddComment($body: String!) { addComment(input: {body: $body}) { subject { id } } }"}, want: types.TierWriteRemote},

		// Case insensitivity of methods
		{name: "--method delete (lowercase) → admin", args: []string{"--method", "delete", "/repos/owner/repo"}, want: types.TierAdminRemote},
		{name: "--method post (lowercase) → write", args: []string{"--method", "post", "/repos/owner/repo/issues"}, want: types.TierWriteRemote},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build full args with "gh api" prefix for Classify
			args := append([]string{"gh", "api"}, tt.args...)
			got := Classify(args)
			if got.Tier != tt.want {
				t.Errorf("Classify(%v) tier = %v, want %v", args, got.Tier, tt.want)
			}
			if got.Unknown {
				t.Errorf("Classify(%v) unexpected unknown=true", args)
			}
		})
	}
}

func TestTierComparison(t *testing.T) {
	tests := []struct {
		claimed  types.Tier
		required types.Tier
		want     bool
	}{
		// Exact match — only identical tiers match
		{types.TierReadRemote, types.TierReadRemote, true},
		{types.TierReadSensitiveRemote, types.TierReadSensitiveRemote, true},
		{types.TierWriteLocal, types.TierWriteLocal, true},
		{types.TierWriteRemote, types.TierWriteRemote, true},
		{types.TierAdminLocal, types.TierAdminLocal, true},
		{types.TierAdminRemote, types.TierAdminRemote, true},

		// No hierarchy — higher tiers do NOT allow lower tiers
		{types.TierAdminLocal, types.TierWriteLocal, false},
		{types.TierAdminRemote, types.TierWriteRemote, false},
		{types.TierWriteLocal, types.TierReadRemote, false},
		{types.TierWriteRemote, types.TierReadRemote, false},
		{types.TierAdminLocal, types.TierReadRemote, false},
		{types.TierAdminRemote, types.TierReadRemote, false},

		// Lower tier still denied for higher required
		{types.TierWriteLocal, types.TierAdminLocal, false},
		{types.TierWriteRemote, types.TierAdminRemote, false},

		// read vs read-sensitive — independent
		{types.TierReadRemote, types.TierReadSensitiveRemote, false},
		{types.TierReadSensitiveRemote, types.TierReadRemote, false},
		{types.TierWriteLocal, types.TierReadSensitiveRemote, false},
		{types.TierAdminRemote, types.TierReadSensitiveRemote, false},

		// Unknown never allows
		{types.TierUnknown, types.TierReadRemote, false},
		{types.TierReadRemote, types.TierUnknown, false},
		{types.TierUnknown, types.TierUnknown, false},

		// Cross-locality: never
		{types.TierAdminLocal, types.TierWriteRemote, false},
		{types.TierAdminRemote, types.TierWriteLocal, false},
		{types.TierWriteLocal, types.TierWriteRemote, false},
		{types.TierWriteRemote, types.TierWriteLocal, false},
	}

	for _, tt := range tests {
		got := tt.claimed.Allows(tt.required)
		if got != tt.want {
			t.Errorf("%v.Allows(%v) = %v, want %v", tt.claimed, tt.required, got, tt.want)
		}
	}
}
