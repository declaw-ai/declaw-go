package declaw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func (c *apiClient) doRequest(ctx context.Context, method, path string, body io.Reader, contentType string) ([]byte, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("reading request body: %w", err)
		}
	}

	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := c.retryDelay * time.Duration(attempt)
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

		return nil, errorFromResponse(resp, respBody, "")
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
func (c *apiClient) post(ctx context.Context, path string, body interface{}) ([]byte, error) {
	r, err := c.jsonBody(body)
	if err != nil {
		return nil, err
	}
	ct := ""
	if body != nil {
		ct = "application/json"
	}
	return c.doRequest(ctx, http.MethodPost, path, r, ct)
}

// postRaw performs an HTTP POST request with a raw binary body.
func (c *apiClient) postRaw(ctx context.Context, path string, body []byte) ([]byte, error) {
	return c.doRequest(ctx, http.MethodPost, path, bytes.NewReader(body), "application/octet-stream")
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
	req.Header.Set("Connection", "keep-alive")

	streamClient := &http.Client{
		Transport: &http.Transport{
			DisableCompression: true,
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
