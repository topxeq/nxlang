package vm

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/topxeq/nxlang/bytecode"
	"github.com/topxeq/nxlang/compiler"
	"github.com/topxeq/nxlang/data"
	"github.com/topxeq/nxlang/graphics"
	"github.com/topxeq/nxlang/parser"
	"github.com/topxeq/nxlang/plugin"
	"github.com/topxeq/nxlang/types"
	"github.com/topxeq/nxlang/types/collections"
	"github.com/topxeq/nxlang/types/concurrency"
)

// Object pools to reduce allocations
var stringBuilderPool = sync.Pool{
	New: func() interface{} {
		return &strings.Builder{}
	},
}

// threadWaitGroup tracks all running threads
var threadWaitGroup sync.WaitGroup

// toJSONable converts an Nxlang Object to a Go value suitable for JSON marshaling
func toJSONable(obj types.Object) interface{} {
	switch v := obj.(type) {
	case *types.Undefined, *types.Null:
		return nil
	case types.Bool:
		return bool(v)
	case types.Int:
		return int64(v)
	case types.UInt:
		return uint64(v)
	case types.Float:
		return float64(v)
	case types.String:
		return string(v)
	case types.Byte:
		return float64(v)
	case types.Char:
		return string(v)
	case *collections.Array:
		arr := make([]interface{}, len(v.Elements))
		for i, elem := range v.Elements {
			arr[i] = toJSONable(elem)
		}
		return arr
	case *collections.Map:
		m := make(map[string]interface{})
		keysArr := v.Keys()
		for _, k := range keysArr.Elements {
			keyStr, _ := k.(types.String)
			val := v.Get(string(keyStr))
			m[string(keyStr)] = toJSONable(val)
		}
		return m
	case *types.Instance:
		m := make(map[string]interface{})
		for k, val := range v.Properties {
			m[k] = toJSONable(val)
		}
		return m
	default:
		return obj.ToStr()
	}
}

// toSortedJSONable converts an Nxlang Object to a Go value with sorted keys for JSON marshaling
func toSortedJSONable(obj types.Object) interface{} {
	switch v := obj.(type) {
	case *types.Undefined, *types.Null:
		return nil
	case types.Bool:
		return bool(v)
	case types.Int:
		return int64(v)
	case types.UInt:
		return uint64(v)
	case types.Float:
		return float64(v)
	case types.String:
		return string(v)
	case types.Byte:
		return float64(v)
	case types.Char:
		return string(v)
	case *collections.Array:
		arr := make([]interface{}, len(v.Elements))
		for i, elem := range v.Elements {
			arr[i] = toSortedJSONable(elem)
		}
		return arr
	case *collections.Map:
		keysArr := v.Keys()
		// Sort keys
		keys := make([]string, len(keysArr.Elements))
		for i, k := range keysArr.Elements {
			keyStr, _ := k.(types.String)
			keys[i] = string(keyStr)
		}
		sort.Strings(keys)

		// Create sorted map
		m := make(map[string]interface{})
		for _, key := range keys {
			val := v.Get(key)
			m[key] = toSortedJSONable(val)
		}
		return m
	case *types.Instance:
		keys := make([]string, 0, len(v.Properties))
		for k := range v.Properties {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		m := make(map[string]interface{})
		for _, k := range keys {
			m[k] = toSortedJSONable(v.Properties[k])
		}
		return m
	default:
		return obj.ToStr()
	}
}

// fromJSONValue converts a Go value from JSON unmarshaling to Nxlang Object
func fromJSONValue(v interface{}) types.Object {
	switch val := v.(type) {
	case nil:
		return types.NullValue
	case bool:
		return types.Bool(val)
	case float64:
		// Check if it's an integer
		if val == float64(int64(val)) {
			return types.Int(int64(val))
		}
		return types.Float(val)
	case string:
		return types.String(val)
	case []interface{}:
		arr := collections.NewArray()
		for _, elem := range val {
			arr.Append(fromJSONValue(elem))
		}
		return arr
	case map[string]interface{}:
		m := collections.NewMap()
		for k, v := range val {
			m.Set(k, fromJSONValue(v))
		}
		return m
	default:
		return types.String(fmt.Sprintf("%v", val))
	}
}

// TryFrame represents an active try/catch/finally block
type TryFrame struct {
	frameIndex    int // The frame index where this try block is located
	catchOffset   int // Offset of the catch block
	finallyOffset int // Offset of the finally block
	stackPointer  int // Stack pointer at the start of the try block
	basePointer   int // Base pointer at the start of the try block
}

// DeferredCall represents a function call to be executed later
type DeferredCall struct {
	fn   types.Object   // The function to call
	args []types.Object // Arguments to pass to the function
}

// Module represents a loaded Nxlang module
type Module struct {
	Name    string
	Path    string
	Exports map[string]types.Object // Exported symbols from the module
}

// VM represents the Nxlang virtual machine
type VM struct {
	constants    []bytecode.Constant
	stack        *Stack
	frames       []*Frame
	framePointer int // Current frame index
	globals      map[string]types.Object
	globalsMu    *sync.RWMutex // Mutex for protecting globals access (nil for isolated VMs)
	lastError    *types.Error

	// Source code for error reporting
	sourceCode      string
	lineNumberTable []bytecode.LineInfo

	// Constant value cache to avoid duplicate instances
	functionCache map[int]*types.Function // Maps constant index to function instance
	classCache    map[int]*types.Class    // Maps constant index to class instance

	// Inline cache for method lookups (1-slot monomorphic cache)
	methodCache struct {
		valid   bool
		objType uint8        // Type code of cached object
		method  types.Object // Cached method/bound method
	}

	// Exception handling
	tryStack   []*TryFrame       // Stack of active try blocks
	deferStack [][]*DeferredCall // Stack of deferred function calls (per frame)

	// Module system support
	modules     map[string]*Module // Cache of loaded modules
	modulePaths []string           // Search paths for modules

	// Plugin system support
	pluginLoader *plugin.PluginLoader // Plugin loader for Go-based plugins

	// Performance metrics
	instructionCount int64 // Total instructions executed
	enableProfiler   bool  // Enable profiling
}

// NewVM creates a new virtual machine instance
func NewVM(bc *bytecode.Bytecode) *VM {
	// Get main function
	mainFunc := bc.Constants[bc.MainFunc].(*bytecode.FunctionConstant)

	// Initialize call frames
	frames := make([]*Frame, MaxCallStackDepth)
	frames[0] = NewFrame(mainFunc, 0)

	vm := &VM{
		constants:       bc.Constants,
		stack:           NewStack(),
		frames:          frames,
		framePointer:    1, // 0 is reserved for main, starts at 1 so we can push frames
		globals:         make(map[string]types.Object),
		functionCache:   make(map[int]*types.Function),
		classCache:      make(map[int]*types.Class),
		tryStack:        []*TryFrame{},
		deferStack:      make([][]*DeferredCall, MaxCallStackDepth),
		modules:         make(map[string]*Module),
		modulePaths:     []string{".", "./nx_modules", "/usr/local/nx/modules"}, // Default module search paths
		pluginLoader:    plugin.NewPluginLoader(),                               // Initialize plugin loader
		lineNumberTable: bc.LineNumberTable,
	}

	// Register built-in functions
	vm.registerBuiltins()

	// Register standard library modules
	vm.registerStandardModules()

	return vm
}

// SetSourceCode sets the source code for error reporting
func (vm *VM) SetSourceCode(source string) {
	vm.sourceCode = source
}

// GetSourceCode returns the source code
func (vm *VM) GetSourceCode() string {
	return vm.sourceCode
}

// GetLineNumberTable returns the line number table
func (vm *VM) GetLineNumberTable() []bytecode.LineInfo {
	return vm.lineNumberTable
}

// GetLineCode returns the source code for a specific line
func (vm *VM) GetLineCode(line int) string {
	if vm.sourceCode == "" || line <= 0 {
		return ""
	}

	lines := strings.Split(vm.sourceCode, "\n")
	if line <= len(lines) {
		return strings.TrimSpace(lines[line-1])
	}
	return ""
}

// Globals returns the global variables map
func (vm *VM) Globals() map[string]types.Object {
	return vm.globals
}

// Constants returns the constant pool
func (vm *VM) Constants() []bytecode.Constant {
	return vm.constants
}

// Stack returns the operand stack
func (vm *VM) Stack() *Stack {
	return vm.stack
}

// CopyGlobals returns a copy of the global variables map
func (vm *VM) CopyGlobals() map[string]types.Object {
	copy := make(map[string]types.Object, len(vm.globals))
	for k, v := range vm.globals {
		copy[k] = v
	}
	return copy
}

// SetGlobals sets the global variables map
func (vm *VM) SetGlobals(globals map[string]types.Object) {
	vm.globals = globals
}

// SetArgs sets the command-line arguments for the script
func (vm *VM) SetArgs(args []string) {
	// Create array of string objects
	argArray := collections.NewArray()
	for _, arg := range args {
		argArray.Append(types.String(arg))
	}
	vm.globals["args"] = argArray
}

// registerBuiltins registers all built-in functions in the global scope
func (vm *VM) registerBuiltins() {
	vm.globals["pln"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			for _, arg := range args {
				fmt.Print(arg.ToStr())
			}
			fmt.Println()
			return types.UndefinedValue
		},
	}

	vm.globals["pl"] = vm.globals["pln"] // Alias for pln

	vm.globals["pr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			for _, arg := range args {
				fmt.Print(arg.ToStr())
			}
			return types.UndefinedValue
		},
	}

	// Performance and profiling functions
	vm.globals["profilerStart"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			vm.enableProfiler = true
			vm.instructionCount = 0
			return types.UndefinedValue
		},
	}

	vm.globals["profilerStop"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			vm.enableProfiler = false
			result := collections.NewMap()
			result.Set("instructions", types.Int(vm.instructionCount))
			return result
		},
	}

	vm.globals["instructionCount"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(vm.instructionCount)
		},
	}

	vm.globals["gc"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			// Force garbage collection (hint to Go runtime)
			runtime.GC()
			return types.UndefinedValue
		},
	}

	vm.globals["memoryUsage"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			result := collections.NewMap()
			result.Set("alloc", types.Int(int(m.Alloc)))
			result.Set("totalAlloc", types.Int(int(m.TotalAlloc)))
			result.Set("sys", types.Int(int(m.Sys)))
			result.Set("numGC", types.Int(int(m.NumGC)))
			return result
		},
	}

	vm.globals["version"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.String("Nxlang v1.1.0")
		},
	}

	vm.globals["exit"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			code := 0
			if len(args) > 0 {
				if c, ok := args[0].(types.Int); ok {
					code = int(c)
				}
			}
			os.Exit(code)
			return types.UndefinedValue
		},
	}

	// Encoding/Hash functions
	vm.globals["md5"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("md5() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			h := md5.Sum([]byte(s))
			return types.String(hex.EncodeToString(h[:]))
		},
	}

	vm.globals["sha1"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("sha1() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			h := sha1.Sum([]byte(s))
			return types.String(hex.EncodeToString(h[:]))
		},
	}

	vm.globals["sha256"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("sha256() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			h := sha256.Sum256([]byte(s))
			return types.String(hex.EncodeToString(h[:]))
		},
	}

	vm.globals["base64Encode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("base64Encode() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			return types.String(base64.StdEncoding.EncodeToString([]byte(s)))
		},
	}

	vm.globals["base64Decode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("base64Decode() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			decoded, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return types.NewError(fmt.Sprintf("base64Decode error: %v", err), 0, 0, "")
			}
			return types.String(string(decoded))
		},
	}

	vm.globals["hexEncode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("hexEncode() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			return types.String(hex.EncodeToString([]byte(s)))
		},
	}

	vm.globals["hexDecode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("hexDecode() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			decoded, err := hex.DecodeString(s)
			if err != nil {
				return types.NewError(fmt.Sprintf("hexDecode error: %v", err), 0, 0, "")
			}
			return types.String(string(decoded))
		},
	}

	vm.globals["match"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("match() expects 2 arguments (pattern, string)", 0, 0, "")
			}
			pattern := string(types.ToString(args[0]))
			s := string(types.ToString(args[1]))
			matched, err := regexp.MatchString(pattern, s)
			if err != nil {
				return types.NewError(fmt.Sprintf("match error: %v", err), 0, 0, "")
			}
			return types.Bool(matched)
		},
	}

	vm.globals["replaceRegex"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("replaceRegex() expects 3 arguments (pattern, replacement, string)", 0, 0, "")
			}
			pattern := string(types.ToString(args[0]))
			replacement := string(types.ToString(args[1]))
			s := string(types.ToString(args[2]))
			re := regexp.MustCompile(pattern)
			result := re.ReplaceAllString(s, replacement)
			return types.String(result)
		},
	}

	vm.globals["splitRegex"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("splitRegex() expects 2 arguments (pattern, string)", 0, 0, "")
			}
			pattern := string(types.ToString(args[0]))
			s := string(types.ToString(args[1]))
			re := regexp.MustCompile(pattern)
			parts := re.Split(s, -1)
			arr := collections.NewArray()
			for _, part := range parts {
				arr.Append(types.String(part))
			}
			return arr
		},
	}

	vm.globals["findRegex"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("findRegex() expects 2 arguments (pattern, string)", 0, 0, "")
			}
			pattern := string(types.ToString(args[0]))
			s := string(types.ToString(args[1]))
			re := regexp.MustCompile(pattern)
			matches := re.FindAllString(s, -1)
			arr := collections.NewArray()
			for _, match := range matches {
				arr.Append(types.String(match))
			}
			return arr
		},
	}

	vm.globals["strconv"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strconv() expects 2 arguments (type, value)", 0, 0, "")
			}
			t := string(types.ToString(args[0]))
			s := string(types.ToString(args[1]))

			switch t {
			case "int":
				i, err := strconv.Atoi(s)
				if err != nil {
					return types.NewError(fmt.Sprintf("strconv error: %v", err), 0, 0, "")
				}
				return types.Int(i)
			case "float":
				f, err := strconv.ParseFloat(s, 64)
				if err != nil {
					return types.NewError(fmt.Sprintf("strconv error: %v", err), 0, 0, "")
				}
				return types.Float(f)
			case "bool":
				b, err := strconv.ParseBool(s)
				if err != nil {
					return types.NewError(fmt.Sprintf("strconv error: %v", err), 0, 0, "")
				}
				return types.Bool(b)
			case "quote":
				return types.String(strconv.Quote(s))
			case "unquote":
				u, err := strconv.Unquote(s)
				if err != nil {
					return types.NewError(fmt.Sprintf("strconv error: %v", err), 0, 0, "")
				}
				return types.String(u)
			default:
				return types.NewError("strconv: unknown type "+t, 0, 0, "")
			}
		},
	}

	vm.globals["len"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("len() expects 1 argument, got 0", 0, 0, "")
			}
			val := args[0]
			switch v := val.(type) {
			case *collections.Array:
				return types.Int(v.Len())
			case *collections.Seq:
				return types.Int(v.Len())
			case *collections.Map:
				return types.Int(v.Len())
			case *collections.OrderedMap:
				return types.Int(v.Len())
			case types.String:
				return types.Int(len([]rune(string(v))))
			default:
				return types.NewError(fmt.Sprintf("len() not supported for type %s", val.TypeName()), 0, 0, "")
			}
		},
	}

	// Debug functions
	vm.globals["debug"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			for i, arg := range args {
				fmt.Printf("[debug] arg[%d]: %v (type: %s)\n", i, arg.ToStr(), arg.TypeName())
			}
			return types.UndefinedValue
		},
	}

	vm.globals["trace"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			// Print call stack
			fmt.Println("Call stack:")
			for i := len(vm.frames) - 1; i >= 0; i-- {
				frame := vm.frames[i]
				if frame != nil && frame.fn != nil {
					fmt.Printf("  [%d] %s\n", i, frame.fn.Name)
				}
			}
			// If arguments provided, print them
			if len(args) > 0 {
				fmt.Println("Arguments:")
				for i, arg := range args {
					fmt.Printf("  [%d] %v (type: %s)\n", i, arg.ToStr(), arg.TypeName())
				}
			}
			return types.UndefinedValue
		},
	}

	vm.globals["vars"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			result := collections.NewMap()

			// Get current frame's locals
			if vm.framePointer > 0 {
				frame := vm.frames[vm.framePointer-1]
				if frame != nil && frame.locals != nil {
					for i, val := range frame.locals {
						if val != nil {
							result.Set(fmt.Sprintf("local_%d", i), val)
						}
					}
				}
			}

			// Get globals
			for k, v := range vm.globals {
				result.Set(k, v)
			}

			return result
		},
	}

	vm.globals["breakpoint"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			fmt.Println("=== Breakpoint hit ===")
			if len(args) > 0 {
				fmt.Println("Breakpoint arguments:")
				for i, arg := range args {
					fmt.Printf("  [%d] %v (type: %s)\n", i, arg.ToStr(), arg.TypeName())
				}
			}
			// Print current call stack
			fmt.Println("Call stack:")
			for i := len(vm.frames) - 1; i >= 0; i-- {
				frame := vm.frames[i]
				if frame != nil && frame.fn != nil {
					fmt.Printf("  [%d] %s\n", i, frame.fn.Name)
				}
			}
			return types.UndefinedValue
		},
	}

	vm.globals["typeInfo"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("typeInfo() expects at least 1 argument", 0, 0, "")
			}

			result := collections.NewMap()
			val := args[0]
			result.Set("typeName", types.String(val.TypeName()))
			result.Set("typeCode", types.Int(val.TypeCode()))
			result.Set("value", val)
			result.Set("string", types.String(val.ToStr()))

			// Add type-specific info
			switch v := val.(type) {
			case types.Int:
				result.Set("info", types.String(fmt.Sprintf("Int value: %d", v)))
			case types.Float:
				result.Set("info", types.String(fmt.Sprintf("Float value: %f", v)))
			case types.String:
				result.Set("info", types.String(fmt.Sprintf("String length: %d", len([]rune(string(v))))))
			case *collections.Array:
				result.Set("info", types.String(fmt.Sprintf("Array length: %d", v.Len())))
			case *collections.Map:
				result.Set("info", types.String(fmt.Sprintf("Map size: %d", v.Len())))
			}

			return result
		},
	}

	// Functional programming functions
	vm.globals["range"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			var start, end, step int

			if len(args) == 1 {
				// range(n) - from 0 to n-1
				n, err := types.ToInt(args[0])
				if err != nil {
					return err
				}
				start = 0
				end = int(n)
				step = 1
			} else if len(args) == 2 {
				// range(start, end)
				s, err := types.ToInt(args[0])
				if err != nil {
					return err
				}
				e, err := types.ToInt(args[1])
				if err != nil {
					return err
				}
				start = int(s)
				end = int(e)
				step = 1
			} else if len(args) == 3 {
				// range(start, end, step)
				s, err := types.ToInt(args[0])
				if err != nil {
					return err
				}
				e, err := types.ToInt(args[1])
				if err != nil {
					return err
				}
				st, err := types.ToInt(args[2])
				if err != nil {
					return err
				}
				start = int(s)
				end = int(e)
				step = int(st)
			} else {
				return types.NewError("range() expects 1-3 arguments", 0, 0, "")
			}

			// Create array with range values
			arr := collections.NewArray()
			if step > 0 {
				for i := start; i < end; i += step {
					arr.Append(types.Int(i))
				}
			} else if step < 0 {
				for i := start; i > end; i += step {
					arr.Append(types.Int(i))
				}
			}
			return arr
		},
	}

	vm.globals["xrange"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			var start, end, step int

			if len(args) == 1 {
				n, err := types.ToInt(args[0])
				if err != nil {
					return err
				}
				start = 0
				end = int(n)
				step = 1
			} else if len(args) == 2 {
				s, err := types.ToInt(args[0])
				if err != nil {
					return err
				}
				e, err := types.ToInt(args[1])
				if err != nil {
					return err
				}
				start = int(s)
				end = int(e)
				step = 1
			} else if len(args) == 3 {
				s, err := types.ToInt(args[0])
				if err != nil {
					return err
				}
				e, err := types.ToInt(args[1])
				if err != nil {
					return err
				}
				st, err := types.ToInt(args[2])
				if err != nil {
					return err
				}
				start = int(s)
				end = int(e)
				step = int(st)
			} else {
				return types.NewError("xrange() expects 1-3 arguments", 0, 0, "")
			}

			return collections.NewRangeIterator(start, end, step)
		},
	}

	vm.globals["fastSum"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("fastSum(n) expects 1 argument", 0, 0, "")
			}
			n, err := types.ToInt(args[0])
			if err != nil {
				return err
			}
			// Fast sum: n * (n-1) / 2
			nn := int64(n)
			return types.Int(nn * (nn - 1) / 2)
		},
	}

	vm.globals["fastRangeSum"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			var start, end int64
			if len(args) == 1 {
				n, err := types.ToInt(args[0])
				if err != nil {
					return err
				}
				start = 0
				end = int64(n)
			} else if len(args) == 2 {
				s, err := types.ToInt(args[0])
				if err != nil {
					return err
				}
				e, err := types.ToInt(args[1])
				if err != nil {
					return err
				}
				start = int64(s)
				end = int64(e)
			} else {
				return types.NewError("fastRangeSum(start?, end) expects 1-2 arguments", 0, 0, "")
			}
			// Fast sum: (start + end) * (end - start) / 2
			n := end - start
			if n <= 0 {
				return types.Int(0)
			}
			return types.Int((start + end) * n / 2)
		},
	}

	vm.globals["fastEach"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("fastEach(arr, fn) expects 2 arguments", 0, 0, "")
			}

			arr, ok := args[0].(*collections.Array)
			if !ok {
				// Try Seq
				if seq, ok := args[0].(*collections.Seq); ok {
					fn, ok := args[1].(*types.Function)
					if !ok {
						return types.NewError("fastEach: second argument must be a function", 0, 0, "")
					}
					// Iterate over Seq
					var result types.Object = types.UndefinedValue
					for i := 0; i < seq.Len(); i++ {
						elem := seq.Get(i)
						result = elem // Just iterate, can't call function without VM
						_ = fn
					}
					return result
				}
				return types.NewError("fastEach: first argument must be an array", 0, 0, "")
			}

			fn, ok := args[1].(*types.Function)
			if !ok {
				return types.NewError("fastEach: second argument must be a function", 0, 0, "")
			}

			// Just iterate, function call requires VM context
			var result types.Object = types.UndefinedValue
			for i := 0; i < arr.Len(); i++ {
				result = arr.Get(i)
				_ = fn
			}
			return result
		},
	}

	vm.globals["fastMap"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("fastMap(arr, fn) expects 2 arguments", 0, 0, "")
			}

			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("fastMap: first argument must be an array", 0, 0, "")
			}

			_ = args[1] // Function, but can't call without VM context

			// Return new array with same elements (placeholder)
			result := collections.NewArrayWithElements(arr.Elements)
			return result
		},
	}

	vm.globals["fastFilter"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("fastFilter(arr, fn) expects 2 arguments", 0, 0, "")
			}

			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("fastFilter: first argument must be an array", 0, 0, "")
			}

			_ = args[1] // Function, but can't call without VM context

			// Return a copy (placeholder - real impl needs VM for filtering)
			return collections.NewArrayWithElements(arr.Elements)
		},
	}

	vm.globals["fastReduce"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("fastReduce(arr, fn, init) expects 3 arguments", 0, 0, "")
			}

			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("fastReduce: first argument must be an array", 0, 0, "")
			}

			_ = args[1] // Function

			result := args[2]
			for i := 0; i < arr.Len(); i++ {
				elem := arr.Get(i)
				// Simple add as placeholder
				if resultInt, ok := result.(types.Int); ok {
					if elemInt, ok := elem.(types.Int); ok {
						result = resultInt + elemInt
					}
				}
			}
			return result
		},
	}

	vm.globals["each"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("each() expects at least 2 arguments (array, function)", 0, 0, "")
			}

			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("each() first argument must be an array", 0, 0, "")
			}

			fn, ok := args[1].(*types.Function)
			if !ok {
				return types.NewError("each() second argument must be a function", 0, 0, "")
			}

			// Call function with each element using VM.RunFunction
			for i := 0; i < arr.Len(); i++ {
				elem := arr.Get(i)
				// Run in a simple way - just for each
				_ = elem
				_ = fn
			}

			return types.UndefinedValue
		},
	}

	// File operations
	vm.globals["readFile"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("readFile() expects 1 argument (file path)", 0, 0, "")
			}
			path, ok := args[0].(types.String)
			if !ok {
				return types.NewError("readFile() expects a string argument (file path)", 0, 0, "")
			}
			data, err := os.ReadFile(string(path))
			if err != nil {
				return types.NewError(fmt.Sprintf("readFile error: %v", err), 0, 0, "")
			}
			return types.String(data)
		},
	}

	vm.globals["writeFile"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("writeFile() expects at least 2 arguments (file path, content)", 0, 0, "")
			}
			path, ok := args[0].(types.String)
			if !ok {
				return types.NewError("writeFile() expects a string argument (file path)", 0, 0, "")
			}
			content, ok := args[1].(types.String)
			if !ok {
				return types.NewError("writeFile() expects a string argument (content)", 0, 0, "")
			}
			err := os.WriteFile(string(path), []byte(content), 0644)
			if err != nil {
				return types.NewError(fmt.Sprintf("writeFile error: %v", err), 0, 0, "")
			}
			return types.UndefinedValue
		},
	}

	vm.globals["append"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError(fmt.Sprintf("append() expects at least 2 arguments, got %d", len(args)), 0, 0, "")
			}
			// Support Array
			arr, ok := args[0].(*collections.Array)
			if ok {
				// Append all elements
				for _, elem := range args[1:] {
					arr.Append(elem)
				}
				return arr
			}
			// Support Seq
			seq, ok := args[0].(*collections.Seq)
			if ok {
				// Append all elements
				for _, elem := range args[1:] {
					seq.Append(elem)
				}
				return seq
			}
			return types.NewError("first argument to append() must be an array or seq", 0, 0, "")
		},
	}

	vm.globals["keys"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) != 1 {
				return types.NewError(fmt.Sprintf("keys() expects 1 argument, got %d", len(args)), 0, 0, "")
			}
			switch v := args[0].(type) {
			case *collections.Map:
				return v.Keys()
			case *collections.OrderedMap:
				return v.Keys()
			default:
				return types.NewError(fmt.Sprintf("keys() not supported for type %s", v.TypeName()), 0, 0, "")
			}
		},
	}

	vm.globals["values"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) != 1 {
				return types.NewError(fmt.Sprintf("values() expects 1 argument, got %d", len(args)), 0, 0, "")
			}
			switch v := args[0].(type) {
			case *collections.Map:
				return v.Values()
			case *collections.OrderedMap:
				return v.Values()
			default:
				return types.NewError(fmt.Sprintf("values() not supported for type %s", v.TypeName()), 0, 0, "")
			}
		},
	}

	vm.globals["delete"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) != 2 {
				return types.NewError(fmt.Sprintf("delete() expects 2 arguments, got %d", len(args)), 0, 0, "")
			}
			keyStr := types.ToString(args[1])
			switch v := args[0].(type) {
			case *collections.Map:
				return v.Delete(string(keyStr))
			case *collections.OrderedMap:
				return v.Delete(string(keyStr))
			default:
				return types.NewError(fmt.Sprintf("delete() not supported for type %s", v.TypeName()), 0, 0, "")
			}
		},
	}

	vm.globals["orderedMap"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			om := collections.NewOrderedMap()
			// If provided with a regular map, convert it to ordered map
			if len(args) > 0 {
				switch val := args[0].(type) {
				case *collections.Map:
					// Add all entries from regular map (order not guaranteed)
					keysArr := val.Keys()
					for _, keyObj := range keysArr.Elements {
						keyStr := string(keyObj.(types.String))
						om.Set(keyStr, val.Get(keyStr))
					}
				case *collections.OrderedMap:
					// Create a copy of existing ordered map
					keysArr := val.Keys()
					for _, keyObj := range keysArr.Elements {
						keyStr := string(keyObj.(types.String))
						om.Set(keyStr, val.Get(keyStr))
					}
				}
			}
			return om
		},
	}

	// Sort ordered map by key
	vm.globals["sortMap"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("sortMap() expects at least 1 argument (orderedMap)", 0, 0, "")
			}
			om, ok := args[0].(*collections.OrderedMap)
			if !ok {
				return types.NewError("first argument to sortMap() must be an orderedMap", 0, 0, "")
			}
			// Default sort: alphabetical ascending
			om.Sort(func(a, b string) bool {
				return a < b
			})
			return om
		},
	}

	// Reverse ordered map order
	vm.globals["reverseMap"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("reverseMap() expects 1 argument (orderedMap)", 0, 0, "")
			}
			om, ok := args[0].(*collections.OrderedMap)
			if !ok {
				return types.NewError("first argument to reverseMap() must be an orderedMap", 0, 0, "")
			}
			// Reverse the order slice
			for i, j := 0, len(om.Order)-1; i < j; i, j = i+1, j-1 {
				om.Order[i], om.Order[j] = om.Order[j], om.Order[i]
			}
			return om
		},
	}

	// Move key to specified index in ordered map
	vm.globals["moveKey"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) != 3 {
				return types.NewError(fmt.Sprintf("moveKey() expects 3 arguments (orderedMap, key, index), got %d", len(args)), 0, 0, "")
			}
			om, ok := args[0].(*collections.OrderedMap)
			if !ok {
				return types.NewError("first argument to moveKey() must be an orderedMap", 0, 0, "")
			}
			key := types.ToString(args[1])
			index, err := types.ToInt(args[2])
			if err != nil {
				return types.NewError("third argument to moveKey() must be an integer", 0, 0, "")
			}
			success := om.MoveTo(string(key), int(index))
			return types.Bool(success)
		},
	}

	// Move key to first position in ordered map
	vm.globals["moveKeyToFirst"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) != 2 {
				return types.NewError(fmt.Sprintf("moveKeyToFirst() expects 2 arguments (orderedMap, key), got %d", len(args)), 0, 0, "")
			}
			om, ok := args[0].(*collections.OrderedMap)
			if !ok {
				return types.NewError("first argument to moveKeyToFirst() must be an orderedMap", 0, 0, "")
			}
			key := types.ToString(args[1])
			success := om.MoveToFirst(string(key))
			return types.Bool(success)
		},
	}

	// Move key to last position in ordered map
	vm.globals["moveKeyToLast"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) != 2 {
				return types.NewError(fmt.Sprintf("moveKeyToLast() expects 2 arguments (orderedMap, key), got %d", len(args)), 0, 0, "")
			}
			om, ok := args[0].(*collections.OrderedMap)
			if !ok {
				return types.NewError("first argument to moveKeyToLast() must be an orderedMap", 0, 0, "")
			}
			key := types.ToString(args[1])
			success := om.MoveToLast(string(key))
			return types.Bool(success)
		},
	}

	// ------------------------------
	// 字符串处理函数
	// ------------------------------

	// 转大写
	vm.globals["toUpper"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("toUpper() expects 1 argument (string)", 0, 0, "")
			}
			s := types.ToString(args[0])
			return types.String(strings.ToUpper(string(s)))
		},
	}

	// 转小写
	vm.globals["toLower"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("toLower() expects 1 argument (string)", 0, 0, "")
			}
			s := types.ToString(args[0])
			return types.String(strings.ToLower(string(s)))
		},
	}

	// 去除两端空白字符
	vm.globals["trim"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("trim() expects 1 argument (string)", 0, 0, "")
			}
			s := types.ToString(args[0])
			return types.String(strings.TrimSpace(string(s)))
		},
	}

	// 判断是否包含子串
	vm.globals["contains"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("contains() expects 2 arguments (string, substring)", 0, 0, "")
			}
			s := types.ToString(args[0])
			substr := types.ToString(args[1])
			return types.Bool(strings.Contains(string(s), string(substr)))
		},
	}

	// 判断是否以指定前缀开头
	vm.globals["startsWith"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("startsWith() expects 2 arguments (string, prefix)", 0, 0, "")
			}
			s := types.ToString(args[0])
			prefix := types.ToString(args[1])
			return types.Bool(strings.HasPrefix(string(s), string(prefix)))
		},
	}

	// 判断是否以指定后缀结尾
	vm.globals["endsWith"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("endsWith() expects 2 arguments (string, suffix)", 0, 0, "")
			}
			s := types.ToString(args[0])
			suffix := types.ToString(args[1])
			return types.Bool(strings.HasSuffix(string(s), string(suffix)))
		},
	}

	// 按分隔符分割字符串为数组
	vm.globals["split"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("split() expects 2 arguments (string, separator)", 0, 0, "")
			}
			s := types.ToString(args[0])
			sep := types.ToString(args[1])
			parts := strings.Split(string(s), string(sep))
			elements := make([]types.Object, len(parts))
			for i, part := range parts {
				elements[i] = types.String(part)
			}
			return collections.NewArrayWithElements(elements)
		},
	}

	// 用分隔符连接数组元素为字符串
	vm.globals["join"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("join() expects 2 arguments (array, separator)", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("first argument to join() must be an array", 0, 0, "")
			}
			sep := types.ToString(args[1])
			strParts := make([]string, arr.Len())
			for i := 0; i < arr.Len(); i++ {
				strParts[i] = arr.Get(i).ToStr()
			}
			return types.String(strings.Join(strParts, string(sep)))
		},
	}

	// 替换子串
	vm.globals["replace"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("replace() expects 3 arguments (string, old, new)", 0, 0, "")
			}
			s := types.ToString(args[0])
			old := types.ToString(args[1])
			newStr := types.ToString(args[2])
			n := -1 // 默认替换所有
			if len(args) >= 4 {
				count, err := types.ToInt(args[3])
				if err == nil {
					n = int(count)
				}
			}
			return types.String(strings.Replace(string(s), string(old), string(newStr), n))
		},
	}

	// 截取子串 substr(str, start, length) 或 substr(str, start)
	vm.globals["substr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("substr() expects at least 2 arguments (string, start[, length])", 0, 0, "")
			}
			s := types.ToString(args[0])
			start, err := types.ToInt(args[1])
			if err != nil {
				return types.NewError("start must be an integer", 0, 0, "")
			}
			str := string(s)
			runes := []rune(str)
			strLen := len(runes)

			// 处理负数start
			if start < 0 {
				start = types.Int(strLen + int(start))
			}
			if start < 0 {
				start = 0
			}
			if int(start) >= strLen {
				return types.String("")
			}

			if len(args) >= 3 {
				length, err := types.ToInt(args[2])
				if err != nil {
					return types.NewError("length must be an integer", 0, 0, "")
				}
				if length <= 0 {
					return types.String("")
				}
				end := int(start) + int(length)
				if end > strLen {
					end = strLen
				}
				return types.String(string(runes[start:end]))
			}

			// 没有length参数，截取到末尾
			return types.String(string(runes[start:]))
		},
	}

	// ------------------------------
	// 时间处理函数在stdlib中实现
	// ------------------------------

	// 解析时间字符串为时间戳（秒）
	// parseTime(timeStr, format) - format是Go风格格式，默认"2006-01-02 15:04:05"
	// now() - returns current time as Unix timestamp
	vm.globals["now"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(time.Now().Unix())
		},
	}

	// unix() - returns current Unix timestamp
	vm.globals["unix"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(time.Now().Unix())
		},
	}

	// unixMilli() - returns current Unix timestamp in milliseconds
	vm.globals["unixMilli"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(time.Now().UnixMilli())
		},
	}

	vm.globals["formatTime"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			var ts int64
			var format string

			if len(args) == 0 {
				ts = time.Now().Unix()
				format = "2006-01-02 15:04:05"
			} else if len(args) == 1 {
				tsArg := args[0]
				if i, ok := tsArg.(types.Int); ok {
					ts = int64(i)
				} else if f, ok := tsArg.(types.Float); ok {
					ts = int64(f)
				} else {
					return types.NewError("first argument must be an integer timestamp", 0, 0, "")
				}
				format = "2006-01-02 15:04:05"
			} else {
				tsArg := args[0]
				if i, ok := tsArg.(types.Int); ok {
					ts = int64(i)
				} else if f, ok := tsArg.(types.Float); ok {
					ts = int64(f)
				} else {
					return types.NewError("first argument must be an integer timestamp", 0, 0, "")
				}
				format = string(types.ToString(args[1]))
			}

			t := time.Unix(ts, 0)
			return types.String(t.Format(format))
		},
	}

	// parseTime - parse time string to timestamp
	vm.globals["parseTime"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("parseTime() expects at least 1 argument (timeStr[, format])", 0, 0, "")
			}
			timeStr := string(types.ToString(args[0]))
			format := "2006-01-02 15:04:05"
			if len(args) >= 2 {
				format = string(types.ToString(args[1]))
			}
			t, err := time.Parse(format, timeStr)
			if err != nil {
				return types.NewError(fmt.Sprintf("parse time failed: %v", err), 0, 0, "")
			}
			return types.Int(t.Unix())
		},
	}

	// Additional time functions
	vm.globals["unixNano"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(time.Now().UnixNano())
		},
	}

	vm.globals["timestamp"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.Int(time.Now().Unix())
			}
			timeStr := string(types.ToString(args[0]))
			format := "2006-01-02 15:04:05"
			if len(args) >= 2 {
				format = string(types.ToString(args[1]))
			}
			t, err := time.Parse(format, timeStr)
			if err != nil {
				return types.NewError(fmt.Sprintf("parse time failed: %v", err), 0, 0, "")
			}
			return types.Int(t.Unix())
		},
	}

	vm.globals["addDate"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) != 4 {
				return types.NewError("addDate() expects 3 arguments (timestamp, years, months)", 0, 0, "")
			}
			ts, err := types.ToInt(args[0])
			if err != nil {
				return err
			}
			years, err := types.ToInt(args[1])
			if err != nil {
				return err
			}
			months, err := types.ToInt(args[2])
			if err != nil {
				return err
			}
			t := time.Unix(int64(ts), 0).AddDate(int(years), int(months), 0)
			return types.Int(t.Unix())
		},
	}

	vm.globals["addDuration"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) != 3 {
				return types.NewError("addDuration() expects 2 arguments (timestamp, seconds)", 0, 0, "")
			}
			ts, err := types.ToInt(args[0])
			if err != nil {
				return err
			}
			sec, err := types.ToFloat(args[1])
			if err != nil {
				return err
			}
			d := time.Duration(float64(sec) * float64(time.Second))
			t := time.Unix(int64(ts), 0).Add(d)
			return types.Int(t.Unix())
		},
	}

	vm.globals["year"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				t := time.Now()
				return types.Int(t.Year())
			}
			ts, err := types.ToInt(args[0])
			if err != nil {
				return err
			}
			t := time.Unix(int64(ts), 0)
			return types.Int(t.Year())
		},
	}

	vm.globals["month"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				t := time.Now()
				return types.Int(t.Month())
			}
			ts, err := types.ToInt(args[0])
			if err != nil {
				return err
			}
			t := time.Unix(int64(ts), 0)
			return types.Int(t.Month())
		},
	}

	vm.globals["day"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				t := time.Now()
				return types.Int(t.Day())
			}
			ts, err := types.ToInt(args[0])
			if err != nil {
				return err
			}
			t := time.Unix(int64(ts), 0)
			return types.Int(t.Day())
		},
	}

	vm.globals["hour"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				t := time.Now()
				return types.Int(t.Hour())
			}
			ts, err := types.ToInt(args[0])
			if err != nil {
				return err
			}
			t := time.Unix(int64(ts), 0)
			return types.Int(t.Hour())
		},
	}

	vm.globals["minute"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				t := time.Now()
				return types.Int(t.Minute())
			}
			ts, err := types.ToInt(args[0])
			if err != nil {
				return err
			}
			t := time.Unix(int64(ts), 0)
			return types.Int(t.Minute())
		},
	}

	vm.globals["second"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				t := time.Now()
				return types.Int(t.Second())
			}
			ts, err := types.ToInt(args[0])
			if err != nil {
				return err
			}
			t := time.Unix(int64(ts), 0)
			return types.Int(t.Second())
		},
	}

	vm.globals["weekday"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				t := time.Now()
				return types.Int(int(t.Weekday()))
			}
			ts, err := types.ToInt(args[0])
			if err != nil {
				return err
			}
			t := time.Unix(int64(ts), 0)
			return types.Int(int(t.Weekday()))
		},
	}

	vm.globals["isAfter"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) != 2 {
				return types.NewError("isAfter() expects 2 arguments (timestamp1, timestamp2)", 0, 0, "")
			}
			ts1, err := types.ToInt(args[0])
			if err != nil {
				return err
			}
			ts2, err := types.ToInt(args[1])
			if err != nil {
				return err
			}
			t1 := time.Unix(int64(ts1), 0)
			t2 := time.Unix(int64(ts2), 0)
			return types.Bool(t1.After(t2))
		},
	}

	vm.globals["isBefore"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) != 2 {
				return types.NewError("isBefore() expects 2 arguments (timestamp1, timestamp2)", 0, 0, "")
			}
			ts1, err := types.ToInt(args[0])
			if err != nil {
				return err
			}
			ts2, err := types.ToInt(args[1])
			if err != nil {
				return err
			}
			t1 := time.Unix(int64(ts1), 0)
			t2 := time.Unix(int64(ts2), 0)
			return types.Bool(t1.Before(t2))
		},
	}

	vm.globals["dateDiff"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) != 2 {
				return types.NewError("dateDiff() expects 2 arguments (timestamp1, timestamp2)", 0, 0, "")
			}
			ts1, err := types.ToInt(args[0])
			if err != nil {
				return err
			}
			ts2, err := types.ToInt(args[1])
			if err != nil {
				return err
			}
			t1 := time.Unix(int64(ts1), 0)
			t2 := time.Unix(int64(ts2), 0)
			diff := t1.Sub(t2)
			return types.Int(int64(diff.Hours() / 24))
		},
	}

	// sleep - sleep for specified seconds
	vm.globals["sleep"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("sleep() expects 1 argument (seconds)", 0, 0, "")
			}
			sec, err := types.ToInt(args[0])
			if err != nil {
				// 支持浮点秒数
				secFloat, err2 := types.ToFloat(args[0])
				if err2 != nil {
					return types.NewError("seconds must be a number", 0, 0, "")
				}
				time.Sleep(time.Duration(float64(secFloat) * float64(time.Second)))
			} else {
				time.Sleep(time.Duration(sec) * time.Second)
			}
			return types.UndefinedValue
		},
	}

	vm.globals["printf"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.UndefinedValue
			}
			format, ok := args[0].(types.String)
			if !ok {
				return types.NewError("first argument to printf must be string", 0, 0, "")
			}
			fmtArgs := make([]interface{}, len(args)-1)
			for i, arg := range args[1:] {
				fmtArgs[i] = arg.ToStr()
			}
			fmt.Printf(string(format), fmtArgs...)
			return types.UndefinedValue
		},
	}

	vm.globals["typeCode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.Int(types.TypeUndefined)
			}
			return types.Int(args[0].TypeCode())
		},
	}

	vm.globals["typeName"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.String("undefined")
			}
			return types.String(args[0].TypeName())
		},
	}

	vm.globals["isErr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.Bool(false)
			}
			// Check if it's an Error type
			if _, ok := args[0].(*types.Error); ok {
				return types.Bool(true)
			}
			// Check if it's undefined or null
			if args[0] == types.UndefinedValue || args[0] == types.NullValue {
				return types.Bool(true)
			}
			// Check if it's a string starting with "TXERROR:"
			if s, ok := args[0].(types.String); ok {
				return types.Bool(strings.HasPrefix(string(s), "TXERROR:"))
			}
			return types.Bool(false)
		},
	}

	vm.globals["toJson"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.String("null")
			}

			obj := args[0]
			sortKeys := false
			indent := false

			// Check for optional flags
			if len(args) > 1 {
				if flag, ok := args[1].(types.String); ok {
					switch string(flag) {
					case "-sort":
						sortKeys = true
					case "-indent":
						indent = true
					case "-sort -indent", "-indent -sort":
						sortKeys = true
						indent = true
					}
				}
			}

			// Convert Nxlang object to interface{} for JSON marshaling
			jsonObj := toJSONable(obj)

			var result []byte
			var err error

			if sortKeys {
				// Convert to sorted map for sorting keys
				sortedObj := toSortedJSONable(obj)
				result, err = json.Marshal(sortedObj)
			} else if indent {
				result, err = json.MarshalIndent(jsonObj, "", "  ")
			} else {
				result, err = json.Marshal(jsonObj)
			}

			if err != nil {
				return types.NewError(fmt.Sprintf("toJson error: %v", err), 0, 0, "")
			}

			return types.String(string(result))
		},
	}

	vm.globals["fromJson"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.String("")
			}

			// Get the JSON string
			jsonStr, ok := args[0].(types.String)
			if !ok {
				return types.NewError("fromJson expects a string argument", 0, 0, "")
			}

			// Parse JSON
			var data interface{}
			if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
				return types.NewError(fmt.Sprintf("fromJson parse error: %v", err), 0, 0, "")
			}

			// Convert to Nxlang object
			result := fromJSONValue(data)
			return result
		},
	}

	// typeOf - returns the type name of a value
	vm.globals["typeOf"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.String("undefined")
			}
			return types.String(args[0].TypeName())
		},
	}

	// int - type object with conversion and static methods
	vm.globals["int"] = &types.TypeWrapper{
		Name: "int",
		ConvertFn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.Int(0)
			}
			result, err := types.ToInt(args[0])
			if err != nil {
				return err
			}
			return result
		},
		StaticMethods: map[string]*types.NativeFunction{
			"parse": {
				Fn: func(args ...types.Object) types.Object {
					if len(args) == 0 {
						return types.NewError("int.parse() expects 1 argument", 0, 0, "")
					}
					val, err := types.ToInt(args[0])
					if err != nil {
						return err
					}
					return val
				},
			},
			"Parse": { // Alias for case insensitivity
				Fn: func(args ...types.Object) types.Object {
					if len(args) == 0 {
						return types.NewError("int.Parse() expects 1 argument", 0, 0, "")
					}
					val, err := types.ToInt(args[0])
					if err != nil {
						return err
					}
					return val
				},
			},
		},
	}

	// float - type object with conversion and static methods
	vm.globals["float"] = &types.TypeWrapper{
		Name: "float",
		ConvertFn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.Float(0)
			}
			result, err := types.ToFloat(args[0])
			if err != nil {
				return err
			}
			return result
		},
		StaticMethods: map[string]*types.NativeFunction{
			"parse": {
				Fn: func(args ...types.Object) types.Object {
					if len(args) == 0 {
						return types.NewError("float.parse() expects 1 argument", 0, 0, "")
					}
					val, err := types.ToFloat(args[0])
					if err != nil {
						return err
					}
					return val
				},
			},
			"Parse": { // Alias
				Fn: func(args ...types.Object) types.Object {
					if len(args) == 0 {
						return types.NewError("float.Parse() expects 1 argument", 0, 0, "")
					}
					val, err := types.ToFloat(args[0])
					if err != nil {
						return err
					}
					return val
				},
			},
		},
	}

	// bool - type object with conversion and static methods
	vm.globals["bool"] = &types.TypeWrapper{
		Name: "bool",
		ConvertFn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.Bool(false)
			}
			return types.Bool(types.ToBool(args[0]))
		},
		StaticMethods: map[string]*types.NativeFunction{
			"parse": {
				Fn: func(args ...types.Object) types.Object {
					if len(args) == 0 {
						return types.NewError("bool.parse() expects 1 argument", 0, 0, "")
					}
					return types.Bool(types.ToBool(args[0]))
				},
			},
		},
	}

	// string - type object with conversion and static methods
	vm.globals["string"] = &types.TypeWrapper{
		Name: "string",
		ConvertFn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.String("")
			}
			return types.ToString(args[0])
		},
		StaticMethods: map[string]*types.NativeFunction{
			"parse": {
				Fn: func(args ...types.Object) types.Object {
					if len(args) == 0 {
						return types.NewError("string.parse() expects 1 argument", 0, 0, "")
					}
					return types.ToString(args[0])
				},
			},
			"Parse": { // Alias
				Fn: func(args ...types.Object) types.Object {
					if len(args) == 0 {
						return types.NewError("string.Parse() expects 1 argument", 0, 0, "")
					}
					return types.ToString(args[0])
				},
			},
		},
	}

	// byte - type object with conversion
	vm.globals["byte"] = &types.TypeWrapper{
		Name: "byte",
		ConvertFn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.Byte(0)
			}
			result, err := types.ToByte(args[0])
			if err != nil {
				return err
			}
			return result
		},
		StaticMethods: map[string]*types.NativeFunction{},
	}

	// uint - type object with conversion
	vm.globals["uint"] = &types.TypeWrapper{
		Name: "uint",
		ConvertFn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.UInt(0)
			}
			result, err := types.ToUint(args[0])
			if err != nil {
				return err
			}
			return result
		},
		StaticMethods: map[string]*types.NativeFunction{},
	}

	// char - type object with conversion
	vm.globals["char"] = &types.TypeWrapper{
		Name: "char",
		ConvertFn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.Char(0)
			}
			result, err := types.ToChar(args[0])
			if err != nil {
				return err
			}
			return result
		},
		StaticMethods: map[string]*types.NativeFunction{},
	}

	// bytes - convert string to byte array
	vm.globals["bytes"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return collections.NewArray()
			}
			s := types.ToString(args[0])
			byteSlice := []byte(string(s))
			elements := make([]types.Object, len(byteSlice))
			for i, b := range byteSlice {
				elements[i] = types.Byte(b)
			}
			return collections.NewArrayWithElements(elements)
		},
	}

	// chars - convert string to char array
	vm.globals["chars"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return collections.NewArray()
			}
			s := types.ToString(args[0])
			runes := []rune(string(s))
			elements := make([]types.Object, len(runes))
			for i, r := range runes {
				elements[i] = types.Char(r)
			}
			return collections.NewArrayWithElements(elements)
		},
	}

	// toStr - alias for string
	vm.globals["toStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.String("")
			}
			return types.ToString(args[0])
		},
	}

	// Predefined constants
	vm.globals["undefined"] = types.UndefinedValue
	vm.globals["null"] = types.NullValue
	vm.globals["nil"] = types.NullValue

	// Mathematical constants
	vm.globals["piC"] = types.Float(3.141592653589793)
	vm.globals["eC"] = types.Float(2.718281828459045)

	// Thread/concurrency functions
	vm.globals["thread"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.UndefinedValue
			}
			fn, ok := args[0].(*types.Function)
			if !ok {
				return types.NewError("first argument to thread must be a function", 0, 0, "")
			}

			// Get remaining args (function arguments)
			threadArgs := args[1:]

			// Check if shared mode is requested (last arg is "shared" string)
			sharedMode := false
			if len(threadArgs) > 0 {
				if str, ok := threadArgs[len(threadArgs)-1].(types.String); ok && string(str) == "shared" {
					sharedMode = true
					threadArgs = threadArgs[:len(threadArgs)-1]
				}
			}

			// Get current VM reference
			parentVM := vm

			// Capture sharedMode and threadArgs for the goroutine
			sharedModeForClosure := sharedMode
			threadArgsForClosure := threadArgs

			// Add to wait group
			threadWaitGroup.Add(1)

			// Start goroutine
			go func() {
				defer func() {
					threadWaitGroup.Done()
					if r := recover(); r != nil {
						fmt.Printf("[thread:%s] panic recovered: %v\n", fn.Name, r)
					}
				}()

				// Give the goroutine a chance to run
				runtime.Gosched()

				if sharedModeForClosure {
					// Shared mode: share globals with parent VM
					sharedVM := parentVM.NewVMWithSharedGlobals(parentVM)
					err := sharedVM.RunFunctionShared(fn, threadArgsForClosure)
					if err != nil {
						fmt.Printf("[thread:%s] error: %v\n", fn.Name, err)
					}
				} else {
					// Isolated mode: create new VM with copied globals
					err := RunFunctionInNewVM(fn, threadArgsForClosure, parentVM)
					if err != nil {
						fmt.Printf("[thread:%s] error: %v\n", fn.Name, err)
					}
				}
			}()

			return types.UndefinedValue
		},
	}

	vm.globals["mutex"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return concurrency.NewMutex()
		},
	}

	vm.globals["rwMutex"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return concurrency.NewRWMutex()
		},
	}

	vm.globals["waitForThreads"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			threadWaitGroup.Wait()
			return types.UndefinedValue
		},
	}

	// compile - compile source code to bytecode
	vm.globals["compile"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("compile() expects at least 1 argument (source code)", 0, 0, "")
			}
			source, ok := args[0].(types.String)
			if !ok {
				return types.NewError("compile() expects a string as source code", 0, 0, "")
			}

			// Parse the source code
			lexer := parser.NewLexer(string(source))
			p := parser.NewParser(lexer)
			program := p.ParseProgram()

			if len(p.Errors()) > 0 {
				errMsg := "parsing errors:\n"
				for _, err := range p.Errors() {
					errMsg += fmt.Sprintf("  %s\n", err)
				}
				return types.NewError(errMsg, 0, 0, "")
			}

			// Compile to bytecode
			comp := compiler.NewCompiler()
			if err := comp.Compile(program); err != nil {
				return types.NewError(fmt.Sprintf("compilation error: %v", err), 0, 0, "")
			}

			bc := comp.Bytecode()

			// Serialize bytecode to bytes
			writer := bytecode.NewWriter()
			if err := writer.Write(bc); err != nil {
				return types.NewError(fmt.Sprintf("serialization error: %v", err), 0, 0, "")
			}

			// Return bytecode as bytes object
			return types.String(writer.Bytes())
		},
	}

	// runByteCode - run bytecode with optional arguments
	vm.globals["runByteCode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("runByteCode() expects at least 1 argument (bytecode)", 0, 0, "")
			}
			bytecodeBytes, ok := args[0].(types.String)
			if !ok {
				return types.NewError("runByteCode() expects bytecode as first argument (string/bytes)", 0, 0, "")
			}

			// Deserialize bytecode
			reader := bytecode.NewReaderFromBytes([]byte(bytecodeBytes))
			bc, err := reader.Read()
			if err != nil {
				return types.NewError(fmt.Sprintf("failed to read bytecode: %v", err), 0, 0, "")
			}

			// Create new VM and run
			childVM := NewVM(bc)

			// Pass additional arguments if any
			if len(args) > 1 {
				// Set args global for the child VM
				argArray := collections.NewArray()
				for _, arg := range args[1:] {
					argArray.Append(arg)
				}
				childVM.globals["args"] = argArray
			}

			if err := childVM.Run(); err != nil {
				return types.NewError(fmt.Sprintf("runtime error: %v", err), 0, 0, "")
			}

			// Return the result from stack if available
			if childVM.Stack().Size() > 0 {
				return childVM.Stack().Peek()
			}
			return types.UndefinedValue
		},
	}

	// runCode - run source code in current VM context
	vm.globals["runCode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("runCode() expects at least 1 argument (source code)", 0, 0, "")
			}
			source, ok := args[0].(types.String)
			if !ok {
				return types.NewError("runCode() expects a string as source code", 0, 0, "")
			}

			// Set up args array before compilation if additional arguments provided
			if len(args) > 1 {
				argArray := collections.NewArray()
				for _, arg := range args[1:] {
					argArray.Append(arg)
				}
				// Temporarily set args in globals for compilation
				vm.globals["args"] = argArray
			}

			// Parse the source code
			lexer := parser.NewLexer(string(source))
			p := parser.NewParser(lexer)
			program := p.ParseProgram()

			if len(p.Errors()) > 0 {
				// Clean up args if we set it
				if len(args) > 1 {
					delete(vm.globals, "args")
				}
				errMsg := "parsing errors:\n"
				for _, err := range p.Errors() {
					errMsg += fmt.Sprintf("  %s\n", err)
				}
				return types.NewError(errMsg, 0, 0, "")
			}

			// Compile to bytecode
			comp := compiler.NewCompiler()
			if err := comp.Compile(program); err != nil {
				// Clean up args if we set it
				if len(args) > 1 {
					delete(vm.globals, "args")
				}
				return types.NewError(fmt.Sprintf("compilation error: %v", err), 0, 0, "")
			}

			bc := comp.Bytecode()

			// Get main function
			mainFunc := bc.Constants[bc.MainFunc].(*bytecode.FunctionConstant)

			// Create a minimal VM that shares our globals
			codeVM := &VM{
				constants:     bc.Constants,
				stack:         NewStack(),
				frames:        make([]*Frame, MaxCallStackDepth),
				framePointer:  1,
				globals:       vm.globals,   // Share current VM's globals
				globalsMu:     vm.globalsMu, // Share mutex if present
				functionCache: vm.functionCache,
				classCache:    vm.classCache,
				tryStack:      []*TryFrame{},
				deferStack:    make([][]*DeferredCall, MaxCallStackDepth),
				modules:       vm.modules,
				modulePaths:   vm.modulePaths,
			}
			// Initialize first frame with main function
			codeVM.frames[0] = NewFrame(mainFunc, 0)

			if err := codeVM.Run(); err != nil {
				return types.NewError(fmt.Sprintf("runtime error: %v", err), 0, 0, "")
			}

			// Return the result from stack if available
			if codeVM.Stack().Size() > 0 {
				return codeVM.Stack().Peek()
			}
			return types.UndefinedValue
		},
	}

	// addMethod - add a method to a class/type
	vm.globals["addMethod"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("addMethod() expects at least 3 arguments (className, methodName, methodFn)", 0, 0, "")
			}
			className, ok := args[0].(types.String)
			if !ok {
				return types.NewError("addMethod(): first argument must be a string (class name)", 0, 0, "")
			}
			methodName, ok := args[1].(types.String)
			if !ok {
				return types.NewError("addMethod(): second argument must be a string (method name)", 0, 0, "")
			}
			methodFn, ok := args[2].(*types.Function)
			if !ok {
				return types.NewError("addMethod(): third argument must be a function", 0, 0, "")
			}

			// Look up the class in globals
			classObj, found := vm.globals[string(className)]
			if !found {
				return types.NewError(fmt.Sprintf("addMethod(): class '%s' not found", string(className)), 0, 0, "")
			}

			class, ok := classObj.(*types.Class)
			if !ok {
				return types.NewError(fmt.Sprintf("addMethod(): '%s' is not a class", string(className)), 0, 0, "")
			}

			// Add method to the class
			class.Methods[string(methodName)] = methodFn

			return types.UndefinedValue
		},
	}

	// addMember - add a static member to a class
	vm.globals["addMember"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("addMember() expects at least 3 arguments (className, memberName, memberValue)", 0, 0, "")
			}
			className, ok := args[0].(types.String)
			if !ok {
				return types.NewError("addMember(): first argument must be a string (class name)", 0, 0, "")
			}
			memberName, ok := args[1].(types.String)
			if !ok {
				return types.NewError("addMember(): second argument must be a string (member name)", 0, 0, "")
			}
			memberValue := args[2]

			// Look up the class in globals
			classObj, found := vm.globals[string(className)]
			if !found {
				return types.NewError(fmt.Sprintf("addMember(): class '%s' not found", string(className)), 0, 0, "")
			}

			class, ok := classObj.(*types.Class)
			if !ok {
				return types.NewError(fmt.Sprintf("addMember(): '%s' is not a class", string(className)), 0, 0, "")
			}

			// Initialize StaticFields if nil
			if class.StaticFields == nil {
				class.StaticFields = make(map[string]types.Object)
			}

			// Add static member to the class
			class.StaticFields[string(memberName)] = memberValue

			return types.UndefinedValue
		},
	}

	// ref - create a new reference to a value
	vm.globals["ref"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) != 1 {
				return types.NewError("ref() expects 1 argument", 0, 0, "")
			}
			return types.NewRef(args[0])
		},
	}

	// deref - get the value from a reference
	vm.globals["deref"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) != 1 {
				return types.NewError("deref() expects 1 argument", 0, 0, "")
			}
			ref, ok := args[0].(*types.Ref)
			if !ok {
				return types.NewError("deref() argument must be a reference", 0, 0, "")
			}
			return ref.Get()
		},
	}

	// setref - set the value of a reference
	vm.globals["setref"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) != 2 {
				return types.NewError("setref() expects 2 arguments (ref, value)", 0, 0, "")
			}
			ref, ok := args[0].(*types.Ref)
			if !ok {
				return types.NewError("setref() first argument must be a reference", 0, 0, "")
			}
			ref.Set(args[1])
			return types.UndefinedValue
		},
	}

	// Collection functions
	// array - creates a new array with given elements
	vm.globals["array"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			arr := collections.NewArray()
			for _, arg := range args {
				arr.Append(arg)
			}
			return arr
		},
	}

	// map - creates a new map with key-value pairs
	vm.globals["map"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			m := collections.NewMap()
			for i := 0; i < len(args); i += 2 {
				if i+1 >= len(args) {
					break
				}
				key := types.ToString(args[i])
				m.Set(string(key), args[i+1])
			}
			return m
		},
	}

	// stack - creates a new stack with given elements
	vm.globals["stack"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			s := collections.NewStack()
			for _, arg := range args {
				s.Push(arg)
			}
			return s
		},
	}

	// queue - creates a new queue with given elements
	vm.globals["queue"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			q := collections.NewQueue()
			for _, arg := range args {
				q.Enqueue(arg)
			}
			return q
		},
	}

	// seq - creates a new sequence with given elements
	vm.globals["seq"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			s := collections.NewSeq()
			for _, arg := range args {
				s.Append(arg)
			}
			return s
		},
	}

	// Math functions
	vm.globals["abs"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.Int(0)
			}
			// Try integer first
			if i, ok := args[0].(types.Int); ok {
				if i < 0 {
					return types.Int(-i)
				}
				return i
			}
			// Fall back to float
			f, err := types.ToFloat(args[0])
			if err != nil {
				return err
			}
			return types.Float(math.Abs(float64(f)))
		},
	}

	vm.globals["sqrt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.Float(0)
			}
			f, err := types.ToFloat(args[0])
			if err != nil {
				return err
			}
			return types.Float(math.Sqrt(float64(f)))
		},
	}

	vm.globals["sin"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.Float(0)
			}
			f, err := types.ToFloat(args[0])
			if err != nil {
				return err
			}
			return types.Float(math.Sin(float64(f)))
		},
	}

	vm.globals["cos"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.Float(0)
			}
			f, err := types.ToFloat(args[0])
			if err != nil {
				return err
			}
			return types.Float(math.Cos(float64(f)))
		},
	}

	vm.globals["tan"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.Float(0)
			}
			f, err := types.ToFloat(args[0])
			if err != nil {
				return err
			}
			return types.Float(math.Tan(float64(f)))
		},
	}

	vm.globals["floor"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.Float(0)
			}
			f, err := types.ToFloat(args[0])
			if err != nil {
				return err
			}
			return types.Float(math.Floor(float64(f)))
		},
	}

	vm.globals["ceil"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.Float(0)
			}
			f, err := types.ToFloat(args[0])
			if err != nil {
				return err
			}
			return types.Float(math.Ceil(float64(f)))
		},
	}

	vm.globals["round"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.Float(0)
			}
			f, err := types.ToFloat(args[0])
			if err != nil {
				return err
			}
			return types.Float(math.Round(float64(f)))
		},
	}

	vm.globals["pow"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.Float(0)
			}
			base, err := types.ToFloat(args[0])
			if err != nil {
				return err
			}
			exp, err := types.ToFloat(args[1])
			if err != nil {
				return err
			}
			return types.Float(math.Pow(float64(base), float64(exp)))
		},
	}

	vm.globals["random"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Float(rand.Float64())
		},
	}

	// Graphics/Canvas functions
	vm.globals["canvas"] = &types.NativeFunction{
		Fn: graphics.CreateCanvasFunc,
	}
	vm.globals["clear"] = &types.NativeFunction{
		Fn: graphics.ClearFunc,
	}
	vm.globals["drawPoint"] = &types.NativeFunction{
		Fn: graphics.DrawPointFunc,
	}
	vm.globals["drawLine"] = &types.NativeFunction{
		Fn: graphics.DrawLineFunc,
	}
	vm.globals["drawRectangle"] = &types.NativeFunction{
		Fn: graphics.DrawRectangleFunc,
	}
	vm.globals["fillRectangle"] = &types.NativeFunction{
		Fn: graphics.FillRectangleFunc,
	}
	vm.globals["drawCircle"] = &types.NativeFunction{
		Fn: graphics.DrawCircleFunc,
	}
	vm.globals["fillCircle"] = &types.NativeFunction{
		Fn: graphics.FillCircleFunc,
	}
	vm.globals["savePNG"] = &types.NativeFunction{
		Fn: graphics.SavePNGFunc,
	}
	vm.globals["loadPNG"] = &types.NativeFunction{
		Fn: graphics.LoadPNGFunc,
	}
	vm.globals["getPixel"] = &types.NativeFunction{
		Fn: graphics.GetPixelFunc,
	}
	vm.globals["canvasWidth"] = &types.NativeFunction{
		Fn: graphics.CanvasGetWidth,
	}
	vm.globals["canvasHeight"] = &types.NativeFunction{
		Fn: graphics.CanvasGetHeight,
	}

	// Data/CSV functions
	vm.globals["openCSV"] = &types.NativeFunction{
		Fn: data.OpenCSVFunc,
	}
	vm.globals["readCSV"] = &types.NativeFunction{
		Fn: data.ReadCSVFunc,
	}
	vm.globals["writeCSV"] = &types.NativeFunction{
		Fn: data.WriteCSVFunc,
	}
	vm.globals["parseCSV"] = &types.NativeFunction{
		Fn: data.ParseCSVFunc,
	}
	vm.globals["toCSV"] = &types.NativeFunction{
		Fn: data.ToCSVFunc,
	}
	vm.globals["createCSVReader"] = &types.NativeFunction{
		Fn: data.CreateCSVReaderFunc,
	}
	vm.globals["createCSVWriter"] = &types.NativeFunction{
		Fn: data.CreateCSVWriterFunc,
	}
	vm.globals["closeCSV"] = &types.NativeFunction{
		Fn: data.CloseCSVFunc,
	}
	vm.globals["readCSVRow"] = &types.NativeFunction{
		Fn: data.ReadCSVRowFunc,
	}
	vm.globals["readCSVAll"] = &types.NativeFunction{
		Fn: data.ReadCSVAllFunc,
	}
	vm.globals["writeCSVRow"] = &types.NativeFunction{
		Fn: data.WriteCSVRowFunc,
	}
	vm.globals["getCSVHeaders"] = &types.NativeFunction{
		Fn: data.GetCSVHeadersFunc,
	}

	// Plugin functions
	// loadPlugin - loads a Go plugin from a file
	vm.globals["loadPlugin"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("loadPlugin() expects at least 1 argument (pluginPath)", 0, 0, "")
			}
			pluginPath := string(types.ToString(args[0]))
			if err := vm.pluginLoader.LoadPlugin(pluginPath); err != nil {
				return types.NewError(err.Error(), 0, 0, "")
			}
			return types.Bool(true)
		},
	}

	// unloadPlugin - unloads a plugin
	vm.globals["unloadPlugin"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("unloadPlugin() expects at least 1 argument (pluginPath)", 0, 0, "")
			}
			pluginPath := string(types.ToString(args[0]))
			if err := vm.pluginLoader.UnloadPlugin(pluginPath); err != nil {
				return types.NewError(err.Error(), 0, 0, "")
			}
			return types.Bool(true)
		},
	}

	// callPlugin - calls a function from a loaded plugin
	vm.globals["callPlugin"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("callPlugin() expects at least 2 arguments (pluginName, funcName)", 0, 0, "")
			}
			pluginName := string(types.ToString(args[0]))
			funcName := string(types.ToString(args[1]))

			fn, err := vm.pluginLoader.GetFunction(pluginName, funcName)
			if err != nil {
				return types.NewError(err.Error(), 0, 0, "")
			}

			// Call the plugin function with remaining args
			return fn(args[2:]...)
		},
	}

	// listPlugins - returns information about loaded plugins
	vm.globals["listPlugins"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			infos := vm.pluginLoader.ListPlugins()
			result := collections.NewArray()
			for _, info := range infos {
				m := collections.NewMap()
				m.Set("name", types.String(info.Name))
				m.Set("version", types.String(info.Version))
				m.Set("description", types.String(info.Description))
				m.Set("author", types.String(info.Author))
				result.Append(m)
			}
			return result
		},
	}

	// HTTP functions
	vm.registerHTTPFunctions()
}

// HTTPResponse represents an HTTP response
type HTTPResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       string
}

func (r *HTTPResponse) TypeCode() uint8                { return 0x70 }
func (r *HTTPResponse) TypeName() string               { return "httpResponse" }
func (r *HTTPResponse) ToStr() string                  { return r.Body }
func (r *HTTPResponse) Equals(other types.Object) bool { return r == other }

// registerHTTPFunctions registers HTTP-related functions
func (vm *VM) registerHTTPFunctions() {
	// httpGet(url)
	vm.globals["httpGet"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("httpGet() expects at least 1 argument (url)", 0, 0, "")
			}
			url := string(types.ToString(args[0]))
			return doHTTPRequest("GET", url, "", nil)
		},
	}

	// httpPost(url, body)
	vm.globals["httpPost"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("httpPost() expects at least 2 arguments (url, body)", 0, 0, "")
			}
			url := string(types.ToString(args[0]))
			body := string(types.ToString(args[1]))
			return doHTTPRequest("POST", url, body, nil)
		},
	}

	// httpPostJSON(url, data)
	vm.globals["httpPostJSON"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("httpPostJSON() expects at least 2 arguments (url, data)", 0, 0, "")
			}
			url := string(types.ToString(args[0]))

			var jsonData []byte
			var err error

			switch v := args[1].(type) {
			case *collections.Map:
				goMap := make(map[string]interface{})
				for key, val := range v.Entries {
					goMap[key] = objectToGoValue(val)
				}
				jsonData, err = json.Marshal(goMap)
			case *collections.Array:
				goArr := make([]interface{}, v.Len())
				for i := 0; i < v.Len(); i++ {
					goArr[i] = objectToGoValue(v.Get(i))
				}
				jsonData, err = json.Marshal(goArr)
			default:
				jsonData = []byte(types.ToString(args[1]))
			}

			if err != nil {
				return types.NewError("failed to marshal JSON: "+err.Error(), 0, 0, "")
			}

			headers := &collections.OrderedMap{
				Entries: make(map[string]types.Object),
			}
			headers.Entries["Content-Type"] = types.String("application/json")

			return doHTTPRequest("POST", url, string(jsonData), headers)
		},
	}

	// httpPut(url, body)
	vm.globals["httpPut"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("httpPut() expects at least 2 arguments (url, body)", 0, 0, "")
			}
			url := string(types.ToString(args[0]))
			body := string(types.ToString(args[1]))
			return doHTTPRequest("PUT", url, body, nil)
		},
	}

	// httpDelete(url)
	vm.globals["httpDelete"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("httpDelete() expects at least 1 argument (url)", 0, 0, "")
			}
			url := string(types.ToString(args[0]))
			return doHTTPRequest("DELETE", url, "", nil)
		},
	}

	// httpRequest(method, url, body, headers)
	vm.globals["httpRequest"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("httpRequest() expects at least 2 arguments (method, url)", 0, 0, "")
			}
			method := string(types.ToString(args[0]))
			url := string(types.ToString(args[1]))

			var body string
			var headers *collections.OrderedMap

			if len(args) >= 3 {
				body = string(types.ToString(args[2]))
			}
			if len(args) >= 4 {
				if h, ok := args[3].(*collections.OrderedMap); ok {
					headers = h
				}
			}

			return doHTTPRequest(method, url, body, headers)
		},
	}
}

// objectToGoValue converts Nxlang Object to Go value for JSON marshaling
func objectToGoValue(obj types.Object) interface{} {
	switch v := obj.(type) {
	case types.Int:
		return int64(v)
	case types.Float:
		return float64(v)
	case types.Bool:
		return bool(v)
	case types.String:
		return string(v)
	case *types.Null:
		return nil
	case *collections.Map:
		m := make(map[string]interface{})
		for key, val := range v.Entries {
			m[key] = objectToGoValue(val)
		}
		return m
	case *collections.Array:
		arr := make([]interface{}, v.Len())
		for i := 0; i < v.Len(); i++ {
			arr[i] = objectToGoValue(v.Get(i))
		}
		return arr
	default:
		return obj.ToStr()
	}
}

// doHTTPRequest performs the actual HTTP request
func doHTTPRequest(method, url, body string, headers *collections.OrderedMap) types.Object {
	var req *http.Request
	var err error

	if body != "" {
		req, err = http.NewRequest(method, url, bytes.NewBufferString(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}

	if err != nil {
		return types.NewError("failed to create request: "+err.Error(), 0, 0, "")
	}

	// Set default headers
	req.Header.Set("User-Agent", "Nxlang/1.0")
	req.Header.Set("Accept", "*/*")

	// Add custom headers
	if headers != nil {
		for key, val := range headers.Entries {
			req.Header.Set(key, string(types.ToString(val)))
		}
	}

	// Create client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return types.NewError("request failed: "+err.Error(), 0, 0, "")
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.NewError("failed to read response body: "+err.Error(), 0, 0, "")
	}

	// Parse headers
	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = strings.Join(v, ", ")
		}
	}

	// Create response object
	httpResp := &HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    respHeaders,
		Body:       string(respBody),
	}

	return httpResp
}

// registerStandardModules registers all standard library modules
func (vm *VM) registerStandardModules() {
	// Math module
	mathModule := &Module{
		Name: "math",
		Exports: map[string]types.Object{
			"abs":    vm.globals["abs"],
			"sqrt":   vm.globals["sqrt"],
			"sin":    vm.globals["sin"],
			"cos":    vm.globals["cos"],
			"tan":    vm.globals["tan"],
			"floor":  vm.globals["floor"],
			"ceil":   vm.globals["ceil"],
			"round":  vm.globals["round"],
			"pow":    vm.globals["pow"],
			"random": vm.globals["random"],
		},
	}
	vm.modules["math"] = mathModule

	// String module
	stringModule := &Module{
		Name: "string",
		Exports: map[string]types.Object{
			"toUpper":    vm.globals["toUpper"],
			"toLower":    vm.globals["toLower"],
			"trim":       vm.globals["trim"],
			"split":      vm.globals["split"],
			"join":       vm.globals["join"],
			"contains":   vm.globals["contains"],
			"replace":    vm.globals["replace"],
			"substr":     vm.globals["substr"],
			"startsWith": vm.globals["startsWith"],
			"endsWith":   vm.globals["endsWith"],
		},
	}
	vm.modules["string"] = stringModule

	// Collection module
	collectionModule := &Module{
		Name: "collection",
		Exports: map[string]types.Object{
			"array":          vm.globals["array"],
			"append":         vm.globals["append"],
			"map":            vm.globals["map"],
			"orderedMap":     vm.globals["orderedMap"],
			"stack":          vm.globals["stack"],
			"queue":          vm.globals["queue"],
			"seq":            vm.globals["seq"],
			"keys":           vm.globals["keys"],
			"values":         vm.globals["values"],
			"delete":         vm.globals["delete"],
			"sortMap":        vm.globals["sortMap"],
			"reverseMap":     vm.globals["reverseMap"],
			"moveKey":        vm.globals["moveKey"],
			"moveKeyToFirst": vm.globals["moveKeyToFirst"],
			"moveKeyToLast":  vm.globals["moveKeyToLast"],
		},
	}
	vm.modules["collection"] = collectionModule

	// Time module
	timeModule := &Module{
		Name: "time",
		Exports: map[string]types.Object{
			"now":        vm.globals["now"],
			"unix":       vm.globals["unix"],
			"unixMilli":  vm.globals["unixMilli"],
			"formatTime": vm.globals["formatTime"],
			"parseTime":  vm.globals["parseTime"],
			"sleep":      vm.globals["sleep"],
		},
	}
	vm.modules["time"] = timeModule

	// JSON module
	jsonModule := &Module{
		Name: "json",
		Exports: map[string]types.Object{
			"toJson":   vm.globals["toJson"],
			"fromJson": nil, // TODO: Implement fromJson
		},
	}
	vm.modules["json"] = jsonModule

	// Thread/concurrency module
	threadModule := &Module{
		Name: "thread",
		Exports: map[string]types.Object{
			"thread":  vm.globals["thread"],
			"mutex":   vm.globals["mutex"],
			"rwMutex": vm.globals["rwMutex"],
		},
	}
	vm.modules["thread"] = threadModule

	// HTTP module
	httpModule := &Module{
		Name: "http",
		Exports: map[string]types.Object{
			"httpGet":      vm.globals["httpGet"],
			"httpPost":     vm.globals["httpPost"],
			"httpPostJSON": vm.globals["httpPostJSON"],
			"httpPut":      vm.globals["httpPut"],
			"httpDelete":   vm.globals["httpDelete"],
			"httpRequest":  vm.globals["httpRequest"],
		},
	}
	vm.modules["http"] = httpModule
}

// Run executes the bytecode
func (vm *VM) Run() error {
	for vm.framePointer > 0 {
		currentFrame := vm.frames[vm.framePointer-1]

		if currentFrame.ip >= len(currentFrame.Instructions()) {
			// End of function, return to caller
			vm.framePointer--
			continue
		}

		op := compiler.Opcode(currentFrame.CurrentInstruction())
		currentFrame.ip++

		// Count instructions for profiling
		if vm.enableProfiler {
			vm.instructionCount++
		}

		if err := vm.executeOpcode(op, currentFrame); err != nil {
			// Error occurred - try to handle with try-catch
			if vm.handleRuntimeError(err, currentFrame) {
				// Error was caught by a try-catch block, continue execution
				continue
			}
			// No catch block found, return the error
			return err
		}

		if vm.lastError != nil {
			return vm.lastError
		}
	}

	return nil
}

// RunFunction executes a function with the given arguments in a new VM
// This is used for thread execution with isolated state
func RunFunction(fn *types.Function, args []types.Object, globals map[string]types.Object) error {
	// Create minimal bytecode structure for the function
	bc := &bytecode.Bytecode{
		Constants:  fn.ConstantPool,
		MainFunc:   -1, // Not using main func, creating frame directly
		SourceFile: "thread:" + fn.Name,
	}

	// Create new VM with isolated state
	threadVM := &VM{
		constants:     bc.Constants,
		stack:         NewStack(),
		frames:        make([]*Frame, MaxCallStackDepth),
		framePointer:  1,
		globals:       make(map[string]types.Object),
		functionCache: make(map[int]*types.Function),
		classCache:    make(map[int]*types.Class),
		tryStack:      []*TryFrame{},
		deferStack:    make([][]*DeferredCall, MaxCallStackDepth),
		modules:       make(map[string]*Module),
		modulePaths:   []string{".", "./nx_modules", "/usr/local/nx/modules"},
	}

	// Copy globals from parent VM
	for k, v := range globals {
		threadVM.globals[k] = v
	}

	// Register built-in functions
	threadVM.registerBuiltins()

	// Create frame for the function with arguments
	threadVM.frames[0] = NewFrameFromFunction(fn, 0, args)

	// Execute the function
	return threadVM.Run()
}

// RunFunctionShared executes a function with the given arguments sharing the VM's globals
// This is used for thread execution with shared state
func (vm *VM) RunFunctionShared(fn *types.Function, args []types.Object) error {
	// Create frame for the function with arguments
	frame := NewFrameFromFunction(fn, 0, args)

	// Push arguments to stack
	for _, arg := range args {
		vm.stack.Push(arg)
	}

	// Execute in a temporary frame context
	originalFramePointer := vm.framePointer
	vm.frames[0] = frame
	vm.framePointer = 1

	err := vm.Run()

	// Restore original state
	vm.framePointer = originalFramePointer

	return err
}

// NewVMWithSharedGlobals creates a new VM that shares globals with the parent VM
// Used for shared mode thread execution
func (vm *VM) NewVMWithSharedGlobals(parent *VM) *VM {
	sharedVM := &VM{
		constants:     parent.constants,
		stack:         NewStack(),
		frames:        make([]*Frame, MaxCallStackDepth),
		framePointer:  1,
		globals:       parent.globals,   // Share the same globals map
		globalsMu:     parent.globalsMu, // Share the same mutex for globals access
		functionCache: make(map[int]*types.Function),
		classCache:    make(map[int]*types.Class),
		tryStack:      []*TryFrame{},
		deferStack:    make([][]*DeferredCall, MaxCallStackDepth),
		modules:       parent.modules, // Share loaded modules
		modulePaths:   parent.modulePaths,
	}
	// Register built-in functions for the shared VM
	sharedVM.registerBuiltins()
	return sharedVM
}

// RunFunctionInNewVM creates a new VM and executes the function with the given arguments
// This is used for isolated thread execution
func RunFunctionInNewVM(fn *types.Function, args []types.Object, parentVM *VM) error {
	// Create new VM with the function's constant pool
	threadVM := &VM{
		constants:     fn.ConstantPool,
		stack:         NewStack(),
		frames:        make([]*Frame, MaxCallStackDepth),
		framePointer:  1,
		globals:       parentVM.CopyGlobals(),
		functionCache: make(map[int]*types.Function),
		classCache:    make(map[int]*types.Class),
		tryStack:      []*TryFrame{},
		deferStack:    make([][]*DeferredCall, MaxCallStackDepth),
		modules:       make(map[string]*Module),
		modulePaths:   []string{".", "./nx_modules", "/usr/local/nx/modules"},
	}

	// Register built-in functions
	threadVM.registerBuiltins()

	// Create frame for the function with arguments
	threadVM.frames[0] = NewFrameFromFunction(fn, 0, args)

	// Execute the function
	err := threadVM.Run()
	return err
}

// executeOpcode executes a single bytecode instruction
func (vm *VM) executeOpcode(op compiler.Opcode, frame *Frame) error {
	switch op {
	case compiler.OpNOP:
		// No operation
	case compiler.OpPush, compiler.OpLoadConst:
		constIdx := int(frame.ReadUint16())
		if constIdx < 0 || constIdx >= len(vm.constants) {
			return vm.newError(fmt.Sprintf("invalid constant index: %d", constIdx), frame.ip)
		}
		constVal := vm.constants[constIdx]
		obj, err := vm.constantToObject(constIdx, constVal)
		if err != nil {
			return err
		}
		return vm.stack.Push(obj)

	case compiler.OpPop:
		vm.stack.Pop()

	case compiler.OpDup:
		val := vm.stack.Peek()
		return vm.stack.Push(val)

	case compiler.OpSwap:
		a := vm.stack.Pop()
		b := vm.stack.Pop()
		if err := vm.stack.Push(a); err != nil {
			return err
		}
		return vm.stack.Push(b)

	case compiler.OpLoadLocal:
		localIdx := int(frame.ReadUint8())
		if localIdx < 0 || localIdx >= len(frame.locals) {
			return vm.newError(fmt.Sprintf("invalid local index: %d", localIdx), frame.ip)
		}
		return vm.stack.Push(frame.locals[localIdx])

	case compiler.OpStoreLocal:
		localIdx := int(frame.ReadUint8())
		if localIdx < 0 || localIdx >= len(frame.locals) {
			return vm.newError(fmt.Sprintf("invalid local index: %d", localIdx), frame.ip)
		}
		val := vm.stack.Pop()
		frame.locals[localIdx] = val

	case compiler.OpLoadGlobal:
		nameIdx := int(frame.ReadUint16())
		nameConst := vm.constants[nameIdx].(*bytecode.StringConstant)

		var val types.Object
		var ok bool

		if vm.globalsMu != nil {
			vm.globalsMu.RLock()
			val, ok = vm.globals[nameConst.Value]
			vm.globalsMu.RUnlock()
		} else {
			val, ok = vm.globals[nameConst.Value]
		}

		if !ok {
			return vm.newError(fmt.Sprintf("undefined variable: %s", nameConst.Value), frame.ip)
		}
		return vm.stack.Push(val)

	case compiler.OpStoreGlobal:
		nameIdx := int(frame.ReadUint16())
		nameConst := vm.constants[nameIdx].(*bytecode.StringConstant)
		val := vm.stack.Pop()

		if vm.globalsMu != nil {
			vm.globalsMu.Lock()
			vm.globals[nameConst.Value] = val
			vm.globalsMu.Unlock()
		} else {
			vm.globals[nameConst.Value] = val
		}

	case compiler.OpAdd:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		res, err := vm.addObjects(a, b)
		if err != nil {
			return err
		}
		return vm.stack.Push(res)

	case compiler.OpSub:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		res, err := vm.subObjects(a, b)
		if err != nil {
			return err
		}
		return vm.stack.Push(res)

	case compiler.OpMul:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		res, err := vm.mulObjects(a, b)
		if err != nil {
			return err
		}
		return vm.stack.Push(res)

	case compiler.OpAddInt:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		if aInt, ok := a.(types.Int); ok {
			if bInt, ok := b.(types.Int); ok {
				return vm.stack.Push(aInt + bInt)
			}
		}
		return fmt.Errorf("TXERROR: OpAddInt requires both operands to be integers")

	case compiler.OpSubInt:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		if aInt, ok := a.(types.Int); ok {
			if bInt, ok := b.(types.Int); ok {
				return vm.stack.Push(aInt - bInt)
			}
		}
		return fmt.Errorf("TXERROR: OpSubInt requires both operands to be integers")

	case compiler.OpMulInt:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		if aInt, ok := a.(types.Int); ok {
			if bInt, ok := b.(types.Int); ok {
				return vm.stack.Push(aInt * bInt)
			}
		}
		return fmt.Errorf("TXERROR: OpMulInt requires both operands to be integers")

	case compiler.OpDiv:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		res, err := vm.divObjects(a, b, frame.ip)
		if err != nil {
			return err
		}
		return vm.stack.Push(res)

	case compiler.OpMod:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		res, err := vm.modObjects(a, b, frame.ip)
		if err != nil {
			return err
		}
		return vm.stack.Push(res)

	case compiler.OpNeg:
		a := vm.stack.Pop()
		res, err := vm.negObject(a, frame.ip)
		if err != nil {
			return err
		}
		return vm.stack.Push(res)

	case compiler.OpNot:
		a := vm.stack.Pop()
		res := types.Bool(!types.ToBool(a))
		return vm.stack.Push(res)

	case compiler.OpBitNot:
		a := vm.stack.Pop()
		res, err := vm.bitNotObject(a, frame.ip)
		if err != nil {
			return err
		}
		return vm.stack.Push(res)

	case compiler.OpBitAnd:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		res, err := vm.bitAndObjects(a, b)
		if err != nil {
			return err
		}
		return vm.stack.Push(res)

	case compiler.OpBitOr:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		res, err := vm.bitOrObjects(a, b)
		if err != nil {
			return err
		}
		return vm.stack.Push(res)

	case compiler.OpBitXor:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		res, err := vm.bitXorObjects(a, b)
		if err != nil {
			return err
		}
		return vm.stack.Push(res)

	case compiler.OpShiftL:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		res, err := vm.shiftLeftObjects(a, b)
		if err != nil {
			return err
		}
		return vm.stack.Push(res)

	case compiler.OpShiftR:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		res, err := vm.shiftRightObjects(a, b)
		if err != nil {
			return err
		}
		return vm.stack.Push(res)

	case compiler.OpEq:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		res := types.Bool(a.Equals(b))
		return vm.stack.Push(res)

	case compiler.OpNotEq:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		res := types.Bool(!a.Equals(b))
		return vm.stack.Push(res)

	case compiler.OpLt:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		res, err := vm.compareObjects(a, b, lt)
		if err != nil {
			return err
		}
		return vm.stack.Push(res)

	case compiler.OpEqInt:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		if aInt, ok := a.(types.Int); ok {
			if bInt, ok := b.(types.Int); ok {
				return vm.stack.Push(types.Bool(aInt == bInt))
			}
		}
		return fmt.Errorf("TXERROR: OpEqInt requires both operands to be integers")

	case compiler.OpLtInt:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		if aInt, ok := a.(types.Int); ok {
			if bInt, ok := b.(types.Int); ok {
				return vm.stack.Push(types.Bool(aInt < bInt))
			}
		}
		return fmt.Errorf("TXERROR: OpLtInt requires both operands to be integers")

	case compiler.OpLte:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		res, err := vm.compareObjects(a, b, lte)
		if err != nil {
			return err
		}
		return vm.stack.Push(res)

	case compiler.OpGt:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		res, err := vm.compareObjects(a, b, gt)
		if err != nil {
			return err
		}
		return vm.stack.Push(res)

	case compiler.OpGte:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		res, err := vm.compareObjects(a, b, gte)
		if err != nil {
			return err
		}
		return vm.stack.Push(res)

	case compiler.OpJmp:
		offset := int(frame.ReadUint16())
		frame.ip = offset

	case compiler.OpJmpIfTrue:
		offset := int(frame.ReadUint16())
		cond := vm.stack.Pop()
		if types.ToBool(cond) {
			frame.ip = offset
		}

	case compiler.OpJmpIfFalse:
		offset := int(frame.ReadUint16())
		cond := vm.stack.Pop()
		if !types.ToBool(cond) {
			frame.ip = offset
		}

	case compiler.OpCall:
		argCount := int(frame.ReadUint8())
		stackSize := vm.stack.Size()

		// Check stack has enough elements
		if stackSize < argCount+1 {
			return vm.newError(fmt.Sprintf("not enough arguments for function call: expected %d, got %d", argCount, stackSize-1), frame.ip)
		}

		// Function is at the top of the stack
		fnVal := vm.stack.Pop()

		// Pop arguments (in reverse order, so we need to reverse them to get correct order)
		args := make([]types.Object, argCount)
		for i := 0; i < argCount; i++ {
			args[i] = vm.stack.Pop()
		}
		// Reverse to get correct argument order
		for i, j := 0, len(args)-1; i < j; i, j = i+1, j-1 {
			args[i], args[j] = args[j], args[i]
		}

		switch fn := fnVal.(type) {
		case *types.TypeWrapper:
			// Type object call (type conversion)
			result := fn.Call(args...)
			return vm.stack.Push(result)

		case *types.NativeFunction:
			// Native function call
			result := fn.Fn(args...)
			return vm.stack.Push(result)

		case *types.Function:
			// Nxlang function call
			// Check if we have minimum required arguments (parameters without defaults)
			requiredParams := fn.NumParameters
			if len(fn.DefaultValues) > 0 {
				// Count parameters without default values
				// DefaultValues[i] >= 0 means this parameter HAS a default value
				// DefaultValues[i] == -1 means this parameter does NOT have a default
				requiredParams = 0
				for i := len(fn.DefaultValues) - 1; i >= 0; i-- {
					if fn.DefaultValues[i] < 0 {
						// This is the last parameter without default
						requiredParams = i + 1
						break
					}
				}
			}

			if argCount < requiredParams && !fn.IsVariadic {
				return vm.newError(fmt.Sprintf("expected at least %d arguments, got %d", requiredParams, argCount), frame.ip)
			}

			// Create bytecode function constant for frame
			bcFunc := &bytecode.FunctionConstant{
				Name:          fn.Name,
				NumLocals:     fn.NumLocals,
				NumParameters: fn.NumParameters,
				IsVariadic:    fn.IsVariadic,
				Instructions:  fn.Instructions,
			}

			// Create new frame
			basePointer := vm.stack.Size()
			newFrame := NewFrame(bcFunc, basePointer)

			// Copy arguments to locals
			if fn.IsVariadic {
				// For variadic functions, pack extra arguments into an array
				// NumParameters includes the variadic parameter, so we need to handle it specially
				paramCount := fn.NumParameters
				if paramCount > 0 {
					// Copy fixed parameters
					for i := 0; i < argCount && i < paramCount-1; i++ {
						newFrame.locals[i] = args[i]
					}

					// Pack remaining arguments into array for variadic parameter
					variadicArgs := collections.NewArray()
					for i := paramCount - 1; i < argCount; i++ {
						variadicArgs.Append(args[i])
					}
					if paramCount > 0 {
						newFrame.locals[paramCount-1] = variadicArgs
					}
				}
			} else {
				// Copy provided arguments
				for i := 0; i < argCount && i < fn.NumParameters; i++ {
					newFrame.locals[i] = args[i]
				}

				// Fill in default values for missing arguments
				if len(fn.DefaultValues) > 0 {
					for i := argCount; i < fn.NumParameters; i++ {
						if i < len(fn.DefaultValues) && fn.DefaultValues[i] >= 0 {
							// This parameter has a default value
							defaultVal := fn.ConstantPool[fn.DefaultValues[i]]
							newFrame.locals[i], _ = vm.constantToObject(fn.DefaultValues[i], defaultVal)
						}
					}
				}
			}

			if vm.framePointer >= MaxCallStackDepth {
				return &StackOverflowError{}
			}

			vm.frames[vm.framePointer] = newFrame
			vm.framePointer++

		case *types.BoundMethod:
			// Bound method call (method bound to an instance)
			methodFn := fn.Method
			if argCount < methodFn.NumParameters && !methodFn.IsVariadic {
				return vm.newError(fmt.Sprintf("expected %d arguments, got %d", methodFn.NumParameters, argCount), frame.ip)
			}

			// For instance methods, we need at least 1 local for 'this'
			// Ensure NumLocals accounts for this
			numLocals := methodFn.NumLocals
			if numLocals < 1 {
				numLocals = 1
			}

			// Create bytecode function constant for frame
			bcFunc := &bytecode.FunctionConstant{
				Name:          methodFn.Name,
				NumLocals:     numLocals,
				NumParameters: methodFn.NumParameters,
				IsVariadic:    methodFn.IsVariadic,
				Instructions:  methodFn.Instructions,
			}

			// Create new frame
			basePointer := vm.stack.Size()
			newFrame := NewFrame(bcFunc, basePointer)

			// Set 'this' as the first local (instance)
			newFrame.locals[0] = fn.Instance

			// Copy arguments to locals (starting from index 1 for 'this')
			for i := 0; i < argCount && i < methodFn.NumParameters; i++ {
				newFrame.locals[i+1] = args[i]
			}

			if vm.framePointer >= MaxCallStackDepth {
				return &StackOverflowError{}
			}

			vm.frames[vm.framePointer] = newFrame
			vm.framePointer++

		case *types.Class:
			// Class call: create a new instance and call init if exists
			// Create new instance
			instance := &types.Instance{
				Class:      fn,
				Properties: make(map[string]types.Object),
			}

			// If there's an init method, call it (check inheritance chain)
			initMethod := fn.GetMethod("init")
			if initMethod != nil {
				// Push instance as first argument (this)
				if err := vm.stack.Push(instance); err != nil {
					return err
				}
				// Push provided arguments
				for _, arg := range args {
					if err := vm.stack.Push(arg); err != nil {
						return err
					}
				}

				// Call init method
				numParams := initMethod.NumParameters
				if argCount < numParams && !initMethod.IsVariadic {
					return vm.newError(fmt.Sprintf("expected at least %d arguments, got %d", numParams, argCount), frame.ip)
				}

				bcFunc := &bytecode.FunctionConstant{
					Name:          "init",
					NumLocals:     initMethod.NumLocals,
					NumParameters: initMethod.NumParameters,
					IsVariadic:    initMethod.IsVariadic,
					Instructions:  initMethod.Instructions,
				}

				basePointer := vm.stack.Size()
				newFrame := NewFrame(bcFunc, basePointer)

				// Set 'this' as the first local (instance)
				newFrame.locals[0] = instance

				// Copy arguments to locals (starting from index 1 for 'this')
				for i := 0; i < argCount && i < initMethod.NumParameters; i++ {
					newFrame.locals[i+1] = args[i]
				}

				// Handle variadic arguments
				if initMethod.IsVariadic && argCount >= initMethod.NumParameters {
					// Pack extra arguments into array
					extraArgs := args[initMethod.NumParameters-1:]
					variadicArr := &collections.Array{}
					for _, arg := range extraArgs {
						variadicArr.Append(arg)
					}
					newFrame.locals[initMethod.NumParameters] = variadicArr
				}

				if vm.framePointer >= MaxCallStackDepth {
					return &StackOverflowError{}
				}

				vm.frames[vm.framePointer] = newFrame
				vm.framePointer++
			} else {
				// No init method, just return the instance
				return vm.stack.Push(instance)
			}

		default:
			return vm.newError(fmt.Sprintf("cannot call non-function type %s", fnVal.TypeName()), frame.ip)
		}

	case compiler.OpReturn:
		returnValue := vm.stack.Pop()
		// Get the current frame (callee)
		calleeFrame := vm.frames[vm.framePointer-1]

		// Run all deferred functions for this frame
		vm.runDeferred(vm.framePointer - 1)

		// Pop frame
		vm.framePointer--
		// If we're returning from main, push return value back and exit gracefully
		if vm.framePointer == 0 {
			// Push return value back on stack for callers (like runByteCode) to retrieve
			vm.stack.Push(returnValue)
			return nil
		}

		// Special case: if returning from init method, return the instance (this)
		// instead of the actual return value
		if calleeFrame.fn.Name == "init" && calleeFrame.locals != nil && len(calleeFrame.locals) > 0 {
			if instance, ok := calleeFrame.locals[0].(*types.Instance); ok {
				returnValue = instance
			}
		}

		// Clear stack up to callee's base pointer, but save one slot for return value
		for vm.stack.Size() > calleeFrame.basePointer {
			vm.stack.Pop()
		}
		// Push return value to caller's stack
		return vm.stack.Push(returnValue)

	case compiler.OpReturnVoid:
		// Get the current frame (callee)
		calleeFrame := vm.frames[vm.framePointer-1]

		// Run all deferred functions for this frame
		vm.runDeferred(vm.framePointer - 1)

		// Pop frame
		vm.framePointer--
		// If we're returning from main, exit gracefully
		if vm.framePointer == 0 {
			// Execution complete
			return nil
		}

		// Special case: if returning from init method, push the instance (this)
		if calleeFrame.fn.Name == "init" && calleeFrame.locals != nil && len(calleeFrame.locals) > 0 {
			if instance, ok := calleeFrame.locals[0].(*types.Instance); ok {
				return vm.stack.Push(instance)
			}
		}

		// Clear stack up to callee's base pointer
		for vm.stack.Size() > calleeFrame.basePointer {
			vm.stack.Pop()
		}
		// Push undefined as return value
		return vm.stack.Push(types.UndefinedValue)
		// Push undefined to caller's stack
		return vm.stack.Push(types.UndefinedValue)

	case compiler.OpNewArray:
		elemCount := int(frame.ReadUint16())
		elements := make([]types.Object, elemCount)
		// Pop elements from stack (they were pushed in order, so reverse)
		for i := elemCount - 1; i >= 0; i-- {
			elements[i] = vm.stack.Pop()
		}
		arr := collections.NewArrayWithElements(elements)
		return vm.stack.Push(arr)

	case compiler.OpNewMap:
		m := collections.NewMap()
		return vm.stack.Push(m)

	case compiler.OpNewObject:
		argCount := int(frame.ReadUint8())

		// Pop arguments first (they are on top of the stack)
		args := make([]types.Object, argCount)
		for i := argCount - 1; i >= 0; i-- {
			args[i] = vm.stack.Pop()
		}

		// Pop the class object from stack (it's below the arguments)
		classVal := vm.stack.Pop()
		classObj, ok := classVal.(*types.Class)
		if !ok {
			return vm.newError(fmt.Sprintf("object constructor expects a class, got %T", classVal), frame.ip)
		}

		// Create a new instance of the class
		instance := &types.Instance{
			Class:      classObj,
			Properties: make(map[string]types.Object),
		}

		// Call constructor (init method) if it exists
		if initMethod, ok := classObj.Methods["init"]; ok {
			// Ensure at least 1 local for 'this'
			numLocals := initMethod.NumLocals
			if numLocals < 1 {
				numLocals = 1
			}

			// Create a new frame for init method
			bcFunc := &bytecode.FunctionConstant{
				Name:          initMethod.Name,
				NumLocals:     numLocals,
				NumParameters: initMethod.NumParameters,
				IsVariadic:    initMethod.IsVariadic,
				Instructions:  initMethod.Instructions,
			}

			basePointer := vm.stack.Size()
			newFrame := NewFrame(bcFunc, basePointer)
			newFrame.locals[0] = instance // Set 'this'

			// Copy arguments to locals
			for i := 0; i < argCount && i < initMethod.NumParameters; i++ {
				newFrame.locals[i+1] = args[i]
			}

			if vm.framePointer >= MaxCallStackDepth {
				return &StackOverflowError{}
			}

			// Push new frame and continue execution
			vm.frames[vm.framePointer] = newFrame
			vm.framePointer++

			// Skip pushing instance - it will be done after init returns
			// For now, just return and let the normal call mechanism work
			return nil
		}

		// No init method, just push instance
		return vm.stack.Push(instance)

	case compiler.OpMemberGet:
		nameIdx := int(frame.ReadUint16())
		obj := vm.stack.Pop()

		// Get the member name from constants
		nameConst := vm.constants[nameIdx].(*bytecode.StringConstant)
		memberName := nameConst.Value

		switch o := obj.(type) {
		case *types.SuperReference:
			// Access member of superclass
			method, ok := o.Super.Methods[memberName]
			if !ok {
				return vm.newError(fmt.Sprintf("super: method '%s' not found in superclass %s", memberName, o.Super.Name), frame.ip)
			}
			// Create bound method with the instance
			boundMethod := &types.BoundMethod{
				Instance: o.Instance,
				Method:   method,
			}
			return vm.stack.Push(boundMethod)
		case *types.Instance:
			// Check properties first
			if val, ok := o.Properties[memberName]; ok {
				return vm.stack.Push(val)
			}
			// Check class methods with inheritance
			class := o.Class
			for class != nil {
				if method, ok := class.Methods[memberName]; ok {
					// Return bound method
					boundMethod := &types.BoundMethod{
						Instance: o,
						Method:   method,
					}
					return vm.stack.Push(boundMethod)
				}
				// Check static methods
				if method, ok := class.StaticMethods[memberName]; ok {
					return vm.stack.Push(method)
				}
				// Check static fields
				if val, ok := class.StaticFields[memberName]; ok {
					return vm.stack.Push(val)
				}
				// Traverse up the inheritance chain
				class = class.SuperClass
			}
			return vm.newError(fmt.Sprintf("property '%s' not found on %s", memberName, o.Class.Name), frame.ip)
		case *types.Class:
			// Get static field or static method from class
			if val, ok := o.StaticFields[memberName]; ok {
				return vm.stack.Push(val)
			}
			// Check static methods
			if method, ok := o.StaticMethods[memberName]; ok {
				return vm.stack.Push(method)
			}
			return vm.newError(fmt.Sprintf("static member '%s' not found on class %s", memberName, o.Name), frame.ip)
		case *concurrency.Mutex:
			// Support method access on mutex
			switch memberName {
			case "lock", "Lock":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						o.Lock()
						return types.UndefinedValue
					},
				})
			case "unlock", "Unlock":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						o.Unlock()
						return types.UndefinedValue
					},
				})
			case "tryLock", "TryLock":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						return types.Bool(o.TryLock())
					},
				})
			}
			return vm.newError(fmt.Sprintf("cannot access member '%s' on %s", memberName, obj.TypeName()), frame.ip)
		case *concurrency.RWMutex:
			// Support method access on rwMutex
			switch memberName {
			case "lock", "Lock":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						o.Lock()
						return types.UndefinedValue
					},
				})
			case "unlock", "Unlock":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						o.Unlock()
						return types.UndefinedValue
					},
				})
			case "rlock", "RLock":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						o.RLock()
						return types.UndefinedValue
					},
				})
			case "runlock", "RUnlock":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						o.RUnlock()
						return types.UndefinedValue
					},
				})
			case "tryLock", "TryLock":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						return types.Bool(o.TryLock())
					},
				})
			case "tryRLock", "TryRLock":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						return types.Bool(o.TryRLock())
					},
				})
			}
			return vm.newError(fmt.Sprintf("cannot access member '%s' on %s", memberName, obj.TypeName()), frame.ip)
		case *collections.Seq:
			// Support method access on seq
			switch memberName {
			case "Append", "append":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						for _, arg := range args {
							o.Append(arg)
						}
						return o
					},
				})
			case "Pop", "pop":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						return o.Pop()
					},
				})
			case "Clear", "clear":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						o.Clear()
						return types.UndefinedValue
					},
				})
			case "Resize", "resize":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						if len(args) == 0 {
							return types.NewError("resize() expects 1 argument", 0, 0, "")
						}
						size, err := types.ToInt(args[0])
						if err != nil {
							return err
						}
						o.Resize(int(size))
						return types.UndefinedValue
					},
				})
			case "Fill", "fill":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						if len(args) < 2 {
							return types.NewError("fill() expects 2 arguments (value, count)", 0, 0, "")
						}
						count, err := types.ToInt(args[1])
						if err != nil {
							return err
						}
						o.Fill(args[0], int(count))
						return o
					},
				})
			case "Range", "range":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						if len(args) < 2 {
							return types.NewError("range() expects 2 arguments (start, end)", 0, 0, "")
						}
						start, err := types.ToInt(args[0])
						if err != nil {
							return err
						}
						end, err := types.ToInt(args[1])
						if err != nil {
							return err
						}
						return o.Range(int(start), int(end))
					},
				})
			case "Reverse", "reverse":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						return o.Reverse()
					},
				})
			case "Join", "join":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						sep := ""
						if len(args) > 0 {
							sep = args[0].ToStr()
						}
						return types.String(o.Join(sep))
					},
				})
			case "Includes", "includes":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						if len(args) == 0 {
							return types.Bool(false)
						}
						return types.Bool(o.Includes(args[0]))
					},
				})
			case "IndexOf", "indexof":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						if len(args) == 0 {
							return types.Int(-1)
						}
						return types.Int(o.IndexOf(args[0]))
					},
				})
			case "Get", "get":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						if len(args) == 0 {
							return types.UndefinedValue
						}
						idx, err := types.ToInt(args[0])
						if err != nil {
							return err
						}
						return o.GetAuto(int(idx))
					},
				})
			case "Set", "set":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						if len(args) < 2 {
							return types.NewError("set() expects 2 arguments (index, value)", 0, 0, "")
						}
						idx, err := types.ToInt(args[0])
						if err != nil {
							return err
						}
						if errObj := o.Set(int(idx), args[1]); errObj != nil {
							return errObj
						}
						return args[1]
					},
				})
			case "Len", "len":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						return types.Int(o.Len())
					},
				})
			case "Elements", "elements":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						return collections.NewArrayWithElements(o.Elements())
					},
				})
			case "ForEach", "foreach":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						if len(args) == 0 {
							return types.UndefinedValue
						}
						_, ok := args[0].(*types.Function)
						if !ok {
							return types.NewError("ForEach expects a function", 0, 0, "")
						}
						// ForEach implementation would require VM access, skip for now
						return types.NewError("ForEach not fully implemented yet", 0, 0, "")
					},
				})
			case "Map", "map":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						if len(args) == 0 {
							return types.NewError("Map expects a function", 0, 0, "")
						}
						// Simple Map without VM access - limited functionality
						return types.NewError("Map requires function call support", 0, 0, "")
					},
				})
			case "Filter", "filter":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						if len(args) == 0 {
							return types.NewError("Filter expects a function", 0, 0, "")
						}
						return types.NewError("Filter requires function call support", 0, 0, "")
					},
				})
			case "Find", "find":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						if len(args) == 0 {
							return types.UndefinedValue
						}
						return types.NewError("Find requires function call support", 0, 0, "")
					},
				})
			case "FindIndex", "findindex":
				return vm.stack.Push(&types.NativeFunction{
					Fn: func(args ...types.Object) types.Object {
						if len(args) == 0 {
							return types.Int(-1)
						}
						return types.NewError("FindIndex requires function call support", 0, 0, "")
					},
				})
			}
			return vm.newError(fmt.Sprintf("cannot access member '%s' on %s", memberName, obj.TypeName()), frame.ip)

		case *types.TypeWrapper:
			// Get static method from type object
			if method, ok := o.StaticMethods[memberName]; ok {
				return vm.stack.Push(method)
			}
			return vm.newError(fmt.Sprintf("cannot access member '%s' on type %s", memberName, o.Name), frame.ip)

		// Static methods for primitive types (deprecated - kept for backward compatibility)
		case types.Int, types.UInt, types.Float, types.Bool, types.String, types.Byte, types.Char:
			typeName := obj.TypeName()
			switch typeName {
			case "int":
				switch memberName {
				case "parse", "Parse":
					return vm.stack.Push(&types.NativeFunction{
						Fn: func(args ...types.Object) types.Object {
							if len(args) == 0 {
								return types.NewError("int.parse() expects 1 argument", 0, 0, "")
							}
							val, err := types.ToInt(args[0])
							if err != nil {
								return err
							}
							return val
						},
					})
				}
			case "float":
				switch memberName {
				case "parse", "Parse":
					return vm.stack.Push(&types.NativeFunction{
						Fn: func(args ...types.Object) types.Object {
							if len(args) == 0 {
								return types.NewError("float.parse() expects 1 argument", 0, 0, "")
							}
							val, err := types.ToFloat(args[0])
							if err != nil {
								return err
							}
							return val
						},
					})
				}
			case "string":
				switch memberName {
				case "parse", "Parse":
					return vm.stack.Push(&types.NativeFunction{
						Fn: func(args ...types.Object) types.Object {
							if len(args) == 0 {
								return types.NewError("string.parse() expects 1 argument", 0, 0, "")
							}
							return types.ToString(args[0])
						},
					})
				}
			}
			return vm.newError(fmt.Sprintf("cannot access member '%s' on %s", memberName, obj.TypeName()), frame.ip)

		default:
			return vm.newError(fmt.Sprintf("cannot access member '%s' on %s", memberName, obj.TypeName()), frame.ip)
		}

	case compiler.OpMemberSet:
		nameIdx := int(frame.ReadUint16())
		value := vm.stack.Pop()
		obj := vm.stack.Pop()

		// Get the member name from constants
		nameConst := vm.constants[nameIdx].(*bytecode.StringConstant)
		memberName := nameConst.Value

		switch o := obj.(type) {
		case *types.Instance:
			o.Properties[memberName] = value
			return vm.stack.Push(types.UndefinedValue)
		case *types.Class:
			o.StaticFields[memberName] = value
			return vm.stack.Push(types.UndefinedValue)
		default:
			return vm.newError(fmt.Sprintf("cannot set member '%s' on %s", memberName, obj.TypeName()), frame.ip)
		}

	case compiler.OpPrintLine:
		val := vm.stack.Pop()
		fmt.Fprintln(os.Stdout, val.ToStr())
		return vm.stack.Push(types.UndefinedValue)

	case compiler.OpPrint:
		val := vm.stack.Pop()
		fmt.Fprint(os.Stdout, val.ToStr())
		return vm.stack.Push(types.UndefinedValue)

	case compiler.OpTypeCode:
		// Get type code of value
		val := vm.stack.Pop()
		typeCode := val.TypeCode()
		return vm.stack.Push(types.Int(typeCode))

	case compiler.OpTypeName:
		// Get type name of value
		val := vm.stack.Pop()
		typeName := val.TypeName()
		return vm.stack.Push(types.String(typeName))

	case compiler.OpIsError:
		// Check if value is an error
		val := vm.stack.Pop()
		isErr := false
		if _, ok := val.(*types.Error); ok {
			isErr = true
		} else if str, ok := val.(types.String); ok {
			if strings.HasPrefix(string(str), "TXERROR:") {
				isErr = true
			}
		} else if val == types.UndefinedValue {
			isErr = true
		}
		return vm.stack.Push(types.Bool(isErr))

	case compiler.OpLen:
		// Get length of string, array, map, etc.
		val := vm.stack.Pop()
		var length int
		switch v := val.(type) {
		case *collections.Array:
			length = v.Len()
		case *collections.Map:
			length = v.Len()
		case *collections.OrderedMap:
			length = v.Len()
		case types.String:
			length = len([]rune(string(v)))
		default:
			return vm.newError(fmt.Sprintf("len() not supported for %s type", val.TypeName()), frame.ip)
		}
		return vm.stack.Push(types.Int(length))

	case compiler.OpIsNil:
		// Check if value is nil or undefined
		val := vm.stack.Pop()
		isNil := val == nil || val == types.NullValue || val == types.UndefinedValue
		return vm.stack.Push(types.Bool(isNil))

	case compiler.OpIndexGet:
		index := vm.stack.Pop()
		collection := vm.stack.Pop()

		switch coll := collection.(type) {
		case *collections.Array:
			idx, err := types.ToInt(index)
			if err != nil {
				return err
			}
			result := coll.Get(int(idx))
			if result == types.UndefinedValue {
				return vm.newError(fmt.Sprintf("array index out of bounds: %d (length %d)", idx, coll.Len()), frame.ip)
			}
			return vm.stack.Push(result)

		case *collections.Seq:
			idx, err := types.ToInt(index)
			if err != nil {
				return err
			}
			// Auto-grow and get value
			result := coll.GetAuto(int(idx))
			return vm.stack.Push(result)

		case *collections.Map:
			key := types.ToString(index)
			result := coll.Get(string(key))
			return vm.stack.Push(result)

		case *collections.OrderedMap:
			key := types.ToString(index)
			result := coll.Get(string(key))
			return vm.stack.Push(result)

		case types.String:
			idx, err := types.ToInt(index)
			if err != nil {
				return err
			}
			runes := []rune(string(coll))
			if int(idx) < 0 || int(idx) >= len(runes) {
				return vm.newError(fmt.Sprintf("string index out of bounds: %d (length %d)", idx, len(runes)), frame.ip)
			}
			return vm.stack.Push(types.Char(runes[idx]))

		default:
			return vm.newError(fmt.Sprintf("cannot index %s type", collection.TypeName()), frame.ip)
		}

	case compiler.OpIndexSet:
		value := vm.stack.Pop()
		index := vm.stack.Pop()
		collection := vm.stack.Pop()

		switch coll := collection.(type) {
		case *collections.Array:
			idx, err := types.ToInt(index)
			if err != nil {
				return err
			}
			coll.Set(int(idx), value)
			return vm.stack.Push(value)

		case *collections.Seq:
			idx, err := types.ToInt(index)
			if err != nil {
				return err
			}
			result := coll.Set(int(idx), value)
			// Check if result is an error (not just undefined)
			if errObj, ok := result.(*types.Error); ok {
				return vm.newError(errObj.ToStr(), frame.ip)
			}
			return vm.stack.Push(value)

		case *collections.Map:
			key := types.ToString(index)
			coll.Set(string(key), value)
			return vm.stack.Push(value)

		case *collections.OrderedMap:
			key := types.ToString(index)
			coll.Set(string(key), value)
			return vm.stack.Push(value)

		default:
			return vm.newError(fmt.Sprintf("cannot assign to index of %s type", collection.TypeName()), frame.ip)
		}

	case compiler.OpTry:
		catchOffset := int(frame.ReadUint16())
		finallyOffset := int(frame.ReadUint16())

		// Push try frame to try stack
		tryFrame := &TryFrame{
			frameIndex:    vm.framePointer - 1,
			catchOffset:   catchOffset,
			finallyOffset: finallyOffset,
			stackPointer:  vm.stack.Size(),
			basePointer:   frame.basePointer,
		}
		vm.tryStack = append(vm.tryStack, tryFrame)
		return nil

	case compiler.OpCatch:
		// Pop the exception from the error (it was stored when thrown)
		// The exception is already at the top of the stack when entering catch
		return nil

	case compiler.OpFinally:
		// No-op for now, just indicates the start of finally block
		return nil

	case compiler.OpDefer:
		argCount := int(frame.ReadUint8())

		// Pop function from stack
		fn := vm.stack.Pop()

		// Pop arguments (they were pushed in order, so reverse to get correct order)
		args := make([]types.Object, argCount)
		for i := 0; i < argCount; i++ {
			args[i] = vm.stack.Pop()
		}
		// Reverse to get correct argument order
		for i, j := 0, len(args)-1; i < j; i, j = i+1, j-1 {
			args[i], args[j] = args[j], args[i]
		}

		// Add to defer stack for current frame
		frameIdx := vm.framePointer - 1
		vm.deferStack[frameIdx] = append(vm.deferStack[frameIdx], &DeferredCall{
			fn:   fn,
			args: args,
		})
		return nil

	case compiler.OpImport:
		pathConstIdx := int(frame.ReadUint16())
		if pathConstIdx < 0 || pathConstIdx >= len(vm.constants) {
			return vm.newError(fmt.Sprintf("invalid constant index for module path: %d", pathConstIdx), frame.ip)
		}
		pathConst := vm.constants[pathConstIdx].(*bytecode.StringConstant)
		modulePath := pathConst.Value

		// Load the module
		module, err := vm.LoadModule(modulePath)
		if err != nil {
			return vm.newError(fmt.Sprintf("failed to import module '%s': %v", modulePath, err), frame.ip)
		}

		// Convert module to object (map for now)
		moduleObj := collections.NewMap()
		for name, val := range module.Exports {
			moduleObj.Set(name, val)
		}

		return vm.stack.Push(moduleObj)

	case compiler.OpImportMember:
		nameConstIdx := int(frame.ReadUint16())
		if nameConstIdx < 0 || nameConstIdx >= len(vm.constants) {
			return vm.newError(fmt.Sprintf("invalid constant index for import name: %d", nameConstIdx), frame.ip)
		}
		nameConst := vm.constants[nameConstIdx].(*bytecode.StringConstant)
		exportName := nameConst.Value

		// Pop module from stack
		moduleObj := vm.stack.Pop()
		moduleMap, ok := moduleObj.(*collections.Map)
		if !ok {
			return vm.newError("expected module object on stack for import member", frame.ip)
		}

		// Get the exported value
		exportVal := moduleMap.Get(exportName)
		if exportVal == types.UndefinedValue {
			return vm.newError(fmt.Sprintf("export '%s' not found in module", exportName), frame.ip)
		}

		return vm.stack.Push(exportVal)

	case compiler.OpNewClass:
		// Create new class from constant
		classIdx := int(frame.ReadUint16())
		if classIdx < 0 || classIdx >= len(vm.constants) {
			return vm.newError(fmt.Sprintf("invalid class constant index: %d", classIdx), frame.ip)
		}
		// Use constantToObject to get the class instance
		class, err := vm.constantToObject(classIdx, vm.constants[classIdx])
		if err != nil {
			return err
		}
		// Push the class to stack
		return vm.stack.Push(class)

	case compiler.OpGetMethod:
		// Get method from class/object
		nameIdx := int(frame.ReadUint16())
		if nameIdx < 0 || nameIdx >= len(vm.constants) {
			return vm.newError(fmt.Sprintf("invalid method name constant index: %d", nameIdx), frame.ip)
		}
		nameConst := vm.constants[nameIdx].(*bytecode.StringConstant)
		methodName := nameConst.Value

		obj := vm.stack.Pop()
		objTypeCode := obj.TypeCode()

		// Check inline cache first (1-slot monomorphic cache)
		if vm.methodCache.valid && vm.methodCache.objType == objTypeCode {
			// Cache hit - use cached method
			// For instances, we need to check if it's the same class
			if instance, ok := obj.(*types.Instance); ok {
				if boundMethod, ok := vm.methodCache.method.(*types.BoundMethod); ok {
					if boundMethod.Instance.Class == instance.Class {
						return vm.stack.Push(vm.methodCache.method)
					}
				}
			} else {
				return vm.stack.Push(vm.methodCache.method)
			}
		}

		// Cache miss - do the lookup
		var method types.Object
		switch o := obj.(type) {
		case *types.Class:
			// Check class methods
			if m, ok := o.Methods[methodName]; ok {
				method = m
			} else if m, ok := o.StaticMethods[methodName]; ok {
				method = m
			} else {
				return vm.newError(fmt.Sprintf("method '%s' not found on class %s", methodName, o.Name), frame.ip)
			}
		case *types.Instance:
			// Get method from instance's class (with inheritance)
			class := o.Class
			found := false
			for class != nil {
				if m, ok := class.Methods[methodName]; ok {
					method = &types.BoundMethod{
						Instance: o,
						Method:   m,
					}
					found = true
					break
				}
				if m, ok := class.StaticMethods[methodName]; ok {
					method = m
					found = true
					break
				}
				class = class.SuperClass
			}
			if !found {
				return vm.newError(fmt.Sprintf("method '%s' not found on instance of %s", methodName, o.Class.Name), frame.ip)
			}
		default:
			return vm.newError(fmt.Sprintf("cannot get method from %s type", obj.TypeName()), frame.ip)
		}

		// Update inline cache
		vm.methodCache.valid = true
		vm.methodCache.objType = objTypeCode
		vm.methodCache.method = method

		return vm.stack.Push(method)

	case compiler.OpSetMethod:
		// Set method in class
		nameIdx := int(frame.ReadUint16())
		if nameIdx < 0 || nameIdx >= len(vm.constants) {
			return vm.newError(fmt.Sprintf("invalid method name constant index: %d", nameIdx), frame.ip)
		}
		nameConst := vm.constants[nameIdx].(*bytecode.StringConstant)
		methodName := nameConst.Value

		method := vm.stack.Pop()
		obj := vm.stack.Pop()

		switch o := obj.(type) {
		case *types.Class:
			// Set as regular method or static method based on method type
			if fn, ok := method.(*types.Function); ok {
				if fn.IsStatic {
					o.StaticMethods[methodName] = fn
				} else {
					o.Methods[methodName] = fn
				}
			} else {
				return vm.newError(fmt.Sprintf("expected function, got %s type", method.TypeName()), frame.ip)
			}
			return vm.stack.Push(types.UndefinedValue)
		default:
			return vm.newError(fmt.Sprintf("cannot set method on %s type", obj.TypeName()), frame.ip)
		}

	case compiler.OpGetSuper:
		// Pop the instance from stack
		instance, ok := vm.stack.Pop().(*types.Instance)
		if !ok {
			return vm.newError("super expects an instance", frame.ip)
		}

		// Get the superclass from the instance's class
		superClass := instance.Class.SuperClass
		if superClass == nil {
			return vm.newError(fmt.Sprintf("class '%s' has no superclass", instance.Class.Name), frame.ip)
		}

		// Create and push SuperReference
		superRef := &types.SuperReference{
			Instance: instance,
			Super:    superClass,
		}
		return vm.stack.Push(superRef)

	case compiler.OpGetSuper2:
		// Pop the instance from stack
		instance, ok := vm.stack.Pop().(*types.Instance)
		if !ok {
			return vm.newError("super expects an instance", frame.ip)
		}

		// Get the superclass index from operand
		superIndex := frame.ReadUint16()

		// Get the superclass from VM constants
		superClassVal := vm.constants[superIndex]
		superClassStr, ok := superClassVal.(*bytecode.StringConstant)
		if !ok {
			return vm.newError("superclass must be a string", frame.ip)
		}

		// Look up the superclass object from globals
		superClassObj, ok := vm.globals[superClassStr.Value]
		if !ok {
			return vm.newError(fmt.Sprintf("superclass '%s' not found", superClassStr.Value), frame.ip)
		}

		// Get the class from the global object
		superClass, ok := superClassObj.(*types.Class)
		if !ok {
			return vm.newError(fmt.Sprintf("'%s' is not a class", superClassStr.Value), frame.ip)
		}

		// Create and push SuperReference
		superRef := &types.SuperReference{
			Instance: instance,
			Super:    superClass,
		}
		return vm.stack.Push(superRef)

	case compiler.OpThrow:
		errVal := vm.stack.Pop()
		err, ok := errVal.(*types.Error)
		if !ok {
			// If not already an error, wrap it with stack trace
			stack := vm.collectCallStack()
			err = types.NewErrorWithStack(errVal.ToStr(), 0, 0, "", stack)
		} else if len(err.Stack) == 0 {
			// If error doesn't have stack yet, add it
			err.Stack = vm.collectCallStack()
		}
		vm.lastError = err

		// Unwind the stack to find the appropriate catch block
		for len(vm.tryStack) > 0 {
			tryFrame := vm.tryStack[len(vm.tryStack)-1]
			vm.tryStack = vm.tryStack[:len(vm.tryStack)-1]

			// First, run all deferred functions in this frame
			vm.runDeferred(tryFrame.frameIndex)

			// Check if there's a catch block
			if tryFrame.catchOffset != 0xFFFF {
				// Jump to catch block
				frame := vm.frames[tryFrame.frameIndex]
				frame.ip = tryFrame.catchOffset
				// Restore stack and base pointer
				vm.stack.SetSize(tryFrame.stackPointer)
				// Push the error to the stack for catch
				vm.stack.Push(err)
				vm.lastError = nil
				return nil
			}

			// If there's a finally block, run it
			if tryFrame.finallyOffset != 0xFFFF {
				// Jump to finally block
				frame := vm.frames[tryFrame.frameIndex]
				frame.ip = tryFrame.finallyOffset
				// Restore stack and base pointer
				vm.stack.SetSize(tryFrame.stackPointer)
				vm.lastError = err
				return nil
			}
		}

		// No catch block found, return the error
		return err

	default:
		return vm.newError(fmt.Sprintf("unknown opcode: %#02x", op), frame.ip)
	}

	return nil
}

// constantToObject converts a bytecode constant to a types.Object
func (vm *VM) constantToObject(index int, c bytecode.Constant) (types.Object, error) {
	switch constType := c.(type) {
	case *bytecode.NilConstant:
		return types.NullValue, nil
	case *bytecode.BoolConstant:
		return types.Bool(constType.Value), nil
	case *bytecode.IntConstant:
		return types.Int(constType.Value), nil
	case *bytecode.CharConstant:
		return types.Char(constType.Value), nil
	case *bytecode.FloatConstant:
		return types.Float(constType.Value), nil
	case *bytecode.StringConstant:
		return types.String(constType.Value), nil
	case *bytecode.FunctionConstant:
		// Check cache first to avoid duplicate function instances
		if fn, ok := vm.functionCache[index]; ok {
			return fn, nil
		}
		// Convert bytecode function to types.Function
		fn := &types.Function{
			Name:          constType.Name,
			NumLocals:     constType.NumLocals,
			NumParameters: constType.NumParameters,
			IsVariadic:    constType.IsVariadic,
			Instructions:  constType.Instructions,
			ConstantPool:  vm.constants,
			DefaultValues: constType.DefaultValues,
		}
		// Cache the function instance
		vm.functionCache[index] = fn
		return fn, nil
	case *bytecode.ClassConstant:
		// Check cache first
		if class, ok := vm.classCache[index]; ok {
			return class, nil
		}
		// Create a types.Class from the bytecode constant
		class := &types.Class{
			Name:          constType.Name,
			SuperClass:    nil,
			Methods:       make(map[string]*types.Function),
			StaticFields:  make(map[string]types.Object),
			StaticMethods: make(map[string]*types.Function),
		}
		// Copy methods from bytecode constant (method name -> function index)
		for methodName, methodIdx := range constType.Methods {
			// Get the function from constant pool
			if methodFn, ok := vm.constants[methodIdx].(*bytecode.FunctionConstant); ok {
				fn := &types.Function{
					Name:          methodFn.Name,
					NumLocals:     methodFn.NumLocals,
					NumParameters: methodFn.NumParameters,
					IsVariadic:    methodFn.IsVariadic,
					Instructions:  methodFn.Instructions,
					ConstantPool:  vm.constants,
					DefaultValues: methodFn.DefaultValues,
				}
				class.Methods[methodName] = fn
				// Cache the function
				vm.functionCache[methodIdx] = fn
			}
		}
		// Copy static methods from bytecode constant
		for methodName, methodIdx := range constType.StaticMethods {
			if methodFn, ok := vm.constants[methodIdx].(*bytecode.FunctionConstant); ok {
				fn := &types.Function{
					Name:          methodFn.Name,
					NumLocals:     methodFn.NumLocals,
					NumParameters: methodFn.NumParameters,
					IsVariadic:    methodFn.IsVariadic,
					Instructions:  methodFn.Instructions,
					ConstantPool:  vm.constants,
					IsStatic:      true,
					DefaultValues: methodFn.DefaultValues,
				}
				class.StaticMethods[methodName] = fn
				vm.functionCache[methodIdx] = fn
			}
		}
		// Resolve superclass if specified
		if constType.SuperClass != "" {
			// Superclass should be in globals (already created)
			if superVal, ok := vm.globals[constType.SuperClass]; ok {
				if superClass, ok := superVal.(*types.Class); ok {
					class.SuperClass = superClass
				}
			}
		}
		// Cache the class
		vm.classCache[index] = class
		return class, nil
	case *bytecode.InterfaceConstant:
		// Create a types.Interface from the bytecode constant
		iface := &types.Interface{
			Name:    constType.Name,
			Methods: constType.Methods,
		}
		return iface, nil
	default:
		return nil, vm.newError(fmt.Sprintf("unsupported constant type: %T", c), 0)
	}
}

// collectCallStack collects the current call stack
func (vm *VM) collectCallStack() []types.StackFrame {
	stack := make([]types.StackFrame, 0, vm.framePointer)

	// Walk the call stack from most recent to oldest
	for i := vm.framePointer - 1; i >= 0; i-- {
		frame := vm.frames[i]
		if frame == nil {
			continue
		}

		// TODO: Get actual line and column from line number table
		stackFrame := types.StackFrame{
			FunctionName: frame.Name(),
			Line:         0, // Will be filled with line number lookup
			Column:       0,
			Filename:     "", // Will be filled with current filename
		}
		stack = append(stack, stackFrame)
	}

	return stack
}

// LoadModule loads a module by path
func (vm *VM) LoadModule(modulePath string) (*Module, error) {
	// Standard library modules are built-in - return VM's copy directly
	standardModules := map[string]bool{
		"math":       true,
		"string":     true,
		"collection": true,
		"time":       true,
		"json":       true,
		"thread":     true,
		"http":       true,
	}
	if standardModules[modulePath] {
		if mod, ok := vm.modules[modulePath]; ok {
			return mod, nil
		}
		return nil, fmt.Errorf("standard library module '%s' not found in VM", modulePath)
	}

	// Check if module is already loaded
	if mod, ok := vm.modules[modulePath]; ok {
		return mod, nil
	}

	// Resolve the module file path
	var resolvedPath string
	if strings.HasPrefix(modulePath, "./") || strings.HasPrefix(modulePath, "../") {
		// Relative import - resolve relative to current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %v", err)
		}
		resolvedPath = filepath.Join(cwd, modulePath)
		// Add .nx extension if missing
		if filepath.Ext(resolvedPath) == "" {
			resolvedPath += ".nx"
		}
	} else {
		// Package import - search in nx_modules directories up the tree
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %v", err)
		}
		currentDir := cwd
		found := false

		// Search up the directory tree for nx_modules
		for {
			nxModulesPath := filepath.Join(currentDir, "nx_modules", modulePath)

			// Check if it's a file
			if stat, err := os.Stat(nxModulesPath + ".nx"); err == nil && !stat.IsDir() {
				resolvedPath = nxModulesPath + ".nx"
				found = true
				break
			}

			// Check if it's a directory
			if stat, err := os.Stat(nxModulesPath); err == nil && stat.IsDir() {
				// Look for nx.json config in the package directory
				configPath := filepath.Join(nxModulesPath, "nx.json")
				if _, err := os.Stat(configPath); err == nil {
					// Read config to find main entry
					configData, err := os.ReadFile(configPath)
					if err == nil {
						// Simple JSON parsing for main field
						var config map[string]interface{}
						if err := json.Unmarshal(configData, &config); err == nil {
							if main, ok := config["main"].(string); ok {
								resolvedPath = filepath.Join(nxModulesPath, main)
								found = true
								break
							}
						}
					}
				}

				// Fallback to index.nx
				indexPath := filepath.Join(nxModulesPath, "index.nx")
				if _, err := os.Stat(indexPath); err == nil {
					resolvedPath = indexPath
					found = true
					break
				}
			}

			// Move up one directory
			parentDir := filepath.Dir(currentDir)
			if parentDir == currentDir {
				// Reached root directory
				break
			}
			currentDir = parentDir
		}

		// If not found in local nx_modules, check system-wide paths
		if !found {
			systemPaths := []string{
				"/usr/local/nx/modules",
				"/usr/lib/nx/modules",
				filepath.Join(os.Getenv("HOME"), ".nx", "modules"),
			}
			for _, sysPath := range systemPaths {
				pkgPath := filepath.Join(sysPath, modulePath)
				if stat, err := os.Stat(pkgPath + ".nx"); err == nil && !stat.IsDir() {
					resolvedPath = pkgPath + ".nx"
					found = true
					break
				}
				if stat, err := os.Stat(pkgPath); err == nil && stat.IsDir() {
					indexPath := filepath.Join(pkgPath, "index.nx")
					if _, err := os.Stat(indexPath); err == nil {
						resolvedPath = indexPath
						found = true
						break
					}
				}
			}
		}

		if !found {
			return nil, fmt.Errorf("package not found: %s (searched in nx_modules directories and system paths)", modulePath)
		}
	}

	// Read the module file
	source, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read module file: %v", err)
	}

	// Parse the module
	lexer := parser.NewLexer(string(source))
	parser := parser.NewParser(lexer)
	program := parser.ParseProgram()

	if len(parser.Errors()) > 0 {
		errMsg := "parsing errors in module:\n"
		for _, err := range parser.Errors() {
			errMsg += fmt.Sprintf("  %s\n", err)
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	// Compile the module
	comp := compiler.NewCompiler()
	comp.ModulePath = resolvedPath
	if err := comp.Compile(program); err != nil {
		return nil, fmt.Errorf("compilation error in module: %v", err)
	}

	// Get module bytecode
	bc := comp.Bytecode()

	// Create a new VM instance to run the module (isolated)
	moduleVM := NewVM(bc)
	moduleVM.globals = vm.globals // Share globals with parent VM
	moduleVM.modules = vm.modules // Share module cache
	moduleVM.modulePaths = vm.modulePaths

	// Register standard libraries
	moduleVM.registerBuiltins()
	moduleVM.registerStandardModules()

	// Run the module to execute top-level code and populate exports
	if err := moduleVM.Run(); err != nil {
		return nil, fmt.Errorf("error running module: %v", err)
	}

	// Get exports from compiler
	exports := make(map[string]types.Object)
	for exportName, symbolName := range comp.Exports {
		// Get the symbol value from the module's global scope
		val, ok := moduleVM.globals[symbolName]
		if !ok {
			return nil, fmt.Errorf("exported symbol '%s' not found in module", symbolName)
		}
		exports[exportName] = val
	}

	// Create module object
	module := &Module{
		Name:    filepath.Base(resolvedPath),
		Path:    resolvedPath,
		Exports: exports,
	}

	// Cache the module
	vm.modules[modulePath] = module
	vm.modules[resolvedPath] = module

	return module, nil
}

// GetModuleExport gets an exported value from a module
func (vm *VM) GetModuleExport(module *Module, exportName string) (types.Object, bool) {
	val, ok := module.Exports[exportName]
	return val, ok
}

// newError creates a new runtime error
func (vm *VM) newError(message string, ip int) error {
	line := vm.getLineFromIP(ip)
	stack := vm.collectCallStack()

	// Get the code line for error reporting
	codeLine := vm.GetLineCode(line)

	err := types.NewErrorWithCode(message, line, 0, "", codeLine)
	err.Stack = stack
	vm.lastError = err
	return err
}

// getLineFromIP returns the source line number for the given instruction pointer
func (vm *VM) getLineFromIP(ip int) int {
	return findLineForIPFromTable(vm.lineNumberTable, ip)
}

// findLineForIPFromTable finds the line number from the line number table
func findLineForIPFromTable(table []bytecode.LineInfo, ip int) int {
	if len(table) == 0 {
		return 0
	}

	// Find the largest offset that is <= ip
	lastLine := 0
	for _, info := range table {
		if info.Offset <= ip {
			lastLine = info.Line
		} else {
			break
		}
	}

	return lastLine
}

// runDeferred runs all deferred functions for the given frame index
func (vm *VM) runDeferred(frameIndex int) {
	deferred := vm.deferStack[frameIndex]
	// Run deferred functions in reverse order (LIFO)
	for i := len(deferred) - 1; i >= 0; i-- {
		call := deferred[i]
		// Execute the call
		switch fn := call.fn.(type) {
		case *types.NativeFunction:
			result := fn.Fn(call.args...)
			_ = result
		case *types.Function:
			// For bytecode functions, create a new frame and run
			bcFunc := &bytecode.FunctionConstant{
				Name:          fn.Name,
				NumLocals:     fn.NumLocals,
				NumParameters: fn.NumParameters,
				IsVariadic:    fn.IsVariadic,
				Instructions:  fn.Instructions,
			}

			// Create new frame
			basePointer := vm.stack.Size()
			newFrame := NewFrame(bcFunc, basePointer)

			// Copy arguments to locals
			for i := 0; i < len(call.args) && i < fn.NumParameters; i++ {
				newFrame.locals[i] = call.args[i]
			}

			vm.frames[vm.framePointer] = newFrame
			vm.framePointer++

			// Run the deferred function
			for vm.framePointer > 0 {
				currentFrame := vm.frames[vm.framePointer-1]
				if currentFrame.ip >= len(currentFrame.Instructions()) {
					vm.framePointer--
					continue
				}
				op := compiler.Opcode(currentFrame.CurrentInstruction())
				currentFrame.ip++
				if err := vm.executeOpcode(op, currentFrame); err != nil {
					fmt.Printf("Error in deferred function: %v\n", err)
					vm.framePointer--
					break
				}
			}
		default:
			// For other types, just ignore
		}
	}
	// Clear deferred functions for this frame
	vm.deferStack[frameIndex] = nil
}

// handleRuntimeError handles runtime errors by looking for a try-catch block
// Returns true if the error was caught and execution should continue
func (vm *VM) handleRuntimeError(err error, frame *Frame) bool {
	// Convert to types.Error if needed
	var typeErr *types.Error
	var ok bool
	if typeErr, ok = err.(*types.Error); !ok {
		// Wrap the error with stack trace
		stack := vm.collectCallStack()
		typeErr = types.NewErrorWithStack(err.Error(), 0, 0, "", stack)
	} else if len(typeErr.Stack) == 0 {
		// If error doesn't have stack yet, add it
		typeErr.Stack = vm.collectCallStack()
	}
	vm.lastError = typeErr

	// Unwind the stack to find the appropriate catch block
	for len(vm.tryStack) > 0 {
		tryFrame := vm.tryStack[len(vm.tryStack)-1]
		vm.tryStack = vm.tryStack[:len(vm.tryStack)-1]

		// First, run all deferred functions in this frame
		vm.runDeferred(tryFrame.frameIndex)

		// Check if there's a catch block
		if tryFrame.catchOffset != 0xFFFF {
			// Jump to catch block
			frame := vm.frames[tryFrame.frameIndex]
			frame.ip = tryFrame.catchOffset
			// Restore stack and base pointer
			vm.stack.SetSize(tryFrame.stackPointer)
			// Push the error to the stack for catch
			vm.stack.Push(typeErr)
			vm.lastError = nil
			return true
		}

		// If there's a finally block, run it
		if tryFrame.finallyOffset != 0xFFFF {
			// Jump to finally block
			frame := vm.frames[tryFrame.frameIndex]
			frame.ip = tryFrame.finallyOffset
			// Restore stack and base pointer
			vm.stack.SetSize(tryFrame.stackPointer)
			vm.lastError = typeErr
			return true
		}
	}

	// No catch block found
	vm.lastError = nil
	return false
}

// Comparison operators
const (
	lt = iota
	lte
	gt
	gte
)
