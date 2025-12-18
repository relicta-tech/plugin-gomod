// Package main implements the GoMod plugin for Relicta.
package main

import (
	"context"
	"fmt"

	"github.com/relicta-tech/relicta-plugin-sdk/helpers"
	"github.com/relicta-tech/relicta-plugin-sdk/plugin"
)

// GoModPlugin implements the Publish Go modules to proxy.golang.org plugin.
type GoModPlugin struct{}

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
			"properties": {}
		}`,
	}
}

// Execute runs the plugin for a given hook.
func (p *GoModPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
	switch req.Hook {
	case plugin.HookPostPublish:
		if req.DryRun {
			return &plugin.ExecuteResponse{
				Success: true,
				Message: "Would execute gomod plugin",
			}, nil
		}
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "GoMod plugin executed successfully",
		}, nil
	default:
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Hook %s not handled", req.Hook),
		}, nil
	}
}

// Config holds the plugin configuration.
type Config struct {
	ModulePath string `json:"module_path"`
	Private    bool   `json:"private"`
}

// ParseConfig parses the plugin configuration with defaults.
func ParseConfig(config map[string]any) Config {
	cp := helpers.NewConfigParser(config)
	return Config{
		ModulePath: cp.GetString("module_path", "", ""),
		Private:    cp.GetBool("private", false),
	}
}

// Validate validates the plugin configuration.
func (p *GoModPlugin) Validate(_ context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	vb := helpers.NewValidationBuilder()
	vb.RequireString(config, "module_path", "")
	return vb.Build(), nil
}
