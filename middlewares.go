package bolo

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"
)

// BindMiddlewares - Bind middlewares in order
func BindMiddlewares(app App, p *Plugin) {
	logrus.Debug("bolo.BindMiddlewares " + p.GetName())

	goEnv := app.GetConfiguration().Get("GO_ENV")

	router := app.GetRouter()
	router.Pre(middleware.RemoveTrailingSlashWithConfig(middleware.TrailingSlashConfig{
		RedirectCode: http.StatusMovedPermanently,
	}))

	router.Use(middleware.Gzip())
	router.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowCredentials: app.GetConfiguration().GetBoolF("CORS_ALLOW_CREDENTIALS", true),
		MaxAge:           app.GetConfiguration().GetIntF("CORS_MAX_AGE", 18000), // seccounds
		AllowOriginFunc: func(origin string) (bool, error) {
			return true, nil
		},
	}))

	router.Use(acceptResolverMiddleware(app))

	// Access-Control-Allow-Credentials

	router.Use(initAppCtx(app))

	if goEnv == "development" {
		router.Debug = true
	}
}

func isPublicRoute(url string) bool {
	return strings.HasPrefix(url, "/health") || strings.HasPrefix(url, "/public")
}

// Middleware that update echo context to use custom methods
func initAppCtx(app App) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := NewRequestContext(&RequestContextOpts{App: app, EchoContext: c})
			return next(ctx)
		}
	}
}

func acceptResolverMiddleware(app App) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			acceptType := NegotiateContentType(c.Request(), app.GetContentTypes(), app.GetDefaultContentType())
			c.Set("responseContentType", acceptType)

			return next(c)
		}
	}
}
