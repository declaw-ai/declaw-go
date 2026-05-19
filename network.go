package declaw

import "strings"

// AllTraffic is a wildcard that matches all domains/traffic.
const AllTraffic = "*"

// SandboxNetworkOpts configures network access for a sandbox.
type SandboxNetworkOpts struct {
	// AllowOut is a list of domain patterns allowed for outbound traffic.
	AllowOut []string `json:"allow_out,omitempty"`

	// DenyOut is a list of domain patterns denied for outbound traffic.
	DenyOut []string `json:"deny_out,omitempty"`

	// AllowPublicTraffic enables inbound public traffic to the sandbox.
	// Use a pointer to distinguish "not set" from "explicitly false".
	AllowPublicTraffic *bool `json:"allow_public_traffic,omitempty"`

	// MaskRequestHost, when non-nil and true, masks the original request
	// host in proxied requests.
	MaskRequestHost *bool `json:"mask_request_host,omitempty"`
}

// DomainMatches checks if a domain matches a wildcard pattern.
// Patterns support a leading wildcard: "*.example.com" matches "api.example.com"
// and "sub.api.example.com". An exact match is also supported.
// The special pattern "*" matches any domain.
func DomainMatches(pattern, domain string) bool {
	if pattern == AllTraffic {
		return true
	}

	pattern = strings.ToLower(pattern)
	domain = strings.ToLower(domain)

	if pattern == domain {
		return true
	}

	// Handle wildcard prefix: "*.example.com"
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // ".example.com"
		return strings.HasSuffix(domain, suffix)
	}

	return false
}
