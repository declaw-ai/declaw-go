package declaw

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultMaxRetries = 3
	defaultRetryDelay = 500 * time.Millisecond
)

// apiClient is the internal HTTP client used by all SDK operations.
// It handles authentication, retries, and error mapping.
type apiClient struct {
	httpClient *http.Client
	config     *Config
	maxRetries int
	retryDelay time.Duration
}

// newAPIClient creates a new apiClient with the given configuration.
func newAPIClient(config *Config) *apiClient {
	client := &http.Client{}
	if config.RequestTimeout > 0 {
		client.Timeout = config.RequestTimeout
	}

	return &apiClient{
		httpClient: client,
		config:     config,
		maxRetries: defaultMaxRetries,
		retryDelay: defaultRetryDelay,
	}
}

// reqOpt mutates an outgoing request. Variadic so the existing doRequest callers
// need no change.
type reqOpt func(*http.Request)

// withHeader sets a header on the request.
func withHeader(k, v string) reqOpt {
	return func(r *http.Request) { r.Header.Set(k, v) }
}

// retryJitter spreads a retry delay so clients that failed together do not all
// come back at the same instant. Equal jitter: keep half the backoff to preserve
// growth, randomize the other half to break the lockstep.
//
// Without it every client retries at exactly delay x attempt, so a blip that
// trips N clients produces N simultaneous retries, then N more — the server sees
// the same thundering herd on each round instead of a spread.
func retryJitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	half := d / 2
	return half + time.Duration(rand.Int63n(int64(half)+1))
}

// retryAfter reads a Retry-After header expressed in seconds.
//
// The bool distinguishes "absent or unparseable" (caller falls back to its own
// backoff) from a genuine Retry-After: 0 meaning retry immediately. Returning a
// bare 0 for both conflates them, and the Python and TS clients keep that
// distinction — a train whose members disagree on a wire behavior is worse than
// one that is uniformly wrong.
//
// The HTTP-date form is deliberately unsupported: this API does not emit it, and
// guessing wrong would sleep for hours.
func retryAfter(resp *http.Response) (time.Duration, bool) {
	v := resp.Header.Get("Retry-After")
	if v == "" {
		return 0, false
	}
	secs, err := strconv.Atoi(v)
	if err != nil || secs < 0 {
		return 0, false
	}
	if secs > 60 {
		secs = 60 // never let a server pin a client for minutes
	}
	return time.Duration(secs) * time.Second, true
}

func (c *apiClient) doRequest(ctx context.Context, method, path string, body io.Reader, contentType string, opts ...reqOpt) ([]byte, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("reading request body: %w", err)
		}
	}

	var lastErr error
	// pending carries a server-supplied Retry-After into the next iteration.
	// pendingSet is separate so a Retry-After of 0 ("come back now") is not read
	// as "no header".
	var pending time.Duration
	var pendingSet bool

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := retryJitter(c.retryDelay * time.Duration(attempt))
			if pendingSet {
				delay = pending // server told us when to come back
				pending, pendingSet = 0, false
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		url := c.config.BaseURL() + path

		var reqBody io.Reader
		if bodyBytes != nil {
			reqBody = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
		if err != nil {
			return nil, err
		}

		if c.config.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		for _, opt := range opts {
			opt(req)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return respBody, nil
		}

		if resp.StatusCode >= 500 && attempt < c.maxRetries {
			lastErr = errorFromResponse(resp, respBody, "")
			continue
		}

		err = errorFromResponse(resp, respBody, "")

		// A 409 carrying idempotency_in_progress means the ORIGINAL create is
		// still running and this key already owns it. Retrying the identical
		// request is not a duplicate — it is how the caller recovers the sandbox
		// ID when the first response was lost, which is the whole point of
		// sending the key. Branch on the code, never the status: 409 on this
		// endpoint also means template_not_ready, which retrying cannot fix.
		if resp.StatusCode == http.StatusConflict && attempt < c.maxRetries {
			var se *SandboxError
			if errors.As(err, &se) && se.Code == CodeIdempotencyInProgress {
				pending, pendingSet = retryAfter(resp)
				lastErr = err
				continue
			}
		}

		return nil, err
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("request failed after %d retries", c.maxRetries)
}

func (c *apiClient) jsonBody(v interface{}) (io.Reader, error) {
	if v == nil {
		return nil, nil
	}
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(v); err != nil {
		return nil, fmt.Errorf("encoding request body: %w", err)
	}
	return buf, nil
}

// get performs an HTTP GET request.
func (c *apiClient) get(ctx context.Context, path string) ([]byte, error) {
	return c.doRequest(ctx, http.MethodGet, path, nil, "")
}

// post performs an HTTP POST request with a JSON body.
func (c *apiClient) post(ctx context.Context, path string, body interface{}, opts ...reqOpt) ([]byte, error) {
	r, err := c.jsonBody(body)
	if err != nil {
		return nil, err
	}
	ct := ""
	if body != nil {
		ct = "application/json"
	}
	return c.doRequest(ctx, http.MethodPost, path, r, ct, opts...)
}

// postRaw performs an HTTP POST request with a raw binary body.
func (c *apiClient) postRaw(ctx context.Context, path string, body []byte) ([]byte, error) {
	return c.doRequest(ctx, http.MethodPost, path, bytes.NewReader(body), "application/octet-stream")
}

// postGzip performs an HTTP POST request with a gzip-compressed body
// (Content-Type: application/gzip).
func (c *apiClient) postGzip(ctx context.Context, path string, body []byte) ([]byte, error) {
	return c.doRequest(ctx, http.MethodPost, path, bytes.NewReader(body), "application/gzip")
}

// putRaw performs an HTTP PUT request with a raw binary body
// (Content-Type: application/octet-stream).
func (c *apiClient) putRaw(ctx context.Context, path string, body []byte) ([]byte, error) {
	return c.doRequest(ctx, http.MethodPut, path, bytes.NewReader(body), "application/octet-stream")
}

// patch performs an HTTP PATCH request with a JSON body.
func (c *apiClient) patch(ctx context.Context, path string, body interface{}) ([]byte, error) {
	r, err := c.jsonBody(body)
	if err != nil {
		return nil, err
	}
	ct := ""
	if body != nil {
		ct = "application/json"
	}
	return c.doRequest(ctx, http.MethodPatch, path, r, ct)
}

// put performs an HTTP PUT request with a raw body.
func (c *apiClient) put(ctx context.Context, path string, body interface{}) ([]byte, error) {
	switch v := body.(type) {
	case io.Reader:
		return c.doRequest(ctx, http.MethodPut, path, v, "application/octet-stream")
	case []byte:
		return c.doRequest(ctx, http.MethodPut, path, bytes.NewReader(v), "application/octet-stream")
	default:
		r, err := c.jsonBody(body)
		if err != nil {
			return nil, err
		}
		ct := ""
		if body != nil {
			ct = "application/json"
		}
		return c.doRequest(ctx, http.MethodPut, path, r, ct)
	}
}

// delete performs an HTTP DELETE request.
func (c *apiClient) delete(ctx context.Context, path string) ([]byte, error) {
	return c.doRequest(ctx, http.MethodDelete, path, nil, "")
}

// deleteJSON performs an HTTP DELETE request with a JSON body.
func (c *apiClient) deleteJSON(ctx context.Context, path string, body interface{}) ([]byte, error) {
	r, err := c.jsonBody(body)
	if err != nil {
		return nil, err
	}
	ct := ""
	if body != nil {
		ct = "application/json"
	}
	return c.doRequest(ctx, http.MethodDelete, path, r, ct)
}

// stream performs an HTTP request and returns the raw response for streaming.
// It uses a dedicated transport with compression disabled so SSE events are
// delivered without buffering.
func (c *apiClient) stream(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	url := c.config.BaseURL() + path

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	if c.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	streamClient := &http.Client{
		Transport: &http.Transport{
			DisableCompression: true,
			ForceAttemptHTTP2:  true,
		},
	}
	return streamClient.Do(req)
}

// configFromSandboxOpts creates a Config by merging sandbox options with defaults.
func configFromSandboxOpts(opts *sandboxOpts) *Config {
	cfg := NewConfig()
	if opts.APIKey != "" {
		cfg.APIKey = opts.APIKey
	}
	if opts.Domain != "" {
		cfg.Domain = opts.Domain
	}
	if opts.APIURL != "" {
		cfg.APIURL = opts.APIURL
	}
	if opts.RequestTimeout > 0 {
		cfg.RequestTimeout = opts.RequestTimeout
	}
	return cfg
}

// resolveSandboxOpts applies all SandboxOption functions and returns the resolved options.
func resolveSandboxOpts(opts []SandboxOption) *sandboxOpts {
	o := &sandboxOpts{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
