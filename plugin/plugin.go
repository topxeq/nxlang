// Package plugin provides plugin system for Nxlang
// Plugins are Go-based extensions that can be loaded into the Nxlang VM
package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"reflect"

	"github.com/topxeq/nxlang/types"
)

// PluginInfo contains metadata about a plugin
type PluginInfo struct {
	Name        string // Plugin name
	Version     string // Plugin version
	Description string // Plugin description
	Author      string // Plugin author
}

// PluginFunction represents a function exported by a plugin
type PluginFunction struct {
	Name string                    // Function name as seen in Nxlang
	Fn   func(args ...types.Object) types.Object // The actual function
}

// NxPlugin is the interface that all Nxlang plugins must implement
type NxPlugin interface {
	// Info returns plugin metadata
	Info() PluginInfo

	// Initialize is called when the plugin is loaded
	Initialize() error

	// GetFunctions returns all functions exported by this plugin
	GetFunctions() []PluginFunction

	// Cleanup is called when the plugin is unloaded
	Cleanup() error
}

// PluginWrapper wraps a loaded Go plugin
type PluginWrapper struct {
	plugin     *plugin.Plugin
	path       string
	info       PluginInfo
	functions  map[string]func(args ...types.Object) types.Object
	initialized bool
}

// PluginLoader handles loading and managing plugins
type PluginLoader struct {
	plugins map[string]*PluginWrapper
}

// NewPluginLoader creates a new plugin loader
func NewPluginLoader() *PluginLoader {
	return &PluginLoader{
		plugins: make(map[string]*PluginWrapper),
	}
}

// LoadPlugin loads a plugin from a .nxp file (Nxlang plugin)
// The plugin file should be a Go plugin (.so on Linux, .dylib on macOS, .dll on Windows)
func (pl *PluginLoader) LoadPlugin(pluginPath string) error {
	// Resolve absolute path
	absPath, err := filepath.Abs(pluginPath)
	if err != nil {
		return fmt.Errorf("failed to resolve plugin path: %v", err)
	}

	// Check if plugin is already loaded
	if _, exists := pl.plugins[absPath]; exists {
		return fmt.Errorf("plugin already loaded: %s", absPath)
	}

	// Load the Go plugin
	p, err := plugin.Open(absPath)
	if err != nil {
		return fmt.Errorf("failed to load plugin: %v", err)
	}

	// Create wrapper
	wrapper := &PluginWrapper{
		plugin:     p,
		path:       absPath,
		functions:  make(map[string]func(args ...types.Object) types.Object),
		initialized: false,
	}

	// Look for NxPlugin symbol
	symbol, err := p.Lookup("NxPlugin")
	if err != nil {
		return fmt.Errorf("plugin must export NxPlugin symbol: %v", err)
	}

	// Use reflection to get the underlying plugin object and call its methods
	// symbol could be *interface or interface value
	symbolValue := reflect.ValueOf(symbol)

	// Dereference pointer if needed
	if symbolValue.Kind() == reflect.Ptr {
		symbolValue = symbolValue.Elem()
	}

	// If it's an interface, get the underlying value
	if symbolValue.Kind() == reflect.Interface {
		symbolValue = symbolValue.Elem()
	}

	if !symbolValue.IsValid() {
		return fmt.Errorf("NxPlugin symbol is invalid")
	}

	// Get plugin info using reflection
	infoMethod := symbolValue.MethodByName("Info")
	if infoMethod.IsValid() {
		results := infoMethod.Call(nil)
		if len(results) > 0 {
			if info, ok := results[0].Interface().(PluginInfo); ok {
				wrapper.info = info
			}
		}
	}

	// Initialize plugin using reflection
	initMethod := symbolValue.MethodByName("Initialize")
	if initMethod.IsValid() {
		results := initMethod.Call(nil)
		if len(results) > 0 {
			if err, ok := results[0].Interface().(error); ok && err != nil {
				return fmt.Errorf("plugin initialization failed: %v", err)
			}
		}
	}
	wrapper.initialized = true

	// Get functions using reflection
	getFuncsMethod := symbolValue.MethodByName("GetFunctions")
	if getFuncsMethod.IsValid() {
		results := getFuncsMethod.Call(nil)
		if len(results) > 0 {
			// Get the slice of PluginFunction
			funcsValue := results[0]
			if funcsValue.Kind() == reflect.Slice {
				for i := 0; i < funcsValue.Len(); i++ {
					elem := funcsValue.Index(i)
					if elem.Kind() == reflect.Struct {
						// Get Name field
						nameField := elem.FieldByName("Name")
						if nameField.IsValid() {
							funcName := nameField.String()
							// Get Fn field (which is a function)
							fnField := elem.FieldByName("Fn")
							if fnField.IsValid() && fnField.Kind() == reflect.Func {
								// Create a wrapper function that calls via reflection
								// This avoids type mismatches across plugin boundaries
								wrapper.functions[funcName] = func(args ...types.Object) types.Object {
									// Build arguments for reflection call
									reflectArgs := make([]reflect.Value, len(args))
									for j, arg := range args {
										reflectArgs[j] = reflect.ValueOf(arg)
									}
									// Call the function via reflection
									callResults := fnField.Call(reflectArgs)
									if len(callResults) > 0 {
										if result, ok := callResults[0].Interface().(types.Object); ok {
											return result
										}
									}
									return types.UndefinedValue
								}
							}
						}
					}
				}
			}
		}
	}

	// Store plugin
	pl.plugins[absPath] = wrapper

	return nil
}

// UnloadPlugin unloads a plugin
func (pl *PluginLoader) UnloadPlugin(pluginPathOrName string) error {
	// First, try to find by plugin name in loaded plugins
	for path, wrapper := range pl.plugins {
		if wrapper.info.Name == pluginPathOrName {
			// Found by name, use the stored path
			pluginPathOrName = path
			break
		}
		// Also check if it matches the base name
		if filepath.Base(path) == pluginPathOrName ||
		   filepath.Base(path) == pluginPathOrName+".nxp" {
			pluginPathOrName = path
			break
		}
	}

	// Resolve absolute path
	absPath, err := filepath.Abs(pluginPathOrName)
	if err != nil {
		return fmt.Errorf("failed to resolve plugin path: %v", err)
	}

	wrapper, exists := pl.plugins[absPath]
	if !exists {
		return fmt.Errorf("plugin not loaded: %s", absPath)
	}

	// Cleanup plugin using reflection
	// Note: Go plugins cannot be truly unloaded, but we can call cleanup
	p, err := wrapper.plugin.Lookup("NxPlugin")
	if err == nil {
		symbolValue := reflect.ValueOf(p)
		if symbolValue.Kind() == reflect.Ptr {
			symbolValue = symbolValue.Elem()
		}
		if symbolValue.Kind() == reflect.Interface {
			symbolValue = symbolValue.Elem()
		}
		cleanupMethod := symbolValue.MethodByName("Cleanup")
		if cleanupMethod.IsValid() {
			results := cleanupMethod.Call(nil)
			if len(results) > 0 {
				if err, ok := results[0].Interface().(error); ok && err != nil {
					fmt.Fprintf(os.Stderr, "Plugin cleanup warning: %v\n", err)
				}
			}
		}
	}

	// Remove from registry
	delete(pl.plugins, absPath)

	return nil
}

// GetFunction returns a function from a loaded plugin
func (pl *PluginLoader) GetFunction(pluginName, funcName string) (func(args ...types.Object) types.Object, error) {
	// Try qualified name first (e.g., "example.hello")
	qualifiedName := pluginName + "." + funcName

	// Search all plugins for the qualified function name
	for _, wrapper := range pl.plugins {
		if fn, exists := wrapper.functions[qualifiedName]; exists {
			return fn, nil
		}
	}

	// Try just the function name (for plugins that don't use qualified names)
	// First try to find by plugin name
	for path, wrapper := range pl.plugins {
		// Check if plugin name matches (either by path or by plugin info name)
		nameMatch := filepath.Base(path) == pluginName ||
		             filepath.Base(path) == pluginName+".nxp" ||
		             wrapper.info.Name == pluginName

		if nameMatch {
			if fn, exists := wrapper.functions[funcName]; exists {
				return fn, nil
			}
		}
	}

	// If not found by plugin name, search all plugins for the function
	for _, wrapper := range pl.plugins {
		if fn, exists := wrapper.functions[funcName]; exists {
			return fn, nil
		}
	}

	return nil, fmt.Errorf("function '%s' not found in any loaded plugin", funcName)
}

// ListPlugins returns information about all loaded plugins
func (pl *PluginLoader) ListPlugins() []PluginInfo {
	infos := make([]PluginInfo, 0, len(pl.plugins))
	for _, wrapper := range pl.plugins {
		infos = append(infos, wrapper.info)
	}
	return infos
}

// LoadPluginFromDir loads all plugins from a directory
func (pl *PluginLoader) LoadPluginFromDir(dirPath string) error {
	// Find all plugin files
	pattern := filepath.Join(dirPath, "*.nxp")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to find plugins: %v", err)
	}

	// Also check for .so, .dylib, .dll extensions
	for _, ext := range []string{".so", ".dylib", ".dll"} {
		pattern := filepath.Join(dirPath, "*"+ext)
		moreFiles, err := filepath.Glob(pattern)
		if err == nil {
			files = append(files, moreFiles...)
		}
	}

	var lastErr error
	for _, file := range files {
		if err := pl.LoadPlugin(file); err != nil {
			lastErr = err
			fmt.Fprintf(os.Stderr, "Warning: failed to load plugin %s: %v\n", file, err)
		}
	}

	return lastErr
}

// Global plugin loader instance
var globalLoader *PluginLoader

// GetGlobalLoader returns the global plugin loader
func GetGlobalLoader() *PluginLoader {
	if globalLoader == nil {
		globalLoader = NewPluginLoader()
	}
	return globalLoader
}

// Load loads a plugin using the global loader
func Load(pluginPath string) error {
	return GetGlobalLoader().LoadPlugin(pluginPath)
}

// Call calls a function from a loaded plugin
func Call(pluginName, funcName string, args ...types.Object) (types.Object, error) {
	fn, err := GetGlobalLoader().GetFunction(pluginName, funcName)
	if err != nil {
		return types.NewError(err.Error(), 0, 0, ""), err
	}
	return fn(args...), nil
}
