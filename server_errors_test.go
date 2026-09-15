package bolo

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sensitiveMarker = "INTERNAL_SECRET_MARKER_DQ81"

type sensitiveTestError struct {
	Details string
	Token   string
}

func (e *sensitiveTestError) Error() string {
	return "operation failed: " + e.Details
}

// ERR-01
func TestInternalServerErrorHandlerDoesNotExposeInternalError(t *testing.T) {
	app := NewApp(&AppOptions{})

	newJSONContext := func(t *testing.T) (*RequestContext, *httptest.ResponseRecorder) {
		req := httptest.NewRequest(http.MethodGet, "/broken", nil)
		req.Header.Set(echo.HeaderAccept, "application/json")
		rec := httptest.NewRecorder()
		c := app.GetRouter().NewContext(req, rec)
		// precondition: consuming apps middleware stores the app on the echo context
		c.Set("app", app)

		ctx := NewRequestContext(&RequestContextOpts{App: app, EchoContext: c})
		require.Equal(t, "application/json", ctx.GetResponseContentType())

		return ctx, rec
	}

	cases := []struct {
		name string
		err  error
	}{
		{
			name: "plain internal error",
			err:  errors.New("db connection refused " + sensitiveMarker),
		},
		{
			name: "wrapped internal error",
			err:  fmt.Errorf("user query failed: %w", errors.New("password="+sensitiveMarker)),
		},
		{
			name: "error with exported fields carrying sensitive data",
			err: &sensitiveTestError{
				Details: "dump contains " + sensitiveMarker,
				Token:   sensitiveMarker,
			},
		},
		{
			name: "HTTPError with internal error carrying sensitive data",
			err: &HTTPError{
				Code:     http.StatusInternalServerError,
				Message:  "Internal Server Error",
				Internal: errors.New(sensitiveMarker),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, rec := newJSONContext(t)

			internalServerErrorHandler(tc.err, ctx)

			assert.Equal(t, http.StatusInternalServerError, rec.Code)
			assert.NotContains(t, rec.Body.String(), sensitiveMarker,
				"internal error details should never leak into the response body")
		})
	}
}

// ERR-02
func TestValidationErrorTitleDefaults(t *testing.T) {
	app := NewApp(&AppOptions{})

	buildValidationErrors := func(t *testing.T) validator.ValidationErrors {
		type payload struct {
			Email string `validate:"required,email"`
		}

		v := validator.New()
		err := v.Struct(payload{Email: "not-an-email"})
		require.Error(t, err)

		ve, ok := err.(validator.ValidationErrors)
		require.True(t, ok)

		return ve
	}

	newHTMLContext := func(t *testing.T) *RequestContext {
		req := httptest.NewRequest(http.MethodGet, "/broken", nil)
		req.Header.Set(echo.HeaderAccept, "text/html")
		rec := httptest.NewRecorder()
		c := app.GetRouter().NewContext(req, rec)
		// precondition: consuming apps middleware stores the app on the echo context
		c.Set("app", app)

		ctx := NewRequestContext(&RequestContextOpts{App: app, EchoContext: c})
		require.Equal(t, "text/html", ctx.GetResponseContentType())

		return ctx
	}

	ve := buildValidationErrors(t)

	t.Run("empty title gains the default title", func(t *testing.T) {
		ctx := newHTMLContext(t)
		require.Empty(t, ctx.Title)

		validationError(ve, ve, ctx)

		assert.Equal(t, "Bad request", ctx.Title)
	})

	t.Run("informed title is preserved", func(t *testing.T) {
		ctx := newHTMLContext(t)
		ctx.Title = "Titulo customizado"

		validationError(ve, ve, ctx)

		assert.Equal(t, "Titulo customizado", ctx.Title)
	})
}

// ERR-02
func TestTemplateRendererRenderPropagatesErrorWithoutWritingResponse(t *testing.T) {
	t.Setenv("TEMPLATE_FOLDER", "testdata/themes")

	app := NewApp(&AppOptions{})
	require.NoError(t, app.SetTheme("site"))
	// precondition: app templates must be loaded and usable before rendering
	require.NoError(t, app.LoadTemplates())

	req := httptest.NewRequest(http.MethodGet, "/broken", nil)
	rec := httptest.NewRecorder()
	c := app.GetRouter().NewContext(req, rec)

	ctx := NewRequestContext(&RequestContextOpts{App: app, EchoContext: c})

	renderer := &TemplateRenderer{templates: app.GetTemplates()}

	err := renderer.Render(io.Discard, "missing-template", &TemplateCTX{Ctx: ctx}, c)

	assert.Error(t, err,
		"renderer should return the render error to the caller instead of swallowing it")
	assert.Equal(t, http.StatusOK, rec.Code,
		"renderer should not write the http response by itself")
	assert.Equal(t, 0, rec.Body.Len(),
		"renderer should not write a body by itself")
}
