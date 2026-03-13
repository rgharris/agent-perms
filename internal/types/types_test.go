package types

import "testing"

func TestParseActionRoundTrip(t *testing.T) {
	actions := []struct {
		str    string
		action Action
	}{
		{"read", ActionRead},
		{"read-sensitive", ActionReadSensitive},
		{"write", ActionWrite},
		{"admin", ActionAdmin},
	}
	for _, tt := range actions {
		t.Run(tt.str, func(t *testing.T) {
			parsed, ok := ParseAction(tt.str)
			if !ok {
				t.Fatalf("ParseAction(%q) returned ok=false", tt.str)
			}
			if parsed != tt.action {
				t.Errorf("ParseAction(%q) = %v, want %v", tt.str, parsed, tt.action)
			}
			if parsed.String() != tt.str {
				t.Errorf("Action(%v).String() = %q, want %q", parsed, parsed.String(), tt.str)
			}
		})
	}
}

func TestParseActionUnknown(t *testing.T) {
	invalid := []string{"", "readwrite", "ADMIN", "local", "remote", "sensitive", "read sensitive"}
	for _, s := range invalid {
		action, ok := ParseAction(s)
		if ok {
			t.Errorf("ParseAction(%q) = (%v, true), want (_, false)", s, action)
		}
		if action != ActionUnknown {
			t.Errorf("ParseAction(%q) action = %v, want ActionUnknown", s, action)
		}
	}
}

func TestParseScopeRoundTrip(t *testing.T) {
	scopes := []struct {
		str   string
		scope Scope
	}{
		{"local", ScopeLocal},
		{"remote", ScopeRemote},
	}
	for _, tt := range scopes {
		t.Run(tt.str, func(t *testing.T) {
			parsed, ok := ParseScope(tt.str)
			if !ok {
				t.Fatalf("ParseScope(%q) returned ok=false", tt.str)
			}
			if parsed != tt.scope {
				t.Errorf("ParseScope(%q) = %v, want %v", tt.str, parsed, tt.scope)
			}
			if parsed.String() != tt.str {
				t.Errorf("Scope(%v).String() = %q, want %q", parsed, parsed.String(), tt.str)
			}
		})
	}
}

func TestParseScopeUnknown(t *testing.T) {
	invalid := []string{"", "read", "write", "admin", "nowhere", "sensitive"}
	for _, s := range invalid {
		scope, ok := ParseScope(s)
		if ok {
			t.Errorf("ParseScope(%q) = (%v, true), want (_, false)", s, scope)
		}
		if scope != ScopeNone {
			t.Errorf("ParseScope(%q) scope = %v, want ScopeNone", s, scope)
		}
	}
}

func TestTierString(t *testing.T) {
	tests := []struct {
		tier Tier
		want string
	}{
		{TierReadLocal, "read local"},
		{TierReadRemote, "read remote"},
		{TierReadSensitiveLocal, "read-sensitive local"},
		{TierReadSensitiveRemote, "read-sensitive remote"},
		{TierWriteLocal, "write local"},
		{TierWriteRemote, "write remote"},
		{TierAdminLocal, "admin local"},
		{TierAdminRemote, "admin remote"},
		{TierUnknown, "unknown"},
	}
	for _, tt := range tests {
		if got := tt.tier.String(); got != tt.want {
			t.Errorf("Tier%v.String() = %q, want %q", tt.tier, got, tt.want)
		}
	}
}

func TestAllows(t *testing.T) {
	tests := []struct {
		claimed  Tier
		required Tier
		want     bool
	}{
		// Exact match — every tier allows itself
		{TierReadLocal, TierReadLocal, true},
		{TierReadRemote, TierReadRemote, true},
		{TierReadSensitiveLocal, TierReadSensitiveLocal, true},
		{TierReadSensitiveRemote, TierReadSensitiveRemote, true},
		{TierWriteLocal, TierWriteLocal, true},
		{TierWriteRemote, TierWriteRemote, true},
		{TierAdminLocal, TierAdminLocal, true},
		{TierAdminRemote, TierAdminRemote, true},

		// No hierarchy — higher tiers do NOT allow lower tiers
		{TierWriteLocal, TierReadLocal, false},
		{TierWriteRemote, TierReadRemote, false},
		{TierAdminLocal, TierReadLocal, false},
		{TierAdminRemote, TierReadRemote, false},
		{TierAdminLocal, TierWriteLocal, false},
		{TierAdminRemote, TierWriteRemote, false},

		// read vs read-sensitive — independent
		{TierReadLocal, TierReadSensitiveLocal, false},
		{TierReadSensitiveLocal, TierReadLocal, false},
		{TierReadRemote, TierReadSensitiveRemote, false},
		{TierReadSensitiveRemote, TierReadRemote, false},

		// read-sensitive is independent from write/admin
		{TierWriteLocal, TierReadSensitiveLocal, false},
		{TierWriteRemote, TierReadSensitiveRemote, false},
		{TierAdminLocal, TierReadSensitiveLocal, false},
		{TierAdminRemote, TierReadSensitiveRemote, false},

		// Cross-scope: never
		{TierReadLocal, TierReadRemote, false},
		{TierReadRemote, TierReadLocal, false},
		{TierAdminLocal, TierWriteRemote, false},
		{TierAdminRemote, TierWriteLocal, false},
		{TierWriteLocal, TierWriteRemote, false},
		{TierWriteRemote, TierWriteLocal, false},

		// Unknown never allows anything
		{TierUnknown, TierReadLocal, false},
		{TierReadLocal, TierUnknown, false},
		{TierUnknown, TierUnknown, false},
	}

	for _, tt := range tests {
		got := tt.claimed.Allows(tt.required)
		if got != tt.want {
			t.Errorf("%v.Allows(%v) = %v, want %v", tt.claimed, tt.required, got, tt.want)
		}
	}
}

func TestMax(t *testing.T) {
	tests := []struct {
		name string
		a, b Tier
		want Tier
	}{
		// Higher action wins
		{name: "read vs write", a: TierReadRemote, b: TierWriteRemote, want: TierWriteRemote},
		{name: "read-sensitive vs write", a: TierReadSensitiveRemote, b: TierWriteRemote, want: TierWriteRemote},
		{name: "read-sensitive vs admin", a: TierReadSensitiveRemote, b: TierAdminRemote, want: TierAdminRemote},
		{name: "read vs read-sensitive", a: TierReadRemote, b: TierReadSensitiveRemote, want: TierReadSensitiveRemote},
		{name: "write vs admin", a: TierWriteRemote, b: TierAdminRemote, want: TierAdminRemote},

		// Equal action — wider scope wins
		{name: "read local vs read remote", a: TierReadLocal, b: TierReadRemote, want: TierReadRemote},
		{name: "write local vs write remote", a: TierWriteLocal, b: TierWriteRemote, want: TierWriteRemote},

		// Same tier returns same tier
		{name: "read remote vs read remote", a: TierReadRemote, b: TierReadRemote, want: TierReadRemote},
		{name: "admin local vs admin local", a: TierAdminLocal, b: TierAdminLocal, want: TierAdminLocal},

		// Unknown is treated as zero
		{name: "unknown vs read", a: TierUnknown, b: TierReadRemote, want: TierReadRemote},
		{name: "write vs unknown", a: TierWriteLocal, b: TierUnknown, want: TierWriteLocal},
		{name: "unknown vs unknown", a: TierUnknown, b: TierUnknown, want: TierUnknown},

		// Order shouldn't matter
		{name: "admin vs read (reversed)", a: TierAdminRemote, b: TierReadRemote, want: TierAdminRemote},
		{name: "write vs read-sensitive (reversed)", a: TierWriteRemote, b: TierReadSensitiveRemote, want: TierWriteRemote},

		// Cross-scope: higher action still wins regardless of scope
		{name: "read remote vs write local", a: TierReadRemote, b: TierWriteLocal, want: TierWriteLocal},
		{name: "admin local vs read-sensitive remote", a: TierAdminLocal, b: TierReadSensitiveRemote, want: TierAdminLocal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Max(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("Max(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
