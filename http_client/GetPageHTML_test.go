package http_client

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetPageHTML covers CLI-02: GetPageHTML with a fake CustomHTTPClient,
// no network involved.
func TestGetPageHTML(t *testing.T) {
	t.Run("CLI-02/200 returns the response body", func(t *testing.T) {
		body := newTraceableBody("<html>page</html>")
		fake := &recordingClient{resp: newFakeResponse(http.StatusOK, body)}
		withFakeHTTPClient(t, fake)

		html, err := GetPageHTML("http://example.test/page", nil)

		require.NoError(t, err)
		assert.Equal(t, "<html>page</html>", html)
	})

	t.Run("CLI-02/forwards headers to the transport", func(t *testing.T) {
		fake := &recordingClient{resp: newFakeResponse(http.StatusOK, newTraceableBody("ok"))}
		withFakeHTTPClient(t, fake)

		headers := http.Header{}
		headers.Set("Authorization", "Bearer token")
		headers.Set("Accept", "text/html")

		_, err := GetPageHTML("http://example.test/page", headers)

		require.NoError(t, err)
		req := fake.lastRequest()
		require.NotNil(t, req)
		assert.Equal(t, "Bearer token", req.Header.Get("Authorization"))
		assert.Equal(t, "text/html", req.Header.Get("Accept"))
	})

	t.Run("CLI-02/propagates body read errors and closes the body", func(t *testing.T) {
		body := &failingBody{}
		fake := &recordingClient{resp: newFakeResponse(http.StatusOK, body)}
		withFakeHTTPClient(t, fake)

		html, err := GetPageHTML("http://example.test/page", nil)

		require.Error(t, err)
		assert.ErrorIs(t, err, errFakeReadFailure)
		assert.Empty(t, html)
		assert.True(t, body.closed, "response body must be closed even on read failure")
	})

	t.Run("CLI-02/propagates transport errors", func(t *testing.T) {
		fake := &recordingClient{err: errFakeTransport}
		withFakeHTTPClient(t, fake)

		html, err := GetPageHTML("http://example.test/page", nil)

		require.Error(t, err)
		assert.ErrorIs(t, err, errFakeTransport)
		assert.Empty(t, html)
	})

	t.Run("CLI-02/closes the response body on success", func(t *testing.T) {
		body := newTraceableBody("ok")
		fake := &recordingClient{resp: newFakeResponse(http.StatusOK, body)}
		withFakeHTTPClient(t, fake)

		_, err := GetPageHTML("http://example.test/page", nil)

		require.NoError(t, err)
		assert.True(t, body.closed, "response body must be closed after a successful read")
	})

	t.Run("CLI-02/non-2xx response returns an error", func(t *testing.T) {
		// Expected contract: high level helpers must treat non-2xx as an
		// error instead of returning the error page body as valid content.
		// Known defect: today the 404 body is returned with a nil error.
		body := newTraceableBody("404 page body")
		fake := &recordingClient{resp: newFakeResponse(http.StatusNotFound, body)}
		withFakeHTTPClient(t, fake)

		html, err := GetPageHTML("http://example.test/missing", nil)

		assert.Error(t, err, "non-2xx responses must produce an error")
		assert.Empty(t, html, "non-2xx error body must not be returned as content")
	})
}
