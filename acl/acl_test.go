package acl_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/go-bolo/bolo/acl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withTempWorkingDir isolates the CWD (LoadRoles reads ./acl.json from it)
// and restores the previous dir on cleanup. Never use with t.Parallel.
func withTempWorkingDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))

	t.Cleanup(func() {
		require.NoError(t, os.Chdir(oldWd))
	})

	return dir
}

func TestRole(t *testing.T) {
	r, _ := acl.NewRole(&acl.NewRoleOpts{Name: "editor"})
	assert.Equal(t, 0, len(r.Permissions))
	assert.False(t, r.Can("find_image"))

	r.AddPermission("upload_image")
	r.AddPermission("find_image")

	assert.Equal(t, 2, len(r.Permissions))
	assert.True(t, r.Can("find_image"))

	r.RemovePermission("find_image")

	assert.Equal(t, 1, len(r.Permissions))
	assert.False(t, r.Can("find_image"))
}

func TestNewRole(t *testing.T) {
	type args struct {
		opts *acl.NewRoleOpts
	}
	tests := []struct {
		name    string
		args    args
		want    *acl.Role
		wantErr bool
	}{
		{
			"success empty",
			args{opts: &acl.NewRoleOpts{
				Name: "faxineira",
			}},
			&acl.Role{
				Name: "faxineira",
			},
			false,
		},
		{
			"error no name",
			args{opts: &acl.NewRoleOpts{}},
			nil,
			true,
		},
		{
			"success with permissions",
			args{opts: &acl.NewRoleOpts{
				Name:        "porteiro",
				Permissions: []string{"block-user-access"},
			}},
			&acl.Role{
				Name:        "porteiro",
				Permissions: []string{"block-user-access"},
			},
			false,
		},
		{
			"success with systemRole",
			args{opts: &acl.NewRoleOpts{
				Name:         "editor",
				IsSystemRole: true,
			}},
			&acl.Role{
				Name:         "editor",
				IsSystemRole: true,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := acl.NewRole(tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRole() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewRole() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRole_Can(t *testing.T) {
	editoRole := acl.Role{
		Name:          "editor",
		Permissions:   []string{"update_image", "find_image", "create_content"},
		CanAddInUsers: true,
		IsSystemRole:  true,
	}

	type args struct {
		permission string
	}
	tests := []struct {
		name string
		r    *acl.Role
		args args
		want bool
	}{
		{
			"can find_image",
			&editoRole,
			args{permission: "find_image"},
			true,
		},
		{
			"cant delete_content",
			&editoRole,
			args{permission: "delete_content"},
			false,
		},
		{
			"cant create_content",
			&editoRole,
			args{permission: "create_content"},
			true,
		},
		{
			"lixeiro cant jogar_lixo_na_rua",
			&acl.Role{
				Name:          "lixeiro",
				Permissions:   []string{"pegar_lixo"},
				CanAddInUsers: true,
				IsSystemRole:  false,
			},
			args{permission: "jogar_lixo_na_rua"},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.Can(tt.args.permission); got != tt.want {
				t.Errorf("Role.Can() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ACL-03: repeated AddPermission should not duplicate; a new permission
// should preserve the existing ones.
func TestRole_AddPermission(t *testing.T) {
	type args struct {
		permission string
	}
	tests := []struct {
		name          string
		r             *acl.Role
		args          args
		wantPerms     []string
		wantCanTarget bool
	}{
		{
			"add to role without permissions",
			&acl.Role{Name: "editor"},
			args{permission: "upload_image"},
			[]string{"upload_image"},
			true,
		},
		{
			"repeated permission should not duplicate",
			&acl.Role{Name: "editor", Permissions: []string{"upload_image"}},
			args{permission: "upload_image"},
			[]string{"upload_image"},
			true,
		},
		{
			"new permission should preserve existing ones",
			&acl.Role{Name: "editor", Permissions: []string{"find_image"}},
			args{permission: "upload_image"},
			[]string{"find_image", "upload_image"},
			true,
		},
		{
			"repeated permission among others should not duplicate",
			&acl.Role{Name: "editor", Permissions: []string{"find_image", "upload_image"}},
			args{permission: "find_image"},
			[]string{"find_image", "upload_image"},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.r.AddPermission(tt.args.permission)

			assert.Equal(t, tt.wantPerms, tt.r.Permissions)
			assert.Equal(t, tt.wantCanTarget, tt.r.Can(tt.args.permission))
		})
	}
}

// ACL-03: removing an absent permission or removing from an empty list are
// no-ops; removing an existing permission preserves the other permissions.
func TestRole_RemovePermission(t *testing.T) {
	type args struct {
		permission string
	}
	tests := []struct {
		name      string
		r         *acl.Role
		args      args
		wantPerms []string
	}{
		{
			"remove on role with empty permission list is a no-op",
			&acl.Role{Name: "editor"},
			args{permission: "upload_image"},
			nil,
		},
		{
			"remove absent permission is a no-op",
			&acl.Role{Name: "editor", Permissions: []string{"find_image"}},
			args{permission: "upload_image"},
			[]string{"find_image"},
		},
		{
			"remove existing permission preserves the other permissions",
			&acl.Role{Name: "editor", Permissions: []string{"create_content", "update_image", "find_image"}},
			args{permission: "update_image"},
			[]string{"create_content", "find_image"},
		},
		{
			"remove first permission preserves the remaining ones",
			&acl.Role{Name: "editor", Permissions: []string{"create_content", "find_image"}},
			args{permission: "create_content"},
			[]string{"find_image"},
		},
		{
			"remove last permission leaves empty list",
			&acl.Role{Name: "editor", Permissions: []string{"create_content"}},
			args{permission: "create_content"},
			[]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.r.RemovePermission(tt.args.permission)

			assert.Equal(t, tt.wantPerms, tt.r.Permissions)
			assert.False(t, tt.r.Can(tt.args.permission))
		})
	}
}

// ACL-06: without acl.json in the CWD, LoadRoles must return valid defaults
// containing the expected system roles; with a valid file it must preserve
// the file content.
func TestLoadRoles(t *testing.T) {
	// no t.Parallel: this fixture changes the process working directory
	t.Run("should use valid defaults when acl.json does not exist", func(t *testing.T) {
		withTempWorkingDir(t)

		got, err := acl.LoadRoles()
		require.NoError(t, err)

		roles := map[string]*acl.Role{}
		require.NoError(t, json.Unmarshal([]byte(got), &roles), "default roles should be valid JSON")

		for _, name := range []string{"administrator", "authenticated", "unAuthenticated", "owner"} {
			require.Contains(t, roles, name, "default roles should contain system role "+name)
			assert.True(t, roles[name].IsSystemRole, "role %s should be a system role", name)
			assert.Equal(t, name, roles[name].Name)
		}
	})

	t.Run("should preserve roles when acl.json is valid", func(t *testing.T) {
		dir := withTempWorkingDir(t)

		content := `{"editor":{"name":"editor","permissions":["update_page"],"canAddInUsers":false,"isSystemRole":false}}`
		require.NoError(t, os.WriteFile(filepath.Join(dir, "acl.json"), []byte(content), 0600))

		got, err := acl.LoadRoles()
		require.NoError(t, err)
		assert.Equal(t, content, got, "LoadRoles should preserve the acl.json content")

		roles := map[string]*acl.Role{}
		require.NoError(t, json.Unmarshal([]byte(got), &roles))
		require.Contains(t, roles, "editor")
		assert.Equal(t, []string{"update_page"}, roles["editor"].Permissions)
	})

	t.Run("default roles should contain expected system roles", func(t *testing.T) {
		dir := withTempWorkingDir(t)
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "no-acl-here"), 0755))
		require.NoError(t, os.Chdir(filepath.Join(dir, "no-acl-here")))

		got, err := acl.LoadRoles()
		require.NoError(t, err)

		roles := map[string]*acl.Role{}
		require.NoError(t, json.Unmarshal([]byte(got), &roles))

		admin, ok := roles["administrator"]
		require.True(t, ok, "defaults should contain the administrator sentinel role")
		assert.True(t, admin.CanAddInUsers, "administrator should be able to add users")
	})
}

// ACL-01: a role with the permission grants access; without it (including
// empty permission list) denies.
func TestRole_Can_ACL01_PermissionContract(t *testing.T) {
	tests := []struct {
		name string
		r    *acl.Role
		want bool
	}{
		{
			"role with permission grants access",
			&acl.Role{Name: "editor", Permissions: []string{"update_page"}},
			true,
		},
		{
			"role without the permission denies access",
			&acl.Role{Name: "editor", Permissions: []string{"find_page"}},
			false,
		},
		{
			"role with nil permission list denies access",
			&acl.Role{Name: "editor"},
			false,
		},
		{
			"role with empty permission list denies access",
			&acl.Role{Name: "editor", Permissions: []string{}},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.r.Can("update_page"))
		})
	}
}

// ACL-02: Can on a nil role must deny access without panicking.
// Known failure: nil pointer dereference on the Permissions field today.
func TestRole_Can_ACL02_NilReceiver(t *testing.T) {
	var r *acl.Role

	var got bool
	assert.NotPanics(t, func() {
		got = r.Can("update_page")
	}, "Role.Can with nil receiver should not panic")

	assert.False(t, got, "nil role should deny any permission")
}
