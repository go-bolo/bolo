package http_client

import (
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
)

func GetPageHTML(url string, headers http.Header) (string, error) {
	resp, err := Get(url, headers)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"url":     url,
			"headers": headers,
			"error":   err,
		}).Error("GetPageHTML error")
		return "", err
	}

	defer resp.Body.Close()

	rdrBody := io.Reader(resp.Body)
	bodyBytes, err := io.ReadAll(rdrBody) // Replace ioutil.ReadAll with io.ReadAll
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err": fmt.Sprintf("%+v\n", err),
		}).Debug("bolo.GetPageHTML error")
		return "", fmt.Errorf("GetPageHTML error: %w", err)
	}

	return string(bodyBytes), nil
}
