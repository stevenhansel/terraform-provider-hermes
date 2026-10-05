// Package hermes contains the typed client for Hermes' dashboard management
// API. Terraform/OpenTofu resources live in internal/provider and depend on
// this package through small domain interfaces.
package hermes

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	maxResponseBytes   = 4 << 20
	defaultHTTPTimeout = 30 * time.Second
)

// Client is the authenticated client for Hermes' dashboard management API.
//
// The client owns the session cookie and serializes mutations because Hermes
// persists dashboard changes to configuration files on disk.
type Client struct {
	baseURL   string
	username  string
	password  string
	http      *http.Client
	userAgent string

	authMu        sync.Mutex
	authenticated bool
	writeMu       sync.Mutex
}

// NewClient creates a Hermes dashboard client using the development user
// agent. Providers should normally use NewClientWithVersion.
func NewClient(endpoint, username, password string) (*Client, error) {
	return NewClientWithVersion(endpoint, username, password, "dev")
}

// NewClientWithVersion creates a Hermes dashboard client with a versioned
// user agent for compatibility diagnostics.
func NewClientWithVersion(endpoint, username, password, version string) (*Client, error) {
	trimmedEndpoint := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	parsed, err := url.Parse(trimmedEndpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("endpoint must be an absolute http(s) URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("endpoint scheme must be http or https")
	}
	if strings.TrimSpace(username) == "" || password == "" {
		return nil, fmt.Errorf("username and password are required; enable Hermes' basic dashboard provider for machine access")
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}

	return &Client{
		baseURL:   trimmedEndpoint,
		username:  username,
		password:  password,
		http:      &http.Client{Jar: jar, Timeout: defaultHTTPTimeout},
		userAgent: providerUserAgent(version),
	}, nil
}

func profileQuery(profile string) string {
	if strings.TrimSpace(profile) == "" {
		return ""
	}
	return url.Values{"profile": []string{profile}}.Encode()
}

func providerUserAgent(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		version = "dev"
	}
	return "terraform-provider-hermes/" + version
}
