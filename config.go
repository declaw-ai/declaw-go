package declaw

import (
	"fmt"
	"os"
	"time"
)

// Config holds the connection configuration for the Declaw API.
// It reads defaults from environment variables DECLAW_API_KEY and DECLAW_DOMAIN.
type Config struct {
	// APIKey is the Declaw API key for authentication.
	APIKey string

	// Domain is the API server hostname (default: "api.declaw.ai").
	Domain string

	// Port is the API server port (default: 443).
	Port int

	// APIURL is an explicit base URL override. When set, Domain and Port are ignored.
	APIURL string

	// RequestTimeout is the default timeout for API requests.
	RequestTimeout time.Duration
}

// ConfigOption is a functional option for configuring a Config.
type ConfigOption func(*Config)

// WithAPIKey sets the API key.
func WithAPIKey(key string) ConfigOption {
	return func(c *Config) {
		c.APIKey = key
	}
}

// WithDomain sets the API domain.
func WithDomain(domain string) ConfigOption {
	return func(c *Config) {
		c.Domain = domain
	}
}

// WithAPIURL sets an explicit API base URL, overriding domain and port.
func WithAPIURL(url string) ConfigOption {
	return func(c *Config) {
		c.APIURL = url
	}
}

// WithRequestTimeout sets the default request timeout.
func WithRequestTimeout(d time.Duration) ConfigOption {
	return func(c *Config) {
		c.RequestTimeout = d
	}
}

// NewConfig creates a Config with defaults from environment variables,
// then applies the given options. Environment variables:
//   - DECLAW_API_KEY: API key for authentication
//   - DECLAW_DOMAIN: API server domain (default: "api.declaw.ai")
//   - DECLAW_API_URL: Explicit base URL override (when set, Domain and Port are ignored)
//
// The URL scheme is determined by the port: https for 443, http for 80,
// https for all other ports.
func NewConfig(opts ...ConfigOption) *Config {
	domain := os.Getenv("DECLAW_DOMAIN")
	if domain == "" {
		domain = "api.declaw.ai"
	}

	c := &Config{
		APIKey: os.Getenv("DECLAW_API_KEY"),
		Domain: domain,
		Port:   443,
		APIURL: os.Getenv("DECLAW_API_URL"),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// BaseURL returns the computed base URL for API requests.
// If APIURL is set explicitly, it is returned as-is.
// Otherwise, the URL is constructed from Domain and Port with the
// appropriate scheme (https for port 443, http for port 80, https otherwise).
func (c *Config) BaseURL() string {
	if c.APIURL != "" {
		return c.APIURL
	}

	scheme := "https"
	if c.Port == 80 {
		scheme = "http"
	}

	if (scheme == "https" && c.Port == 443) || (scheme == "http" && c.Port == 80) {
		return fmt.Sprintf("%s://%s", scheme, c.Domain)
	}

	return fmt.Sprintf("%s://%s:%d", scheme, c.Domain, c.Port)
}
