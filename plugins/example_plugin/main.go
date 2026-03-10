// Example plugin for Nxlang
// Build this plugin with: go build -buildmode=plugin -o example.nxp plugins/example_plugin/main.go
package main

import (
	"strings"

	"github.com/topxeq/nxlang/plugin"
	"github.com/topxeq/nxlang/types"
	"github.com/topxeq/nxlang/types/collections"
)

// ExamplePlugin implements the NxPlugin interface
type ExamplePlugin struct {
	initialized bool
}

// Info returns plugin metadata
func (p *ExamplePlugin) Info() plugin.PluginInfo {
	return plugin.PluginInfo{
		Name:        "example",
		Version:     "1.0.0",
		Description: "Example plugin for Nxlang demonstrating plugin capabilities",
		Author:      "Nxlang Team",
	}
}

// Initialize is called when the plugin is loaded
func (p *ExamplePlugin) Initialize() error {
	p.initialized = true
	return nil
}

// GetFunctions returns all functions exported by this plugin
func (p *ExamplePlugin) GetFunctions() []plugin.PluginFunction {
	return []plugin.PluginFunction{
		{
			Name: "example.hello",
			Fn: func(args ...types.Object) types.Object {
				if len(args) < 1 {
					return types.String("Hello from Example Plugin!")
				}
				name := types.ToString(args[0])
				return types.String("Hello, " + string(name) + "! From Example Plugin.")
			},
		},
		{
			Name: "example.upper",
			Fn: func(args ...types.Object) types.Object {
				if len(args) < 1 {
					return types.NewError("example.upper() expects at least 1 argument", 0, 0, "")
				}
				s := string(types.ToString(args[0]))
				return types.String(strings.ToUpper(s))
			},
		},
		{
			Name: "example.add",
			Fn: func(args ...types.Object) types.Object {
				if len(args) < 2 {
					return types.NewError("example.add() expects at least 2 arguments", 0, 0, "")
				}
				a, err := types.ToInt(args[0])
				if err != nil {
					return err
				}
				b, err := types.ToInt(args[1])
				if err != nil {
					return err
				}
				return types.Int(int(a) + int(b))
			},
		},
		{
			Name: "example.multiply",
			Fn: func(args ...types.Object) types.Object {
				if len(args) < 2 {
					return types.NewError("example.multiply() expects at least 2 arguments", 0, 0, "")
				}
				a, err := types.ToFloat(args[0])
				if err != nil {
					return err
				}
				b, err := types.ToFloat(args[1])
				if err != nil {
					return err
				}
				return types.Float(float64(a) * float64(b))
			},
		},
		{
			Name: "example.info",
			Fn: func(args ...types.Object) types.Object {
				info := p.Info()
				m := collections.NewMap()
				m.Set("name", types.String(info.Name))
				m.Set("version", types.String(info.Version))
				m.Set("description", types.String(info.Description))
				m.Set("author", types.String(info.Author))
				return m
			},
		},
	}
}

// Cleanup is called when the plugin is unloaded
func (p *ExamplePlugin) Cleanup() error {
	p.initialized = false
	return nil
}

// NxPlugin is the exported symbol that Nxlang looks for in plugins
var NxPlugin plugin.NxPlugin = &ExamplePlugin{}
