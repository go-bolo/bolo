package http_client

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newDestFile opens a destination *os.File inside a temp dir. Note that
// DownloadFile closes the *os.File it receives, so callers must reopen the
// path to inspect its contents. extraFlags can add O_TRUNC etc.; by default
// the file is opened WITHOUT O_TRUNC so pre-existing markers stay observable.
func newDestFile(t *testing.T, name, initialContent string, extraFlags int) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|extraFlags, 0o600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	if initialContent != "" {
		_, err = f.WriteString(initialContent)
		require.NoError(t, err)
		_, err = f.Seek(0, io.SeekStart)
		require.NoError(t, err)
	}
	return f
}

// reopenAndRead reopens a closed file path and returns its full content.
func reopenAndRead(t *testing.T, f *os.File) string {
	t.Helper()
	content, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	return string(content)
}

// newReadOnlyDest opens an existing file read-only, so any write through it
// fails (used to exercise the write failure path of DownloadFile).
func newReadOnlyDest(t *testing.T, name, initialContent string) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	require.NoError(t, err)
	_, err = f.WriteString(initialContent)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	ro, err := os.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ro.Close() })
	return ro
}

// TestDownloadFile covers CLI-03 and CLI-04 against a fake CustomHTTPClient.
func TestDownloadFile(t *testing.T) {
	t.Run("CLI-03/200 writes the exact bytes and closes dest and body", func(t *testing.T) {
		dest := newDestFile(t, "dest.bin", "", os.O_TRUNC)
		body := newTraceableBody("downloaded-bytes-123")
		fake := &recordingClient{resp: newFakeResponse(http.StatusOK, body)}
		withFakeHTTPClient(t, fake)

		ok, err := DownloadFile("http://example.test/file.bin", dest, nil)

		require.NoError(t, err)
		assert.True(t, ok)
		assert.True(t, body.closed, "response body must be closed after download")
		assert.Error(t, dest.Close(), "DownloadFile must close the destination *os.File")
		assert.Equal(t, "downloaded-bytes-123", reopenAndRead(t, dest))
	})

	t.Run("CLI-03/forwards method URL and headers", func(t *testing.T) {
		dest := newDestFile(t, "dest.bin", "", os.O_TRUNC)
		fake := &recordingClient{resp: newFakeResponse(http.StatusOK, newTraceableBody("x"))}
		withFakeHTTPClient(t, fake)

		headers := http.Header{}
		headers.Set("X-Trace", "trace-1")

		_, err := DownloadFile("http://example.test/file.bin", dest, headers)

		require.NoError(t, err)
		req := fake.lastRequest()
		require.NotNil(t, req)
		assert.Equal(t, http.MethodGet, req.Method)
		assert.Equal(t, "http://example.test/file.bin", req.URL.String())
		assert.Equal(t, "trace-1", req.Header.Get("X-Trace"))
	})

	t.Run("CLI-03/transport failure returns false with error", func(t *testing.T) {
		dest := newDestFile(t, "dest.bin", "", os.O_TRUNC)
		fake := &recordingClient{err: errFakeTransport}
		withFakeHTTPClient(t, fake)

		ok, err := DownloadFile("http://example.test/file.bin", dest, nil)

		require.Error(t, err)
		assert.ErrorIs(t, err, errFakeTransport)
		assert.False(t, ok)
		assert.Error(t, dest.Close(), "dest must be closed even on transport failure")
		assert.Empty(t, reopenAndRead(t, dest))
	})

	t.Run("CLI-03/body read failure returns false with error", func(t *testing.T) {
		dest := newDestFile(t, "dest.bin", "", os.O_TRUNC)
		body := &failingBody{}
		fake := &recordingClient{resp: newFakeResponse(http.StatusOK, body)}
		withFakeHTTPClient(t, fake)

		ok, err := DownloadFile("http://example.test/file.bin", dest, nil)

		require.Error(t, err)
		assert.ErrorIs(t, err, errFakeReadFailure)
		assert.False(t, ok)
		assert.True(t, body.closed, "response body must be closed on read failure")
		assert.Error(t, dest.Close())
	})

	t.Run("CLI-03/destination write failure returns false with error", func(t *testing.T) {
		dest := newReadOnlyDest(t, "dest.bin", "original")
		body := newTraceableBody("some content")
		fake := &recordingClient{resp: newFakeResponse(http.StatusOK, body)}
		withFakeHTTPClient(t, fake)

		ok, err := DownloadFile("http://example.test/file.bin", dest, nil)

		require.Error(t, err)
		assert.False(t, ok)
		assert.True(t, body.closed, "response body must be closed on write failure")
		assert.Error(t, dest.Close())
	})

	t.Run("CLI-04/404 returns error without overwriting the pre-existing marker", func(t *testing.T) {
		marker := "PRE-EXISTING-MARKER-MUST-STAY-1234567890"
		dest := newDestFile(t, "dest.bin", marker, 0) // no O_TRUNC
		body := newTraceableBody("404 error page body")
		fake := &recordingClient{resp: newFakeResponse(http.StatusNotFound, body)}
		withFakeHTTPClient(t, fake)

		ok, err := DownloadFile("http://example.test/missing.bin", dest, nil)

		// Expected contract: non-2xx must fail the download and must not
		// write the error body into the destination.
		// Known defect: today the 404 body is copied over the file and
		// DownloadFile reports success.
		assert.False(t, ok, "non-2xx download must report failure")
		assert.Error(t, err, "non-2xx download must return an error")
		assert.True(t, body.closed, "response body must be closed even on non-2xx")
		assert.Error(t, dest.Close())
		assert.Equal(t, marker, reopenAndRead(t, dest), "pre-existing content must remain intact")
	})

	t.Run("CLI-04/500 returns error without overwriting the pre-existing marker", func(t *testing.T) {
		marker := "PRE-EXISTING-MARKER-MUST-STAY-1234567890"
		dest := newDestFile(t, "dest.bin", marker, 0) // no O_TRUNC
		body := newTraceableBody("500 internal error body")
		fake := &recordingClient{resp: newFakeResponse(http.StatusInternalServerError, body)}
		withFakeHTTPClient(t, fake)

		ok, err := DownloadFile("http://example.test/broken.bin", dest, nil)

		assert.False(t, ok, "non-2xx download must report failure")
		assert.Error(t, err, "non-2xx download must return an error")
		assert.True(t, body.closed, "response body must be closed even on non-2xx")
		assert.Error(t, dest.Close())
		assert.Equal(t, marker, reopenAndRead(t, dest), "pre-existing content must remain intact")
	})
}
