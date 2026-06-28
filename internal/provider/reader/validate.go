package reader

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ErrSSRFBlocked is returned when a URL targets a private/internal address.
var ErrSSRFBlocked = fmt.Errorf("url blocked for security: internal address")

// privateIPBlocks contains CIDR ranges for private/internal networks.
var privateIPBlocks []*net.IPNet

func init() {
	cidrs := []string{
		"127.0.0.0/8",    // loopback
		"10.0.0.0/8",     // private
		"172.16.0.0/12",  // private
		"192.168.0.0/16", // private
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique local
		"169.254.0.0/16", // link-local
	}
	for _, c := range cidrs {
		_, block, err := net.ParseCIDR(c)
		if err == nil {
			privateIPBlocks = append(privateIPBlocks, block)
		}
	}
}

// ValidateURL checks that a URL is safe to fetch.
// Returns an error if the URL points to a private/internal address.
// Intended to prevent SSRF attacks when processing citation URLs.
func ValidateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}

	// Only allow http/https.
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: scheme %q not allowed", ErrSSRFBlocked, u.Scheme)
	}

	host := u.Hostname()

	// Block localhost variants.
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" {
		return fmt.Errorf("%w: localhost is not allowed", ErrSSRFBlocked)
	}

	// Block internal hostnames.
	if strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return fmt.Errorf("%w: internal hostname %q is not allowed", ErrSSRFBlocked, host)
	}

	// Resolve and check against private IP ranges.
	ips, err := net.LookupIP(host)
	if err != nil {
		// DNS failure — let it through (could be a transient error,
		// and blocking it would be overly restrictive for citation URLs)
		return nil
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("%w: %s resolves to private IP %s", ErrSSRFBlocked, host, ip)
		}
	}

	return nil
}

// isPrivateIP checks if an IP falls within private/reserved ranges.
func isPrivateIP(ip net.IP) bool {
	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}
