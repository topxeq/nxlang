# Nxlang Plugin System Guide

## Overview

Nxlang's plugin system allows developers to extend the language with Go-based plugins. Plugins are compiled as Go shared libraries and can be loaded at runtime to provide new functions to Nxlang code.

## Building Plugins

### Requirements

- Go 1.21 or later
- Plugin must be built with `-buildmode=plugin` flag
- Only supported on Linux and macOS (Windows has limited plugin support)

### Building the Example Plugin

```bash
cd /path/to/nxlang
go build -buildmode=plugin -o example.nxp plugins/example_plugin/main.go
```

## Creating a Plugin

### Basic Structure

A Nxlang plugin must:

1. Import the `github.com/topxeq/nxlang/plugin` package
2. Implement the `plugin.NxPlugin` interface
3. Export a variable named `NxPlugin` of type `plugin.NxPlugin`

### Example Plugin

```go
package main

import (
    "github.com/topxeq/nxlang/plugin"
    "github.com/topxeq/nxlang/types"
)

// MyPlugin implements the NxPlugin interface
type MyPlugin struct{}

// Info returns plugin metadata
func (p *MyPlugin) Info() plugin.PluginInfo {
    return plugin.PluginInfo{
        Name:        "myplugin",
        Version:     "1.0.0",
        Description: "My custom plugin",
        Author:      "Your Name",
    }
}

// Initialize is called when the plugin is loaded
func (p *MyPlugin) Initialize() error {
    // Setup code here
    return nil
}

// GetFunctions returns all functions exported by this plugin
func (p *MyPlugin) GetFunctions() []plugin.PluginFunction {
    return []plugin.PluginFunction{
        {
            Name: "myplugin.greet",
            Fn: func(args ...types.Object) types.Object {
                if len(args) < 1 {
                    return types.String("Hello!")
                }
                name := string(types.ToString(args[0]))
                return types.String("Hello, " + name + "!")
            },
        },
    }
}

// Cleanup is called when the plugin is unloaded
func (p *MyPlugin) Cleanup() error {
    // Cleanup code here
    return nil
}

// NxPlugin is the exported symbol
var NxPlugin plugin.NxPlugin = &MyPlugin{}
```

## Using Plugins in Nxlang

### Loading a Plugin

```nx
// Load a plugin from file
var result = loadPlugin("path/to/plugin.nxp")
if isErr(result) {
    pln("Failed to load plugin:", result)
} else {
    pln("Plugin loaded successfully!")
}
```

### Calling Plugin Functions

```nx
// Call a function from the loaded plugin
var greeting = callPlugin("myplugin", "greet", "World")
pln(greeting)  // Output: Hello, World!
```

### Listing Loaded Plugins

```nx
// Get information about all loaded plugins
var plugins = listPlugins()
for i, p in plugins {
    pln("Plugin:", p["name"], "v" + p["version"])
    pln("  Description:", p["description"])
}
```

### Unloading a Plugin

```nx
// Unload a plugin
var result = unloadPlugin("path/to/plugin.nxp")
if isErr(result) {
    pln("Failed to unload plugin:", result)
}
```

## Built-in Plugin Functions

| Function | Description |
|----------|-------------|
| `loadPlugin(path)` | Load a plugin from the specified path |
| `unloadPlugin(path)` | Unload a plugin |
| `callPlugin(name, func, ...args)` | Call a plugin function |
| `listPlugins()` | Return list of loaded plugins |

## Plugin Helper Package

The `pluginhelper` package provides convenience functions for creating plugins:

```go
import "github.com/topxeq/nxlang/plugin/pluginhelper"

// Create a simple plugin
p := pluginhelper.NewSimplePlugin("myplugin", "1.0.0", "My Plugin", "Author")

// Add functions
p.AddFunction("greet", func(args ...types.Object) types.Object {
    return types.String("Hello!")
})

// Use p as plugin.NxPlugin
```

## Example Plugin Functions

The example plugin (`example.nxp`) provides these functions:

| Function | Description |
|----------|-------------|
| `example.hello(name)` | Return a greeting message |
| `example.upper(str)` | Convert string to uppercase |
| `example.add(a, b)` | Add two numbers |
| `example.multiply(a, b)` | Multiply two numbers |
| `example.info()` | Return plugin information |

## Best Practices

1. **Error Handling**: Always return proper error objects using `types.NewError()`
2. **Type Conversion**: Use `types.ToInt()`, `types.ToString()`, etc. for type conversion
3. **Documentation**: Document all exported functions
4. **Versioning**: Use semantic versioning for plugin versions
5. **Cleanup**: Release resources in the `Cleanup()` method

## Limitations

- Go plugins cannot be truly unloaded (Go runtime limitation)
- Plugin support is platform-dependent
- All plugin functions receive and return `types.Object`

## Troubleshooting

### "plugin must export NxPlugin symbol"

Make sure your plugin has:
```go
var NxPlugin plugin.NxPlugin = &MyPlugin{}
```

### "function not found in any loaded plugin"

Check that:
1. The plugin was loaded successfully
2. The function name matches exactly (including case)
3. The function is registered in `GetFunctions()`

### Build errors on Windows

Go plugin support on Windows is limited. Consider using WSL or a Linux/macOS environment for plugin development.
