package types

// Action represents the permission action level.
type Action int

const (
	ActionUnknown       Action = iota
	ActionRead                 // query state, no mutations
	ActionReadSensitive        // query state, exposes secrets (e.g., pulumi env open)
	ActionWrite                // create/modify resources
	ActionAdmin                // destructive or security-critical operations
)

// String returns the canonical string name for an action.
func (a Action) String() string {
	switch a {
	case ActionRead:
		return "read"
	case ActionReadSensitive:
		return "read-sensitive"
	case ActionWrite:
		return "write"
	case ActionAdmin:
		return "admin"
	default:
		return "unknown"
	}
}

// ParseAction parses an action string. Returns (ActionUnknown, false) if unrecognized.
func ParseAction(s string) (Action, bool) {
	switch s {
	case "read":
		return ActionRead, true
	case "read-sensitive":
		return ActionReadSensitive, true
	case "write":
		return ActionWrite, true
	case "admin":
		return ActionAdmin, true
	default:
		return ActionUnknown, false
	}
}

// Scope represents the locality of a command's side effects.
type Scope int

const (
	ScopeNone   Scope = iota // only used for TierUnknown; all classified tiers require a scope
	ScopeLocal               // operates on local state only (git log, git commit, go fmt)
	ScopeRemote              // operates on or queries remote state (gh pr list, git push, pulumi up)
)

// String returns the canonical string name for a scope.
func (s Scope) String() string {
	switch s {
	case ScopeLocal:
		return "local"
	case ScopeRemote:
		return "remote"
	default:
		return ""
	}
}

// ParseScope parses a scope string. Returns (ScopeNone, false) if unrecognized.
// An empty string is not a valid scope — use ScopeNone directly for unscoped tiers.
func ParseScope(s string) (Scope, bool) {
	switch s {
	case "local":
		return ScopeLocal, true
	case "remote":
		return ScopeRemote, true
	default:
		return ScopeNone, false
	}
}

// Tier represents the permission tier required to run a command.
// It combines an Action (read/read-sensitive/write/admin) with a
// Scope (local/remote). All classified tiers require a scope;
// only TierUnknown uses ScopeNone.
type Tier struct {
	Action Action
	Scope  Scope
}

// Canonical tier values. Classifier code uses these directly.
var (
	TierUnknown            = Tier{ActionUnknown, ScopeNone}
	TierReadLocal          = Tier{ActionRead, ScopeLocal}
	TierReadRemote         = Tier{ActionRead, ScopeRemote}
	TierReadSensitiveLocal = Tier{ActionReadSensitive, ScopeLocal}
	TierReadSensitiveRemote = Tier{ActionReadSensitive, ScopeRemote}
	TierWriteLocal         = Tier{ActionWrite, ScopeLocal}
	TierWriteRemote        = Tier{ActionWrite, ScopeRemote}
	TierAdminLocal         = Tier{ActionAdmin, ScopeLocal}
	TierAdminRemote        = Tier{ActionAdmin, ScopeRemote}
)

// String returns the canonical string representation of a tier.
// Examples: "read local", "read-sensitive remote", "write local", "admin remote".
func (t Tier) String() string {
	s := t.Action.String()
	if t.Scope != ScopeNone {
		s += " " + t.Scope.String()
	}
	return s
}

// Allows reports whether a claimed tier matches the required tier.
// Exact match only — every field must match. There is no hierarchy:
// write does not imply read, admin does not imply write, and
// read-sensitive is independent from all other tiers.
// Unknown tiers never match anything, including themselves.
func (claimed Tier) Allows(required Tier) bool {
	if claimed.Action == ActionUnknown || required.Action == ActionUnknown {
		return false
	}
	return claimed == required
}
