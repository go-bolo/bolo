package bolo

import (
	"fmt"

	"github.com/labstack/echo/v4"
)

type BaseListReponse struct {
	Meta BaseMetaResponse `json:"meta"`
}

type BaseMetaResponse struct {
	Count int64 `json:"count"`
}

type BaseErrorResponse struct {
	Messages []BaseErrorResponseMessage `json:"messages"`
}

type BaseErrorResponseMessage struct {
	Status  string `json:"status"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type EmptyResponse struct{}

func ParseHTTPErrorToResponse(ctx *RequestContext, he HTTPErrorInterface) *BaseErrorResponse {
	respData := BaseErrorResponse{
		Messages: []BaseErrorResponseMessage{
			{
				Status:  ParseHTTPCodeToStatus(he.GetCode()),
				Code:    he.GetCode(),
				Message: fmt.Sprintf("%v", he.GetMessage()),
			},
		},
	}

	return &respData
}

func ParseEchoHTTPErrorToResponse(ctx *RequestContext, he *echo.HTTPError) *BaseErrorResponse {
	respData := BaseErrorResponse{
		Messages: []BaseErrorResponseMessage{
			{
				Status:  ParseHTTPCodeToStatus(he.Code),
				Code:    he.Code,
				Message: he.Error(),
			},
		},
	}

	return &respData
}

func ParseHTTPCodeToStatus(status int) string {
	if status < 300 {
		return "success"
	}

	if status >= 300 && status < 400 {
		return "success"
	}

	if status >= 400 && status < 500 {
		return "warning"
	}

	return "danger"
}
