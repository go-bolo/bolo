package bolo

// Middleware contract tests for the pipeline registered by BindMiddlewares
// (matrix IDs HTTP-01..HTTP-05). Fixtures use only NewApp + BindMiddlewares,
// never corrective CORS/Recover middlewares. Set env with t.Setenv BEFORE
// creating the app: BindMiddlewares reads the configuration at bind time.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unsetenvWithRestore unsets an env var and restores the previous state on
// cleanup: an empty value is not equivalent to an absent one.
func unsetenvWithRestore(t *testing.T, key string) {
	t.Helper()

	if oldValue, ok := os.LookupEnv(key); ok {
		t.Cleanup(func() {
			os.Setenv(key, oldValue)
		})
	} else {
		t.Cleanup(func() {
			os.Unsetenv(key)
		})
	}

	require.NoError(t, os.Unsetenv(key))
}

// newMiddlewareTestApp builds a fresh App with the middleware chain bound by
// BindMiddlewares.
func newMiddlewareTestApp(t *testing.T) *AppStruct {
	t.Helper()

	app := NewApp(&AppOptions{}).(*AppStruct)
	BindMiddlewares(app, &Plugin{Name: "bolo"})

	return app
}

// bindMiddlewareCheckRoute registers a route answering with the content type
// negotiated by the middleware pipeline.
func bindMiddlewareCheckRoute(app *AppStruct) {
	app.GetRouter().GET("/middleware-check", func(c echo.Context) error {
		ctx := c.(*RequestContext)
		return c.JSON(http.StatusOK, map[string]string{
			"contentType": ctx.GetResponseContentType(),
		})
	})
}

func doMiddlewareRequest(t *testing.T, router *echo.Echo, method, url, origin string, extraHeaders map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, url, nil)
	if origin != "" {
		req.Header.Set(echo.HeaderOrigin, origin)
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec
}

// doMiddlewarePreflight runs a standard CORS preflight against the check
// route.
func doMiddlewarePreflight(t *testing.T, router *echo.Echo, origin, requestMethod string) *httptest.ResponseRecorder {
	return doMiddlewareRequest(t, router, http.MethodOptions, "/middleware-check", origin, map[string]string{
		echo.HeaderAccessControlRequestMethod: requestMethod,
	})
}

// HTTP-01: preflight and real GET from an untrusted Origin must not receive
// a permissive Access-Control-Allow-Origin.
// Known failure: any Origin is reflected today. The allowlist below excludes
// the untrusted origin, so a secure implementation passes this test.
func TestBindMiddlewares_CORS_OriginValidation_HTTP01(t *testing.T) {
	t.Setenv("CORS_ALLOW_ORIGINS", "https://app.example")

	app := newMiddlewareTestApp(t)
	bindMiddlewareCheckRoute(app)
	router := app.GetRouter()

	t.Run("preflight with untrusted origin should not release the origin", func(t *testing.T) {
		rec := doMiddlewarePreflight(t, router, "https://evil.example", http.MethodPost)

		acao := rec.Header().Get(echo.HeaderAccessControlAllowOrigin)
		assert.NotEqual(t, "https://evil.example", acao,
			"preflight must not reflect an untrusted Origin")
		assert.NotEqual(t, "*", acao,
			"preflight must not answer with wildcard Access-Control-Allow-Origin")
	})

	t.Run("real GET with untrusted origin should not release the origin", func(t *testing.T) {
		rec := doMiddlewareRequest(t, router, http.MethodGet, "/middleware-check", "https://evil.example", nil)

		assert.Equal(t, http.StatusOK, rec.Code, "request should still be served")

		acao := rec.Header().Get(echo.HeaderAccessControlAllowOrigin)
		assert.NotEqual(t, "https://evil.example", acao,
			"real GET must not reflect an untrusted Origin")
		assert.NotEqual(t, "*", acao,
			"real GET must not answer with wildcard Access-Control-Allow-Origin")
	})
}

// HTTP-02: credentials must be absent by default and when configured to
// false; true only when explicitly enabled for an allowed origin.
// Known failure: the production default for CORS_ALLOW_CREDENTIALS is true.
func TestBindMiddlewares_CORS_AllowCredentials_HTTP02(t *testing.T) {
	// app.example is allowlisted, so a secure implementation grants the
	// origin and the credentials header follows only the explicit opt-in:
	t.Setenv("CORS_ALLOW_ORIGINS", "https://app.example")

	t.Run("credentials should be absent by default", func(t *testing.T) {
		unsetenvWithRestore(t, "CORS_ALLOW_CREDENTIALS")

		app := newMiddlewareTestApp(t)
		bindMiddlewareCheckRoute(app)

		rec := doMiddlewarePreflight(t, app.GetRouter(), "https://app.example", http.MethodGet)
		require.Equal(t, http.StatusNoContent, rec.Code, "preflight should answer 204")

		assert.Empty(t, rec.Header().Get(echo.HeaderAccessControlAllowCredentials),
			"Access-Control-Allow-Credentials must not be sent by default")
	})

	t.Run("credentials should be absent when configured to false", func(t *testing.T) {
		t.Setenv("CORS_ALLOW_CREDENTIALS", "false")

		app := newMiddlewareTestApp(t)
		bindMiddlewareCheckRoute(app)

		rec := doMiddlewarePreflight(t, app.GetRouter(), "https://app.example", http.MethodGet)
		require.Equal(t, http.StatusNoContent, rec.Code, "preflight should answer 204")

		assert.Empty(t, rec.Header().Get(echo.HeaderAccessControlAllowCredentials),
			"Access-Control-Allow-Credentials must not be sent with CORS_ALLOW_CREDENTIALS=false")
	})

	t.Run("credentials true only when explicitly enabled", func(t *testing.T) {
		t.Setenv("CORS_ALLOW_CREDENTIALS", "true")

		app := newMiddlewareTestApp(t)
		bindMiddlewareCheckRoute(app)

		rec := doMiddlewarePreflight(t, app.GetRouter(), "https://app.example", http.MethodGet)
		require.Equal(t, http.StatusNoContent, rec.Code, "preflight should answer 204")

		assert.Equal(t, "true", rec.Header().Get(echo.HeaderAccessControlAllowCredentials),
			"Access-Control-Allow-Credentials=true requires explicit opt-in")
	})
}

// HTTP-03: CORS_ALLOW_ORIGINS allowlist contract: exact allowed origin,
// reject unlisted and lookalike origins, empty list must not become a
// wildcard, spaces around entries trimmed.
// Known failure: CORS_ALLOW_ORIGINS is not read by production yet, so every
// Origin is reflected.
func TestBindMiddlewares_CORS_OriginAllowlist_HTTP03(t *testing.T) {
	allowedList := "https://good.example, https://api.good.example"

	t.Run("exact allowed origin should pass", func(t *testing.T) {
		t.Setenv("CORS_ALLOW_ORIGINS", allowedList)

		app := newMiddlewareTestApp(t)
		bindMiddlewareCheckRoute(app)

		rec := doMiddlewarePreflight(t, app.GetRouter(), "https://good.example", http.MethodGet)

		assert.Equal(t, "https://good.example", rec.Header().Get(echo.HeaderAccessControlAllowOrigin),
			"exact allowlisted origin should be allowed")
	})

	t.Run("unlisted origin should be rejected", func(t *testing.T) {
		t.Setenv("CORS_ALLOW_ORIGINS", allowedList)

		app := newMiddlewareTestApp(t)
		bindMiddlewareCheckRoute(app)

		rec := doMiddlewarePreflight(t, app.GetRouter(), "https://evil.example", http.MethodGet)

		assert.Empty(t, rec.Header().Get(echo.HeaderAccessControlAllowOrigin),
			"unlisted origin must not receive Access-Control-Allow-Origin")
	})

	t.Run("lookalike origin should be rejected", func(t *testing.T) {
		t.Setenv("CORS_ALLOW_ORIGINS", allowedList)

		app := newMiddlewareTestApp(t)
		bindMiddlewareCheckRoute(app)

		rec := doMiddlewarePreflight(t, app.GetRouter(), "https://good.example.evil.example", http.MethodGet)

		assert.Empty(t, rec.Header().Get(echo.HeaderAccessControlAllowOrigin),
			"origin that only contains an allowlisted entry must be rejected")
	})

	t.Run("empty allowlist should not become a wildcard", func(t *testing.T) {
		t.Setenv("CORS_ALLOW_ORIGINS", "")

		app := newMiddlewareTestApp(t)
		bindMiddlewareCheckRoute(app)

		rec := doMiddlewarePreflight(t, app.GetRouter(), "https://any.example", http.MethodGet)

		assert.Empty(t, rec.Header().Get(echo.HeaderAccessControlAllowOrigin),
			"empty allowlist must fail closed: no wildcard and no reflection")
	})

	t.Run("spaces around allowlist entries should be handled", func(t *testing.T) {
		t.Setenv("CORS_ALLOW_ORIGINS", allowedList)

		app := newMiddlewareTestApp(t)
		bindMiddlewareCheckRoute(app)

		rec := doMiddlewarePreflight(t, app.GetRouter(), "https://api.good.example", http.MethodGet)

		assert.Equal(t, "https://api.good.example", rec.Header().Get(echo.HeaderAccessControlAllowOrigin),
			"entry listed with surrounding spaces should be trimmed and allowed")
	})
}

// HTTP-04: requests without Origin keep working (no CORS headers) and
// content negotiation handles application/json, text/html, missing Accept
// and q-values.
func TestBindMiddlewares_NoOriginAndContentNegotiation_HTTP04(t *testing.T) {
	app := newMiddlewareTestApp(t)
	bindMiddlewareCheckRoute(app)
	router := app.GetRouter()

	getResponseContentType := func(t *testing.T, rec *httptest.ResponseRecorder) string {
		t.Helper()
		require.Equal(t, http.StatusOK, rec.Code)

		body := map[string]string{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body),
			"response should be valid JSON: %s", rec.Body.String())

		return body["contentType"]
	}

	t.Run("request without Origin keeps working and sends no CORS header", func(t *testing.T) {
		rec := doMiddlewareRequest(t, router, http.MethodGet, "/middleware-check", "", nil)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, rec.Header().Get(echo.HeaderAccessControlAllowOrigin),
			"request without Origin must not receive CORS headers")
	})

	t.Run("negotiate application/json", func(t *testing.T) {
		rec := doMiddlewareRequest(t, router, http.MethodGet, "/middleware-check", "", map[string]string{
			echo.HeaderAccept: "application/json",
		})

		assert.Equal(t, "application/json", getResponseContentType(t, rec))
	})

	t.Run("negotiate text/html", func(t *testing.T) {
		rec := doMiddlewareRequest(t, router, http.MethodGet, "/middleware-check", "", map[string]string{
			echo.HeaderAccept: "text/html",
		})

		assert.Equal(t, "text/html", getResponseContentType(t, rec))
	})

	t.Run("missing Accept uses the default content type", func(t *testing.T) {
		rec := doMiddlewareRequest(t, router, http.MethodGet, "/middleware-check", "", nil)

		assert.Equal(t, "application/json", getResponseContentType(t, rec))
	})

	t.Run("q-values select the best offer", func(t *testing.T) {
		rec := doMiddlewareRequest(t, router, http.MethodGet, "/middleware-check", "", map[string]string{
			echo.HeaderAccept: "text/html;q=0.4, application/json;q=0.9",
		})
		assert.Equal(t, "application/json", getResponseContentType(t, rec))

		rec = doMiddlewareRequest(t, router, http.MethodGet, "/middleware-check", "", map[string]string{
			echo.HeaderAccept: "text/html;q=0.9, application/json;q=0.4",
		})
		assert.Equal(t, "text/html", getResponseContentType(t, rec))
	})
}

// HTTP-05: a handler panic must be converted into a 500 by the recovery
// pipeline and the next request must keep working.
// Known failure: no Recover middleware is registered, so the panic escapes.
// It is captured with assert.NotPanics to record the failure without
// aborting the test binary.
func TestBindMiddlewares_PanicRecovery_HTTP05(t *testing.T) {
	app := newMiddlewareTestApp(t)
	bindMiddlewareCheckRoute(app)
	app.GetRouter().GET("/middleware-panic", func(c echo.Context) error {
		panic("HTTP-05 boom")
	})
	router := app.GetRouter()

	t.Run("panic should be recovered as a 500 response", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/middleware-panic", nil)

		assert.NotPanics(t, func() {
			router.ServeHTTP(rec, req)
		}, "pipeline should recover handler panics")

		assert.Equal(t, http.StatusInternalServerError, rec.Code,
			"recovered panic should produce a 500 response")
	})

	t.Run("next request should still work", func(t *testing.T) {
		rec := doMiddlewareRequest(t, router, http.MethodGet, "/middleware-check", "", nil)

		assert.Equal(t, http.StatusOK, rec.Code, "router should keep serving requests")
	})
}
