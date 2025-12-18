// Package main provides tests for the GoMod plugin.
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta-plugin-sdk/plugin"
)

// mockHTTPClient implements HTTPClient for testing.
type mockHTTPClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.DoFunc != nil {
		return m.DoFunc(req)
	}
	return nil, fmt.Errorf("mock not configured")
}

// mockResponse creates an HTTP response with the given status and body.
func mockResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestGetInfo(t *testing.T) {
	t.Parallel()

	p := &GoModPlugin{}
	info := p.GetInfo()

	tests := []struct {
		name     string
		got      any
		expected any
	}{
		{
			name:     "plugin name",
			got:      info.Name,
			expected: "gomod",
		},
		{
			name:     "plugin version",
			got:      info.Version,
			expected: "2.0.0",
		},
		{
			name:     "plugin description",
			got:      info.Description,
			expected: "Publish Go modules to proxy.golang.org",
		},
		{
			name:     "plugin author",
			got:      info.Author,
			expected: "Relicta Team",
		},
		{
			name:     "hooks count",
			got:      len(info.Hooks),
			expected: 1,
		},
		{
			name:     "first hook",
			got:      info.Hooks[0],
			expected: plugin.HookPostPublish,
		},
		{
			name:     "config schema is not empty",
			got:      len(info.ConfigSchema) > 0,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("got %v, expected %v", tt.got, tt.expected)
			}
		})
	}
}

func TestValidateModulePath(t *testing.T) {
	tests := []struct {
		name        string
		modulePath  string
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid github module path",
			modulePath: "github.com/user/repo",
			wantErr:    false,
		},
		{
			name:       "valid github module path with nested package",
			modulePath: "github.com/user/repo/pkg/subpkg",
			wantErr:    false,
		},
		{
			name:       "valid go.dev module path",
			modulePath: "golang.org/x/tools",
			wantErr:    false,
		},
		{
			name:       "valid custom domain",
			modulePath: "example.com/myproject",
			wantErr:    false,
		},
		{
			name:       "valid module with dashes",
			modulePath: "github.com/my-org/my-repo",
			wantErr:    false,
		},
		{
			name:       "valid module with underscores",
			modulePath: "github.com/my_org/my_repo",
			wantErr:    false,
		},
		{
			name:       "valid module with numbers",
			modulePath: "github.com/user123/repo456",
			wantErr:    false,
		},
		{
			name:       "valid gitlab module",
			modulePath: "gitlab.com/company/project",
			wantErr:    false,
		},
		{
			name:       "valid bitbucket module",
			modulePath: "bitbucket.org/team/repo",
			wantErr:    false,
		},
		{
			name:        "empty module path",
			modulePath:  "",
			wantErr:     true,
			errContains: "cannot be empty",
		},
		{
			name:        "path traversal attempt with ..",
			modulePath:  "../github.com/user/repo",
			wantErr:     true,
			errContains: "cannot contain '..'",
		},
		{
			name:        "path traversal in middle",
			modulePath:  "github.com/../evil/repo",
			wantErr:     true,
			errContains: "cannot contain '..'",
		},
		{
			name:        "leading slash",
			modulePath:  "/github.com/user/repo",
			wantErr:     true,
			errContains: "cannot start with '/'",
		},
		{
			name:        "double slashes",
			modulePath:  "github.com//user/repo",
			wantErr:     true,
			errContains: "cannot contain '//'",
		},
		{
			name:        "no domain",
			modulePath:  "user/repo",
			wantErr:     true,
			errContains: "invalid module path format",
		},
		{
			name:        "no path after domain",
			modulePath:  "github.com",
			wantErr:     true,
			errContains: "invalid module path format",
		},
		{
			name:        "just a word",
			modulePath:  "mymodule",
			wantErr:     true,
			errContains: "invalid module path format",
		},
		{
			name:        "module path too long",
			modulePath:  "github.com/" + strings.Repeat("a", 500),
			wantErr:     true,
			errContains: "too long",
		},
		{
			name:        "special characters not allowed",
			modulePath:  "github.com/user@evil/repo",
			wantErr:     true,
			errContains: "invalid module path format",
		},
		{
			name:        "spaces not allowed",
			modulePath:  "github.com/user name/repo",
			wantErr:     true,
			errContains: "invalid module path format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateModulePath(tt.modulePath)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errContains)
				} else if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got '%s'", err.Error())
				}
			}
		})
	}
}

func TestValidateProxyURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid proxy.golang.org",
			url:     "https://proxy.golang.org",
			wantErr: false,
		},
		{
			name:    "valid proxy.golang.org with path",
			url:     "https://proxy.golang.org/github.com/user/repo/@v/v1.0.0.info",
			wantErr: false,
		},
		{
			name:    "valid goproxy.io",
			url:     "https://goproxy.io",
			wantErr: false,
		},
		{
			name:    "valid goproxy.cn",
			url:     "https://goproxy.cn",
			wantErr: false,
		},
		{
			name:    "valid athens proxy",
			url:     "https://athens.example.com",
			wantErr: false,
		},
		{
			name:        "HTTP not allowed",
			url:         "http://proxy.golang.org",
			wantErr:     true,
			errContains: "must use HTTPS",
		},
		{
			name:        "localhost not allowed",
			url:         "https://localhost:8080",
			wantErr:     true,
			errContains: "cannot be localhost",
		},
		{
			name:        "127.0.0.1 not allowed",
			url:         "https://127.0.0.1:8080",
			wantErr:     true,
			errContains: "cannot be localhost",
		},
		{
			name:        "IPv6 localhost not allowed",
			url:         "https://[::1]:8080",
			wantErr:     true,
			errContains: "cannot be localhost",
		},
		{
			name:        "private IP 10.x not allowed",
			url:         "https://10.0.0.1",
			wantErr:     true,
			errContains: "private network",
		},
		{
			name:        "private IP 192.168.x not allowed",
			url:         "https://192.168.1.1",
			wantErr:     true,
			errContains: "private network",
		},
		{
			name:        "private IP 172.x not allowed",
			url:         "https://172.16.0.1",
			wantErr:     true,
			errContains: "private network",
		},
		{
			name:        ".local domain not allowed",
			url:         "https://myproxy.local",
			wantErr:     true,
			errContains: "private network",
		},
		{
			name:        ".internal domain not allowed",
			url:         "https://proxy.internal",
			wantErr:     true,
			errContains: "private network",
		},
		{
			name:        "empty scheme",
			url:         "proxy.golang.org",
			wantErr:     true,
			errContains: "must use HTTPS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProxyURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errContains)
				} else if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got '%s'", err.Error())
				}
			}
		})
	}
}

func TestValidate(t *testing.T) {
	p := &GoModPlugin{}
	ctx := context.Background()

	tests := []struct {
		name      string
		config    map[string]any
		envVars   map[string]string
		wantValid bool
		wantField string
	}{
		{
			name:      "missing module_path",
			config:    map[string]any{},
			wantValid: false,
			wantField: "module_path",
		},
		{
			name: "empty module_path",
			config: map[string]any{
				"module_path": "",
			},
			wantValid: false,
			wantField: "module_path",
		},
		{
			name: "invalid module_path format",
			config: map[string]any{
				"module_path": "invalid-no-domain",
			},
			wantValid: false,
			wantField: "module_path",
		},
		{
			name: "valid config with module_path only",
			config: map[string]any{
				"module_path": "github.com/example/module",
			},
			wantValid: true,
		},
		{
			name: "valid config with all options",
			config: map[string]any{
				"module_path": "github.com/example/module",
				"proxy_url":   "https://proxy.golang.org",
				"private":     false,
				"timeout":     60,
			},
			wantValid: true,
		},
		{
			name: "valid config with private flag",
			config: map[string]any{
				"module_path": "github.com/example/module",
				"private":     true,
			},
			wantValid: true,
		},
		{
			name: "invalid proxy URL - HTTP",
			config: map[string]any{
				"module_path": "github.com/example/module",
				"proxy_url":   "http://proxy.golang.org",
			},
			wantValid: false,
			wantField: "proxy_url",
		},
		{
			name: "invalid proxy URL - localhost",
			config: map[string]any{
				"module_path": "github.com/example/module",
				"proxy_url":   "https://localhost:8080",
			},
			wantValid: false,
			wantField: "proxy_url",
		},
		{
			name: "invalid timeout - negative",
			config: map[string]any{
				"module_path": "github.com/example/module",
				"timeout":     -1,
			},
			wantValid: false,
			wantField: "timeout",
		},
		{
			name: "invalid timeout - zero",
			config: map[string]any{
				"module_path": "github.com/example/module",
				"timeout":     0,
			},
			wantValid: false,
			wantField: "timeout",
		},
		{
			name:   "module_path from env var",
			config: map[string]any{},
			envVars: map[string]string{
				"GO_MODULE_PATH": "github.com/example/module",
			},
			wantValid: true,
		},
		{
			name:      "nil config",
			config:    nil,
			wantValid: false,
			wantField: "module_path",
		},
		{
			name: "module_path is not a string",
			config: map[string]any{
				"module_path": 123,
			},
			wantValid: false,
			wantField: "module_path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear any existing env vars.
			_ = os.Unsetenv("GO_MODULE_PATH")

			// Set env vars for this test.
			for k, v := range tt.envVars {
				_ = os.Setenv(k, v)
				defer func(key string) { _ = os.Unsetenv(key) }(k)
			}

			resp, err := p.Validate(ctx, tt.config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.Valid != tt.wantValid {
				t.Errorf("expected valid=%v, got valid=%v, errors=%v", tt.wantValid, resp.Valid, resp.Errors)
			}

			if !tt.wantValid && tt.wantField != "" && len(resp.Errors) > 0 {
				found := false
				for _, e := range resp.Errors {
					if e.Field == tt.wantField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error on field '%s', got errors: %v", tt.wantField, resp.Errors)
				}
			}
		})
	}
}

func TestParseConfig(t *testing.T) {
	p := &GoModPlugin{}

	tests := []struct {
		name            string
		config          map[string]any
		envVars         map[string]string
		expectedModule  string
		expectedProxy   string
		expectedPrivate bool
		expectedTimeout int
	}{
		{
			name:            "defaults",
			config:          map[string]any{},
			expectedModule:  "",
			expectedProxy:   defaultProxyURL,
			expectedPrivate: false,
			expectedTimeout: defaultTimeout,
		},
		{
			name: "custom values",
			config: map[string]any{
				"module_path": "github.com/example/module",
				"proxy_url":   "https://goproxy.io",
				"private":     true,
				"timeout":     60,
			},
			expectedModule:  "github.com/example/module",
			expectedProxy:   "https://goproxy.io",
			expectedPrivate: true,
			expectedTimeout: 60,
		},
		{
			name:   "env var fallback for module_path",
			config: map[string]any{},
			envVars: map[string]string{
				"GO_MODULE_PATH": "github.com/env/module",
			},
			expectedModule:  "github.com/env/module",
			expectedProxy:   defaultProxyURL,
			expectedPrivate: false,
			expectedTimeout: defaultTimeout,
		},
		{
			name: "config overrides env var",
			config: map[string]any{
				"module_path": "github.com/config/module",
			},
			envVars: map[string]string{
				"GO_MODULE_PATH": "github.com/env/module",
			},
			expectedModule:  "github.com/config/module",
			expectedProxy:   defaultProxyURL,
			expectedPrivate: false,
			expectedTimeout: defaultTimeout,
		},
		{
			name: "empty proxy_url uses default",
			config: map[string]any{
				"module_path": "github.com/example/module",
				"proxy_url":   "",
			},
			expectedModule:  "github.com/example/module",
			expectedProxy:   defaultProxyURL,
			expectedPrivate: false,
			expectedTimeout: defaultTimeout,
		},
		{
			name: "invalid timeout uses default",
			config: map[string]any{
				"module_path": "github.com/example/module",
				"timeout":     -1,
			},
			expectedModule:  "github.com/example/module",
			expectedProxy:   defaultProxyURL,
			expectedPrivate: false,
			expectedTimeout: defaultTimeout,
		},
		{
			name: "zero timeout uses default",
			config: map[string]any{
				"module_path": "github.com/example/module",
				"timeout":     0,
			},
			expectedModule:  "github.com/example/module",
			expectedProxy:   defaultProxyURL,
			expectedPrivate: false,
			expectedTimeout: defaultTimeout,
		},
		{
			name: "private as string true",
			config: map[string]any{
				"module_path": "github.com/example/module",
				"private":     "true",
			},
			expectedModule:  "github.com/example/module",
			expectedProxy:   defaultProxyURL,
			expectedPrivate: true,
			expectedTimeout: defaultTimeout,
		},
		{
			name: "private as string false",
			config: map[string]any{
				"module_path": "github.com/example/module",
				"private":     "false",
			},
			expectedModule:  "github.com/example/module",
			expectedProxy:   defaultProxyURL,
			expectedPrivate: false,
			expectedTimeout: defaultTimeout,
		},
		{
			name:            "nil config uses all defaults",
			config:          nil,
			expectedModule:  "",
			expectedProxy:   defaultProxyURL,
			expectedPrivate: false,
			expectedTimeout: defaultTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear any existing env vars.
			_ = os.Unsetenv("GO_MODULE_PATH")

			// Set env vars for this test.
			for k, v := range tt.envVars {
				_ = os.Setenv(k, v)
				defer func(key string) { _ = os.Unsetenv(key) }(k)
			}

			cfg := p.parseConfig(tt.config)

			if cfg.ModulePath != tt.expectedModule {
				t.Errorf("ModulePath: expected '%s', got '%s'", tt.expectedModule, cfg.ModulePath)
			}
			if cfg.ProxyURL != tt.expectedProxy {
				t.Errorf("ProxyURL: expected '%s', got '%s'", tt.expectedProxy, cfg.ProxyURL)
			}
			if cfg.Private != tt.expectedPrivate {
				t.Errorf("Private: expected %v, got %v", tt.expectedPrivate, cfg.Private)
			}
			if cfg.Timeout != tt.expectedTimeout {
				t.Errorf("Timeout: expected %d, got %d", tt.expectedTimeout, cfg.Timeout)
			}
		})
	}
}

func TestExecuteDryRun(t *testing.T) {
	p := &GoModPlugin{}
	ctx := context.Background()

	tests := []struct {
		name            string
		config          map[string]any
		releaseCtx      plugin.ReleaseContext
		expectedSuccess bool
		expectedModule  string
		expectedVersion string
	}{
		{
			name: "basic dry run",
			config: map[string]any{
				"module_path": "github.com/example/module",
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "1.0.0",
			},
			expectedSuccess: true,
			expectedModule:  "github.com/example/module",
			expectedVersion: "v1.0.0",
		},
		{
			name: "dry run with v prefix",
			config: map[string]any{
				"module_path": "github.com/example/module",
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "v2.0.0",
			},
			expectedSuccess: true,
			expectedModule:  "github.com/example/module",
			expectedVersion: "v2.0.0",
		},
		{
			name: "dry run uses tag_name when version empty",
			config: map[string]any{
				"module_path": "github.com/example/module",
			},
			releaseCtx: plugin.ReleaseContext{
				TagName: "v3.0.0",
			},
			expectedSuccess: true,
			expectedModule:  "github.com/example/module",
			expectedVersion: "v3.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := plugin.ExecuteRequest{
				Hook:    plugin.HookPostPublish,
				Config:  tt.config,
				Context: tt.releaseCtx,
				DryRun:  true,
			}

			resp, err := p.Execute(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.Success != tt.expectedSuccess {
				t.Errorf("Success: expected %v, got %v, error: %s", tt.expectedSuccess, resp.Success, resp.Error)
			}

			if !strings.Contains(resp.Message, "Would notify") {
				t.Errorf("expected dry run message, got: %s", resp.Message)
			}

			if resp.Outputs == nil {
				t.Fatal("expected outputs to be set")
			}

			if resp.Outputs["module_path"] != tt.expectedModule {
				t.Errorf("module_path: expected '%s', got '%v'", tt.expectedModule, resp.Outputs["module_path"])
			}

			if resp.Outputs["version"] != tt.expectedVersion {
				t.Errorf("version: expected '%s', got '%v'", tt.expectedVersion, resp.Outputs["version"])
			}
		})
	}
}

func TestExecutePrivateModule(t *testing.T) {
	p := &GoModPlugin{}
	ctx := context.Background()

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPostPublish,
		Config: map[string]any{
			"module_path": "github.com/example/private-module",
			"private":     true,
		},
		Context: plugin.ReleaseContext{
			Version: "v1.0.0",
		},
		DryRun: false,
	}

	resp, err := p.Execute(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	if !strings.Contains(resp.Message, "Skipping proxy notification") {
		t.Errorf("expected skip message for private module, got: %s", resp.Message)
	}

	// Check outputs.
	if resp.Outputs == nil {
		t.Fatal("expected outputs to be set")
	}

	if resp.Outputs["private"] != true {
		t.Error("expected private=true in outputs")
	}

	if resp.Outputs["skipped"] != true {
		t.Error("expected skipped=true in outputs")
	}
}

func TestExecuteInvalidModulePath(t *testing.T) {
	p := &GoModPlugin{}
	ctx := context.Background()

	tests := []struct {
		name        string
		modulePath  string
		errContains string
	}{
		{
			name:        "empty module path",
			modulePath:  "",
			errContains: "cannot be empty",
		},
		{
			name:        "path traversal",
			modulePath:  "../github.com/evil/repo",
			errContains: "cannot contain '..'",
		},
		{
			name:        "leading slash",
			modulePath:  "/github.com/user/repo",
			errContains: "cannot start with '/'",
		},
		{
			name:        "invalid format",
			modulePath:  "notamodule",
			errContains: "invalid module path format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := plugin.ExecuteRequest{
				Hook: plugin.HookPostPublish,
				Config: map[string]any{
					"module_path": tt.modulePath,
				},
				Context: plugin.ReleaseContext{Version: "v1.0.0"},
				DryRun:  false,
			}

			resp, err := p.Execute(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.Success {
				t.Error("expected failure due to invalid module path")
			}

			if !strings.Contains(resp.Error, tt.errContains) {
				t.Errorf("expected error containing '%s', got: %s", tt.errContains, resp.Error)
			}
		})
	}
}

func TestExecuteInvalidProxyURL(t *testing.T) {
	p := &GoModPlugin{}
	ctx := context.Background()

	tests := []struct {
		name        string
		proxyURL    string
		errContains string
	}{
		{
			name:        "HTTP proxy",
			proxyURL:    "http://proxy.golang.org",
			errContains: "must use HTTPS",
		},
		{
			name:        "localhost proxy",
			proxyURL:    "https://localhost:8080",
			errContains: "cannot be localhost",
		},
		{
			name:        "private IP proxy",
			proxyURL:    "https://192.168.1.1:8080",
			errContains: "private network",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := plugin.ExecuteRequest{
				Hook: plugin.HookPostPublish,
				Config: map[string]any{
					"module_path": "github.com/example/module",
					"proxy_url":   tt.proxyURL,
				},
				Context: plugin.ReleaseContext{Version: "v1.0.0"},
				DryRun:  false,
			}

			resp, err := p.Execute(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.Success {
				t.Error("expected failure due to invalid proxy URL")
			}

			if !strings.Contains(resp.Error, tt.errContains) {
				t.Errorf("expected error containing '%s', got: %s", tt.errContains, resp.Error)
			}
		})
	}
}

func TestExecuteMissingVersion(t *testing.T) {
	p := &GoModPlugin{}
	ctx := context.Background()

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPostPublish,
		Config: map[string]any{
			"module_path": "github.com/example/module",
		},
		Context: plugin.ReleaseContext{
			// No version or tag_name set.
		},
		DryRun: false,
	}

	resp, err := p.Execute(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Success {
		t.Error("expected failure due to missing version")
	}

	if !strings.Contains(resp.Error, "version is required") {
		t.Errorf("expected version required error, got: %s", resp.Error)
	}
}

func TestExecuteHTTPSuccess(t *testing.T) {
	// Store original client and restore after test.
	originalClient := httpClient
	defer func() { httpClient = originalClient }()

	// Set up mock HTTP client.
	var capturedRequest *http.Request
	httpClient = &mockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			capturedRequest = req
			// Return successful response with version info JSON.
			return mockResponse(http.StatusOK, `{"Version":"v1.2.3","Time":"2024-01-01T00:00:00Z"}`), nil
		},
	}

	p := &GoModPlugin{}
	ctx := context.Background()

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPostPublish,
		Config: map[string]any{
			"module_path": "github.com/example/module",
		},
		Context: plugin.ReleaseContext{
			Version: "v1.2.3",
		},
		DryRun: false,
	}

	resp, err := p.Execute(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	if !strings.Contains(resp.Message, "proxy notified") {
		t.Errorf("expected success message, got: %s", resp.Message)
	}

	// Verify HTTP request details.
	if capturedRequest == nil {
		t.Fatal("expected HTTP request to be captured")
	}

	if capturedRequest.Method != http.MethodGet {
		t.Errorf("expected GET method, got: %s", capturedRequest.Method)
	}

	expectedURLPrefix := "https://proxy.golang.org/github.com/example/module/@v/v1.2.3.info"
	if capturedRequest.URL.String() != expectedURLPrefix {
		t.Errorf("expected URL '%s', got: %s", expectedURLPrefix, capturedRequest.URL.String())
	}

	if capturedRequest.Header.Get("User-Agent") != "relicta-gomod-plugin/2.0.0" {
		t.Errorf("expected User-Agent header, got: %s", capturedRequest.Header.Get("User-Agent"))
	}

	// Verify outputs.
	if resp.Outputs == nil {
		t.Fatal("expected outputs to be set")
	}

	if resp.Outputs["version"] != "v1.2.3" {
		t.Errorf("expected version 'v1.2.3', got '%v'", resp.Outputs["version"])
	}
}

func TestExecuteHTTPErrors(t *testing.T) {
	// Store original client and restore after test.
	originalClient := httpClient
	defer func() { httpClient = originalClient }()

	tests := []struct {
		name        string
		mockFunc    func(req *http.Request) (*http.Response, error)
		errContains string
	}{
		{
			name: "network error",
			mockFunc: func(req *http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("network connection refused")
			},
			errContains: "failed to send request",
		},
		{
			name: "404 not found",
			mockFunc: func(req *http.Request) (*http.Response, error) {
				return mockResponse(http.StatusNotFound, "not found"), nil
			},
			errContains: "not found (404)",
		},
		{
			name: "410 gone",
			mockFunc: func(req *http.Request) (*http.Response, error) {
				return mockResponse(http.StatusGone, "version removed"), nil
			},
			errContains: "unavailable (410)",
		},
		{
			name: "500 server error",
			mockFunc: func(req *http.Request) (*http.Response, error) {
				return mockResponse(http.StatusInternalServerError, "internal server error"), nil
			},
			errContains: "status 500",
		},
		{
			name: "502 bad gateway",
			mockFunc: func(req *http.Request) (*http.Response, error) {
				return mockResponse(http.StatusBadGateway, "bad gateway"), nil
			},
			errContains: "status 502",
		},
		{
			name: "503 service unavailable",
			mockFunc: func(req *http.Request) (*http.Response, error) {
				return mockResponse(http.StatusServiceUnavailable, "service unavailable"), nil
			},
			errContains: "status 503",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpClient = &mockHTTPClient{DoFunc: tt.mockFunc}

			p := &GoModPlugin{}
			ctx := context.Background()

			req := plugin.ExecuteRequest{
				Hook: plugin.HookPostPublish,
				Config: map[string]any{
					"module_path": "github.com/example/module",
				},
				Context: plugin.ReleaseContext{Version: "v1.0.0"},
				DryRun:  false,
			}

			resp, err := p.Execute(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.Success {
				t.Error("expected failure due to HTTP error")
			}

			if !strings.Contains(resp.Error, tt.errContains) {
				t.Errorf("expected error containing '%s', got: %s", tt.errContains, resp.Error)
			}
		})
	}
}

func TestExecuteHTTPSuccessStatusCodes(t *testing.T) {
	// Store original client and restore after test.
	originalClient := httpClient
	defer func() { httpClient = originalClient }()

	// Various 2xx status codes should be treated as success.
	successCodes := []int{200, 201, 202}

	for _, statusCode := range successCodes {
		t.Run(fmt.Sprintf("status_%d", statusCode), func(t *testing.T) {
			httpClient = &mockHTTPClient{
				DoFunc: func(req *http.Request) (*http.Response, error) {
					return mockResponse(statusCode, `{}`), nil
				},
			}

			p := &GoModPlugin{}
			ctx := context.Background()

			req := plugin.ExecuteRequest{
				Hook: plugin.HookPostPublish,
				Config: map[string]any{
					"module_path": "github.com/example/module",
				},
				Context: plugin.ReleaseContext{Version: "v1.0.0"},
				DryRun:  false,
			}

			resp, err := p.Execute(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !resp.Success {
				t.Errorf("expected success for status %d, got error: %s", statusCode, resp.Error)
			}
		})
	}
}

func TestExecuteUnhandledHook(t *testing.T) {
	t.Parallel()

	unhandledHooks := []plugin.Hook{
		plugin.HookPreInit,
		plugin.HookPostInit,
		plugin.HookPrePlan,
		plugin.HookPostPlan,
		plugin.HookPreVersion,
		plugin.HookPostVersion,
		plugin.HookPreNotes,
		plugin.HookPostNotes,
		plugin.HookPreApprove,
		plugin.HookPostApprove,
		plugin.HookPrePublish,
		plugin.HookOnSuccess,
		plugin.HookOnError,
	}

	for _, hook := range unhandledHooks {
		t.Run(string(hook), func(t *testing.T) {
			t.Parallel()

			p := &GoModPlugin{}
			ctx := context.Background()

			req := plugin.ExecuteRequest{
				Hook:   hook,
				Config: map[string]any{"module_path": "github.com/example/module"},
				Context: plugin.ReleaseContext{
					Version: "1.0.0",
					TagName: "v1.0.0",
				},
				DryRun: false,
			}

			resp, err := p.Execute(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !resp.Success {
				t.Errorf("Success: got %v, expected true", resp.Success)
			}

			expectedMessage := "Hook " + string(hook) + " not handled"
			if resp.Message != expectedMessage {
				t.Errorf("Message: got %q, expected %q", resp.Message, expectedMessage)
			}
		})
	}
}

func TestTriggerProxyIndexRequestFormat(t *testing.T) {
	// Store original client and restore after test.
	originalClient := httpClient
	defer func() { httpClient = originalClient }()

	var capturedRequest *http.Request
	httpClient = &mockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			capturedRequest = req
			return mockResponse(http.StatusOK, `{}`), nil
		},
	}

	p := &GoModPlugin{}
	ctx := context.Background()

	cfg := &Config{
		ModulePath: "github.com/user/repo",
		ProxyURL:   "https://proxy.golang.org",
		Timeout:    30,
	}

	err := p.triggerProxyIndex(ctx, cfg, "v1.2.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify request URL format.
	expectedURL := "https://proxy.golang.org/github.com/user/repo/@v/v1.2.3.info"
	if capturedRequest.URL.String() != expectedURL {
		t.Errorf("expected URL '%s', got: %s", expectedURL, capturedRequest.URL.String())
	}

	// Verify method.
	if capturedRequest.Method != http.MethodGet {
		t.Errorf("expected GET method, got: %s", capturedRequest.Method)
	}

	// Verify User-Agent header.
	if capturedRequest.Header.Get("User-Agent") != "relicta-gomod-plugin/2.0.0" {
		t.Errorf("expected User-Agent 'relicta-gomod-plugin/2.0.0', got: %s", capturedRequest.Header.Get("User-Agent"))
	}
}

func TestTriggerProxyIndexWithCustomProxy(t *testing.T) {
	// Store original client and restore after test.
	originalClient := httpClient
	defer func() { httpClient = originalClient }()

	var capturedURL string
	httpClient = &mockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			capturedURL = req.URL.String()
			return mockResponse(http.StatusOK, `{}`), nil
		},
	}

	p := &GoModPlugin{}
	ctx := context.Background()

	cfg := &Config{
		ModulePath: "github.com/user/repo",
		ProxyURL:   "https://goproxy.io",
		Timeout:    30,
	}

	err := p.triggerProxyIndex(ctx, cfg, "v2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedURL := "https://goproxy.io/github.com/user/repo/@v/v2.0.0.info"
	if capturedURL != expectedURL {
		t.Errorf("expected URL '%s', got: %s", expectedURL, capturedURL)
	}
}

func TestTriggerProxyIndexWithTrailingSlash(t *testing.T) {
	// Store original client and restore after test.
	originalClient := httpClient
	defer func() { httpClient = originalClient }()

	var capturedURL string
	httpClient = &mockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			capturedURL = req.URL.String()
			return mockResponse(http.StatusOK, `{}`), nil
		},
	}

	p := &GoModPlugin{}
	ctx := context.Background()

	cfg := &Config{
		ModulePath: "github.com/user/repo",
		ProxyURL:   "https://proxy.golang.org/", // Trailing slash
		Timeout:    30,
	}

	err := p.triggerProxyIndex(ctx, cfg, "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Trailing slash should be normalized.
	expectedURL := "https://proxy.golang.org/github.com/user/repo/@v/v1.0.0.info"
	if capturedURL != expectedURL {
		t.Errorf("expected URL '%s', got: %s", expectedURL, capturedURL)
	}
}

func TestGetHTTPClientDefault(t *testing.T) {
	// Store original client and restore after test.
	originalClient := httpClient
	defer func() { httpClient = originalClient }()

	// Reset to nil to test default behavior.
	httpClient = nil

	client := getHTTPClient(30 * 1000000000) // 30 seconds in nanoseconds
	if client == nil {
		t.Error("expected non-nil HTTP client")
	}

	// Should return a new client, not nil.
	defaultClient, ok := client.(*http.Client)
	if !ok {
		t.Error("expected *http.Client type")
	}

	if defaultClient.Timeout == 0 {
		t.Error("expected timeout to be set")
	}
}

func TestGetHTTPClientCustom(t *testing.T) {
	// Store original client and restore after test.
	originalClient := httpClient
	defer func() { httpClient = originalClient }()

	// Set a custom client.
	customClient := &mockHTTPClient{}
	httpClient = customClient

	client := getHTTPClient(30 * 1000000000)
	if client != customClient {
		t.Error("expected custom HTTP client to be returned")
	}
}

func TestCreateDefaultHTTPClientConfig(t *testing.T) {
	client := createDefaultHTTPClient(30 * 1000000000)

	// Verify timeout is set.
	if client.Timeout == 0 {
		t.Error("expected timeout to be set on HTTP client")
	}

	// Verify transport is set.
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected transport to be *http.Transport")
	}

	// Verify TLS config.
	if transport.TLSClientConfig == nil {
		t.Error("expected TLS config to be set")
	}

	// Verify minimum TLS version.
	if transport.TLSClientConfig.MinVersion < tls.VersionTLS13 {
		t.Error("expected minimum TLS version to be TLS 1.3")
	}

	// Verify connection pool settings.
	if transport.MaxIdleConns == 0 {
		t.Error("expected MaxIdleConns to be set")
	}
	if transport.MaxIdleConnsPerHost == 0 {
		t.Error("expected MaxIdleConnsPerHost to be set")
	}
}

func TestVersionPrefixNormalization(t *testing.T) {
	// Store original client and restore after test.
	originalClient := httpClient
	defer func() { httpClient = originalClient }()

	var capturedURL string
	httpClient = &mockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			capturedURL = req.URL.String()
			return mockResponse(http.StatusOK, `{}`), nil
		},
	}

	p := &GoModPlugin{}
	ctx := context.Background()

	tests := []struct {
		name            string
		inputVersion    string
		expectedVersion string
	}{
		{
			name:            "version without v prefix",
			inputVersion:    "1.0.0",
			expectedVersion: "v1.0.0",
		},
		{
			name:            "version with v prefix",
			inputVersion:    "v2.0.0",
			expectedVersion: "v2.0.0",
		},
		{
			name:            "prerelease version without v",
			inputVersion:    "1.0.0-beta.1",
			expectedVersion: "v1.0.0-beta.1",
		},
		{
			name:            "prerelease version with v",
			inputVersion:    "v1.0.0-rc.1",
			expectedVersion: "v1.0.0-rc.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := plugin.ExecuteRequest{
				Hook: plugin.HookPostPublish,
				Config: map[string]any{
					"module_path": "github.com/example/module",
				},
				Context: plugin.ReleaseContext{
					Version: tt.inputVersion,
				},
				DryRun: false,
			}

			resp, err := p.Execute(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !resp.Success {
				t.Errorf("expected success, got error: %s", resp.Error)
			}

			expectedURLSuffix := fmt.Sprintf("/@v/%s.info", tt.expectedVersion)
			if !strings.HasSuffix(capturedURL, expectedURLSuffix) {
				t.Errorf("expected URL to end with '%s', got: %s", expectedURLSuffix, capturedURL)
			}
		})
	}
}

func TestExecuteWithNestedModulePath(t *testing.T) {
	// Store original client and restore after test.
	originalClient := httpClient
	defer func() { httpClient = originalClient }()

	var capturedURL string
	httpClient = &mockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			capturedURL = req.URL.String()
			return mockResponse(http.StatusOK, `{}`), nil
		},
	}

	p := &GoModPlugin{}
	ctx := context.Background()

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPostPublish,
		Config: map[string]any{
			"module_path": "github.com/user/repo/pkg/subpackage",
		},
		Context: plugin.ReleaseContext{
			Version: "v1.0.0",
		},
		DryRun: false,
	}

	resp, err := p.Execute(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	expectedURL := "https://proxy.golang.org/github.com/user/repo/pkg/subpackage/@v/v1.0.0.info"
	if capturedURL != expectedURL {
		t.Errorf("expected URL '%s', got: %s", expectedURL, capturedURL)
	}
}

func TestSSRFProtectionInRedirect(t *testing.T) {
	// Test that the default HTTP client blocks redirects to non-HTTPS.
	client := createDefaultHTTPClient(30 * 1000000000)

	// Mock a redirect scenario by checking the CheckRedirect function.
	if client.CheckRedirect == nil {
		t.Fatal("expected CheckRedirect to be set")
	}

	// Test redirect to HTTP (should fail).
	httpReq, _ := http.NewRequest("GET", "http://evil.com", nil)
	err := client.CheckRedirect(httpReq, []*http.Request{{}, {}, {}})
	if err == nil {
		t.Error("expected error for too many redirects")
	}

	// Test redirect to HTTP scheme (should fail).
	httpReq, _ = http.NewRequest("GET", "http://evil.com", nil)
	err = client.CheckRedirect(httpReq, []*http.Request{{}})
	if err == nil || !strings.Contains(err.Error(), "non-HTTPS") {
		t.Errorf("expected non-HTTPS error, got: %v", err)
	}

	// Test redirect to HTTPS (should pass).
	httpsReq, _ := http.NewRequest("GET", "https://proxy.golang.org", nil)
	err = client.CheckRedirect(httpsReq, []*http.Request{{}})
	if err != nil {
		t.Errorf("expected HTTPS redirect to be allowed, got: %v", err)
	}
}
