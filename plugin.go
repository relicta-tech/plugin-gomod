// Package main implements the GoMod plugin for Relicta.
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/relicta-tech/relicta-plugin-sdk/helpers"
	"github.com/relicta-tech/relicta-plugin-sdk/plugin"
)

// Default Go module proxy URL.
const defaultProxyURL = "https://proxy.golang.org"

// Default timeout in seconds.
const defaultTimeout = 30

// httpClient is the HTTP client used for requests.
// Can be overridden in tests.
var httpClient HTTPClient = nil

// HTTPClient interface for HTTP operations (allows mocking in tests).
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// getHTTPClient returns the HTTP client to use for requests.
func getHTTPClient(timeout time.Duration) HTTPClient {
	if httpClient != nil {
		return httpClient
	}
	return createDefaultHTTPClient(timeout)
}

// createDefaultHTTPClient creates a secure HTTP client with the given timeout.
func createDefaultHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			if req.URL.Scheme != "https" {
				return fmt.Errorf("redirect to non-HTTPS URL not allowed")
			}
			return nil
		},
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     90 * time.Second,
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS13,
			},
		},
	}
}

// Module path validation patterns.
var (
	// modulePathPattern validates Go module paths.
	// Format: host/path (e.g., github.com/user/repo).
	// Allows: alphanumerics, dots, dashes, underscores, and slashes.
	modulePathPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*\.[a-zA-Z0-9._-]+(/[a-zA-Z0-9._-]+)+$`)

	// simpleModulePattern for shorter paths like example.com/repo.
	simpleModulePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*\.[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+$`)
)

// validateModulePath validates a Go module path.
func validateModulePath(modulePath string) error {
	if modulePath == "" {
		return fmt.Errorf("module path cannot be empty")
	}
	if len(modulePath) > 500 {
		return fmt.Errorf("module path too long (max 500 characters)")
	}

	// Check for path traversal attempts.
	if strings.Contains(modulePath, "..") {
		return fmt.Errorf("module path cannot contain '..'")
	}

	// Check for leading slash.
	if strings.HasPrefix(modulePath, "/") {
		return fmt.Errorf("module path cannot start with '/'")
	}

	// Check for double slashes before regex (more specific error message).
	if strings.Contains(modulePath, "//") {
		return fmt.Errorf("module path cannot contain '//'")
	}

	// Check for valid Go module path format.
	if !modulePathPattern.MatchString(modulePath) && !simpleModulePattern.MatchString(modulePath) {
		return fmt.Errorf("invalid module path format: must be like 'github.com/user/repo'")
	}

	return nil
}

// validateProxyURL validates that a proxy URL is safe (SSRF protection).
func validateProxyURL(proxyURL string) error {
	// Only allow HTTPS.
	if !strings.HasPrefix(proxyURL, "https://") {
		return fmt.Errorf("proxy URL must use HTTPS")
	}

	// Parse URL to validate structure.
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("invalid proxy URL: %w", err)
	}

	// Must have a valid host.
	if parsed.Host == "" {
		return fmt.Errorf("proxy URL must have a valid host")
	}

	// SSRF protection: block localhost and private IPs.
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return fmt.Errorf("proxy URL cannot be localhost")
	}

	// Block common private network indicators.
	if strings.HasPrefix(host, "10.") ||
		strings.HasPrefix(host, "192.168.") ||
		strings.HasPrefix(host, "172.") ||
		strings.HasSuffix(host, ".local") ||
		strings.HasSuffix(host, ".internal") {
		return fmt.Errorf("proxy URL cannot point to private network")
	}

	return nil
}

// GoModPlugin implements the Publish Go modules to proxy.golang.org plugin.
type GoModPlugin struct{}

// Config holds the plugin configuration.
type Config struct {
	ModulePath string // Full Go module path (e.g., "github.com/user/repo")
	ProxyURL   string // Go module proxy URL (default: "https://proxy.golang.org")
	Private    bool   // If true, skip proxy notification (private modules)
	Timeout    int    // Request timeout in seconds (default: 30)
}

// GetInfo returns plugin metadata.
func (p *GoModPlugin) GetInfo() plugin.Info {
	return plugin.Info{
		Name:        "gomod",
		Version:     "2.0.0",
		Description: "Publish Go modules to proxy.golang.org",
		Author:      "Relicta Team",
		Hooks: []plugin.Hook{
			plugin.HookPostPublish,
		},
		ConfigSchema: `{
			"type": "object",
			"properties": {
				"module_path": {"type": "string", "description": "Full Go module path (e.g., github.com/user/repo, or use GO_MODULE_PATH env)"},
				"proxy_url": {"type": "string", "description": "Go module proxy URL (default: https://proxy.golang.org)"},
				"private": {"type": "boolean", "description": "Skip proxy notification for private modules", "default": false},
				"timeout": {"type": "integer", "description": "Request timeout in seconds", "default": 30}
			},
			"required": ["module_path"]
		}`,
	}
}

// Execute runs the plugin for a given hook.
func (p *GoModPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
	cfg := p.parseConfig(req.Config)

	switch req.Hook {
	case plugin.HookPostPublish:
		return p.postPublish(ctx, cfg, req.Context, req.DryRun)
	default:
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Hook %s not handled", req.Hook),
		}, nil
	}
}

func (p *GoModPlugin) postPublish(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	// Validate module path.
	if err := validateModulePath(cfg.ModulePath); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid module path: %v", err),
		}, nil
	}

	// Check if this is a private module.
	if cfg.Private {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Skipping proxy notification for private module",
			Outputs: map[string]any{
				"module_path": cfg.ModulePath,
				"private":     true,
				"skipped":     true,
			},
		}, nil
	}

	// Validate proxy URL.
	if err := validateProxyURL(cfg.ProxyURL); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid proxy URL: %v", err),
		}, nil
	}

	// Get version from release context.
	version := releaseCtx.Version
	if version == "" {
		version = releaseCtx.TagName
	}
	if version == "" {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   "version is required for proxy notification",
		}, nil
	}

	// Ensure version has v prefix for Go modules.
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Would notify Go module proxy for %s@%s", cfg.ModulePath, version),
			Outputs: map[string]any{
				"module_path": cfg.ModulePath,
				"version":     version,
				"proxy_url":   cfg.ProxyURL,
			},
		}, nil
	}

	// Trigger proxy to index the module version.
	if err := p.triggerProxyIndex(ctx, cfg, version); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to notify proxy: %v", err),
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Go module proxy notified for %s@%s", cfg.ModulePath, version),
		Outputs: map[string]any{
			"module_path": cfg.ModulePath,
			"version":     version,
			"proxy_url":   cfg.ProxyURL,
		},
	}, nil
}

// triggerProxyIndex sends a request to the Go module proxy to index the version.
func (p *GoModPlugin) triggerProxyIndex(ctx context.Context, cfg *Config, version string) error {
	// Build the proxy URL: {proxy_url}/{module}/@v/{version}.info
	// URL-encode the module path for safety.
	encodedModule := url.PathEscape(cfg.ModulePath)
	// Replace %2F back to / for proper module path format in URL.
	encodedModule = strings.ReplaceAll(encodedModule, "%2F", "/")

	proxyRequestURL := fmt.Sprintf("%s/%s/@v/%s.info",
		strings.TrimSuffix(cfg.ProxyURL, "/"),
		encodedModule,
		version,
	)

	// Validate the final URL.
	if err := validateProxyURL(proxyRequestURL); err != nil {
		return fmt.Errorf("invalid request URL: %w", err)
	}

	// Create HTTP request.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, proxyRequestURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "relicta-gomod-plugin/2.0.0")

	// Get HTTP client with configured timeout.
	timeout := time.Duration(cfg.Timeout) * time.Second
	client := getHTTPClient(timeout)

	// Send request.
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body for error messages.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Handle response status codes.
	switch resp.StatusCode {
	case http.StatusOK:
		// Success - module version is indexed.
		return nil
	case http.StatusNotFound:
		// 404 - module or version not found yet.
		// This can happen if the tag hasn't propagated to the origin.
		return fmt.Errorf("module or version not found (404): %s - the tag may need time to propagate", string(body))
	case http.StatusGone:
		// 410 - version doesn't exist or has been removed.
		return fmt.Errorf("version does not exist or is unavailable (410): %s", string(body))
	default:
		if resp.StatusCode >= 400 {
			return fmt.Errorf("proxy returned error status %d: %s", resp.StatusCode, string(body))
		}
		// Other 2xx/3xx status codes are acceptable.
		return nil
	}
}

// parseConfig parses the raw configuration into a Config struct.
func (p *GoModPlugin) parseConfig(raw map[string]any) *Config {
	parser := helpers.NewConfigParser(raw)

	proxyURL := parser.GetString("proxy_url", "", defaultProxyURL)
	if proxyURL == "" {
		proxyURL = defaultProxyURL
	}

	timeout := parser.GetInt("timeout", defaultTimeout)
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	return &Config{
		ModulePath: parser.GetString("module_path", "GO_MODULE_PATH", ""),
		ProxyURL:   proxyURL,
		Private:    parser.GetBool("private", false),
		Timeout:    timeout,
	}
}

// Validate validates the plugin configuration.
func (p *GoModPlugin) Validate(_ context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	vb := helpers.NewValidationBuilder()
	parser := helpers.NewConfigParser(config)

	// Validate module path.
	modulePath := parser.GetString("module_path", "GO_MODULE_PATH", "")
	if modulePath == "" {
		vb.AddError("module_path", "Go module path is required")
	} else if err := validateModulePath(modulePath); err != nil {
		vb.AddError("module_path", err.Error())
	}

	// Validate proxy URL if provided.
	proxyURL := parser.GetString("proxy_url", "", "")
	if proxyURL != "" {
		if err := validateProxyURL(proxyURL); err != nil {
			vb.AddError("proxy_url", err.Error())
		}
	}

	// Validate timeout if provided.
	if rawTimeout, ok := config["timeout"]; ok {
		switch t := rawTimeout.(type) {
		case int:
			if t <= 0 {
				vb.AddError("timeout", "timeout must be a positive integer")
			}
		case float64:
			if t <= 0 {
				vb.AddError("timeout", "timeout must be a positive integer")
			}
		case string:
			// Allow string conversion but warn about type.
		default:
			vb.AddError("timeout", "timeout must be an integer")
		}
	}

	return vb.Build(), nil
}
