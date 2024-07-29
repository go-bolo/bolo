package bolo

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/go-bolo/bolo/helpers"
	"github.com/go-bolo/bolo/pagination"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

func noEscapeHTML(str string) template.HTML {
	return template.HTML(str)
}

func paginate(ctx *RequestContext, pager *pagination.Pager, queryString string) template.HTML {
	return renderPager(ctx, pager, queryString)
}

type ContentDates interface {
	GetTeaserDatesHTML(separator string) template.HTML
}

func contentDates(record ContentDates, separator string) template.HTML {
	return record.GetTeaserDatesHTML(separator)
}

func truncate(text string, length int, ellipsis string) template.HTML {
	html, err := helpers.Truncate(text, length, ellipsis)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"text":     text,
			"length":   length,
			"ellipsis": ellipsis,
		}).Error("truncate error on truncate text")
	}
	return html
}

func formatDecimalWithDots(value decimal.Decimal) string {
	return helpers.FormatDecimalWithDots(value)
}

func currentDate(format string) string {
	return helpers.FormatCurrencyDate(format)
}

type ResponseMessageTPLCtx struct {
	Ctx     *RequestContext
	Message *ResponseMessage
}

type ResponseMessagesTPLCtx struct {
	Ctx     *RequestContext
	Content string
}

func renderResponseMessages(ctx *RequestContext) template.HTML {
	html := ""
	itemsHTML := ""

	messages := ctx.GetResponseMessages()

	for _, msg := range messages {
		var contentBuffer bytes.Buffer
		err := ctx.RenderTemplate(&contentBuffer, "/components/response-message/response-message", &ResponseMessageTPLCtx{
			Ctx:     ctx,
			Message: msg,
		})
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error":    fmt.Sprintf("%+v\n", errors.Wrap(err, "bolo.theme.Render error on render template")),
				"template": "/components/response-message/response-message",
			}).Error("bolo.theme.renderResponseMessages error on render message")
			continue
		}

		itemsHTML += contentBuffer.String()
	}

	if itemsHTML != "" {
		var contentBuffer bytes.Buffer
		err := ctx.RenderTemplate(&contentBuffer, "/components/response-message/response-messages", &ResponseMessagesTPLCtx{
			Ctx:     ctx,
			Content: itemsHTML,
		})
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error":    fmt.Sprintf("%+v\n", errors.Wrap(err, "bolo.theme.Render error on render template")),
				"template": "/components/response-message/response-messages",
			}).Error("bolo.theme.renderResponseMessages error on render messages")
		}

		html = contentBuffer.String()
	}

	return template.HTML(html)
}
