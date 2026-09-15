package bolo_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	bolo "github.com/go-bolo/bolo"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CTX-01
func TestRequestContext_AddBodyClass_RemoveBodyClass(t *testing.T) {
	app := bolo.NewApp(&bolo.AppOptions{})

	newCtx := func(t *testing.T) *bolo.RequestContext {
		ctx, err := bolo.NewBotContext(app)
		require.NoError(t, err)
		return ctx
	}

	t.Run("add followed by remove should drop the target and preserve the others", func(t *testing.T) {
		ctx := newCtx(t)

		ctx.AddBodyClass("login-page")
		ctx.AddBodyClass("has-sidebar")
		ctx.AddBodyClass("dark-mode")

		ctx.RemoveBodyClass("has-sidebar")

		assert.Equal(t, []string{"login-page", "dark-mode"}, ctx.BodyClass)
		assert.Equal(t, "login-page dark-mode", ctx.GetBodyClassText())
	})

	t.Run("remove with absent target should not add anything", func(t *testing.T) {
		ctx := newCtx(t)

		ctx.AddBodyClass("login-page")
		ctx.RemoveBodyClass("ghost-class")

		assert.Equal(t, []string{"login-page"}, ctx.BodyClass)
		assert.NotContains(t, ctx.GetBodyClassText(), "ghost-class")
	})

	t.Run("add should keep classes unique", func(t *testing.T) {
		ctx := newCtx(t)

		ctx.AddBodyClass("login-page")
		ctx.AddBodyClass("login-page")

		assert.Equal(t, []string{"login-page"}, ctx.BodyClass)
	})
}

// CTX-02
func TestRequestContext_Constructors(t *testing.T) {
	t.Setenv("PAGER_LIMIT", "20")
	t.Setenv("PAGER_LIMIT_MAX", "50")

	app := bolo.NewApp(&bolo.AppOptions{})

	newEchoContext := func(t *testing.T, url string) echo.Context {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		rec := httptest.NewRecorder()
		return app.GetRouter().NewContext(req, rec)
	}

	limitCases := []struct {
		name         string
		url          string
		expectedType string
	}{
		{name: "without limit param uses the default", url: "/", expectedType: "default"},
		{name: "limit at the maximum is accepted", url: "/?limit=50", expectedType: "max"},
		{name: "limit above the maximum falls back to the default", url: "/?limit=51", expectedType: "default"},
		{name: "limit zero falls back to the default", url: "/?limit=0", expectedType: "default"},
		{name: "negative limit falls back to the default", url: "/?limit=-5", expectedType: "default"},
		{name: "invalid limit falls back to the default", url: "/?limit=abc", expectedType: "default"},
	}

	const defaultLimit int64 = 20

	for _, tc := range limitCases {
		t.Run(tc.name, func(t *testing.T) {
			c := newEchoContext(t, tc.url)

			ctxPkg := bolo.NewRequestContext(&bolo.RequestContextOpts{App: app, EchoContext: c})
			ctxApp := app.NewRequestContext(&bolo.RequestContextOpts{App: app, EchoContext: c})

			expectedLimit := defaultLimit
			if tc.expectedType == "max" {
				expectedLimit = 50
			}

			assert.Equal(t, expectedLimit, ctxPkg.Pager.Limit,
				"NewRequestContext should honor the limit contract")
			assert.Equal(t, expectedLimit, ctxApp.Pager.Limit,
				"app.NewRequestContext should honor the same limit contract")
			assert.Equal(t, ctxPkg.Pager.Limit, ctxApp.Pager.Limit,
				"both constructors should produce the same result for equivalent inputs")
		})
	}

	t.Run("preserves the Theme and Layout from the App", func(t *testing.T) {
		require.NoError(t, app.SetTheme("custom-theme"))
		require.NoError(t, app.SetLayout("layouts/custom"))

		c := newEchoContext(t, "/")

		ctxPkg := bolo.NewRequestContext(&bolo.RequestContextOpts{App: app, EchoContext: c})
		ctxApp := app.NewRequestContext(&bolo.RequestContextOpts{App: app, EchoContext: c})

		assert.Equal(t, "custom-theme", ctxPkg.Theme)
		assert.Equal(t, "layouts/custom", ctxPkg.Layout)
		assert.Equal(t, "custom-theme", ctxApp.Theme,
			"app.NewRequestContext should preserve the app theme")
		assert.Equal(t, "layouts/custom", ctxApp.Layout,
			"app.NewRequestContext should preserve the app layout")
	})
}
