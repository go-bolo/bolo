package bolo

// App authorization contract tests. Matrix IDs: ACL-01, ACL-02, ACL-04,
// ACL-05, ACL-07. Each test builds its own App via NewApp; CWD/env fixtures
// are restored on cleanup. Never run with t.Parallel.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-bolo/bolo/acl"
	"github.com/go-bolo/bolo/http_client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// changeToTempWorkingDir isolates the CWD (LoadRoles reads ./acl.json from
// it) and restores the previous dir on cleanup.
func changeToTempWorkingDir(t *testing.T) string {
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

// newACLAppWithRolesLoaded builds a fresh App with an isolated CWD (so
// NewApp/LoadRoles never read a local acl.json) and initializes RolesList
// from the RolesString JSON, like Bootstrap does.
func newACLAppWithRolesLoaded(t *testing.T) *AppStruct {
	t.Helper()

	changeToTempWorkingDir(t)

	app := NewApp(&AppOptions{}).(*AppStruct)

	require.NoError(t, json.Unmarshal([]byte(app.RolesString), &app.RolesList),
		"fixture: RolesString should be valid roles JSON")
	require.NotEmpty(t, app.RolesList, "fixture: RolesList should be initialized")

	return app
}

// newBootableApp is the ACL-07 Bootstrap fixture: temporary SQLite DB,
// templates and plugin routes disabled, so failures are attributable to the
// ACL loading, not to templates/DB/routes.
func newBootableApp(t *testing.T) *AppStruct {
	t.Helper()

	t.Setenv("DB_ENGINE", "sqlite")
	t.Setenv("DB_URI", "file:"+filepath.Join(t.TempDir(), "bolo-acl07.db"))
	t.Setenv("TEMPLATE_DISABLE", "true")

	app := NewApp(&AppOptions{}).(*AppStruct)
	app.SetDisablePluginRoutes(true)

	client := http_client.HttpClient
	t.Cleanup(func() {
		http_client.HttpClient = client
		if app.DB != nil {
			if sqlDB, err := app.DB.DB(); err == nil {
				sqlDB.Close()
			}
		}
	})

	return app
}

// aclTestUser is a minimal UserInterface mock for ACL-05 context tests.
type aclTestUser struct {
	id    string
	roles []string
}

func (u *aclTestUser) GetID() string                 { return u.id }
func (u *aclTestUser) SetID(id string) error         { u.id = id; return nil }
func (u *aclTestUser) GetRoles() []string            { return u.roles }
func (u *aclTestUser) SetRoles(v []string) error     { u.roles = v; return nil }
func (u *aclTestUser) AddRole(role string) error     { u.roles = append(u.roles, role); return nil }
func (u *aclTestUser) RemoveRole(role string) error  { return nil }
func (u *aclTestUser) GetEmail() string              { return "" }
func (u *aclTestUser) SetEmail(v string) error       { return nil }
func (u *aclTestUser) GetUsername() string           { return "" }
func (u *aclTestUser) SetUsername(v string) error    { return nil }
func (u *aclTestUser) GetDisplayName() string        { return "" }
func (u *aclTestUser) SetDisplayName(v string) error { return nil }
func (u *aclTestUser) GetFullName() string           { return "" }
func (u *aclTestUser) SetFullName(v string) error    { return nil }
func (u *aclTestUser) GetLanguage() string           { return "" }
func (u *aclTestUser) SetLanguage(v string) error    { return nil }
func (u *aclTestUser) IsActive() bool                { return true }
func (u *aclTestUser) SetActive(blocked bool) error  { return nil }
func (u *aclTestUser) IsBlocked() bool               { return false }
func (u *aclTestUser) SetBlocked(blocked bool) error { return nil }
func (u *aclTestUser) FillById(ID string) error      { u.id = ID; return nil }

// ACL-01 at App level: administrator bypass, grant in one of several roles,
// role without grant and empty roles list deny.
func TestApp_Can_ACL01_PermissionContract(t *testing.T) {
	newApp := func(t *testing.T) *AppStruct {
		app := newACLAppWithRolesLoaded(t)
		app.RolesList["editor"] = &acl.Role{
			Name:        "editor",
			Permissions: []string{"update_page"},
		}
		app.RolesList["viewer"] = &acl.Role{
			Name:        "viewer",
			Permissions: []string{"find_page"},
		}
		return app
	}

	t.Run("administrator bypasses permission check", func(t *testing.T) {
		app := newApp(t)
		assert.True(t, app.Can("any_permission", []string{"administrator"}))
	})

	t.Run("grant in one of several roles grants access", func(t *testing.T) {
		app := newApp(t)
		assert.True(t, app.Can("update_page", []string{"viewer", "editor"}))
	})

	t.Run("role without the permission denies access", func(t *testing.T) {
		app := newApp(t)
		assert.False(t, app.Can("update_page", []string{"viewer"}))
	})

	t.Run("empty roles list denies access", func(t *testing.T) {
		app := newApp(t)
		assert.False(t, app.Can("update_page", []string{}))
		assert.False(t, app.Can("update_page", nil))
	})
}

// ACL-02: unknown role, nil map entry and unknown-then-valid role must be
// resolved without panicking.
// Known failure: App.Can dereferences a nil role and panics today.
func TestApp_Can_ACL02_UnknownRole(t *testing.T) {
	newApp := func(t *testing.T) *AppStruct {
		app := newACLAppWithRolesLoaded(t)
		app.RolesList["editor"] = &acl.Role{
			Name:        "editor",
			Permissions: []string{"update_page"},
		}
		return app
	}

	t.Run("unknown role should deny without panic", func(t *testing.T) {
		app := newApp(t)

		var got bool
		assert.NotPanics(t, func() {
			got = app.Can("update_page", []string{"ghost-role"})
		}, "App.Can with unknown role should not panic")
		assert.False(t, got, "unknown role should deny the permission")
	})

	t.Run("nil entry in the roles map should deny without panic", func(t *testing.T) {
		app := newApp(t)
		app.RolesList["nil-role"] = nil

		var got bool
		assert.NotPanics(t, func() {
			got = app.Can("update_page", []string{"nil-role"})
		}, "App.Can with nil role entry should not panic")
		assert.False(t, got, "nil role entry should deny the permission")
	})

	t.Run("unknown role followed by valid role with grant should grant", func(t *testing.T) {
		app := newApp(t)

		var got bool
		assert.NotPanics(t, func() {
			got = app.Can("update_page", []string{"ghost-role", "editor"})
		}, "App.Can should skip unknown roles instead of panicking")
		assert.True(t, got, "valid role with grant should concede the permission")
	})
}

// ACL-04: GetRolePermission must reflect grant/revoke; SetRolePermission for
// an unknown role must return an error.
// Known failure: GetRolePermission is a stub returning false and
// SetRolePermission returns no error for unknown roles.
func TestApp_SetGetRolePermission_ACL04(t *testing.T) {
	t.Run("GetRolePermission should reflect grant and revoke", func(t *testing.T) {
		app := newACLAppWithRolesLoaded(t)
		require.NoError(t, app.SetRole("editor", acl.Role{Name: "editor"}),
			"fixture: SetRole should register the editor role")

		err := app.SetRolePermission("editor", "update_page", true)
		require.NoError(t, err)

		assert.True(t, app.GetRolePermission("editor", "update_page"),
			"GetRolePermission should return true after a grant")
		assert.True(t, app.Can("update_page", []string{"editor"}),
			"grant should be visible through App.Can")

		err = app.SetRolePermission("editor", "update_page", false)
		require.NoError(t, err)

		assert.False(t, app.GetRolePermission("editor", "update_page"),
			"GetRolePermission should return false after a revoke")
	})

	t.Run("GetRolePermission should be false for permission not granted", func(t *testing.T) {
		app := newACLAppWithRolesLoaded(t)
		require.NoError(t, app.SetRole("editor", acl.Role{Name: "editor"}))

		assert.False(t, app.GetRolePermission("editor", "update_page"))
	})

	t.Run("SetRolePermission for unknown role should return error", func(t *testing.T) {
		app := newACLAppWithRolesLoaded(t)

		err := app.SetRolePermission("ghost-role", "update_page", true)
		assert.Error(t, err, "SetRolePermission should fail for an unknown role instead of silently ignoring it")
	})
}

// ACL-05: anonymous contexts use the unAuthenticated role, authenticated
// contexts use their own roles, and a user without roles gains no permission.
func TestRequestContext_Can_ACL05(t *testing.T) {
	newApp := func(t *testing.T) *AppStruct {
		app := newACLAppWithRolesLoaded(t)
		app.RolesList["unAuthenticated"] = &acl.Role{
			Name:         "unAuthenticated",
			Permissions:  []string{"find_public_page"},
			IsSystemRole: true,
		}
		app.RolesList["authenticated"] = &acl.Role{
			Name:         "authenticated",
			Permissions:  []string{},
			IsSystemRole: true,
		}
		app.RolesList["editor"] = &acl.Role{
			Name:        "editor",
			Permissions: []string{"update_page"},
		}
		return app
	}

	t.Run("anonymous context should use the unAuthenticated role", func(t *testing.T) {
		app := newApp(t)
		ctx, err := NewBotContext(app)
		require.NoError(t, err)
		require.False(t, ctx.IsAuthenticated)

		roles := *ctx.GetAuthenticatedRoles()
		assert.Equal(t, []string{"unAuthenticated"}, roles)

		assert.True(t, ctx.Can("find_public_page"),
			"anonymous context should inherit unAuthenticated grants")
		assert.False(t, ctx.Can("update_page"),
			"anonymous context should not inherit other roles grants")
	})

	t.Run("authenticated context should use its own roles", func(t *testing.T) {
		app := newApp(t)
		ctx, err := NewBotContext(app)
		require.NoError(t, err)

		ctx.SetAuthenticatedUserAndFillRoles(&aclTestUser{id: "42", roles: []string{"editor"}})

		roles := *ctx.GetAuthenticatedRoles()
		assert.Contains(t, roles, "editor")
		assert.Contains(t, roles, "authenticated")
		assert.NotContains(t, roles, "unAuthenticated")

		assert.True(t, ctx.Can("update_page"),
			"authenticated context should use the user role grants")
		assert.False(t, ctx.Can("find_public_page"),
			"authenticated context should not inherit unAuthenticated grants")
	})

	t.Run("authenticated user without roles should gain no permission", func(t *testing.T) {
		app := newApp(t)
		ctx, err := NewBotContext(app)
		require.NoError(t, err)

		ctx.SetAuthenticatedUserAndFillRoles(&aclTestUser{id: "42", roles: nil})

		assert.False(t, ctx.Can("update_page"),
			"user without roles should not gain permission even when other roles have it")
		assert.False(t, ctx.Can("find_public_page"),
			"user without roles should not gain unAuthenticated permissions")
	})
}

// ACL-07: Bootstrap must fail on malformed ACL JSON, missing mandatory
// sentinel role or unreadable acl.json; a missing file still boots with the
// valid defaults.
// Known failure: the errors are swallowed and the app boots silently today.
func TestApp_Bootstrap_ACL07(t *testing.T) {
	t.Run("control: should bootstrap with valid defaults when acl.json is absent", func(t *testing.T) {
		changeToTempWorkingDir(t)
		app := newBootableApp(t)

		// Precondition control: failures in the other subtests must come
		// from the ACL loading, not from DB/templates/events.
		err := app.Bootstrap()

		require.NoError(t, err)
		require.Contains(t, app.RolesList, "administrator")
		require.Contains(t, app.RolesList, "unAuthenticated")
	})

	t.Run("should return error when acl.json is malformed", func(t *testing.T) {
		dir := changeToTempWorkingDir(t)
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, "acl.json"),
			[]byte(`{"administrator": {`),
			0600,
		))

		app := newBootableApp(t)
		require.Contains(t, app.RolesString, "administrator",
			"fixture: malformed acl.json content should be the loaded RolesString")

		err := app.Bootstrap()

		assert.Error(t, err, "Bootstrap should fail on malformed ACL JSON instead of booting silently")
	})

	t.Run("should return error when mandatory system role is missing", func(t *testing.T) {
		dir := changeToTempWorkingDir(t)
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, "acl.json"),
			[]byte(`{"authenticated":{"name":"authenticated","permissions":[],"isSystemRole":true}}`),
			0600,
		))

		app := newBootableApp(t)

		err := app.Bootstrap()

		assert.Error(t, err, "Bootstrap should fail when the administrator sentinel role is missing")
	})

	t.Run("should return error when acl.json is unreadable", func(t *testing.T) {
		dir := changeToTempWorkingDir(t)
		// A directory named acl.json produces a read error without depending
		// on filesystem permissions (which vary with the running user).
		require.NoError(t, os.Mkdir(filepath.Join(dir, "acl.json"), 0755))

		app := newBootableApp(t)

		err := app.Bootstrap()

		assert.Error(t, err, "Bootstrap should fail when acl.json cannot be read instead of using defaults silently")
	})
}
