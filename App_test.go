package bolo_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	approvals "github.com/approvals/go-approval-tests"
	"github.com/go-bolo/bolo"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

var testAppInstance bolo.App

func GetTestAppInstance() bolo.App {
	if testAppInstance != nil {
		return testAppInstance
	}

	app := bolo.Init(&bolo.AppOptions{})

	err := app.Bootstrap()
	if err != nil {
		panic(err)
	}

	testAppInstance = app

	return app
}

func TestNewApp(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "should return a valid default app with required data",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bolo.NewApp(&bolo.AppOptions{})

			approvals.VerifyJSONStruct(t, got)
		})
	}
}

func TestApp_Bootstrap(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "should return a valid default app with required data",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bolo.NewApp(&bolo.AppOptions{})
			err := got.Bootstrap()
			assert.Nil(t, err)

			approvals.VerifyJSONStruct(t, got)
		})
	}
}

func TestRequest_CRUD(t *testing.T) {
	app := GetTestApp()
	app.RegisterPlugin(&URLShortenerPlugin{Name: "example"})
	err := app.Bootstrap()
	assert.Nil(t, err)
	err = app.GetDB().AutoMigrate(
		&URLModel{},
	)

	app.SetRolePermission("unAuthenticated", "create_url", true)

	assert.Nil(t, err)

	c, _ := bolo.NewBotContext(app)

	savedRecord1 := URLModel{
		Title: "Google",
		Path:  "http://www.google.com",
	}
	savedRecord1.Save(c)

	savedRecord2 := URLModel{
		Title: "Bing",
		Path:  "http://www.bing.com",
	}
	savedRecord2.Save(c)

	assert.Nil(t, err)

	type fields struct {
		Plugins map[string]bolo.Plugin
	}
	type args struct {
		accept      string
		queryParams string
		data        io.Reader
		url         string
		method      string
	}
	tests := []struct {
		name           string
		fields         fields
		args           args
		wantHas        bool
		expectedStatus int
		expectedError  *bolo.HTTPError
	}{
		{
			name: "should run a action with success",
			args: args{
				method: http.MethodGet,
				url:    "/urls",
				accept: "application/json",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "JSON findOne should return 404 with invalid id",
			args: args{
				method: http.MethodGet,
				url:    "/urls/1111111111",
				accept: "application/json",
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "JSON create should create a new record",
			args: args{
				method: http.MethodPost,
				url:    "/api/v1/urls",
				accept: "application/json",
				data:   strings.NewReader(`{"url":{"title":"example","path":"http://www.example.com"}}`),
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "JSON get count",
			args: args{
				method: http.MethodGet,
				url:    "/api/v1/urls/count",
				accept: "application/json",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "JSON should run update action",
			args: args{
				method: http.MethodPost,
				url:    "/api/v1/urls/1",
				accept: "application/json",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "JSON should run delete action",
			args: args{
				method: http.MethodDelete,
				url:    "/api/v1/urls/1",
				accept: "application/json",
			},
			expectedStatus: http.StatusNoContent,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := app.GetRouter()

			req := httptest.NewRequest(tt.args.method, tt.args.url, tt.args.data)
			req.Header.Set(echo.HeaderAccept, tt.args.accept)
			// Body content type:
			req.Header.Set(echo.HeaderContentType, "application/json")

			rec := httptest.NewRecorder() // run the request:
			e.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			switch tt.args.accept {
			case "application/json":
				approvals.VerifyJSONBytes(t, rec.Body.Bytes())
			}
		})
	}
}
