package declaw

import (
	"testing"
	"time"
)

func TestNewConfig_Defaults(t *testing.T) {
	t.Setenv("DECLAW_API_KEY", "")
	t.Setenv("DECLAW_DOMAIN", "")
	t.Setenv("DECLAW_API_URL", "")

	cfg := NewConfig()

	if cfg.Domain != "api.declaw.ai" {
		t.Errorf("expected default domain %q, got %q", "api.declaw.ai", cfg.Domain)
	}
	if cfg.Port != 443 {
		t.Errorf("expected default port 443, got %d", cfg.Port)
	}
	if cfg.APIKey != "" {
		t.Errorf("expected empty API key when env not set, got %q", cfg.APIKey)
	}
	if cfg.APIURL != "" {
		t.Errorf("expected empty APIURL by default, got %q", cfg.APIURL)
	}
	if cfg.RequestTimeout != 0 {
		t.Errorf("expected zero RequestTimeout by default, got %v", cfg.RequestTimeout)
	}
}

func TestNewConfig_ReadsEnvVars(t *testing.T) {
	t.Setenv("DECLAW_API_KEY", "test-key-abc")
	t.Setenv("DECLAW_DOMAIN", "custom.declaw.dev")
	t.Setenv("DECLAW_API_URL", "")

	cfg := NewConfig()

	if cfg.APIKey != "test-key-abc" {
		t.Errorf("expected API key %q from env, got %q", "test-key-abc", cfg.APIKey)
	}
	if cfg.Domain != "custom.declaw.dev" {
		t.Errorf("expected domain %q from env, got %q", "custom.declaw.dev", cfg.Domain)
	}
}

func TestNewConfig_EnvDomainFallsBackToDefault(t *testing.T) {
	t.Setenv("DECLAW_DOMAIN", "")
	t.Setenv("DECLAW_API_URL", "")

	cfg := NewConfig()

	if cfg.Domain != "api.declaw.ai" {
		t.Errorf("expected fallback domain %q, got %q", "api.declaw.ai", cfg.Domain)
	}
}

func TestNewConfig_WithAPIKey_OverridesEnv(t *testing.T) {
	t.Setenv("DECLAW_API_KEY", "env-key")

	cfg := NewConfig(WithAPIKey("explicit-key"))

	if cfg.APIKey != "explicit-key" {
		t.Errorf("expected WithAPIKey to override env, got %q", cfg.APIKey)
	}
}

func TestNewConfig_WithDomain_OverridesEnv(t *testing.T) {
	t.Setenv("DECLAW_DOMAIN", "env-domain.ai")

	cfg := NewConfig(WithDomain("override.ai"))

	if cfg.Domain != "override.ai" {
		t.Errorf("expected WithDomain to override env, got %q", cfg.Domain)
	}
}

func TestNewConfig_WithAPIURL(t *testing.T) {
	cfg := NewConfig(WithAPIURL("http://localhost:3000"))

	if cfg.APIURL != "http://localhost:3000" {
		t.Errorf("expected APIURL %q, got %q", "http://localhost:3000", cfg.APIURL)
	}
}

func TestNewConfig_WithRequestTimeout(t *testing.T) {
	cfg := NewConfig(WithRequestTimeout(30 * time.Second))

	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", cfg.RequestTimeout)
	}
}

func TestNewConfig_MultipleOptions(t *testing.T) {
	t.Setenv("DECLAW_API_KEY", "env-key")
	t.Setenv("DECLAW_DOMAIN", "env-domain.ai")

	cfg := NewConfig(
		WithAPIKey("opt-key"),
		WithDomain("opt-domain.ai"),
		WithRequestTimeout(10*time.Second),
	)

	if cfg.APIKey != "opt-key" {
		t.Errorf("expected API key %q, got %q", "opt-key", cfg.APIKey)
	}
	if cfg.Domain != "opt-domain.ai" {
		t.Errorf("expected domain %q, got %q", "opt-domain.ai", cfg.Domain)
	}
	if cfg.RequestTimeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", cfg.RequestTimeout)
	}
}

func TestNewConfig_OptionPrecedence(t *testing.T) {
	// Explicit option > env var > default
	t.Setenv("DECLAW_API_KEY", "env-key")
	t.Setenv("DECLAW_API_URL", "")

	// Without option: env var wins over default
	cfg1 := NewConfig()
	if cfg1.APIKey != "env-key" {
		t.Errorf("expected env var key %q, got %q", "env-key", cfg1.APIKey)
	}

	// With option: option wins over env var
	cfg2 := NewConfig(WithAPIKey("opt-key"))
	if cfg2.APIKey != "opt-key" {
		t.Errorf("expected option key %q, got %q", "opt-key", cfg2.APIKey)
	}
}

func TestNewConfig_LastOptionWins(t *testing.T) {
	cfg := NewConfig(
		WithAPIKey("first"),
		WithAPIKey("second"),
	)

	if cfg.APIKey != "second" {
		t.Errorf("expected last option to win, got %q", cfg.APIKey)
	}
}

func TestNewConfig_EmptyAPIKey(t *testing.T) {
	t.Setenv("DECLAW_API_KEY", "")

	cfg := NewConfig()

	if cfg.APIKey != "" {
		t.Errorf("expected empty API key, got %q", cfg.APIKey)
	}
}

func TestNewConfig_WithAPIKeyEmpty(t *testing.T) {
	t.Setenv("DECLAW_API_KEY", "env-key")

	cfg := NewConfig(WithAPIKey(""))

	if cfg.APIKey != "" {
		t.Errorf("expected empty API key from explicit option, got %q", cfg.APIKey)
	}
}

func TestBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected string
	}{
		{
			name: "default port 443 uses https without port",
			config: Config{
				Domain: "api.declaw.ai",
				Port:   443,
			},
			expected: "https://api.declaw.ai",
		},
		{
			name: "port 80 uses http without port",
			config: Config{
				Domain: "api.declaw.ai",
				Port:   80,
			},
			expected: "http://api.declaw.ai",
		},
		{
			name: "custom port 8080 uses https with port",
			config: Config{
				Domain: "localhost",
				Port:   8080,
			},
			expected: "https://localhost:8080",
		},
		{
			name: "custom port 3000 uses https with port",
			config: Config{
				Domain: "dev.declaw.ai",
				Port:   3000,
			},
			expected: "https://dev.declaw.ai:3000",
		},
		{
			name: "port 8443 uses https with port",
			config: Config{
				Domain: "proxy.internal",
				Port:   8443,
			},
			expected: "https://proxy.internal:8443",
		},
		{
			name: "APIURL override takes precedence",
			config: Config{
				Domain: "should-be-ignored.com",
				Port:   443,
				APIURL: "http://localhost:9999/custom",
			},
			expected: "http://localhost:9999/custom",
		},
		{
			name: "APIURL override with empty domain",
			config: Config{
				APIURL: "https://custom-endpoint.io",
			},
			expected: "https://custom-endpoint.io",
		},
		{
			name: "port 1 uses https with port",
			config: Config{
				Domain: "test.ai",
				Port:   1,
			},
			expected: "https://test.ai:1",
		},
		{
			name: "port 0 uses https with port",
			config: Config{
				Domain: "test.ai",
				Port:   0,
			},
			expected: "https://test.ai:0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.config.BaseURL()
			if got != tc.expected {
				t.Errorf("BaseURL() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestBaseURL_ViaNewConfig(t *testing.T) {
	t.Setenv("DECLAW_API_KEY", "")
	t.Setenv("DECLAW_DOMAIN", "")
	t.Setenv("DECLAW_API_URL", "")

	cfg := NewConfig()
	url := cfg.BaseURL()

	if url != "https://api.declaw.ai" {
		t.Errorf("expected default BaseURL %q, got %q", "https://api.declaw.ai", url)
	}
}

func TestBaseURL_ViaNewConfigWithDomain(t *testing.T) {
	t.Setenv("DECLAW_DOMAIN", "")
	t.Setenv("DECLAW_API_URL", "")

	cfg := NewConfig(WithDomain("custom.example.com"))
	url := cfg.BaseURL()

	if url != "https://custom.example.com" {
		t.Errorf("expected BaseURL %q, got %q", "https://custom.example.com", url)
	}
}

func TestBaseURL_ViaNewConfigWithAPIURL(t *testing.T) {
	cfg := NewConfig(WithAPIURL("http://localhost:3000"))
	url := cfg.BaseURL()

	if url != "http://localhost:3000" {
		t.Errorf("expected BaseURL %q, got %q", "http://localhost:3000", url)
	}
}

func TestConfigOption_FunctionalOption(t *testing.T) {
	// Verify ConfigOption type is a function that modifies Config
	var opt ConfigOption = func(c *Config) {
		c.Domain = "custom-via-func.com"
		c.Port = 9090
	}

	t.Setenv("DECLAW_DOMAIN", "")
	cfg := NewConfig(opt)

	if cfg.Domain != "custom-via-func.com" {
		t.Errorf("expected domain from custom option, got %q", cfg.Domain)
	}
	if cfg.Port != 9090 {
		t.Errorf("expected port from custom option, got %d", cfg.Port)
	}
}

func TestConfig_ZeroValue(t *testing.T) {
	// Zero value Config should still produce a BaseURL
	var cfg Config
	url := cfg.BaseURL()

	// Port 0 is not 443 and not 80, so scheme is https with port
	if url != "https://:0" {
		t.Errorf("zero Config BaseURL = %q, want %q", url, "https://:0")
	}
}

func TestWithDomain_DomainOnly(t *testing.T) {
	t.Setenv("DECLAW_DOMAIN", "")

	cfg := NewConfig(WithDomain("api.staging.declaw.ai"))

	if cfg.Domain != "api.staging.declaw.ai" {
		t.Errorf("expected domain %q, got %q", "api.staging.declaw.ai", cfg.Domain)
	}
	// Port should remain default
	if cfg.Port != 443 {
		t.Errorf("expected port 443 when domain has no port, got %d", cfg.Port)
	}
}

func TestWithRequestTimeout_ZeroDuration(t *testing.T) {
	cfg := NewConfig(WithRequestTimeout(0))

	if cfg.RequestTimeout != 0 {
		t.Errorf("expected zero timeout, got %v", cfg.RequestTimeout)
	}
}

func TestWithRequestTimeout_LargeDuration(t *testing.T) {
	cfg := NewConfig(WithRequestTimeout(24 * time.Hour))

	if cfg.RequestTimeout != 24*time.Hour {
		t.Errorf("expected 24h timeout, got %v", cfg.RequestTimeout)
	}
}
