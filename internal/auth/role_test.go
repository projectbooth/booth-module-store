package auth

import "testing"

func TestRoleInWorkspace(t *testing.T) {
	cases := []struct {
		name      string
		groups    []string
		workspace string
		want      Role
	}{
		{"owner", []string{"/workspaces/acme/owner"}, "acme", RoleOwner},
		{"viewer", []string{"/workspaces/acme/viewer"}, "acme", RoleViewer},
		{"no membership in requested workspace", []string{"/workspaces/other/owner"}, "acme", ""},
		{"no groups at all", nil, "acme", ""},
		{"unrelated IdP groups are ignored", []string{"/some/other/group", "engineering"}, "acme", ""},
		{"strongest role wins when several match", []string{"/workspaces/acme/viewer", "/workspaces/acme/owner"}, "acme", RoleOwner},
		{"role in one workspace doesn't leak into another", []string{"/workspaces/acme/owner", "/workspaces/beta/viewer"}, "beta", RoleViewer},
		{"unrecognized role name grants nothing", []string{"/workspaces/acme/admin"}, "acme", ""},
		{"prefix of a slug is not a match", []string{"/workspaces/acme-corp/owner"}, "acme", ""},
		{"trailing junk is not a match", []string{"/workspaces/acme/owner/extra"}, "acme", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := RoleInWorkspace(c.groups, c.workspace); got != c.want {
				t.Errorf("RoleInWorkspace(%v, %q) = %q, want %q", c.groups, c.workspace, got, c.want)
			}
		})
	}
}
