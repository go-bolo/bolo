package bolo

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-bolo/bolo/pagination"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VIEW-01
func TestRenderPager(t *testing.T) {
	t.Setenv("TEMPLATE_FOLDER", "testdata/themes")

	app := NewApp(&AppOptions{})
	require.NoError(t, app.SetTheme("site"))
	require.NoError(t, app.LoadTemplates())

	newCtx := func(t *testing.T) *RequestContext {
		req := httptest.NewRequest(http.MethodGet, "/items", nil)
		rec := httptest.NewRecorder()
		c := app.GetRouter().NewContext(req, rec)

		return NewRequestContext(&RequestContextOpts{App: app, EchoContext: c})
	}

	newPager := func() *pagination.Pager {
		pager := pagination.NewPager()
		pager.CurrentUrl = "/items"
		pager.Limit = 10
		return pager
	}

	t.Run("pager with total zero renders empty html", func(t *testing.T) {
		pager := newPager()
		pager.Count = 0
		pager.Page = 1

		html := renderPager(newCtx(t), pager, "")

		assert.Equal(t, template.HTML(""), html)
	})

	t.Run("pager on the last page renders valid links without absurd values", func(t *testing.T) {
		pager := newPager()
		pager.Count = 30
		pager.Page = 3

		html := renderPager(newCtx(t), pager, "")
		rendered := string(html)

		assert.Contains(t, rendered, `<a class="page active" href="/items?page=3">3</a>`)
		assert.Contains(t, rendered, `href="/items?page=1">1</a>`)
		assert.Contains(t, rendered, `href="/items?page=2">2</a>`)
		assert.Contains(t, rendered, `<a class="previous" href="/items?page=2">2</a>`)
		assert.NotContains(t, rendered, `class="next"`)
		assert.NotContains(t, rendered, "page=4")
	})

	t.Run("pager with limit zero renders empty html", func(t *testing.T) {
		pager := newPager()
		pager.Count = 10
		pager.Page = 1
		pager.Limit = 0

		html := renderPager(newCtx(t), pager, "")

		assert.Equal(t, template.HTML(""), html)
	})

	t.Run("pager with negative limit renders empty html", func(t *testing.T) {
		pager := newPager()
		pager.Count = 10
		pager.Page = 1
		pager.Limit = -1

		html := renderPager(newCtx(t), pager, "")

		assert.Equal(t, template.HTML(""), html)
	})
}
