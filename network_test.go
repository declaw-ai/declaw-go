package declaw

import (
	"testing"
)

// --- AllTraffic constant ---

func TestAllTraffic_Value(t *testing.T) {
	if AllTraffic != "*" {
		t.Errorf("AllTraffic = %q, want %q", AllTraffic, "*")
	}
}

func TestAllTraffic_IsWildcard(t *testing.T) {
	// AllTraffic should match any domain when used in DomainMatches
	if !DomainMatches(AllTraffic, "anything.com") {
		t.Error("AllTraffic should match any domain")
	}
}

// --- SandboxNetworkOpts ---

func TestSandboxNetworkOpts_AllFields(t *testing.T) {
	allowPublic := true
	maskHost := true
	opts := SandboxNetworkOpts{
		AllowOut:           []string{"*.openai.com", "pypi.org"},
		DenyOut:            []string{"*"},
		AllowPublicTraffic: &allowPublic,
		MaskRequestHost:    &maskHost,
	}

	if len(opts.AllowOut) != 2 {
		t.Errorf("AllowOut length = %d, want 2", len(opts.AllowOut))
	}
	if opts.AllowOut[0] != "*.openai.com" {
		t.Errorf("AllowOut[0] = %q, want expected", opts.AllowOut[0])
	}
	if opts.AllowOut[1] != "pypi.org" {
		t.Errorf("AllowOut[1] = %q, want expected", opts.AllowOut[1])
	}
	if len(opts.DenyOut) != 1 {
		t.Errorf("DenyOut length = %d, want 1", len(opts.DenyOut))
	}
	if opts.DenyOut[0] != "*" {
		t.Errorf("DenyOut[0] = %q, want %q", opts.DenyOut[0], "*")
	}
	if opts.AllowPublicTraffic == nil || !*opts.AllowPublicTraffic {
		t.Error("AllowPublicTraffic should be non-nil and true")
	}
	if opts.MaskRequestHost == nil || !*opts.MaskRequestHost {
		t.Error("MaskRequestHost should be non-nil and true")
	}
}

func TestSandboxNetworkOpts_ZeroValue(t *testing.T) {
	var opts SandboxNetworkOpts

	if opts.AllowOut != nil {
		t.Error("zero AllowOut should be nil")
	}
	if opts.DenyOut != nil {
		t.Error("zero DenyOut should be nil")
	}
	if opts.AllowPublicTraffic != nil {
		t.Error("zero AllowPublicTraffic should be nil")
	}
	if opts.MaskRequestHost != nil {
		t.Error("zero MaskRequestHost should be nil")
	}
}

func TestSandboxNetworkOpts_AllowPublicTrafficFalse(t *testing.T) {
	allowPublic := false
	opts := SandboxNetworkOpts{
		AllowPublicTraffic: &allowPublic,
	}

	if opts.AllowPublicTraffic == nil {
		t.Fatal("AllowPublicTraffic should not be nil")
	}
	if *opts.AllowPublicTraffic {
		t.Error("AllowPublicTraffic should be false")
	}
}

func TestSandboxNetworkOpts_DenyAllAllowSpecific(t *testing.T) {
	opts := SandboxNetworkOpts{
		DenyOut:  []string{"*"},
		AllowOut: []string{"1.1.1.1", "8.8.8.0/24", "*.github.com"},
	}

	if len(opts.DenyOut) != 1 {
		t.Errorf("DenyOut length = %d, want 1", len(opts.DenyOut))
	}
	if len(opts.AllowOut) != 3 {
		t.Errorf("AllowOut length = %d, want 3", len(opts.AllowOut))
	}
}

func TestSandboxNetworkOpts_EmptyLists(t *testing.T) {
	opts := SandboxNetworkOpts{
		AllowOut: []string{},
		DenyOut:  []string{},
	}

	if len(opts.AllowOut) != 0 {
		t.Errorf("AllowOut length = %d, want 0", len(opts.AllowOut))
	}
	if len(opts.DenyOut) != 0 {
		t.Errorf("DenyOut length = %d, want 0", len(opts.DenyOut))
	}
}

func TestSandboxNetworkOpts_MaskRequestHostFalse(t *testing.T) {
	maskHost := false
	opts := SandboxNetworkOpts{
		MaskRequestHost: &maskHost,
	}

	if opts.MaskRequestHost == nil {
		t.Fatal("MaskRequestHost should not be nil")
	}
	if *opts.MaskRequestHost {
		t.Error("MaskRequestHost should be false")
	}
}

func TestSandboxNetworkOpts_MaskRequestHostNil(t *testing.T) {
	opts := SandboxNetworkOpts{}

	if opts.MaskRequestHost != nil {
		t.Error("MaskRequestHost should be nil when not set")
	}
}

// --- DomainMatches ---

func TestDomainMatches(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		domain  string
		want    bool
	}{
		// Exact matches
		{
			name:    "exact match",
			pattern: "example.com",
			domain:  "example.com",
			want:    true,
		},
		{
			name:    "exact match with subdomain",
			pattern: "api.example.com",
			domain:  "api.example.com",
			want:    true,
		},
		{
			name:    "no match different domain",
			pattern: "example.com",
			domain:  "other.com",
			want:    false,
		},
		{
			name:    "no match subdomain vs base",
			pattern: "api.example.com",
			domain:  "example.com",
			want:    false,
		},

		// Wildcard pattern "*"
		{
			name:    "wildcard star matches anything",
			pattern: "*",
			domain:  "anything.com",
			want:    true,
		},
		{
			name:    "wildcard star matches complex domain",
			pattern: "*",
			domain:  "deep.nested.sub.domain.example.co.uk",
			want:    true,
		},
		{
			name:    "wildcard star matches empty-like domain",
			pattern: "*",
			domain:  "x",
			want:    true,
		},

		// Wildcard prefix "*.example.com"
		{
			name:    "wildcard prefix matches subdomain",
			pattern: "*.example.com",
			domain:  "api.example.com",
			want:    true,
		},
		{
			name:    "wildcard prefix matches deep subdomain",
			pattern: "*.example.com",
			domain:  "deep.api.example.com",
			want:    true,
		},
		{
			name:    "wildcard prefix matches very deep subdomain",
			pattern: "*.example.com",
			domain:  "a.b.c.d.example.com",
			want:    true,
		},
		{
			name:    "wildcard prefix does not match base domain",
			pattern: "*.example.com",
			domain:  "example.com",
			want:    false,
		},
		{
			name:    "wildcard prefix does not match different domain",
			pattern: "*.example.com",
			domain:  "api.other.com",
			want:    false,
		},
		{
			name:    "wildcard prefix does not match partial suffix",
			pattern: "*.example.com",
			domain:  "notexample.com",
			want:    false,
		},

		// Case insensitivity
		{
			name:    "case insensitive exact match",
			pattern: "Example.COM",
			domain:  "example.com",
			want:    true,
		},
		{
			name:    "case insensitive domain",
			pattern: "example.com",
			domain:  "EXAMPLE.COM",
			want:    true,
		},
		{
			name:    "case insensitive wildcard",
			pattern: "*.EXAMPLE.COM",
			domain:  "api.example.com",
			want:    true,
		},
		{
			name:    "case insensitive wildcard domain",
			pattern: "*.example.com",
			domain:  "API.EXAMPLE.COM",
			want:    true,
		},

		// Edge cases
		{
			name:    "empty pattern empty domain",
			pattern: "",
			domain:  "",
			want:    true,
		},
		{
			name:    "empty pattern non-empty domain",
			pattern: "",
			domain:  "example.com",
			want:    false,
		},
		{
			name:    "non-empty pattern empty domain",
			pattern: "example.com",
			domain:  "",
			want:    false,
		},
		{
			name:    "just star wildcard",
			pattern: "*",
			domain:  "",
			want:    true,
		},
		{
			name:    "wildcard dot only",
			pattern: "*.",
			domain:  "x.",
			want:    true,
		},

		// Additional subdomain patterns
		{
			name:    "wildcard openai",
			pattern: "*.openai.com",
			domain:  "api.openai.com",
			want:    true,
		},
		{
			name:    "wildcard anthropic no match",
			pattern: "*.openai.com",
			domain:  "api.anthropic.com",
			want:    false,
		},

		// Tricky suffix matching
		{
			name:    "similar suffix but different domain",
			pattern: "*.example.com",
			domain:  "myexample.com",
			want:    false,
		},
		{
			name:    "domain with trailing dot vs pattern without",
			pattern: "example.com",
			domain:  "example.com.",
			want:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DomainMatches(tc.pattern, tc.domain)
			if got != tc.want {
				t.Errorf("DomainMatches(%q, %q) = %v, want %v",
					tc.pattern, tc.domain, got, tc.want)
			}
		})
	}
}

func TestDomainMatches_AllTrafficConstant(t *testing.T) {
	// AllTraffic constant specifically
	domains := []string{
		"example.com",
		"api.openai.com",
		"deep.nested.domain.co.uk",
		"localhost",
		"192.168.1.1",
		"",
	}

	for _, d := range domains {
		t.Run(d, func(t *testing.T) {
			if !DomainMatches(AllTraffic, d) {
				t.Errorf("AllTraffic should match %q", d)
			}
		})
	}
}

func TestDomainMatches_WildcardPrefix_MultipleLevels(t *testing.T) {
	// The implementation uses HasSuffix on ".example.com",
	// so "a.b.example.com" should match because it ends with ".example.com"
	if !DomainMatches("*.example.com", "a.b.example.com") {
		t.Error("*.example.com should match a.b.example.com")
	}
	if !DomainMatches("*.example.com", "x.y.z.example.com") {
		t.Error("*.example.com should match x.y.z.example.com")
	}
}

func TestDomainMatches_NoWildcardPrefix_NoPartialMatch(t *testing.T) {
	// Without wildcard prefix, only exact match should work
	if DomainMatches("example.com", "api.example.com") {
		t.Error("exact pattern should not match subdomain")
	}
	if DomainMatches("example.com", "sub.example.com") {
		t.Error("exact pattern should not match subdomain")
	}
}

func TestDomainMatches_SpecialCharacters(t *testing.T) {
	// Dots in pattern are literal, not regex wildcards
	if DomainMatches("example.com", "exampleXcom") {
		t.Error("dots should be literal, not regex wildcards")
	}
}

func TestDomainMatches_IPAddresses(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		domain  string
		want    bool
	}{
		{
			name:    "exact IP match",
			pattern: "192.168.1.1",
			domain:  "192.168.1.1",
			want:    true,
		},
		{
			name:    "different IP no match",
			pattern: "192.168.1.1",
			domain:  "192.168.1.2",
			want:    false,
		},
		{
			name:    "wildcard matches IP",
			pattern: "*",
			domain:  "10.0.0.1",
			want:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DomainMatches(tc.pattern, tc.domain)
			if got != tc.want {
				t.Errorf("DomainMatches(%q, %q) = %v, want %v",
					tc.pattern, tc.domain, got, tc.want)
			}
		})
	}
}
