package bolo

import (
	"bytes"
	"regexp"

	"github.com/labstack/echo/v4"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
	"github.com/tdewolff/minify/v2/json"
	"github.com/tdewolff/minify/v2/svg"
)

var m *minify.M

func init() {
	m = minify.New()
	m.AddFunc("text/html", html.Minify)
	m.AddFunc("text/css", css.Minify)
	m.AddFunc("text/html", html.Minify)
	m.AddFunc("image/svg+xml", svg.Minify)
	m.AddFuncRegexp(regexp.MustCompile("^(application|text)/(x-)?(java|ecma)script$"), js.Minify)
	m.AddFuncRegexp(regexp.MustCompile("[/+]json$"), json.Minify)
}

func MinifiHTML(templateName string, data interface{}, c *RequestContext) (string, error) {
	html := new(bytes.Buffer)
	err := c.App.RenderTemplate(html, templateName, data)
	if err != nil {
		return "", err
	}

	buf2 := new(bytes.Buffer)
	if err := m.Minify("text/html", buf2, html); err != nil {
		return "", err
	}

	return buf2.String(), nil
}

func MinifiAndRender(code int, name string, data interface{}, c *RequestContext) error {
	var err error

	if c.Echo().Renderer == nil {
		return echo.ErrRendererNotRegistered
	}
	buf := new(bytes.Buffer)
	if err = c.Echo().Renderer.Render(buf, name, data, c); err != nil {
		return nil
	}

	buf2 := new(bytes.Buffer)
	if err := m.Minify("text/html", buf2, buf); err != nil {
		panic(err)
	}

	return c.HTMLBlob(code, buf2.Bytes())
}
