package auth

import "regexp"

// Role is a caller's workspace role (ADR 0025). Exactly three values.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

// groupRE is ADR 0025's workspace-membership group shape — the same grammar
// booth-core derives roles from.
var groupRE = regexp.MustCompile(`^/workspaces/([a-z0-9-]+)/(owner|editor|viewer)$`)

// rank orders roles by strength; an unrecognized value ranks 0 (no access).
func rank(r Role) int {
	switch r {
	case RoleOwner:
		return 3
	case RoleEditor:
		return 2
	case RoleViewer:
		return 1
	}
	return 0
}

// RoleInWorkspace returns the strongest role the token's groups grant in workspace, or
// "" if they grant none. Non-matching groups (a user's unrelated IdP groups) are
// ignored, as in booth-core.
func RoleInWorkspace(groups []string, workspace string) Role {
	var best Role
	for _, g := range groups {
		m := groupRE.FindStringSubmatch(g)
		if m == nil || m[1] != workspace {
			continue
		}
		if r := Role(m[2]); rank(r) > rank(best) {
			best = r
		}
	}
	return best
}
