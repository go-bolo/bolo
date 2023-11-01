package bolo_test

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	bolo "github.com/go-bolo/bolo"
	"github.com/gookit/event"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// Mocks:

// URLShortener is a plugin that shortens URLs.
type URLShortenerPlugin struct {
	Name       string
	App        bolo.App `json:"-"`
	Controller bolo.HTTPController
}

// Init initializes the plugin.
func (p *URLShortenerPlugin) Init(app bolo.App) error {
	p.Controller = &URLController{}

	app.SetModel("url", &URLModel{})

	app.GetEvents().On("bindRoutes", event.ListenerFunc(func(e event.Event) error {
		return p.BindRoutes(app)
	}), event.Normal)

	return nil
}

// GetName returns the name of the plugin.
func (p *URLShortenerPlugin) GetName() string {
	return p.Name
}

// SetName sets the name of the plugin.
func (p *URLShortenerPlugin) SetName(name string) error {
	p.Name = name
	return nil
}

func (p *URLShortenerPlugin) GetMigrations() []*bolo.Migration {
	return []*bolo.Migration{}
}

func (p *URLShortenerPlugin) BindRoutes(app bolo.App) error {
	ctl := p.Controller

	router := app.SetRouterGroup("urls", "/urls")
	router.GET("", ctl.Query)
	router.GET("/:id", ctl.FindOne)

	routerApi := app.SetRouterGroup("url-api", "/api/v1/urls")
	app.SetResource("url-api", ctl, routerApi)

	return nil
}

type JSONResponse struct {
	bolo.BaseListReponse
	URLs *[]*URLModel `json:"url"`
}

type CountJSONResponse struct {
	bolo.BaseMetaResponse
}

type FindOneJSONResponse struct {
	URL *URLModel `json:"url"`
}

type BodyRequest struct {
	URL *URLModel `json:"url"`
}

type URLController struct{}

func (ctl *URLController) Query(c echo.Context) error {
	data := struct {
		Name string `json:"name"`
	}{
		Name: "oi",
	}

	if c.QueryParam("errorToReturn") != "" {
		eCode := c.QueryParam("errorCode")
		eMessage := c.QueryParam("errorMessage")
		eCodeInt, _ := strconv.Atoi(eCode)
		if eCodeInt == 0 {
			eCodeInt = 500
		}

		return &bolo.HTTPError{
			Code:     eCodeInt,
			Message:  eMessage,
			Internal: errors.New(eMessage),
		}
	}

	return c.JSON(200, &data)
}

func (ctl *URLController) Create(c echo.Context) error {
	var err error
	ctx := c.(*bolo.RequestContext)

	can := ctx.Can("create_url")
	if !can {
		return &bolo.HTTPError{
			Code:    http.StatusForbidden,
			Message: "Forbidden",
		}
	}

	var body BodyRequest

	if err := c.Bind(&body); err != nil {
		if er, ok := err.(*echo.HTTPError); ok {

			return &bolo.HTTPError{
				Code:     er.Code,
				Message:  er.Message,
				Internal: er.Internal,
			}
		}

		return &bolo.HTTPError{
			Code:     http.StatusInternalServerError,
			Message:  "Invalid body data",
			Internal: fmt.Errorf("urls.Create error on parse body: %w", err),
		}
	}

	record := body.URL
	record.ID = 0

	if ctx.IsAuthenticated {
		creatorID := ctx.AuthenticatedUser.GetID()
		record.CreatorID = &creatorID
	}

	if err := c.Validate(record); err != nil {
		if _, ok := err.(*echo.HTTPError); ok {
			return err
		}
		return &bolo.HTTPError{
			Code:     http.StatusInternalServerError,
			Message:  "Error on validate data",
			Internal: err,
		}
	}

	err = record.Save(ctx)
	if err != nil {
		return err
	}

	resp := FindOneJSONResponse{
		URL: record,
	}

	return c.JSON(http.StatusCreated, &resp)
}

func (ctl *URLController) FindOne(c echo.Context) error {
	ctx := c.(*bolo.RequestContext)
	id := ctx.Param("id")

	record, err := FindOneURL(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &bolo.HTTPError{
				Code:     http.StatusNotFound,
				Message:  "Not found",
				Internal: err,
			}
		}

		return &bolo.HTTPError{
			Code:     http.StatusInternalServerError,
			Message:  "Error on FindOneURL",
			Internal: err,
		}
	}

	return c.JSON(http.StatusOK, &record)
}

func (ctl *URLController) Count(c echo.Context) error {
	r := bolo.DefaultResponse{
		Data: CountJSONResponse{
			BaseMetaResponse: bolo.BaseMetaResponse{
				Count: 90,
			},
		},
	}

	return c.JSON(http.StatusOK, &r.Data)
}

func (ctl *URLController) Update(c echo.Context) error {
	r := bolo.DefaultResponse{
		Data: FindOneJSONResponse{
			URL: &URLModel{
				ID: 13,
			},
		},
	}

	return c.JSON(http.StatusOK, &r)
}

func (ctl *URLController) Delete(c echo.Context) error {
	return c.JSON(http.StatusNoContent, struct{}{})
}
