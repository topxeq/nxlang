// Package pluginhelper provides helper functions for creating Nxlang plugins
// Plugin developers can use this package to simplify plugin development
package pluginhelper

import (
	"github.com/topxeq/nxlang/plugin"
	"github.com/topxeq/nxlang/types"
)

// PluginBase is a helper struct for creating plugins
type PluginBase struct {
	info      plugin.PluginInfo
	functions []plugin.PluginFunction
}

// NewPluginBase creates a new plugin base with the given info
func NewPluginBase(name, version, description, author string) *PluginBase {
	return &PluginBase{
		info: plugin.PluginInfo{
			Name:        name,
			Version:     version,
			Description: description,
			Author:      author,
		},
		functions: make([]plugin.PluginFunction, 0),
	}
}

// Info returns the plugin info
func (pb *PluginBase) Info() plugin.PluginInfo {
	return pb.info
}

// AddFunction adds a function to the plugin
func (pb *PluginBase) AddFunction(name string, fn func(args ...types.Object) types.Object) {
	pb.functions = append(pb.functions, plugin.PluginFunction{
		Name: name,
		Fn:   fn,
	})
}

// AddIntFunction adds a function that takes int arguments and returns an int
func (pb *PluginBase) AddIntFunction(name string, fn func(args ...int) int) {
	wrapper := func(args ...types.Object) types.Object {
		intArgs := make([]int, len(args))
		for i, arg := range args {
			if v, err := types.ToInt(arg); err != nil {
				return types.NewError(err.Error(), 0, 0, "")
			} else {
				intArgs[i] = int(v)
			}
		}
		return types.Int(fn(intArgs...))
	}
	pb.AddFunction(name, wrapper)
}

// AddStringFunction adds a function that takes string arguments and returns a string
func (pb *PluginBase) AddStringFunction(name string, fn func(args ...string) string) {
	wrapper := func(args ...types.Object) types.Object {
		strArgs := make([]string, len(args))
		for i, arg := range args {
			strArgs[i] = string(types.ToString(arg))
		}
		return types.String(fn(strArgs...))
	}
	pb.AddFunction(name, wrapper)
}

// GetFunctions returns all registered functions
func (pb *PluginBase) GetFunctions() []plugin.PluginFunction {
	return pb.functions
}

// Initialize is a no-op initialization
func (pb *PluginBase) Initialize() error {
	return nil
}

// Cleanup is a no-op cleanup
func (pb *PluginBase) Cleanup() error {
	return nil
}

// SimplePlugin is a simplified plugin wrapper for basic use cases
type SimplePlugin struct {
	*PluginBase
}

// NewSimplePlugin creates a new simple plugin
func NewSimplePlugin(name, version, description, author string) *SimplePlugin {
	return &SimplePlugin{
		PluginBase: NewPluginBase(name, version, description, author),
	}
}
