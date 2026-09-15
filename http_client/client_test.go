package http_client

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errFakeTransport simulates a transport level failure, without any network.
var errFakeTransport = errors.New("fake transport failure")

// errFakeReadFailure simulates a response body read failure.
var errFakeReadFailure = errors.New("fake read failure")

// recordingClient is a fake CustomHTTPClient. It never touches the network:
// it records every request it receives (including a traceable copy of the
// request body) and replays a configured response/error.
type recordingClient struct {
	resp     *http.Response
	err      error
	doCalls  int
	requests []*http.Request
	bodies   [][]byte
}

func (c *recordingClient) Do(req *http.Request) (*http.Response, error) {
	c.doCalls++
	c.requests = append(c.requests, req)

	if req.Body != nil {
		bodyBytes, readErr := io.ReadAll(req.Body)
		_ = req.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		c.bodies = append(c.bodies, bodyBytes)
		// Restore the body so the recorded request stays reusable/readable.
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	} else {
		c.bodies = append(c.bodies, nil)
	}

	return c.resp, c.err
}

func (c *recordingClient) lastRequest() *http.Request {
	if len(c.requests) == 0 {
		return nil
	}
	return c.requests[len(c.requests)-1]
}

func (c *recordingClient) lastBody() []byte {
	if len(c.bodies) == 0 {
		return nil
	}
	return c.bodies[len(c.bodies)-1]
}

// traceableBody is a response body that records whether it was closed.
type traceableBody struct {
	reader *bytes.Reader
	closed bool
}

func newTraceableBody(content string) *traceableBody {
	return &traceableBody{reader: bytes.NewReader([]byte(content))}
}

func (b *traceableBody) Read(p []byte) (int, error) {
	return b.reader.Read(p)
}

func (b *traceableBody) Close() error {
	b.closed = true
	return nil
}

// failingBody always fails to read and records whether it was closed.
type failingBody struct {
	closed bool
}

func (b *failingBody) Read(p []byte) (int, error) {
	return 0, errFakeReadFailure
}

func (b *failingBody) Close() error {
	b.closed = true
	return nil
}

// newFakeResponse builds a traceable *http.Response for the fake client.
func newFakeResponse(status int, body io.ReadCloser) *http.Response {
	resp := &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     http.Header{},
		Body:       body,
	}
	return resp
}

// withFakeHTTPClient swaps the package level HttpClient global for the fake
// and restores the original with t.Cleanup.
func withFakeHTTPClient(t *testing.T, fake CustomHTTPClient) {
	t.Helper()
	original := HttpClient
	HttpClient = fake
	t.Cleanup(func() { HttpClient = original })
}

// unsetEnvWithRestore unsets an env var and restores its previous state with
// t.Cleanup. t.Setenv cannot express absence (empty string is still present).
func unsetEnvWithRestore(t *testing.T, key string) {
	t.Helper()
	original, hadValue := os.LookupEnv(key)
	require.NoError(t, os.Unsetenv(key))
	t.Cleanup(func() {
		if hadValue {
			require.NoError(t, os.Setenv(key, original))
		} else {
			require.NoError(t, os.Unsetenv(key))
		}
	})
}

// ---- CLI-01: generic Get/Post/form contracts ----

func TestClientGet(t *testing.T) {
	t.Run("CLI-01/Get sends method URL and headers and delivers the response", func(t *testing.T) {
		body := newTraceableBody("get-response")
		fake := &recordingClient{resp: newFakeResponse(http.StatusOK, body)}
		withFakeHTTPClient(t, fake)

		headers := http.Header{}
		headers.Set("X-Custom-Header", "custom-value")

		resp, err := Get("http://example.test/page", headers)

		require.NoError(t, err)
		require.NotNil(t, resp)

		assert.Equal(t, 1, fake.doCalls)
		req := fake.lastRequest()
		require.NotNil(t, req)
		assert.Equal(t, http.MethodGet, req.Method)
		assert.Equal(t, "http://example.test/page", req.URL.String())
		assert.Equal(t, "custom-value", req.Header.Get("X-Custom-Header"))
		assert.Nil(t, req.Body, "GET must not carry a body")
	})

	t.Run("CLI-01/Get propagates transport errors", func(t *testing.T) {
		fake := &recordingClient{err: errFakeTransport}
		withFakeHTTPClient(t, fake)

		resp, err := Get("http://example.test/page", nil)

		require.Error(t, err)
		assert.ErrorIs(t, err, errFakeTransport)
		assert.Nil(t, resp)
		assert.Equal(t, 1, fake.doCalls)
	})
}

func TestClientPost(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	t.Run("CLI-01/Post sends JSON body method and headers", func(t *testing.T) {
		body := newTraceableBody(`{"ok":true}`)
		fake := &recordingClient{resp: newFakeResponse(http.StatusOK, body)}
		withFakeHTTPClient(t, fake)

		headers := http.Header{}
		headers.Set("Content-Type", "application/json")

		resp, err := Post("http://example.test/api", payload{Name: "bolo", Age: 2}, headers)

		require.NoError(t, err)
		require.NotNil(t, resp)

		req := fake.lastRequest()
		require.NotNil(t, req)
		assert.Equal(t, http.MethodPost, req.Method)
		assert.Equal(t, "http://example.test/api", req.URL.String())
		assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
		assert.JSONEq(t, `{"name":"bolo","age":2}`, string(fake.lastBody()))
	})

	t.Run("CLI-01/Post delivers non-2xx responses to the caller", func(t *testing.T) {
		// Generic Post has no non-2xx rule: the contract is to hand the
		// response to the caller, the rule lives in the high level helpers.
		body := newTraceableBody("server error detail")
		fake := &recordingClient{resp: newFakeResponse(http.StatusInternalServerError, body)}
		withFakeHTTPClient(t, fake)

		resp, err := Post("http://example.test/api", payload{Name: "bolo"}, nil)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("CLI-01/Post fails with marshal error before calling Do", func(t *testing.T) {
		fake := &recordingClient{resp: newFakeResponse(http.StatusOK, newTraceableBody(""))}
		withFakeHTTPClient(t, fake)

		// Channels are not JSON marshalable.
		resp, err := Post("http://example.test/api", make(chan int), nil)

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, 0, fake.doCalls, "marshal must fail before Do is called")
	})

	t.Run("CLI-01/Post propagates transport errors", func(t *testing.T) {
		fake := &recordingClient{err: errFakeTransport}
		withFakeHTTPClient(t, fake)

		resp, err := Post("http://example.test/api", payload{Name: "bolo"}, nil)

		require.Error(t, err)
		assert.ErrorIs(t, err, errFakeTransport)
		assert.Nil(t, resp)
	})
}

func TestClientPostFormURLEncoded(t *testing.T) {
	t.Run("CLI-01/PostFormURLEncoded sends encoded form and decodes JSON response", func(t *testing.T) {
		body := newTraceableBody(`{"ok":true}`)
		fake := &recordingClient{resp: newFakeResponse(http.StatusOK, body)}
		withFakeHTTPClient(t, fake)

		form := url.Values{}
		form.Set("a", "1")
		form.Set("b", "x y")

		var target struct {
			OK bool `json:"ok"`
		}
		err := PostFormURLEncoded("http://example.test/form", form, &target)

		require.NoError(t, err)
		assert.True(t, target.OK)

		req := fake.lastRequest()
		require.NotNil(t, req)
		assert.Equal(t, http.MethodPost, req.Method)
		assert.Equal(t, "http://example.test/form", req.URL.String())
		assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
		assert.Equal(t, "a=1&b=x+y", string(fake.lastBody()))
	})

	t.Run("CLI-01/PostFormURLEncoded returns error on invalid JSON response", func(t *testing.T) {
		fake := &recordingClient{resp: newFakeResponse(http.StatusOK, newTraceableBody("not-json"))}
		withFakeHTTPClient(t, fake)

		var target struct {
			OK bool `json:"ok"`
		}
		err := PostFormURLEncoded("http://example.test/form", url.Values{"a": {"1"}}, &target)

		require.Error(t, err)
		assert.False(t, target.OK, "target must not be populated from an invalid response")
	})

	t.Run("CLI-01/PostFormURLEncoded propagates transport errors", func(t *testing.T) {
		fake := &recordingClient{err: errFakeTransport}
		withFakeHTTPClient(t, fake)

		var target struct {
			OK bool `json:"ok"`
		}
		err := PostFormURLEncoded("http://example.test/form", url.Values{"a": {"1"}}, &target)

		require.Error(t, err)
		assert.ErrorIs(t, err, errFakeTransport)
	})
}

// ---- CLI-05: Init timeout configuration ----

func TestInitTimeoutConfiguration(t *testing.T) {
	// Init overwrites the global HttpClient: save and restore it.
	originalClient := HttpClient
	t.Cleanup(func() { HttpClient = originalClient })

	cases := []struct {
		name     string
		envValue *string // nil means the variable must be absent
		expected time.Duration
	}{
		{
			name:     "CLI-05/absent env uses the 120s default",
			envValue: nil,
			expected: 120 * time.Second,
		},
		{
			name:     "CLI-05/positive value is honored",
			envValue: strPtr("5"),
			expected: 5 * time.Second,
		},
		{
			name:     "CLI-05/zero uses a safe fallback, never unlimited",
			envValue: strPtr("0"),
			expected: 120 * time.Second,
		},
		{
			name:     "CLI-05/negative uses a safe fallback, never unlimited",
			envValue: strPtr("-1"),
			expected: 120 * time.Second,
		},
		{
			name:     "CLI-05/invalid value uses a safe fallback, never unlimited",
			envValue: strPtr("not-a-number"),
			expected: 120 * time.Second,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.envValue == nil {
				unsetEnvWithRestore(t, "HTTP_CLIENT_TIMEOUT")
			} else {
				t.Setenv("HTTP_CLIENT_TIMEOUT", *tc.envValue)
			}

			Init()

			client, ok := HttpClient.(*http.Client)
			require.True(t, ok, "Init must set HttpClient to *http.Client")
			assert.Equal(t, tc.expected, client.Timeout)
			assert.Positive(t, client.Timeout, "timeout must never be zero/negative (unlimited)")
		})
	}
}

func strPtr(v string) *string {
	return &v
}
