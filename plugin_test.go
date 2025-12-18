// Package main provides tests for the GoMod plugin.
package main

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta-plugin-sdk/plugin"
)

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

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		config         map[string]any
		expectedValid  bool
		expectedErrors int
		errorField     string
	}{
		{
			name:           "valid config with module_path",
			config:         map[string]any{"module_path": "github.com/example/module"},
			expectedValid:  true,
			expectedErrors: 0,
			errorField:     "",
		},
		{
			name:           "missing module_path",
			config:         map[string]any{},
			expectedValid:  false,
			expectedErrors: 1,
			errorField:     "module_path",
		},
		{
			name:           "empty module_path",
			config:         map[string]any{"module_path": ""},
			expectedValid:  false,
			expectedErrors: 1,
			errorField:     "module_path",
		},
		{
			name:           "nil config",
			config:         nil,
			expectedValid:  false,
			expectedErrors: 1,
			errorField:     "module_path",
		},
		{
			name:           "module_path with private flag",
			config:         map[string]any{"module_path": "github.com/example/module", "private": true},
			expectedValid:  true,
			expectedErrors: 0,
			errorField:     "",
		},
		{
			name:           "module_path is not a string",
			config:         map[string]any{"module_path": 123},
			expectedValid:  false,
			expectedErrors: 1,
			errorField:     "module_path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := &GoModPlugin{}
			ctx := context.Background()

			resp, err := p.Validate(ctx, tt.config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.Valid != tt.expectedValid {
				t.Errorf("Valid: got %v, expected %v", resp.Valid, tt.expectedValid)
			}

			if len(resp.Errors) != tt.expectedErrors {
				t.Errorf("Errors count: got %d, expected %d", len(resp.Errors), tt.expectedErrors)
			}

			if tt.expectedErrors > 0 && len(resp.Errors) > 0 {
				if resp.Errors[0].Field != tt.errorField {
					t.Errorf("Error field: got %s, expected %s", resp.Errors[0].Field, tt.errorField)
				}
			}
		})
	}
}

func TestParseConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		config             map[string]any
		expectedModulePath string
		expectedPrivate    bool
	}{
		{
			name:               "full config",
			config:             map[string]any{"module_path": "github.com/example/module", "private": true},
			expectedModulePath: "github.com/example/module",
			expectedPrivate:    true,
		},
		{
			name:               "module_path only - private defaults to false",
			config:             map[string]any{"module_path": "github.com/example/module"},
			expectedModulePath: "github.com/example/module",
			expectedPrivate:    false,
		},
		{
			name:               "empty config - defaults applied",
			config:             map[string]any{},
			expectedModulePath: "",
			expectedPrivate:    false,
		},
		{
			name:               "nil config - defaults applied",
			config:             nil,
			expectedModulePath: "",
			expectedPrivate:    false,
		},
		{
			name:               "private as string true",
			config:             map[string]any{"module_path": "github.com/example/module", "private": "true"},
			expectedModulePath: "github.com/example/module",
			expectedPrivate:    true,
		},
		{
			name:               "private as string false",
			config:             map[string]any{"module_path": "github.com/example/module", "private": "false"},
			expectedModulePath: "github.com/example/module",
			expectedPrivate:    false,
		},
		{
			name:               "private explicitly false",
			config:             map[string]any{"module_path": "github.com/example/module", "private": false},
			expectedModulePath: "github.com/example/module",
			expectedPrivate:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := ParseConfig(tt.config)

			if cfg.ModulePath != tt.expectedModulePath {
				t.Errorf("ModulePath: got %s, expected %s", cfg.ModulePath, tt.expectedModulePath)
			}

			if cfg.Private != tt.expectedPrivate {
				t.Errorf("Private: got %v, expected %v", cfg.Private, tt.expectedPrivate)
			}
		})
	}
}

func TestExecuteDryRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		hook            plugin.Hook
		dryRun          bool
		expectedSuccess bool
		expectedMessage string
	}{
		{
			name:            "PostPublish with dry run",
			hook:            plugin.HookPostPublish,
			dryRun:          true,
			expectedSuccess: true,
			expectedMessage: "Would execute gomod plugin",
		},
		{
			name:            "PostPublish without dry run",
			hook:            plugin.HookPostPublish,
			dryRun:          false,
			expectedSuccess: true,
			expectedMessage: "GoMod plugin executed successfully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := &GoModPlugin{}
			ctx := context.Background()

			req := plugin.ExecuteRequest{
				Hook:   tt.hook,
				Config: map[string]any{"module_path": "github.com/example/module"},
				Context: plugin.ReleaseContext{
					Version: "1.0.0",
					TagName: "v1.0.0",
				},
				DryRun: tt.dryRun,
			}

			resp, err := p.Execute(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.Success != tt.expectedSuccess {
				t.Errorf("Success: got %v, expected %v", resp.Success, tt.expectedSuccess)
			}

			if resp.Message != tt.expectedMessage {
				t.Errorf("Message: got %q, expected %q", resp.Message, tt.expectedMessage)
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
