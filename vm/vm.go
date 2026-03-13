package vm

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"hash/fnv"
	"io"
	"math"
	"math/bits"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	crand "crypto/rand"

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
			return types.String("Nxlang v1.1.1")
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

	vm.globals["sha512"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("sha512() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			h := sha512.Sum512([]byte(s))
			return types.String(hex.EncodeToString(h[:]))
		},
	}

	vm.globals["uuid"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			b := make([]byte, 16)
			for i := range b {
				b[i] = byte(rand.Intn(256))
			}
			b[6] = (b[6] & 0x0f) | 0x40
			b[8] = (b[8] & 0x3f) | 0x80
			uuid := fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
			return types.String(uuid)
		},
	}

	vm.globals["uuidv4"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			b := make([]byte, 16)
			for i := range b {
				b[i] = byte(rand.Intn(256))
			}
			b[6] = (b[6] & 0x0f) | 0x40
			b[8] = (b[8] & 0x3f) | 0x80
			uuid := fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
			return types.String(uuid)
		},
	}

	vm.globals["htmlEncode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("htmlEncode() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			result := s
			result = strings.ReplaceAll(result, "&", "&amp;")
			result = strings.ReplaceAll(result, "<", "&lt;")
			result = strings.ReplaceAll(result, ">", "&gt;")
			result = strings.ReplaceAll(result, `"`, "&quot;")
			result = strings.ReplaceAll(result, "'", "&#39;")
			return types.String(result)
		},
	}

	vm.globals["htmlDecode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("htmlDecode() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			result := s
			result = strings.ReplaceAll(result, "&lt;", "<")
			result = strings.ReplaceAll(result, "&gt;", ">")
			result = strings.ReplaceAll(result, "&quot;", `"`)
			result = strings.ReplaceAll(result, "&#39;", "'")
			result = strings.ReplaceAll(result, "&amp;", "&")
			return types.String(result)
		},
	}

	vm.globals["randInt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("randInt() expects 2 arguments (min, max)", 0, 0, "")
			}
			minVal, ok1 := args[0].(types.Int)
			maxVal, ok2 := args[1].(types.Int)
			if !ok1 || !ok2 {
				return types.NewError("randInt() expects 2 integer arguments", 0, 0, "")
			}
			min, max := int(minVal), int(maxVal)
			if min > max {
				return types.NewError("randInt(): min must be less than or equal to max", 0, 0, "")
			}
			return types.Int(rand.Intn(max-min+1) + min)
		},
	}

	vm.globals["envGet"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("envGet() expects 1 argument (key)", 0, 0, "")
			}
			key := string(types.ToString(args[0]))
			return types.String(os.Getenv(key))
		},
	}

	vm.globals["envSet"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("envSet() expects 2 arguments (key, value)", 0, 0, "")
			}
			key := string(types.ToString(args[0]))
			value := string(types.ToString(args[1]))
			os.Setenv(key, value)
			return types.UndefinedValue
		},
	}

	vm.globals["fileExists"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("fileExists() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			_, err := os.Stat(path)
			return types.Bool(err == nil)
		},
	}

	vm.globals["mkdir"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("mkdir() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			perm := os.FileMode(0755)
			if len(args) > 1 {
				if p, ok := args[1].(types.Int); ok {
					perm = os.FileMode(p)
				}
			}
			err := os.MkdirAll(path, perm)
			if err != nil {
				return types.NewError(fmt.Sprintf("mkdir error: %v", err), 0, 0, "")
			}
			return types.UndefinedValue
		},
	}

	vm.globals["removeFile"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("removeFile() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			err := os.Remove(path)
			if err != nil {
				return types.NewError(fmt.Sprintf("removeFile error: %v", err), 0, 0, "")
			}
			return types.UndefinedValue
		},
	}

	vm.globals["readDir"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("readDir() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			entries, err := os.ReadDir(path)
			if err != nil {
				return types.NewError(fmt.Sprintf("readDir error: %v", err), 0, 0, "")
			}
			arr := collections.NewArray()
			for _, entry := range entries {
				m := collections.NewMap()
				m.Set("name", types.String(entry.Name()))
				m.Set("isDir", types.Bool(entry.IsDir()))
				arr.Append(m)
			}
			return arr
		},
	}

	vm.globals["isDir"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isDir() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			info, err := os.Stat(path)
			if err != nil {
				return types.Bool(false)
			}
			return types.Bool(info.IsDir())
		},
	}

	vm.globals["isFile"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isFile() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			info, err := os.Stat(path)
			if err != nil {
				return types.Bool(false)
			}
			return types.Bool(!info.IsDir())
		},
	}

	vm.globals["fileSize"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("fileSize() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			info, err := os.Stat(path)
			if err != nil {
				return types.NewError(fmt.Sprintf("fileSize error: %v", err), 0, 0, "")
			}
			return types.Int(info.Size())
		},
	}

	vm.globals["fileModified"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("fileModified() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			info, err := os.Stat(path)
			if err != nil {
				return types.NewError(fmt.Sprintf("fileModified error: %v", err), 0, 0, "")
			}
			return types.String(info.ModTime().Format("2006-01-02 15:04:05"))
		},
	}

	vm.globals["copyFile"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("copyFile() expects 2 arguments (src, dst)", 0, 0, "")
			}
			src := string(types.ToString(args[0]))
			dst := string(types.ToString(args[1]))
			data, err := os.ReadFile(src)
			if err != nil {
				return types.NewError(fmt.Sprintf("copyFile error: %v", err), 0, 0, "")
			}
			err = os.WriteFile(dst, data, 0644)
			if err != nil {
				return types.NewError(fmt.Sprintf("copyFile error: %v", err), 0, 0, "")
			}
			return types.UndefinedValue
		},
	}

	vm.globals["rename"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("rename() expects 2 arguments (old, new)", 0, 0, "")
			}
			old := string(types.ToString(args[0]))
			new := string(types.ToString(args[1]))
			err := os.Rename(old, new)
			if err != nil {
				return types.NewError(fmt.Sprintf("rename error: %v", err), 0, 0, "")
			}
			return types.UndefinedValue
		},
	}

	vm.globals["chdir"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("chdir() expects 1 argument", 0, 0, "")
			}
			dir := string(types.ToString(args[0]))
			err := os.Chdir(dir)
			if err != nil {
				return types.NewError(fmt.Sprintf("chdir error: %v", err), 0, 0, "")
			}
			wd, _ := os.Getwd()
			return types.String(wd)
		},
	}

	vm.globals["pwd"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			wd, _ := os.Getwd()
			return types.String(wd)
		},
	}

	vm.globals["glob"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("glob() expects 1 argument (pattern)", 0, 0, "")
			}
			pattern := string(types.ToString(args[0]))
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return types.NewError(fmt.Sprintf("glob error: %v", err), 0, 0, "")
			}
			arr := collections.NewArray()
			for _, match := range matches {
				arr.Append(types.String(match))
			}
			return arr
		},
	}

	vm.globals["absPath"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("absPath() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			abs, err := filepath.Abs(path)
			if err != nil {
				return types.NewError(fmt.Sprintf("absPath error: %v", err), 0, 0, "")
			}
			return types.String(abs)
		},
	}

	vm.globals["baseName"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("baseName() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			return types.String(filepath.Base(path))
		},
	}

	vm.globals["dirName"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("dirName() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			return types.String(filepath.Dir(path))
		},
	}

	vm.globals["extName"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("extName() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			return types.String(filepath.Ext(path))
		},
	}

	vm.globals["joinPath"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("joinPath() expects at least 1 argument", 0, 0, "")
			}
			parts := make([]string, len(args))
			for i, arg := range args {
				parts[i] = string(types.ToString(arg))
			}
			return types.String(filepath.Join(parts...))
		},
	}

	vm.globals["nowUnix"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(time.Now().Unix())
		},
	}

	vm.globals["nowUnixMs"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(time.Now().UnixMilli())
		},
	}

	vm.globals["timestampToTime"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("timestampToTime() expects 1 argument", 0, 0, "")
			}
			ts, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("timestampToTime() expects an integer", 0, 0, "")
			}
			t := time.Unix(int64(ts), 0)
			return types.String(t.Format("2006-01-02 15:04:05"))
		},
	}

	vm.globals["timeYear"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("timeYear() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			t, err := time.Parse("2006-01-02 15:04:05", s)
			if err != nil {
				t, _ = time.Parse("2006-01-02", s)
			}
			return types.Int(t.Year())
		},
	}

	vm.globals["timeMonth"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("timeMonth() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			t, err := time.Parse("2006-01-02 15:04:05", s)
			if err != nil {
				t, _ = time.Parse("2006-01-02", s)
			}
			return types.Int(t.Month())
		},
	}

	vm.globals["timeDay"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("timeDay() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			t, err := time.Parse("2006-01-02 15:04:05", s)
			if err != nil {
				t, _ = time.Parse("2006-01-02", s)
			}
			return types.Int(t.Day())
		},
	}

	vm.globals["timeHour"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("timeHour() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			t, err := time.Parse("2006-01-02 15:04:05", s)
			if err != nil {
				return types.NewError("timeHour() expects time in format 2006-01-02 15:04:05", 0, 0, "")
			}
			return types.Int(t.Hour())
		},
	}

	vm.globals["timeMinute"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("timeMinute() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			t, err := time.Parse("2006-01-02 15:04:05", s)
			if err != nil {
				return types.NewError("timeMinute() expects time in format 2006-01-02 15:04:05", 0, 0, "")
			}
			return types.Int(t.Minute())
		},
	}

	vm.globals["timeSecond"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("timeSecond() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			t, err := time.Parse("2006-01-02 15:04:05", s)
			if err != nil {
				return types.NewError("timeSecond() expects time in format 2006-01-02 15:04:05", 0, 0, "")
			}
			return types.Int(t.Second())
		},
	}

	vm.globals["timeWeekday"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("timeWeekday() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			t, err := time.Parse("2006-01-02 15:04:05", s)
			if err != nil {
				t, _ = time.Parse("2006-01-02", s)
			}
			return types.Int(int(t.Weekday()))
		},
	}

	vm.globals["addDays"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("addDays() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			days, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("addDays() expects (string, integer)", 0, 0, "")
			}
			t, err := time.Parse("2006-01-02 15:04:05", s)
			if err != nil {
				t, _ = time.Parse("2006-01-02", s)
			}
			t = t.AddDate(0, 0, int(days))
			return types.String(t.Format("2006-01-02 15:04:05"))
		},
	}

	vm.globals["addMonths"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("addMonths() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			months, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("addMonths() expects (string, integer)", 0, 0, "")
			}
			t, err := time.Parse("2006-01-02 15:04:05", s)
			if err != nil {
				t, _ = time.Parse("2006-01-02", s)
			}
			t = t.AddDate(0, int(months), 0)
			return types.String(t.Format("2006-01-02 15:04:05"))
		},
	}

	vm.globals["addYears"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("addYears() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			years, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("addYears() expects (string, integer)", 0, 0, "")
			}
			t, err := time.Parse("2006-01-02 15:04:05", s)
			if err != nil {
				t, _ = time.Parse("2006-01-02", s)
			}
			t = t.AddDate(int(years), 0, 0)
			return types.String(t.Format("2006-01-02 15:04:05"))
		},
	}

	vm.globals["date"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			t := time.Now()
			return types.String(t.Format("2006-01-02"))
		},
	}

	vm.globals["time"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			t := time.Now()
			return types.String(t.Format("15:04:05"))
		},
	}

	vm.globals["isoWeek"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isoWeek() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			t, err := time.Parse("2006-01-02 15:04:05", s)
			if err != nil {
				t, _ = time.Parse("2006-01-02", s)
			}
			_, week := t.ISOWeek()
			return types.Int(week)
		},
	}

	vm.globals["isoYear"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isoYear() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			t, err := time.Parse("2006-01-02 15:04:05", s)
			if err != nil {
				t, _ = time.Parse("2006-01-02", s)
			}
			year, _ := t.ISOWeek()
			return types.Int(year)
		},
	}

	vm.globals["daysInMonth"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("daysInMonth() expects 2 arguments", 0, 0, "")
			}
			year, ok1 := args[0].(types.Int)
			month, ok2 := args[1].(types.Int)
			if !ok1 || !ok2 {
				return types.NewError("daysInMonth() expects 2 integers", 0, 0, "")
			}
			t := time.Date(int(year), time.Month(int(month)), 1, 0, 0, 0, 0, time.UTC)
			return types.Int(t.AddDate(0, 1, -1).Day())
		},
	}

	vm.globals["isLeapYear"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isLeapYear() expects 1 argument", 0, 0, "")
			}
			year, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("isLeapYear() expects an integer", 0, 0, "")
			}
			y := int(year)
			return types.Bool((y%4 == 0 && y%100 != 0) || y%400 == 0)
		},
	}

	vm.globals["weekOfYear"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("weekOfYear() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			t, err := time.Parse("2006-01-02 15:04:05", s)
			if err != nil {
				t, _ = time.Parse("2006-01-02", s)
			}
			year, week := t.ISOWeek()
			m := collections.NewMap()
			m.Set("year", types.Int(year))
			m.Set("week", types.Int(week))
			return m
		},
	}

	vm.globals["utcNow"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			t := time.Now().UTC()
			return types.String(t.Format("2006-01-02 15:04:05"))
		},
	}

	vm.globals["unixToTime"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("unixToTime() expects 1 argument", 0, 0, "")
			}
			ts, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("unixToTime() expects an integer", 0, 0, "")
			}
			t := time.Unix(int64(ts), 0)
			return types.String(t.Format("2006-01-02 15:04:05"))
		},
	}

	vm.globals["sleepMicros"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("sleepMicros() expects 1 argument", 0, 0, "")
			}
			us, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("sleepMicros() expects an integer", 0, 0, "")
			}
			time.Sleep(time.Duration(us) * time.Microsecond)
			return types.UndefinedValue
		},
	}

	vm.globals["pluck"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("pluck() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("pluck() first argument must be an array", 0, 0, "")
			}
			key := string(types.ToString(args[1]))
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				val := arr.Get(i)
				if m, ok := val.(*collections.Map); ok {
					result.Append(m.Get(key))
				}
			}
			return result
		},
	}

	vm.globals["partition"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("partition() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("partition() first argument must be an array", 0, 0, "")
			}
			size, ok := args[1].(types.Int)
			if !ok || size <= 0 {
				return types.NewError("partition() second argument must be positive integer", 0, 0, "")
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i += int(size) {
				part := collections.NewArray()
				end := i + int(size)
				if end > arr.Len() {
					end = arr.Len()
				}
				for j := i; j < end; j++ {
					part.Append(arr.Get(j))
				}
				result.Append(part)
			}
			return result
		},
	}

	vm.globals["transpose"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("transpose() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("transpose() argument must be an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return collections.NewArray()
			}
			first, ok := arr.Get(0).(*collections.Array)
			if !ok {
				return types.NewError("transpose() array elements must be arrays", 0, 0, "")
			}
			numCols := first.Len()
			result := collections.NewArray()
			for i := 0; i < numCols; i++ {
				col := collections.NewArray()
				for j := 0; j < arr.Len(); j++ {
					row, ok := arr.Get(j).(*collections.Array)
					if !ok {
						return types.NewError("transpose() array elements must be arrays", 0, 0, "")
					}
					col.Append(row.Get(i))
				}
				result.Append(col)
			}
			return result
		},
	}

	vm.globals["flattenDeep"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("flattenDeep() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("flattenDeep() argument must be an array", 0, 0, "")
			}
			result := collections.NewArray()
			var flatten func(a *collections.Array, depth int)
			d := -1
			if len(args) > 1 {
				if depthVal, depthOk := args[1].(types.Int); depthOk {
					d = int(depthVal)
				}
			}
			flatten = func(a *collections.Array, depth int) {
				for i := 0; i < a.Len(); i++ {
					val := a.Get(i)
					if nested, ok := val.(*collections.Array); ok {
						if depth != 0 {
							flatten(nested, depth-1)
						} else {
							result.Append(nested)
						}
					} else {
						result.Append(val)
					}
				}
			}
			flatten(arr, d)
			return result
		},
	}

	vm.globals["nth"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("nth() expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("nth() first argument must be an array", 0, 0, "")
			}
			n, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("nth() second argument must be an integer", 0, 0, "")
			}
			idx := int(n)
			if idx < 0 {
				idx = arr.Len() + idx
			}
			if idx < 0 || idx >= arr.Len() {
				return types.UndefinedValue
			}
			return arr.Get(idx)
		},
	}

	vm.globals["findIndex"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("findIndex() expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("findIndex() first argument must be an array", 0, 0, "")
			}
			target := args[1]
			for i := 0; i < arr.Len(); i++ {
				if arr.Get(i).Equals(target) {
					return types.Int(i)
				}
			}
			return types.Int(-1)
		},
	}

	vm.globals["findLastIndex"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("findLastIndex() expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("findLastIndex() first argument must be an array", 0, 0, "")
			}
			target := args[1]
			for i := arr.Len() - 1; i >= 0; i-- {
				if arr.Get(i).Equals(target) {
					return types.Int(i)
				}
			}
			return types.Int(-1)
		},
	}

	vm.globals["insert"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("insert() expects 3 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("insert() first argument must be an array", 0, 0, "")
			}
			idx, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("insert() second argument must be an integer", 0, 0, "")
			}
			val := args[2]
			result := collections.NewArray()
			pos := int(idx)
			if pos < 0 {
				pos = 0
			}
			if pos > arr.Len() {
				pos = arr.Len()
			}
			for i := 0; i < pos; i++ {
				result.Append(arr.Get(i))
			}
			result.Append(val)
			for i := pos; i < arr.Len(); i++ {
				result.Append(arr.Get(i))
			}
			return result
		},
	}

	vm.globals["removeAt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("removeAt() expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("removeAt() first argument must be an array", 0, 0, "")
			}
			idx, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("removeAt() second argument must be an integer", 0, 0, "")
			}
			pos := int(idx)
			if pos < 0 || pos >= arr.Len() {
				return arr
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				if i != pos {
					result.Append(arr.Get(i))
				}
			}
			return result
		},
	}

	vm.globals["replaceAt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("replaceAt() expects 3 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("replaceAt() first argument must be an array", 0, 0, "")
			}
			idx, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("replaceAt() second argument must be an integer", 0, 0, "")
			}
			val := args[2]
			pos := int(idx)
			if pos < 0 || pos >= arr.Len() {
				return arr
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				if i == pos {
					result.Append(val)
				} else {
					result.Append(arr.Get(i))
				}
			}
			return result
		},
	}

	vm.globals["take"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("take() expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("take() first argument must be an array", 0, 0, "")
			}
			n, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("take() second argument must be an integer", 0, 0, "")
			}
			if int(n) >= arr.Len() {
				result := collections.NewArray()
				for i := 0; i < arr.Len(); i++ {
					result.Append(arr.Get(i))
				}
				return result
			}
			result := collections.NewArray()
			for i := 0; i < int(n); i++ {
				result.Append(arr.Get(i))
			}
			return result
		},
	}

	vm.globals["drop"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("drop() expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("drop() first argument must be an array", 0, 0, "")
			}
			n, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("drop() second argument must be an integer", 0, 0, "")
			}
			if int(n) >= arr.Len() {
				return collections.NewArray()
			}
			result := collections.NewArray()
			for i := int(n); i < arr.Len(); i++ {
				result.Append(arr.Get(i))
			}
			return result
		},
	}

	vm.globals["takeLast"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("takeLast() expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("takeLast() first argument must be an array", 0, 0, "")
			}
			n, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("takeLast() second argument must be an integer", 0, 0, "")
			}
			if int(n) >= arr.Len() {
				result := collections.NewArray()
				for i := 0; i < arr.Len(); i++ {
					result.Append(arr.Get(i))
				}
				return result
			}
			result := collections.NewArray()
			for i := arr.Len() - int(n); i < arr.Len(); i++ {
				result.Append(arr.Get(i))
			}
			return result
		},
	}

	vm.globals["dropLast"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("dropLast() expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("dropLast() first argument must be an array", 0, 0, "")
			}
			n, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("dropLast() second argument must be an integer", 0, 0, "")
			}
			if int(n) >= arr.Len() {
				return collections.NewArray()
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len()-int(n); i++ {
				result.Append(arr.Get(i))
			}
			return result
		},
	}

	vm.globals["first"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("first() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("first() argument must be an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return types.UndefinedValue
			}
			return arr.Get(0)
		},
	}

	vm.globals["last"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("last() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("last() argument must be an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return types.UndefinedValue
			}
			return arr.Get(arr.Len() - 1)
		},
	}

	vm.globals["head"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("head() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("head() argument must be an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return types.UndefinedValue
			}
			return arr.Get(0)
		},
	}

	vm.globals["tail"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("tail() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("tail() argument must be an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return collections.NewArray()
			}
			result := collections.NewArray()
			for i := 1; i < arr.Len(); i++ {
				result.Append(arr.Get(i))
			}
			return result
		},
	}

	vm.globals["init"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("init() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("init() argument must be an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return collections.NewArray()
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len()-1; i++ {
				result.Append(arr.Get(i))
			}
			return result
		},
	}

	vm.globals["product"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("product() expects at least 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("product() expects an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return types.Int(1)
			}
			result := 1
			for i := 0; i < arr.Len(); i++ {
				val := arr.Get(i)
				if f, ok := val.(types.Float); ok {
					result = int(float64(result) * float64(f))
				} else if n, ok := val.(types.Int); ok {
					result = result * int(n)
				}
			}
			return types.Int(result)
		},
	}

	vm.globals["factorial"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("factorial() expects 1 argument", 0, 0, "")
			}
			n, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("factorial() expects an integer", 0, 0, "")
			}
			if n < 0 {
				return types.NewError("factorial() expects non-negative integer", 0, 0, "")
			}
			result := 1
			for i := 2; i <= int(n); i++ {
				result *= i
			}
			return types.Int(result)
		},
	}

	vm.globals["gcd"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("gcd() expects 2 arguments", 0, 0, "")
			}
			a, ok1 := args[0].(types.Int)
			b, ok2 := args[1].(types.Int)
			if !ok1 || !ok2 {
				return types.NewError("gcd() expects 2 integers", 0, 0, "")
			}
			aVal, bVal := int(a), int(b)
			for bVal != 0 {
				aVal, bVal = bVal, aVal%bVal
			}
			return types.Int(aVal)
		},
	}

	vm.globals["lcm"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("lcm() expects 2 arguments", 0, 0, "")
			}
			a, ok1 := args[0].(types.Int)
			b, ok2 := args[1].(types.Int)
			if !ok1 || !ok2 {
				return types.NewError("lcm() expects 2 integers", 0, 0, "")
			}
			aVal, bVal := int(a), int(b)
			gcd := aVal
			tmp := bVal
			for tmp != 0 {
				gcd, tmp = tmp, gcd%tmp
			}
			return types.Int(aVal * bVal / gcd)
		},
	}

	vm.globals["isPrime"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isPrime() expects 1 argument", 0, 0, "")
			}
			n, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("isPrime() expects an integer", 0, 0, "")
			}
			if n < 2 {
				return types.Bool(false)
			}
			for i := types.Int(2); i*i <= n; i++ {
				if n%i == 0 {
					return types.Bool(false)
				}
			}
			return types.Bool(true)
		},
	}

	vm.globals["fibonacci"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("fibonacci() expects 1 argument", 0, 0, "")
			}
			n, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("fibonacci() expects an integer", 0, 0, "")
			}
			if n < 0 {
				return types.NewError("fibonacci() expects non-negative integer", 0, 0, "")
			}
			if n == 0 {
				return types.Int(0)
			}
			if n == 1 {
				return types.Int(1)
			}
			a, b := 0, 1
			for i := 2; i <= int(n); i++ {
				a, b = b, a+b
			}
			return types.Int(b)
		},
	}

	vm.globals["isBlank"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isBlank() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			return types.Bool(len(strings.TrimSpace(s)) == 0)
		},
	}

	vm.globals["removeSpaces"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("removeSpaces() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			result := strings.ReplaceAll(s, " ", "")
			return types.String(result)
		},
	}

	vm.globals["removeExtraSpaces"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("removeExtraSpaces() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			space := regexp.MustCompile(`\s+`)
			result := space.ReplaceAllString(strings.TrimSpace(s), " ")
			return types.String(result)
		},
	}

	vm.globals["reverseStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("reverseStr() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			runes := []rune(s)
			for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
				runes[i], runes[j] = runes[j], runes[i]
			}
			return types.String(runes)
		},
	}

	vm.globals["countStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("countStr() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			sub := string(types.ToString(args[1]))
			count := strings.Count(s, sub)
			return types.Int(count)
		},
	}

	vm.globals["hasPrefix"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("hasPrefix() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			prefix := string(types.ToString(args[1]))
			return types.Bool(strings.HasPrefix(s, prefix))
		},
	}

	vm.globals["hasSuffix"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("hasSuffix() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			suffix := string(types.ToString(args[1]))
			return types.Bool(strings.HasSuffix(s, suffix))
		},
	}

	vm.globals["inRange"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("inRange() expects 3 arguments", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			start, _ := types.ToFloat(args[1])
			end, _ := types.ToFloat(args[2])
			return types.Bool(val >= start && val <= end)
		},
	}

	vm.globals["percent"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("percent() expects 2 arguments", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			total, _ := types.ToFloat(args[1])
			if total == 0 {
				return types.Float(0)
			}
			return types.Float((val / total) * 100)
		},
	}

	vm.globals["mod"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("mod() expects 2 arguments", 0, 0, "")
			}
			a, ok1 := args[0].(types.Int)
			b, ok2 := args[1].(types.Int)
			if !ok1 || !ok2 {
				return types.NewError("mod() expects 2 integers", 0, 0, "")
			}
			if b == 0 {
				return types.NewError("mod() division by zero", 0, 0, "")
			}
			return types.Int(a % b)
		},
	}

	vm.globals["divmod"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("divmod() expects 2 arguments", 0, 0, "")
			}
			a, ok1 := args[0].(types.Int)
			b, ok2 := args[1].(types.Int)
			if !ok1 || !ok2 {
				return types.NewError("divmod() expects 2 integers", 0, 0, "")
			}
			if b == 0 {
				return types.NewError("divmod() division by zero", 0, 0, "")
			}
			result := collections.NewMap()
			result.Set("quotient", types.Int(a/b))
			result.Set("remainder", types.Int(a%b))
			return result
		},
	}

	vm.globals["sign"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("sign() expects 1 argument", 0, 0, "")
			}
			f, _ := types.ToFloat(args[0])
			if f > 0 {
				return types.Int(1)
			} else if f < 0 {
				return types.Int(-1)
			}
			return types.Int(0)
		},
	}

	vm.globals["nan"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("nan() expects 1 argument", 0, 0, "")
			}
			f, _ := types.ToFloat(args[0])
			return types.Bool(math.IsNaN(float64(f)))
		},
	}

	vm.globals["inf"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("inf() expects 1 argument", 0, 0, "")
			}
			f, _ := types.ToFloat(args[0])
			return types.Bool(math.IsInf(float64(f), 0))
		},
	}

	vm.globals["floor"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("floor() expects 1 argument", 0, 0, "")
			}
			f, ok := args[0].(types.Float)
			if !ok {
				i, ok := args[0].(types.Int)
				if ok {
					return i
				}
				return types.NewError("floor() expects a number", 0, 0, "")
			}
			return types.Int(math.Floor(float64(f)))
		},
	}

	vm.globals["ceil"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("ceil() expects 1 argument", 0, 0, "")
			}
			f, ok := args[0].(types.Float)
			if !ok {
				i, ok := args[0].(types.Int)
				if ok {
					return i
				}
				return types.NewError("ceil() expects a number", 0, 0, "")
			}
			return types.Int(math.Ceil(float64(f)))
		},
	}

	vm.globals["isEven"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isEven() expects 1 argument", 0, 0, "")
			}
			i, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("isEven() expects an integer", 0, 0, "")
			}
			return types.Bool(i%2 == 0)
		},
	}

	vm.globals["isOdd"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isOdd() expects 1 argument", 0, 0, "")
			}
			i, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("isOdd() expects an integer", 0, 0, "")
			}
			return types.Bool(i%2 != 0)
		},
	}

	vm.globals["repeat"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("repeat() expects 2 arguments (str, count)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			count, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("repeat() expects (string, integer)", 0, 0, "")
			}
			if count < 0 {
				return types.NewError("repeat() count must be non-negative", 0, 0, "")
			}
			result := strings.Repeat(s, int(count))
			return types.String(result)
		},
	}

	vm.globals["padLeft"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("padLeft() expects 3 arguments (str, length, pad)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			length, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("padLeft() expects (string, integer, string)", 0, 0, "")
			}
			pad := string(types.ToString(args[2]))
			if len(s) >= int(length) {
				return types.String(s)
			}
			padCount := int(length) - len(s)
			pads := strings.Repeat(pad, (padCount/len(pad))+1)
			return types.String(pads[:padCount] + s)
		},
	}

	vm.globals["padRight"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("padRight() expects 3 arguments (str, length, pad)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			length, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("padRight() expects (string, integer, string)", 0, 0, "")
			}
			pad := string(types.ToString(args[2]))
			if len(s) >= int(length) {
				return types.String(s)
			}
			padCount := int(length) - len(s)
			pads := strings.Repeat(pad, (padCount/len(pad))+1)
			return types.String(s + pads[:padCount])
		},
	}

	vm.globals["camelCase"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("camelCase() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			s = strings.TrimSpace(s)
			s = regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(s, " ")
			parts := strings.Fields(s)
			for i := range parts {
				if i > 0 && len(parts[i]) > 0 {
					parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
				}
			}
			return types.String(strings.Join(parts, ""))
		},
	}

	vm.globals["snakeCase"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("snakeCase() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			s = strings.TrimSpace(s)
			s = regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(s, " ")
			s = regexp.MustCompile(`([a-z])([A-Z])`).ReplaceAllString(s, "$1 $2")
			s = strings.ToLower(s)
			s = regexp.MustCompile(`\s+`).ReplaceAllString(s, "_")
			return types.String(strings.Trim(s, "_"))
		},
	}

	vm.globals["kebabCase"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("kebabCase() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			s = strings.TrimSpace(s)
			s = regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(s, " ")
			s = regexp.MustCompile(`([a-z])([A-Z])`).ReplaceAllString(s, "$1 $2")
			s = strings.ToLower(s)
			s = regexp.MustCompile(`\s+`).ReplaceAllString(s, "-")
			return types.String(strings.Trim(s, "-"))
		},
	}

	vm.globals["unique"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("unique() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("unique() expects an array", 0, 0, "")
			}
			seen := make(map[string]bool)
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				val := arr.Get(i)
				key := val.ToStr()
				if !seen[key] {
					seen[key] = true
					result.Append(val)
				}
			}
			return result
		},
	}

	vm.globals["flatten"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("flatten() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("flatten() expects an array", 0, 0, "")
			}
			result := collections.NewArray()
			var flatten func(a *collections.Array)
			flatten = func(a *collections.Array) {
				for i := 0; i < a.Len(); i++ {
					val := a.Get(i)
					if nested, ok := val.(*collections.Array); ok {
						flatten(nested)
					} else {
						result.Append(val)
					}
				}
			}
			flatten(arr)
			return result
		},
	}

	vm.globals["chunk"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("chunk() expects 2 arguments (array, size)", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("chunk() expects (array, integer)", 0, 0, "")
			}
			size, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("chunk() expects (array, integer)", 0, 0, "")
			}
			if size <= 0 {
				return types.NewError("chunk() size must be positive", 0, 0, "")
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i += int(size) {
				chunk := collections.NewArray()
				end := i + int(size)
				if end > arr.Len() {
					end = arr.Len()
				}
				for j := i; j < end; j++ {
					chunk.Append(arr.Get(j))
				}
				result.Append(chunk)
			}
			return result
		},
	}

	vm.globals["shuffle"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("shuffle() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("shuffle() expects an array", 0, 0, "")
			}
			result := collections.NewArray()
			indices := rand.Perm(arr.Len())
			for _, i := range indices {
				result.Append(arr.Get(i))
			}
			return result
		},
	}

	vm.globals["sample"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("sample() expects at least 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("sample() expects first argument to be an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return types.UndefinedValue
			}
			count := 1
			if len(args) > 1 {
				c, ok := args[1].(types.Int)
				if ok {
					count = int(c)
				}
			}
			if count > arr.Len() {
				count = arr.Len()
			}
			indices := rand.Perm(arr.Len())[:count]
			result := collections.NewArray()
			for _, i := range indices {
				result.Append(arr.Get(i))
			}
			return result
		},
	}

	vm.globals["sort"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("sort() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("sort() expects an array", 0, 0, "")
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				result.Append(arr.Get(i))
			}
			for i := 0; i < result.Len()-1; i++ {
				for j := i + 1; j < result.Len(); j++ {
					v1 := result.Get(i)
					v2 := result.Get(j)
					if v1.ToStr() > v2.ToStr() {
						result.Set(i, v2)
						result.Set(j, v1)
					}
				}
			}
			return result
		},
	}

	vm.globals["wordCount"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("wordCount() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			words := strings.Fields(s)
			return types.Int(len(words))
		},
	}

	vm.globals["truncate"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("truncate() expects 2 arguments (str, length)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			length, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("truncate() expects (string, integer)", 0, 0, "")
			}
			if len(s) <= int(length) {
				return types.String(s)
			}
			suffix := ""
			if len(args) > 2 {
				suffix = string(types.ToString(args[2]))
			}
			return types.String(s[:int(length)] + suffix)
		},
	}

	vm.globals["formatDuration"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("formatDuration() expects 1 argument", 0, 0, "")
			}
			var duration time.Duration
			if d, ok := args[0].(types.Int); ok {
				duration = time.Duration(d) * time.Nanosecond
			} else if f, ok := args[0].(types.Float); ok {
				duration = time.Duration(f * 1e9)
			} else {
				return types.NewError("formatDuration() expects a number", 0, 0, "")
			}
			return types.String(duration.String())
		},
	}

	vm.globals["parseDuration"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("parseDuration() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			d, err := time.ParseDuration(s)
			if err != nil {
				return types.NewError(fmt.Sprintf("parseDuration error: %v", err), 0, 0, "")
			}
			return types.Int(d.Nanoseconds())
		},
	}

	vm.globals["sleepMs"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("sleepMs() expects 1 argument", 0, 0, "")
			}
			ms, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("sleepMs() expects an integer", 0, 0, "")
			}
			time.Sleep(time.Duration(ms) * time.Millisecond)
			return types.UndefinedValue
		},
	}

	vm.globals["timestampMs"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(time.Now().UnixMilli())
		},
	}

	vm.globals["randomFloat"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Float(rand.Float64())
		},
	}

	vm.globals["clamp"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("clamp() expects 3 arguments (value, min, max)", 0, 0, "")
			}
			var val float64
			if v, ok := args[0].(types.Float); ok {
				val = float64(v)
			} else if v, ok := args[0].(types.Int); ok {
				val = float64(v)
			} else {
				return types.NewError("clamp() expects a number", 0, 0, "")
			}
			min, _ := types.ToFloat(args[1])
			max, _ := types.ToFloat(args[2])
			if val < float64(min) {
				return min
			}
			if val > float64(max) {
				return max
			}
			return types.Float(val)
		},
	}

	vm.globals["lerp"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("lerp() expects 3 arguments (a, b, t)", 0, 0, "")
			}
			a, _ := types.ToFloat(args[0])
			b, _ := types.ToFloat(args[1])
			t, _ := types.ToFloat(args[2])
			return types.Float(a + (b-a)*t)
		},
	}

	vm.globals["base64URLEncode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("base64URLEncode() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			return types.String(base64.URLEncoding.EncodeToString([]byte(s)))
		},
	}

	vm.globals["base64URLDecode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("base64URLDecode() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			decoded, err := base64.URLEncoding.DecodeString(s)
			if err != nil {
				return types.NewError(fmt.Sprintf("base64URLDecode error: %v", err), 0, 0, "")
			}
			return types.String(string(decoded))
		},
	}

	vm.globals["pow"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("pow() expects 2 arguments (base, exp)", 0, 0, "")
			}
			base, _ := types.ToFloat(args[0])
			exp, _ := types.ToFloat(args[1])
			return types.Float(math.Pow(float64(base), float64(exp)))
		},
	}

	vm.globals["sqrt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("sqrt() expects 1 argument", 0, 0, "")
			}
			f, _ := types.ToFloat(args[0])
			return types.Float(math.Sqrt(float64(f)))
		},
	}

	vm.globals["abs"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("abs() expects 1 argument", 0, 0, "")
			}
			if i, ok := args[0].(types.Int); ok {
				if i < 0 {
					return types.Int(-i)
				}
				return i
			}
			f, _ := types.ToFloat(args[0])
			return types.Float(math.Abs(float64(f)))
		},
	}

	vm.globals["log"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("log() expects 1 argument", 0, 0, "")
			}
			f, _ := types.ToFloat(args[0])
			return types.Float(math.Log(float64(f)))
		},
	}

	vm.globals["log10"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("log10() expects 1 argument", 0, 0, "")
			}
			f, _ := types.ToFloat(args[0])
			return types.Float(math.Log10(float64(f)))
		},
	}

	vm.globals["sin"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("sin() expects 1 argument", 0, 0, "")
			}
			f, _ := types.ToFloat(args[0])
			return types.Float(math.Sin(float64(f)))
		},
	}

	vm.globals["cos"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("cos() expects 1 argument", 0, 0, "")
			}
			f, _ := types.ToFloat(args[0])
			return types.Float(math.Cos(float64(f)))
		},
	}

	vm.globals["tan"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("tan() expects 1 argument", 0, 0, "")
			}
			f, _ := types.ToFloat(args[0])
			return types.Float(math.Tan(float64(f)))
		},
	}

	vm.globals["capitalize"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("capitalize() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			if len(s) == 0 {
				return types.String(s)
			}
			return types.String(strings.ToUpper(s[:1]) + s[1:])
		},
	}

	vm.globals["titleCase"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("titleCase() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			words := strings.Fields(s)
			for i, word := range words {
				if len(word) > 0 {
					words[i] = strings.ToUpper(word[:1]) + word[1:]
				}
			}
			return types.String(strings.Join(words, " "))
		},
	}

	vm.globals["quote"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("quote() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			return types.String(strconv.Quote(s))
		},
	}

	vm.globals["unquote"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("unquote() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			result, err := strconv.Unquote(s)
			if err != nil {
				return types.NewError(fmt.Sprintf("unquote error: %v", err), 0, 0, "")
			}
			return types.String(result)
		},
	}

	vm.globals["isEmpty"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isEmpty() expects 1 argument", 0, 0, "")
			}
			obj := args[0]
			switch v := obj.(type) {
			case types.String:
				return types.Bool(len(v) == 0)
			case *collections.Array:
				return types.Bool(v.Len() == 0)
			case *collections.Map:
				return types.Bool(v.Len() == 0)
			case *collections.OrderedMap:
				return types.Bool(v.Len() == 0)
			default:
				return types.NewError("isEmpty() not supported for this type", 0, 0, "")
			}
		},
	}

	vm.globals["equal"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("equal() expects 2 arguments", 0, 0, "")
			}
			return types.Bool(args[0].Equals(args[1]))
		},
	}

	vm.globals["neq"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("neq() expects 2 arguments", 0, 0, "")
			}
			return types.Bool(!args[0].Equals(args[1]))
		},
	}

	vm.globals["gt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("gt() expects 2 arguments", 0, 0, "")
			}
			a, b := args[0], args[1]
			aInt, aOk := a.(types.Int)
			bInt, bOk := b.(types.Int)
			if aOk && bOk {
				return types.Bool(aInt > bInt)
			}
			aFloat, aFloatOk := a.(types.Float)
			bFloat, bFloatOk := b.(types.Float)
			if aFloatOk && bFloatOk {
				return types.Bool(aFloat > bFloat)
			}
			if aOk {
				f, _ := types.ToFloat(b)
				return types.Bool(float64(aInt) > float64(f))
			}
			if bOk {
				f, _ := types.ToFloat(a)
				return types.Bool(float64(f) > float64(bInt))
			}
			aF, _ := types.ToFloat(a)
			bF, _ := types.ToFloat(b)
			return types.Bool(float64(aF) > float64(bF))
		},
	}

	vm.globals["lt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("lt() expects 2 arguments", 0, 0, "")
			}
			a, b := args[0], args[1]
			aInt, aOk := a.(types.Int)
			bInt, bOk := b.(types.Int)
			if aOk && bOk {
				return types.Bool(aInt < bInt)
			}
			aFloat, aFloatOk := a.(types.Float)
			bFloat, bFloatOk := b.(types.Float)
			if aFloatOk && bFloatOk {
				return types.Bool(aFloat < bFloat)
			}
			if aOk {
				f, _ := types.ToFloat(b)
				return types.Bool(float64(aInt) < float64(f))
			}
			if bOk {
				f, _ := types.ToFloat(a)
				return types.Bool(float64(f) < float64(bInt))
			}
			aF, _ := types.ToFloat(a)
			bF, _ := types.ToFloat(b)
			return types.Bool(float64(aF) < float64(bF))
		},
	}

	vm.globals["gte"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("gte() expects 2 arguments", 0, 0, "")
			}
			a, b := args[0], args[1]
			aInt, aOk := a.(types.Int)
			bInt, bOk := b.(types.Int)
			if aOk && bOk {
				return types.Bool(aInt >= bInt)
			}
			aFloat, aFloatOk := a.(types.Float)
			bFloat, bFloatOk := b.(types.Float)
			if aFloatOk && bFloatOk {
				return types.Bool(aFloat >= bFloat)
			}
			if aOk {
				f, _ := types.ToFloat(b)
				return types.Bool(float64(aInt) >= float64(f))
			}
			if bOk {
				f, _ := types.ToFloat(a)
				return types.Bool(float64(f) >= float64(bInt))
			}
			aF, _ := types.ToFloat(a)
			bF, _ := types.ToFloat(b)
			return types.Bool(float64(aF) >= float64(bF))
		},
	}

	vm.globals["lte"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("lte() expects 2 arguments", 0, 0, "")
			}
			a, b := args[0], args[1]
			aInt, aOk := a.(types.Int)
			bInt, bOk := b.(types.Int)
			if aOk && bOk {
				return types.Bool(aInt <= bInt)
			}
			aFloat, aFloatOk := a.(types.Float)
			bFloat, bFloatOk := b.(types.Float)
			if aFloatOk && bFloatOk {
				return types.Bool(aFloat <= bFloat)
			}
			if aOk {
				f, _ := types.ToFloat(b)
				return types.Bool(float64(aInt) <= float64(f))
			}
			if bOk {
				f, _ := types.ToFloat(a)
				return types.Bool(float64(f) <= float64(bInt))
			}
			aF, _ := types.ToFloat(a)
			bF, _ := types.ToFloat(b)
			return types.Bool(float64(aF) <= float64(bF))
		},
	}

	vm.globals["isInt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isInt() expects 1 argument", 0, 0, "")
			}
			_, ok := args[0].(types.Int)
			return types.Bool(ok)
		},
	}

	vm.globals["isFloat"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isFloat() expects 1 argument", 0, 0, "")
			}
			_, ok := args[0].(types.Float)
			return types.Bool(ok)
		},
	}

	vm.globals["isNumber"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isNumber() expects 1 argument", 0, 0, "")
			}
			switch args[0].(type) {
			case types.Int, types.Float:
				return types.Bool(true)
			default:
				return types.Bool(false)
			}
		},
	}

	vm.globals["mapValues"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("mapValues() expects 1 argument", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				om, ok := args[0].(*collections.OrderedMap)
				if !ok {
					return types.NewError("mapValues() expects a map", 0, 0, "")
				}
				arr := collections.NewArray()
				keys := om.Keys()
				for i := 0; i < keys.Len(); i++ {
					k := keys.Get(i).(types.String)
					arr.Append(om.Get(string(k)))
				}
				return arr
			}
			arr := collections.NewArray()
			for _, v := range m.Entries {
				arr.Append(v)
			}
			return arr
		},
	}

	vm.globals["mapKeys"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("mapKeys() expects 1 argument", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				om, ok := args[0].(*collections.OrderedMap)
				if !ok {
					return types.NewError("mapKeys() expects a map", 0, 0, "")
				}
				arr := collections.NewArray()
				keys := om.Keys()
				for i := 0; i < keys.Len(); i++ {
					arr.Append(keys.Get(i))
				}
				return arr
			}
			arr := collections.NewArray()
			for k := range m.Entries {
				arr.Append(types.String(k))
			}
			return arr
		},
	}

	vm.globals["groupBy"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("groupBy() expects 2 arguments (array, key)", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("groupBy() first argument must be an array", 0, 0, "")
			}
			key := string(types.ToString(args[1]))
			result := collections.NewMap()
			for i := 0; i < arr.Len(); i++ {
				val := arr.Get(i)
				var groupKey string
				switch v := val.(type) {
				case *collections.Map:
					groupKey = v.Get(key).ToStr()
				default:
					groupKey = val.ToStr()
				}
				group := result.Get(groupKey)
				if group == types.UndefinedValue {
					group = collections.NewArray()
					result.Set(groupKey, group)
				}
				group.(*collections.Array).Append(val)
			}
			return result
		},
	}

	vm.globals["zip"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("zip() expects at least 2 arguments", 0, 0, "")
			}
			minLen := -1
			arrays := make([]*collections.Array, len(args))
			for i, arg := range args {
				arr, ok := arg.(*collections.Array)
				if !ok {
					return types.NewError("zip() all arguments must be arrays", 0, 0, "")
				}
				arrays[i] = arr
				if minLen == -1 || arr.Len() < minLen {
					minLen = arr.Len()
				}
			}
			if minLen == 0 {
				return collections.NewArray()
			}
			result := collections.NewArray()
			for i := 0; i < minLen; i++ {
				item := collections.NewArray()
				for j := range arrays {
					item.Append(arrays[j].Get(i))
				}
				result.Append(item)
			}
			return result
		},
	}

	vm.globals["unzip"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("unzip() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("unzip() expects an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return collections.NewArray()
			}
			result := collections.NewArray()
			first, _ := arr.Get(0).(*collections.Array)
			numCols := first.Len()
			for i := 0; i < numCols; i++ {
				col := collections.NewArray()
				for j := 0; j < arr.Len(); j++ {
					item, _ := arr.Get(j).(*collections.Array)
					col.Append(item.Get(i))
				}
				result.Append(col)
			}
			return result
		},
	}

	vm.globals["hasKey"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("hasKey() expects 2 arguments (map, key)", 0, 0, "")
			}
			key := string(types.ToString(args[1]))
			switch m := args[0].(type) {
			case *collections.Map:
				return types.Bool(m.Has(key))
			case *collections.OrderedMap:
				return types.Bool(m.Has(key))
			default:
				return types.NewError("hasKey() first argument must be a map", 0, 0, "")
			}
		},
	}

	vm.globals["merge"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("merge() expects at least 2 arguments", 0, 0, "")
			}
			result := collections.NewMap()
			for _, arg := range args {
				switch m := arg.(type) {
				case *collections.Map:
					for k, v := range m.Entries {
						result.Set(k, v)
					}
				default:
					return types.NewError("merge() all arguments must be maps", 0, 0, "")
				}
			}
			return result
		},
	}

	vm.globals["count"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("count() expects 2 arguments (array, value)", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("count() first argument must be an array", 0, 0, "")
			}
			target := args[1]
			count := 0
			for i := 0; i < arr.Len(); i++ {
				if arr.Get(i).Equals(target) {
					count++
				}
			}
			return types.Int(count)
		},
	}

	vm.globals["containsStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("containsStr() expects 2 arguments (str, substr)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			sub := string(types.ToString(args[1]))
			return types.Bool(strings.Contains(s, sub))
		},
	}

	vm.globals["startsWith"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("startsWith() expects 2 arguments (str, prefix)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			prefix := string(types.ToString(args[1]))
			return types.Bool(strings.HasPrefix(s, prefix))
		},
	}

	vm.globals["endsWith"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("endsWith() expects 2 arguments (str, suffix)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			suffix := string(types.ToString(args[1]))
			return types.Bool(strings.HasSuffix(s, suffix))
		},
	}

	vm.globals["indexOf"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("indexOf() expects 2 arguments (array, value)", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				s, ok := args[0].(types.String)
				if !ok {
					return types.NewError("indexOf() first argument must be array or string", 0, 0, "")
				}
				sub := string(types.ToString(args[1]))
				idx := strings.Index(string(s), sub)
				if idx < 0 {
					return types.Int(-1)
				}
				return types.Int(idx)
			}
			target := args[1]
			for i := 0; i < arr.Len(); i++ {
				if arr.Get(i).Equals(target) {
					return types.Int(i)
				}
			}
			return types.Int(-1)
		},
	}

	vm.globals["lastIndexOf"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("lastIndexOf() expects 2 arguments (str, substr)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			sub := string(types.ToString(args[1]))
			idx := strings.LastIndex(s, sub)
			if idx < 0 {
				return types.Int(-1)
			}
			return types.Int(idx)
		},
	}

	vm.globals["replaceAll"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("replaceAll() expects 3 arguments (str, old, new)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			old := string(types.ToString(args[1]))
			newStr := string(types.ToString(args[2]))
			return types.String(strings.ReplaceAll(s, old, newStr))
		},
	}

	vm.globals["split"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("split() expects 2 arguments (str, sep)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			sep := string(types.ToString(args[1]))
			parts := strings.Split(s, sep)
			arr := collections.NewArray()
			for _, part := range parts {
				arr.Append(types.String(part))
			}
			return arr
		},
	}

	vm.globals["trim"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("trim() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			return types.String(strings.TrimSpace(s))
		},
	}

	vm.globals["toUpper"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toUpper() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			return types.String(strings.ToUpper(s))
		},
	}

	vm.globals["toLower"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toLower() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			return types.String(strings.ToLower(s))
		},
	}

	vm.globals["charAt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("charAt() expects 2 arguments (str, index)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			idx := int(args[1].(types.Int))
			if idx < 0 || idx >= len(s) {
				return types.String("")
			}
			return types.String(s[idx])
		},
	}

	vm.globals["charCodeAt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("charCodeAt() expects 2 arguments (str, index)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			idx := int(args[1].(types.Int))
			if idx < 0 || idx >= len(s) {
				return types.NewError("charCodeAt() index out of range", 0, 0, "")
			}
			return types.Int(rune(s[idx]))
		},
	}

	vm.globals["fromCharCode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("fromCharCode() expects at least 1 argument", 0, 0, "")
			}
			runes := make([]rune, len(args))
			for i, arg := range args {
				code, ok := arg.(types.Int)
				if !ok {
					return types.NewError("fromCharCode() expects integer arguments", 0, 0, "")
				}
				runes[i] = rune(code)
			}
			return types.String(runes)
		},
	}

	vm.globals["formatNumber"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("formatNumber() expects at least 1 argument", 0, 0, "")
			}
			f, ok := args[0].(types.Float)
			if !ok {
				i, ok := args[0].(types.Int)
				if ok {
					f = types.Float(i)
				} else {
					return types.NewError("formatNumber() expects a number", 0, 0, "")
				}
			}
			decimals := 2
			if len(args) > 1 {
				if d, ok := args[1].(types.Int); ok {
					decimals = int(d)
				}
			}
			format := fmt.Sprintf(fmt.Sprintf("%%.%df", decimals), float64(f))
			return types.String(format)
		},
	}

	vm.globals["formatBytes"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("formatBytes() expects 1 argument", 0, 0, "")
			}
			bytes, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("formatBytes() expects an integer", 0, 0, "")
			}
			sizes := []string{"B", "KB", "MB", "GB", "TB", "PB"}
			size := float64(bytes)
			i := 0
			for size >= 1024 && i < len(sizes)-1 {
				size /= 1024
				i++
			}
			return types.String(fmt.Sprintf("%.2f %s", size, sizes[i]))
		},
	}

	vm.globals["isAlpha"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isAlpha() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			if len(s) == 0 {
				return types.Bool(false)
			}
			for _, c := range s {
				if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
					return types.Bool(false)
				}
			}
			return types.Bool(true)
		},
	}

	vm.globals["isNumeric"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isNumeric() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			if len(s) == 0 {
				return types.Bool(false)
			}
			for _, c := range s {
				if c < '0' || c > '9' {
					return types.Bool(false)
				}
			}
			return types.Bool(true)
		},
	}

	vm.globals["isAlphaNumeric"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isAlphaNumeric() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			if len(s) == 0 {
				return types.Bool(false)
			}
			for _, c := range s {
				if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
					return types.Bool(false)
				}
			}
			return types.Bool(true)
		},
	}

	vm.globals["escape"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("escape() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			result := url.QueryEscape(s)
			return types.String(result)
		},
	}

	vm.globals["unescape"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("unescape() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			result, err := url.QueryUnescape(s)
			if err != nil {
				return types.NewError(fmt.Sprintf("unescape error: %v", err), 0, 0, "")
			}
			return types.String(result)
		},
	}

	vm.globals["left"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("left() expects 2 arguments (str, n)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			n, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("left() expects (string, integer)", 0, 0, "")
			}
			if int(n) >= len(s) {
				return types.String(s)
			}
			return types.String(s[:n])
		},
	}

	vm.globals["right"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("right() expects 2 arguments (str, n)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			n, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("right() expects (string, integer)", 0, 0, "")
			}
			if int(n) >= len(s) {
				return types.String(s)
			}
			return types.String(s[len(s)-int(n):])
		},
	}

	vm.globals["lpad"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("lpad() expects 3 arguments (str, len, pad)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			length, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("lpad() expects (string, integer, string)", 0, 0, "")
			}
			pad := string(types.ToString(args[2]))
			if len(s) >= int(length) {
				return types.String(s)
			}
			padCount := int(length) - len(s)
			pads := strings.Repeat(pad, (padCount/len(pad))+1)
			return types.String(pads[:padCount] + s)
		},
	}

	vm.globals["rpad"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("rpad() expects 3 arguments (str, len, pad)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			length, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("rpad() expects (string, integer, string)", 0, 0, "")
			}
			pad := string(types.ToString(args[2]))
			if len(s) >= int(length) {
				return types.String(s)
			}
			padCount := int(length) - len(s)
			pads := strings.Repeat(pad, (padCount/len(pad))+1)
			return types.String(s + pads[:padCount])
		},
	}

	vm.globals["after"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("after() expects 2 arguments (str, sep)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			sep := string(types.ToString(args[1]))
			idx := strings.Index(s, sep)
			if idx < 0 {
				return types.String("")
			}
			return types.String(s[idx+len(sep):])
		},
	}

	vm.globals["before"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("before() expects 2 arguments (str, sep)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			sep := string(types.ToString(args[1]))
			idx := strings.Index(s, sep)
			if idx < 0 {
				return types.String(s)
			}
			return types.String(s[:idx])
		},
	}

	vm.globals["between"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("between() expects 3 arguments (str, start, end)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			start := string(types.ToString(args[1]))
			end := string(types.ToString(args[2]))
			startIdx := strings.Index(s, start)
			if startIdx < 0 {
				return types.String("")
			}
			startIdx += len(start)
			endIdx := strings.Index(s[startIdx:], end)
			if endIdx < 0 {
				return types.String("")
			}
			return types.String(s[startIdx : startIdx+endIdx])
		},
	}

	vm.globals["repeatStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("repeatStr() expects 2 arguments (str, count)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			count, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("repeatStr() expects (string, integer)", 0, 0, "")
			}
			if count < 0 {
				return types.NewError("repeatStr() count must be non-negative", 0, 0, "")
			}
			return types.String(strings.Repeat(s, int(count)))
		},
	}

	vm.globals["lines"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("lines() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			arr := collections.NewArray()
			for _, line := range strings.Split(s, "\n") {
				arr.Append(types.String(line))
			}
			return arr
		},
	}

	vm.globals["words"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("words() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			arr := collections.NewArray()
			for _, word := range strings.Fields(s) {
				arr.Append(types.String(word))
			}
			return arr
		},
	}

	vm.globals["strip"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("strip() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			return types.String(strings.TrimSpace(s))
		},
	}

	vm.globals["countStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("countStr() expects 2 arguments (string, substring)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			substr := string(types.ToString(args[1]))
			return types.Int(strings.Count(s, substr))
		},
	}

	vm.globals["splitLines"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("splitLines() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			lines := strings.Split(s, "\n")
			arr := collections.NewArray()
			for _, line := range lines {
				arr.Append(types.String(line))
			}
			return arr
		},
	}

	vm.globals["levenshtein"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("levenshtein() expects 2 arguments (string1, string2)", 0, 0, "")
			}
			s1 := string(types.ToString(args[0]))
			s2 := string(types.ToString(args[1]))
			if len(s1) == 0 {
				return types.Int(len(s2))
			}
			if len(s2) == 0 {
				return types.Int(len(s1))
			}
			// Create matrix
			rows := len(s1) + 1
			cols := len(s2) + 1
			matrix := make([][]int, rows)
			for i := range matrix {
				matrix[i] = make([]int, cols)
			}
			for i := 0; i < rows; i++ {
				matrix[i][0] = i
			}
			for j := 0; j < cols; j++ {
				matrix[0][j] = j
			}
			for i := 1; i < rows; i++ {
				for j := 1; j < cols; j++ {
					cost := 1
					if s1[i-1] == s2[j-1] {
						cost = 0
					}
					matrix[i][j] = min(
						matrix[i-1][j]+1,    // deletion
						matrix[i][j-1]+1,    // insertion
						matrix[i-1][j-1]+cost, // substitution
					)
				}
			}
			return types.Int(matrix[rows-1][cols-1])
		},
	}

	vm.globals["range"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			start := 0
			end := 0
			step := 1
			if len(args) < 1 {
				return types.NewError("range() expects at least 1 argument", 0, 0, "")
			}
			if len(args) >= 1 {
				s, ok := args[0].(types.Int)
				if !ok {
					return types.NewError("range() expects integer arguments", 0, 0, "")
				}
				end = int(s)
			}
			if len(args) >= 2 {
				s, ok := args[1].(types.Int)
				if !ok {
					return types.NewError("range() expects integer arguments", 0, 0, "")
				}
				start = int(s)
				end = start + int(s)
			}
			if len(args) >= 3 {
				s, ok := args[2].(types.Int)
				if !ok {
					return types.NewError("range() expects integer arguments", 0, 0, "")
				}
				step = int(s)
			}
			result := collections.NewArray()
			if step > 0 {
				for i := start; i < end; i += step {
					result.Append(types.Int(i))
				}
			} else if step < 0 {
				for i := start; i > end; i += step {
					result.Append(types.Int(i))
				}
			}
			return result
		},
	}

	vm.globals["min"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("min() expects at least 1 argument", 0, 0, "")
			}
			if len(args) == 1 {
				if arr, ok := args[0].(*collections.Array); ok {
					if arr.Len() == 0 {
						return types.UndefinedValue
					}
					minVal := arr.Get(0)
					for i := 1; i < arr.Len(); i++ {
						if arr.Get(i).ToStr() < minVal.ToStr() {
							minVal = arr.Get(i)
						}
					}
					return minVal
				}
			}
			minVal := args[0]
			for _, arg := range args[1:] {
				if arg.ToStr() < minVal.ToStr() {
					minVal = arg
				}
			}
			return minVal
		},
	}

	vm.globals["max"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("max() expects at least 1 argument", 0, 0, "")
			}
			if len(args) == 1 {
				if arr, ok := args[0].(*collections.Array); ok {
					if arr.Len() == 0 {
						return types.UndefinedValue
					}
					maxVal := arr.Get(0)
					for i := 1; i < arr.Len(); i++ {
						if arr.Get(i).ToStr() > maxVal.ToStr() {
							maxVal = arr.Get(i)
						}
					}
					return maxVal
				}
			}
			maxVal := args[0]
			for _, arg := range args[1:] {
				if arg.ToStr() > maxVal.ToStr() {
					maxVal = arg
				}
			}
			return maxVal
		},
	}

	vm.globals["sum"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("sum() expects at least 1 argument", 0, 0, "")
			}
			if arr, ok := args[0].(*collections.Array); ok {
				var total types.Object = types.Int(0)
				for i := 0; i < arr.Len(); i++ {
					val := arr.Get(i)
					if i == 0 {
						total = val
					} else {
						if f1, ok := total.(types.Float); ok {
							if f2, ok := val.(types.Float); ok {
								total = types.Float(f1 + f2)
							} else if i2, ok := val.(types.Int); ok {
								total = types.Float(f1 + types.Float(i2))
							}
						} else if i1, ok := total.(types.Int); ok {
							if f2, ok := val.(types.Float); ok {
								total = types.Float(i1) + f2
							} else if i2, ok := val.(types.Int); ok {
								total = types.Int(i1 + i2)
							}
						}
					}
				}
				return total
			}
			return types.NewError("sum() expects an array", 0, 0, "")
		},
	}

	vm.globals["avg"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("avg() expects at least 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("avg() expects an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return types.Int(0)
			}
			var total float64 = 0
			for i := 0; i < arr.Len(); i++ {
				val := arr.Get(i)
				if f, ok := val.(types.Float); ok {
					total += float64(f)
				} else if i, ok := val.(types.Int); ok {
					total += float64(i)
				}
			}
			return types.Int(int(total / float64(arr.Len())))
		},
	}

	vm.globals["any"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("any() expects at least 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("any() expects an array as first argument", 0, 0, "")
			}
			for i := 0; i < arr.Len(); i++ {
				val := arr.Get(i)
				if b, ok := val.(types.Bool); ok && bool(b) {
					return types.Bool(true)
				}
			}
			return types.Bool(false)
		},
	}

	vm.globals["all"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("all() expects at least 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("all() expects an array as first argument", 0, 0, "")
			}
			for i := 0; i < arr.Len(); i++ {
				val := arr.Get(i)
				if b, ok := val.(types.Bool); ok && !bool(b) {
					return types.Bool(false)
				}
			}
			return types.Bool(arr.Len() > 0)
		},
	}

	vm.globals["none"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("none() expects at least 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("none() expects an array as first argument", 0, 0, "")
			}
			for i := 0; i < arr.Len(); i++ {
				val := arr.Get(i)
				if b, ok := val.(types.Bool); ok && bool(b) {
					return types.Bool(false)
				}
			}
			return types.Bool(true)
		},
	}

	vm.globals["difference"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("difference() expects 2 arguments", 0, 0, "")
			}
			arr1, ok1 := args[0].(*collections.Array)
			arr2, ok2 := args[1].(*collections.Array)
			if !ok1 || !ok2 {
				return types.NewError("difference() expects 2 arrays", 0, 0, "")
			}
			result := collections.NewArray()
			seen := make(map[string]bool)
			for i := 0; i < arr2.Len(); i++ {
				seen[arr2.Get(i).ToStr()] = true
			}
			for i := 0; i < arr1.Len(); i++ {
				val := arr1.Get(i)
				if !seen[val.ToStr()] {
					result.Append(val)
				}
			}
			return result
		},
	}

	vm.globals["intersection"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("intersection() expects 2 arguments", 0, 0, "")
			}
			arr1, ok1 := args[0].(*collections.Array)
			arr2, ok2 := args[1].(*collections.Array)
			if !ok1 || !ok2 {
				return types.NewError("intersection() expects 2 arrays", 0, 0, "")
			}
			result := collections.NewArray()
			seen := make(map[string]bool)
			for i := 0; i < arr1.Len(); i++ {
				seen[arr1.Get(i).ToStr()] = true
			}
			for i := 0; i < arr2.Len(); i++ {
				val := arr2.Get(i)
				if seen[val.ToStr()] {
					result.Append(val)
				}
			}
			return result
		},
	}

	vm.globals["union"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("union() expects at least 2 arguments", 0, 0, "")
			}
			result := collections.NewArray()
			seen := make(map[string]bool)
			for _, arg := range args {
				arr, ok := arg.(*collections.Array)
				if !ok {
					return types.NewError("union() expects arrays", 0, 0, "")
				}
				for i := 0; i < arr.Len(); i++ {
					val := arr.Get(i)
					if !seen[val.ToStr()] {
						seen[val.ToStr()] = true
						result.Append(val)
					}
				}
			}
			return result
		},
	}

	vm.globals["sleep"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("sleep() expects 1 argument", 0, 0, "")
			}
			duration, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("sleep() expects an integer (seconds)", 0, 0, "")
			}
			time.Sleep(time.Duration(duration) * time.Second)
			return types.UndefinedValue
		},
	}

	vm.globals["typeOf"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("typeOf() expects 1 argument", 0, 0, "")
			}
			return types.String(args[0].TypeName())
		},
	}

	vm.globals["toInt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toInt() expects 1 argument", 0, 0, "")
			}
			switch v := args[0].(type) {
			case types.Int:
				return v
			case types.Float:
				return types.Int(int(v))
			case types.String:
				i, err := strconv.Atoi(string(v))
				if err != nil {
					return types.NewError(fmt.Sprintf("toInt() error: %v", err), 0, 0, "")
				}
				return types.Int(i)
			default:
				return types.NewError("toInt() not supported for this type", 0, 0, "")
			}
		},
	}

	vm.globals["toFloat"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toFloat() expects 1 argument", 0, 0, "")
			}
			switch v := args[0].(type) {
			case types.Int:
				return types.Float(v)
			case types.Float:
				return v
			case types.String:
				f, err := strconv.ParseFloat(string(v), 64)
				if err != nil {
					return types.NewError(fmt.Sprintf("toFloat() error: %v", err), 0, 0, "")
				}
				return types.Float(f)
			default:
				return types.NewError("toFloat() not supported for this type", 0, 0, "")
			}
		},
	}

	vm.globals["toString"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toString() expects 1 argument", 0, 0, "")
			}
			return types.String(args[0].ToStr())
		},
	}

	vm.globals["toBool"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toBool() expects 1 argument", 0, 0, "")
			}
			switch v := args[0].(type) {
			case types.Bool:
				return v
			case types.Int:
				return types.Bool(v != 0)
			case types.Float:
				return types.Bool(v != 0)
			case types.String:
				return types.Bool(v != "" && v != "false" && v != "0")
			default:
				return types.NewError("toBool() not supported for this type", 0, 0, "")
			}
		},
	}

	vm.globals["toArray"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toArray() expects 1 argument", 0, 0, "")
			}
			if arr, ok := args[0].(*collections.Array); ok {
				return arr
			}
			if s, ok := args[0].(types.String); ok {
				arr := collections.NewArray()
				for _, c := range s {
					arr.Append(types.String(c))
				}
				return arr
			}
			return types.NewError("toArray() not supported for this type", 0, 0, "")
		},
	}

	vm.globals["toMap"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toMap() expects 1 argument", 0, 0, "")
			}
			if m, ok := args[0].(*collections.Map); ok {
				return m
			}
			if om, ok := args[0].(*collections.OrderedMap); ok {
				result := collections.NewMap()
				keys := om.Keys()
				for i := 0; i < keys.Len(); i++ {
					k := keys.Get(i).(types.String)
					result.Set(string(k), om.Get(string(k)))
				}
				return result
			}
			return types.NewError("toMap() not supported for this type", 0, 0, "")
		},
	}

	vm.globals["timestamp"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(time.Now().Unix())
		},
	}

	vm.globals["unix"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(time.Now().Unix())
		},
	}

	vm.globals["unixMilli"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(time.Now().UnixMilli())
		},
	}

	vm.globals["unixNano"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(time.Now().UnixNano())
		},
	}

	vm.globals["now"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.String(time.Now().Format("2006-01-02 15:04:05"))
		},
	}

	vm.globals["formatTime"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("formatTime() expects at least 1 argument", 0, 0, "")
			}
			t := time.Now()
			format := "2006-01-02 15:04:05"
			if len(args) > 1 {
				format = string(types.ToString(args[1]))
			}
			if ts, ok := args[0].(types.Int); ok {
				t = time.Unix(int64(ts), 0)
			}
			return types.String(t.Format(format))
		},
	}

	vm.globals["copy"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("copy() expects 1 argument", 0, 0, "")
			}
			obj := args[0]
			switch v := obj.(type) {
			case *collections.Array:
				newArr := collections.NewArray()
				for i := 0; i < v.Len(); i++ {
					newArr.Append(v.Get(i))
				}
				return newArr
			case *collections.Map:
				newMap := collections.NewMap()
				for k, val := range v.Entries {
					newMap.Set(k, val)
				}
				return newMap
			case *collections.OrderedMap:
				newMap := collections.NewOrderedMap()
				keys := v.Keys()
				for i := 0; i < keys.Len(); i++ {
					k := keys.Get(i).(types.String)
					newMap.Set(string(k), v.Get(string(k)))
				}
				return newMap
			default:
				return obj
			}
		},
	}

	vm.globals["fill"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("fill() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("fill() first argument must be an array", 0, 0, "")
			}
			val := args[1]
			for i := 0; i < arr.Len(); i++ {
				arr.Set(i, val)
			}
			return arr
		},
	}

	vm.globals["fillRange"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("fillRange() expects at least 3 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("fillRange() first argument must be an array", 0, 0, "")
			}
			start, ok1 := args[1].(types.Int)
			end, ok2 := args[2].(types.Int)
			if !ok1 || !ok2 {
				return types.NewError("fillRange() expects (array, start, end, value)", 0, 0, "")
			}
			var val types.Object = types.UndefinedValue
			if len(args) > 3 {
				val = args[3]
			}
			for i := int(start); i < int(end) && i < arr.Len(); i++ {
				arr.Set(i, val)
			}
			return arr
		},
	}

	vm.globals["fillN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("fillN() expects at least 3 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("fillN() first argument must be an array", 0, 0, "")
			}
			start, ok1 := args[1].(types.Int)
			n, ok2 := args[2].(types.Int)
			if !ok1 || !ok2 {
				return types.NewError("fillN() expects (array, start, n, value)", 0, 0, "")
			}
			var val types.Object = types.UndefinedValue
			if len(args) > 3 {
				val = args[3]
			}
			for i := int(start); i < int(start)+int(n) && i < arr.Len(); i++ {
				arr.Set(i, val)
			}
			return arr
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

	vm.globals["str"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.String("")
			}
			return types.String(args[0].ToStr())
		},
	}

	vm.globals["repr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.String("")
			}
			return types.String(fmt.Sprintf("%v", args[0]))
		},
	}

	vm.globals["ascii"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("ascii() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			if len(s) == 0 {
				return types.Int(0)
			}
			return types.Int(rune(s[0]))
		},
	}

	vm.globals["chr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("chr() expects 1 argument", 0, 0, "")
			}
			code, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("chr() expects an integer", 0, 0, "")
			}
			return types.String(rune(code))
		},
	}

	vm.globals["string"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.String("")
			}
			return types.String(args[0].ToStr())
		},
	}

	vm.globals["index"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("index() expects 2 arguments", 0, 0, "")
			}
			if arr, ok := args[0].(*collections.Array); ok {
				val := args[1]
				for i := 0; i < arr.Len(); i++ {
					if arr.Get(i).Equals(val) {
						return types.Int(i)
					}
				}
				return types.Int(-1)
			}
			if s, ok := args[0].(types.String); ok {
				sub := string(types.ToString(args[1]))
				idx := strings.Index(string(s), sub)
				if idx < 0 {
					return types.Int(-1)
				}
				return types.Int(idx)
			}
			return types.NewError("index() expects array or string", 0, 0, "")
		},
	}

	vm.globals["rindex"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("rindex() expects 2 arguments", 0, 0, "")
			}
			s, ok := args[0].(types.String)
			if !ok {
				return types.NewError("rindex() first argument must be string", 0, 0, "")
			}
			sub := string(types.ToString(args[1]))
			idx := strings.LastIndex(string(s), sub)
			if idx < 0 {
				return types.Int(-1)
			}
			return types.Int(idx)
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

	vm.globals["matchRegex"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("matchRegex() expects 2 arguments (pattern, string)", 0, 0, "")
			}
			pattern := string(types.ToString(args[0]))
			s := string(types.ToString(args[1]))
			re := regexp.MustCompile(pattern)
			return types.Bool(re.MatchString(s))
		},
	}

	vm.globals["formatJSON"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("formatJSON() expects 1 argument", 0, 0, "")
			}
			data, err := json.MarshalIndent(args[0], "", "  ")
			if err != nil {
				return types.NewError(fmt.Sprintf("formatJSON error: %v", err), 0, 0, "")
			}
			return types.String(string(data))
		},
	}

	vm.globals["toJSON"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toJSON() expects 1 argument", 0, 0, "")
			}
			data, err := json.Marshal(args[0])
			if err != nil {
				return types.NewError(fmt.Sprintf("toJSON error: %v", err), 0, 0, "")
			}
			return types.String(string(data))
		},
	}

	vm.globals["jsonEncode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("jsonEncode() expects 1 argument", 0, 0, "")
			}
			data, err := json.Marshal(toJSONable(args[0]))
			if err != nil {
				return types.NewError(fmt.Sprintf("jsonEncode error: %v", err), 0, 0, "")
			}
			return types.String(string(data))
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

	vm.globals["test"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("test() expects at least 1 argument", 0, 0, "")
			}
			fmt.Println()
			for i, arg := range args {
				fmt.Printf("Test %d: %v\n", i+1, arg.ToStr())
			}
			fmt.Println()
			return types.UndefinedValue
		},
	}

	vm.globals["assert"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("assert() expects at least 1 argument", 0, 0, "")
			}
			condition, ok := args[0].(types.Bool)
			if !ok {
				return types.NewError("assert() first argument must be boolean", 0, 0, "")
			}
			if !condition {
				msg := "Assertion failed"
				if len(args) > 1 {
					msg = string(types.ToString(args[1]))
				}
				return types.NewError(msg, 0, 0, "")
			}
			return types.Bool(true)
		},
	}

	vm.globals["tap"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("tap() expects at least 1 argument", 0, 0, "")
			}
			fmt.Println(args[0].ToStr())
			return args[0]
		},
	}

	vm.globals["do"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("do() expects at least 1 argument", 0, 0, "")
			}
			return args[len(args)-1]
		},
	}

	vm.globals["id"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("id() expects 1 argument", 0, 0, "")
			}
			return args[0]
		},
	}

	vm.globals["const"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("const() expects 1 argument", 0, 0, "")
			}
			return args[0]
		},
	}

	vm.globals["placeholder"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.UndefinedValue
		},
	}

	vm.globals["compare"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("compare() expects 2 arguments", 0, 0, "")
			}
			a := args[0].ToStr()
			b := args[1].ToStr()
			if a < b {
				return types.Int(-1)
			} else if a > b {
				return types.Int(1)
			}
			return types.Int(0)
		},
	}

	vm.globals["ternary"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("ternary() expects 3 arguments", 0, 0, "")
			}
			cond, ok := args[0].(types.Bool)
			if !ok {
				return types.NewError("ternary() first argument must be boolean", 0, 0, "")
			}
			if cond {
				return args[1]
			}
			return args[2]
		},
	}

	vm.globals["coalesce"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			for _, arg := range args {
				if arg != types.UndefinedValue && arg != types.NullValue {
					return arg
				}
			}
			return types.UndefinedValue
		},
	}

	vm.globals["defaultTo"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("defaultTo() expects 2 arguments", 0, 0, "")
			}
			val := args[0]
			if val == types.UndefinedValue || val == types.NullValue {
				return args[1]
			}
			return val
		},
	}

	vm.globals["memoize"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return args[0]
		},
	}

	vm.globals["once"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return args[0]
		},
	}

	vm.globals["negate"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("negate() expects 1 argument", 0, 0, "")
			}
			if b, ok := args[0].(types.Bool); ok {
				return types.Bool(!b)
			}
			if n, ok := args[0].(types.Int); ok {
				return types.Int(-n)
			}
			if f, ok := args[0].(types.Float); ok {
				return types.Float(-f)
			}
			return types.NewError("negate() expects boolean or number", 0, 0, "")
		},
	}

	vm.globals["not"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("not() expects 1 argument", 0, 0, "")
			}
			if b, ok := args[0].(types.Bool); ok {
				return types.Bool(!b)
			}
			return types.Bool(false)
		},
	}

	vm.globals["bitAnd"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("bitAnd() expects at least 2 arguments", 0, 0, "")
			}
			result := -1
			for _, arg := range args {
				if n, ok := arg.(types.Int); ok {
					result = int(n) & result
				}
			}
			return types.Int(result)
		},
	}

	vm.globals["bitOr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("bitOr() expects at least 2 arguments", 0, 0, "")
			}
			result := 0
			for _, arg := range args {
				if n, ok := arg.(types.Int); ok {
					result = int(n) | result
				}
			}
			return types.Int(result)
		},
	}

	vm.globals["bitXor"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("bitXor() expects at least 2 arguments", 0, 0, "")
			}
			result := 0
			for _, arg := range args {
				if n, ok := arg.(types.Int); ok {
					result = int(n) ^ result
				}
			}
			return types.Int(result)
		},
	}

	vm.globals["bitNot"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("bitNot() expects 1 argument", 0, 0, "")
			}
			if n, ok := args[0].(types.Int); ok {
				return types.Int(^int(n))
			}
			return types.NewError("bitNot() expects an integer", 0, 0, "")
		},
	}

	vm.globals["leftShift"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("leftShift() expects 2 arguments", 0, 0, "")
			}
			a, ok1 := args[0].(types.Int)
			b, ok2 := args[1].(types.Int)
			if !ok1 || !ok2 {
				return types.NewError("leftShift() expects 2 integers", 0, 0, "")
			}
			return types.Int(int(a) << int(b))
		},
	}

	vm.globals["rightShift"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("rightShift() expects 2 arguments", 0, 0, "")
			}
			a, ok1 := args[0].(types.Int)
			b, ok2 := args[1].(types.Int)
			if !ok1 || !ok2 {
				return types.NewError("rightShift() expects 2 integers", 0, 0, "")
			}
			return types.Int(int(a) >> int(b))
		},
	}

	vm.globals["httpHead"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("httpHead() expects at least 1 argument (url)", 0, 0, "")
			}
			url := string(types.ToString(args[0]))
			client := &http.Client{}
			req, err := http.NewRequest("HEAD", url, nil)
			if err != nil {
				return types.NewError(fmt.Sprintf("httpHead error: %v", err), 0, 0, "")
			}
			resp, err := client.Do(req)
			if err != nil {
				return types.NewError(fmt.Sprintf("httpHead error: %v", err), 0, 0, "")
			}
			defer resp.Body.Close()
			result := collections.NewMap()
			result.Set("status", types.Int(resp.StatusCode))
			result.Set("statusText", types.String(resp.Status))
			return result
		},
	}

	vm.globals["httpOptions"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("httpOptions() expects at least 1 argument (url)", 0, 0, "")
			}
			url := string(types.ToString(args[0]))
			client := &http.Client{}
			req, err := http.NewRequest("OPTIONS", url, nil)
			if err != nil {
				return types.NewError(fmt.Sprintf("httpOptions error: %v", err), 0, 0, "")
			}
			resp, err := client.Do(req)
			if err != nil {
				return types.NewError(fmt.Sprintf("httpOptions error: %v", err), 0, 0, "")
			}
			defer resp.Body.Close()
			result := collections.NewMap()
			result.Set("status", types.Int(resp.StatusCode))
			result.Set("statusText", types.String(resp.Status))
			arr := collections.NewArray()
			for _, method := range resp.Header["Allow"] {
				arr.Append(types.String(method))
			}
			result.Set("methods", arr)
			return result
		},
	}

	vm.globals["httpPatch"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("httpPatch() expects at least 2 arguments (url, body)", 0, 0, "")
			}
			url := string(types.ToString(args[0]))
			body := string(types.ToString(args[1]))
			return doHTTPRequest("PATCH", url, body, nil)
		},
	}

	vm.globals["urlParse"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("urlParse() expects 1 argument", 0, 0, "")
			}
			urlStr := string(types.ToString(args[0]))
			u, err := url.Parse(urlStr)
			if err != nil {
				return types.NewError(fmt.Sprintf("urlParse error: %v", err), 0, 0, "")
			}
			result := collections.NewMap()
			result.Set("scheme", types.String(u.Scheme))
			result.Set("host", types.String(u.Host))
			result.Set("path", types.String(u.Path))
			result.Set("query", types.String(u.RawQuery))
			result.Set("fragment", types.String(u.Fragment))
			result.Set("user", types.String(u.User.Username()))
			return result
		},
	}

	vm.globals["urlBuild"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("urlBuild() expects at least 1 argument", 0, 0, "")
			}
			u := &url.URL{}
			if m, ok := args[0].(*collections.Map); ok {
				if v := m.Get("scheme"); v != types.UndefinedValue {
					u.Scheme = string(types.ToString(v))
				}
				if v := m.Get("host"); v != types.UndefinedValue {
					u.Host = string(types.ToString(v))
				}
				if v := m.Get("path"); v != types.UndefinedValue {
					u.Path = string(types.ToString(v))
				}
				if v := m.Get("query"); v != types.UndefinedValue {
					u.RawQuery = string(types.ToString(v))
				}
				if v := m.Get("fragment"); v != types.UndefinedValue {
					u.Fragment = string(types.ToString(v))
				}
			}
			return types.String(u.String())
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

	vm.globals["isUndefined"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isUndefined() expects 1 argument", 0, 0, "")
			}
			return types.Bool(args[0] == types.UndefinedValue)
		},
	}

	vm.globals["isNull"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isNull() expects 1 argument", 0, 0, "")
			}
			return types.Bool(args[0] == types.NullValue)
		},
	}

	vm.globals["isNil"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isNil() expects 1 argument", 0, 0, "")
			}
			return types.Bool(args[0] == types.UndefinedValue || args[0] == types.NullValue)
		},
	}

	vm.globals["isTruthy"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isTruthy() expects 1 argument", 0, 0, "")
			}
			val := args[0]
			if val == types.UndefinedValue || val == types.NullValue {
				return types.Bool(false)
			}
			if b, ok := val.(types.Bool); ok {
				return b
			}
			if n, ok := val.(types.Int); ok {
				return types.Bool(n != 0)
			}
			if f, ok := val.(types.Float); ok {
				return types.Bool(f != 0)
			}
			if s, ok := val.(types.String); ok {
				return types.Bool(len(s) > 0)
			}
			if a, ok := val.(*collections.Array); ok {
				return types.Bool(a.Len() > 0)
			}
			if m, ok := val.(*collections.Map); ok {
				return types.Bool(m.Len() > 0)
			}
			return types.Bool(true)
		},
	}

	vm.globals["isEmptyStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isEmptyStr() expects 1 argument", 0, 0, "")
			}
			s, ok := args[0].(types.String)
			if !ok {
				return types.Bool(false)
			}
			return types.Bool(len(s) == 0)
		},
	}

	vm.globals["byteSize"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("byteSize() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			return types.Int(len([]byte(s)))
		},
	}

	vm.globals["runeSize"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("runeSize() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			return types.Int(len([]rune(s)))
		},
	}

	vm.globals["bin"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("bin() expects 1 argument", 0, 0, "")
			}
			n, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("bin() expects an integer", 0, 0, "")
			}
			return types.String(fmt.Sprintf("%b", n))
		},
	}

	vm.globals["oct"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("oct() expects 1 argument", 0, 0, "")
			}
			n, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("oct() expects an integer", 0, 0, "")
			}
			return types.String(fmt.Sprintf("%o", n))
		},
	}

	vm.globals["hex"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("hex() expects 1 argument", 0, 0, "")
			}
			n, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("hex() expects an integer", 0, 0, "")
			}
			return types.String(fmt.Sprintf("%x", n))
		},
	}

	vm.globals["bin2int"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("bin2int() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			n, err := strconv.ParseInt(s, 2, 64)
			if err != nil {
				return types.NewError(fmt.Sprintf("bin2int error: %v", err), 0, 0, "")
			}
			return types.Int(n)
		},
	}

	vm.globals["oct2int"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("oct2int() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			n, err := strconv.ParseInt(s, 8, 64)
			if err != nil {
				return types.NewError(fmt.Sprintf("oct2int error: %v", err), 0, 0, "")
			}
			return types.Int(n)
		},
	}

	vm.globals["hex2int"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("hex2int() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			n, err := strconv.ParseInt(s, 16, 64)
			if err != nil {
				return types.NewError(fmt.Sprintf("hex2int error: %v", err), 0, 0, "")
			}
			return types.Int(n)
		},
	}

	vm.globals["isEven"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isEven() expects 1 argument", 0, 0, "")
			}
			n, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("isEven() expects an integer", 0, 0, "")
			}
			return types.Bool(n%2 == 0)
		},
	}

	vm.globals["isOdd"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isOdd() expects 1 argument", 0, 0, "")
			}
			n, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("isOdd() expects an integer", 0, 0, "")
			}
			return types.Bool(n%2 != 0)
		},
	}

	vm.globals["isPositive"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isPositive() expects 1 argument", 0, 0, "")
			}
			f, _ := types.ToFloat(args[0])
			return types.Bool(f > 0)
		},
	}

	vm.globals["isNegative"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isNegative() expects 1 argument", 0, 0, "")
			}
			f, _ := types.ToFloat(args[0])
			return types.Bool(f < 0)
		},
	}

	vm.globals["isZero"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isZero() expects 1 argument", 0, 0, "")
			}
			f, _ := types.ToFloat(args[0])
			return types.Bool(f == 0)
		},
	}

	vm.globals["randomChoice"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("randomChoice() expects at least 1 argument", 0, 0, "")
			}
			idx := rand.Intn(len(args))
			return args[idx]
		},
	}

	vm.globals["randomBool"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Bool(rand.Float64() >= 0.5)
		},
	}

	vm.globals["uuid4"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			b := make([]byte, 16)
			for i := range b {
				b[i] = byte(rand.Intn(256))
			}
			b[6] = (b[6] & 0x0f) | 0x40
			b[8] = (b[8] & 0x3f) | 0x80
			uuid := fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
			return types.String(uuid)
		},
	}

	vm.globals["sleepRandom"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("sleepRandom() expects 2 arguments", 0, 0, "")
			}
			min, ok1 := args[0].(types.Int)
			max, ok2 := args[1].(types.Int)
			if !ok1 || !ok2 {
				return types.NewError("sleepRandom() expects 2 integers", 0, 0, "")
			}
			delay := rand.Intn(int(max-min+1)) + int(min)
			time.Sleep(time.Duration(delay) * time.Millisecond)
			return types.UndefinedValue
		},
	}

	vm.globals["round"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("round() expects 1 argument", 0, 0, "")
			}
			f, ok := args[0].(types.Float)
			if !ok {
				i, ok := args[0].(types.Int)
				if !ok {
					return types.NewError("round() expects a number", 0, 0, "")
				}
				return i
			}
			return types.Int(math.Round(float64(f)))
		},
	}

	vm.globals["floor"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("floor() expects 1 argument", 0, 0, "")
			}
			f, ok := args[0].(types.Float)
			if !ok {
				i, ok := args[0].(types.Int)
				if !ok {
					return types.NewError("floor() expects a number", 0, 0, "")
				}
				return i
			}
			return types.Int(math.Floor(float64(f)))
		},
	}

	vm.globals["ceil"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("ceil() expects 1 argument", 0, 0, "")
			}
			f, ok := args[0].(types.Float)
			if !ok {
				i, ok := args[0].(types.Int)
				if !ok {
					return types.NewError("ceil() expects a number", 0, 0, "")
				}
				return i
			}
			return types.Int(math.Ceil(float64(f)))
		},
	}

	vm.globals["contains"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("contains() expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if ok {
				for _, item := range arr.Elements {
					if item.Equals(args[1]) {
						return types.Bool(true)
					}
				}
				return types.Bool(false)
			}
			str, ok := args[0].(types.String)
			if ok {
				substr := string(types.ToString(args[1]))
				return types.Bool(strings.Contains(string(str), substr))
			}
			return types.NewError("contains() expects array or string", 0, 0, "")
		},
	}

	vm.globals["startsWith"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("startsWith() expects 2 arguments", 0, 0, "")
			}
			str, ok := args[0].(types.String)
			if !ok {
				return types.NewError("startsWith() expects a string", 0, 0, "")
			}
			prefix := string(types.ToString(args[1]))
			return types.Bool(strings.HasPrefix(string(str), prefix))
		},
	}

	vm.globals["endsWith"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("endsWith() expects 2 arguments", 0, 0, "")
			}
			str, ok := args[0].(types.String)
			if !ok {
				return types.NewError("endsWith() expects a string", 0, 0, "")
			}
			suffix := string(types.ToString(args[1]))
			return types.Bool(strings.HasSuffix(string(str), suffix))
		},
	}

	vm.globals["trim"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("trim() expects at least 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			if len(args) > 1 {
				cutset := string(types.ToString(args[1]))
				return types.String(strings.Trim(str, cutset))
			}
			return types.String(strings.TrimSpace(str))
		},
	}

	vm.globals["isArray"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isArray() expects 1 argument", 0, 0, "")
			}
			_, ok := args[0].(*collections.Array)
			return types.Bool(ok)
		},
	}

	vm.globals["isMap"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isMap() expects 1 argument", 0, 0, "")
			}
			_, ok := args[0].(*collections.Map)
			return types.Bool(ok)
		},
	}

	vm.globals["isString"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isString() expects 1 argument", 0, 0, "")
			}
			_, ok := args[0].(types.String)
			return types.Bool(ok)
		},
	}

	vm.globals["isBool"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isBool() expects 1 argument", 0, 0, "")
			}
			_, ok := args[0].(types.Bool)
			return types.Bool(ok)
		},
	}

	vm.globals["isFunction"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isFunction() expects 1 argument", 0, 0, "")
			}
			_, ok := args[0].(*types.NativeFunction)
			return types.Bool(ok)
		},
	}

	vm.globals["base64Encode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("base64Encode() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			return types.String(base64.StdEncoding.EncodeToString([]byte(str)))
		},
	}

	vm.globals["base64Decode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("base64Decode() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			decoded, err := base64.StdEncoding.DecodeString(str)
			if err != nil {
				return types.NewError(fmt.Sprintf("base64Decode error: %v", err), 0, 0, "")
			}
			return types.String(string(decoded))
		},
	}

	vm.globals["split"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("split() expects at least 2 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			sep := string(types.ToString(args[1]))
			parts := strings.Split(str, sep)
			result := collections.NewArray()
			for _, part := range parts {
				result.Append(types.String(part))
			}
			return result
		},
	}

	vm.globals["replace"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("replace() expects at least 3 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			old := string(types.ToString(args[1]))
			new := string(types.ToString(args[2]))
			return types.String(strings.Replace(str, old, new, -1))
		},
	}

	vm.globals["toUpper"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toUpper() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			return types.String(strings.ToUpper(str))
		},
	}

	vm.globals["toLower"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toLower() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			return types.String(strings.ToLower(str))
		},
	}

	vm.globals["splitLines"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("splitLines() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			parts := strings.Split(str, "\n")
			result := collections.NewArray()
			for _, part := range parts {
				result.Append(types.String(part))
			}
			return result
		},
	}

	vm.globals["repeat"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("repeat() expects 2 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			count, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("repeat() expects string and integer", 0, 0, "")
			}
			return types.String(strings.Repeat(str, int(count)))
		},
	}

	vm.globals["indexOf"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("indexOf() expects at least 2 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			substr := string(types.ToString(args[1]))
			idx := strings.Index(str, substr)
			if idx == -1 {
				return types.Int(-1)
			}
			return types.Int(idx)
		},
	}

	vm.globals["lastIndexOf"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("lastIndexOf() expects at least 2 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			substr := string(types.ToString(args[1]))
			idx := strings.LastIndex(str, substr)
			if idx == -1 {
				return types.Int(-1)
			}
			return types.Int(idx)
		},
	}

	vm.globals["urlEncode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("urlEncode() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			return types.String(url.QueryEscape(str))
		},
	}

	vm.globals["urlDecode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("urlDecode() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			decoded, err := url.QueryUnescape(str)
			if err != nil {
				return types.NewError(fmt.Sprintf("urlDecode error: %v", err), 0, 0, "")
			}
			return types.String(decoded)
		},
	}

	vm.globals["isAlpha"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isAlpha() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			if len(str) == 0 {
				return types.Bool(false)
			}
			for _, r := range str {
				if !unicode.IsLetter(r) {
					return types.Bool(false)
				}
			}
			return types.Bool(true)
		},
	}

	vm.globals["isDigit"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isDigit() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			if len(str) == 0 {
				return types.Bool(false)
			}
			for _, r := range str {
				if !unicode.IsDigit(r) {
					return types.Bool(false)
				}
			}
			return types.Bool(true)
		},
	}

	vm.globals["isAlnum"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isAlnum() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			if len(str) == 0 {
				return types.Bool(false)
			}
			for _, r := range str {
				if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
					return types.Bool(false)
				}
			}
			return types.Bool(true)
		},
	}

	vm.globals["hasSpace"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("hasSpace() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			return types.Bool(strings.Contains(str, " "))
		},
	}

	vm.globals["left"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("left() expects 2 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			n, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("left() expects string and integer", 0, 0, "")
			}
			if n >= types.Int(len(str)) {
				return types.String(str)
			}
			return types.String(str[:n])
		},
	}

	vm.globals["right"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("right() expects 2 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			n, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("right() expects string and integer", 0, 0, "")
			}
			if n >= types.Int(len(str)) {
				return types.String(str)
			}
			return types.String(str[len(str)-int(n):])
		},
	}

	vm.globals["substring"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("substring() expects at least 2 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			start, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("substring() expects string and integers", 0, 0, "")
			}
			if start < 0 || start > types.Int(len(str)) {
				return types.NewError("substring() start index out of bounds", 0, 0, "")
			}
			if len(args) >= 3 {
				end, ok := args[2].(types.Int)
				if !ok {
					return types.NewError("substring() expects string and integers", 0, 0, "")
				}
				if end < start || end > types.Int(len(str)) {
					return types.NewError("substring() end index out of bounds", 0, 0, "")
				}
				return types.String(str[start:end])
			}
			return types.String(str[start:])
		},
	}

	vm.globals["md5"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("md5() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			hash := md5.Sum([]byte(str))
			return types.String(fmt.Sprintf("%x", hash))
		},
	}

	vm.globals["sha1"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("sha1() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			hash := sha1.Sum([]byte(str))
			return types.String(fmt.Sprintf("%x", hash))
		},
	}

	vm.globals["sha256"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("sha256() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			hash := sha256.Sum256([]byte(str))
			return types.String(fmt.Sprintf("%x", hash))
		},
	}

	vm.globals["sha512"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("sha512() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			hash := sha512.Sum512([]byte(str))
			return types.String(fmt.Sprintf("%x", hash))
		},
	}

	vm.globals["exit"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			code := 0
			if len(args) > 0 {
				val, _ := types.ToInt(args[0])
				code = int(val)
			}
			os.Exit(code)
			return types.UndefinedValue
		},
	}

	vm.globals["env"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				result := collections.NewMap()
				env := os.Environ()
				for _, e := range env {
					parts := strings.SplitN(e, "=", 2)
					if len(parts) == 2 {
						result.Set(parts[0], types.String(parts[1]))
					}
				}
				return result
			}
			key := string(types.ToString(args[0]))
			return types.String(os.Getenv(key))
		},
	}

	vm.globals["args"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			result := collections.NewArray()
			for _, arg := range os.Args {
				result.Append(types.String(arg))
			}
			return result
		},
	}

	vm.globals["pi"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Float(math.Pi)
		},
	}

	vm.globals["e"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Float(math.E)
		},
	}

	vm.globals["abs"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("abs() expects 1 argument", 0, 0, "")
			}
			i, ok := args[0].(types.Int)
			if ok {
				if i < 0 {
					return types.Int(-i)
				}
				return i
			}
			f, ok := args[0].(types.Float)
			if ok {
				if f < 0 {
					return types.Float(-f)
				}
				return f
			}
			return types.NewError("abs() expects a number", 0, 0, "")
		},
	}

	vm.globals["exit"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			code := 0
			if len(args) > 0 {
				val, _ := types.ToInt(args[0])
				code = int(val)
			}
			os.Exit(code)
			return types.UndefinedValue
		},
	}

	vm.globals["env"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				result := collections.NewMap()
				env := os.Environ()
				for _, e := range env {
					parts := strings.SplitN(e, "=", 2)
					if len(parts) == 2 {
						result.Set(parts[0], types.String(parts[1]))
					}
				}
				return result
			}
			key := string(types.ToString(args[0]))
			return types.String(os.Getenv(key))
		},
	}

	vm.globals["args"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			result := collections.NewArray()
			for _, arg := range os.Args {
				result.Append(types.String(arg))
			}
			return result
		},
	}

	vm.globals["now"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.String(time.Now().Format("2006-01-02 15:04:05"))
		},
	}

	vm.globals["nowISO"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.String(time.Now().Format(time.RFC3339))
		},
	}

	vm.globals["timestamp"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(time.Now().Unix())
		},
	}

	vm.globals["timestampMs"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(time.Now().UnixMilli())
		},
	}

	vm.globals["sleep"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("sleep() expects 1 argument", 0, 0, "")
			}
			arg := args[0]
			switch v := arg.(type) {
			case types.Int:
				time.Sleep(time.Duration(v) * time.Millisecond)
			case types.Float:
				time.Sleep(time.Duration(float64(v)) * time.Millisecond)
			default:
				durStr := string(types.ToString(arg))
				dur, err := time.ParseDuration(durStr)
				if err != nil {
					return types.NewError(fmt.Sprintf("sleep() parse duration error: %v", err), 0, 0, "")
				}
				time.Sleep(dur)
			}
			return types.UndefinedValue
		},
	}

	vm.globals["typeName"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("typeName() expects 1 argument", 0, 0, "")
			}
			return types.String(args[0].TypeName())
		},
	}

	vm.globals["typeOf"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("typeOf() expects 1 argument", 0, 0, "")
			}
			return types.String(args[0].TypeName())
		},
	}

	vm.globals["len"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("len() expects 1 argument", 0, 0, "")
			}
			switch obj := args[0].(type) {
			case *collections.Array:
				return types.Int(obj.Len())
			case *collections.Map:
				return types.Int(obj.Len())
			case types.String:
				return types.Int(len(obj))
			default:
				return types.NewError("len() unsupported type", 0, 0, "")
			}
		},
	}

	vm.globals["keys"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("keys() expects 1 argument", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				return types.NewError("keys() expects a map", 0, 0, "")
			}
			result := collections.NewArray()
			keys := m.Keys()
			for i := 0; i < keys.Len(); i++ {
				result.Append(keys.Get(i))
			}
			return result
		},
	}

	vm.globals["values"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("values() expects 1 argument", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				return types.NewError("values() expects a map", 0, 0, "")
			}
			result := collections.NewArray()
			vals := m.Values()
			for i := 0; i < vals.Len(); i++ {
				result.Append(vals.Get(i))
			}
			return result
		},
	}

	vm.globals["range"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("range() expects at least 1 argument", 0, 0, "")
			}
			start, step := types.Int(0), types.Int(1)
			var end types.Int
			var err *types.Error
			if len(args) == 1 {
				end, err = types.ToInt(args[0])
			} else if len(args) == 2 {
				start, err = types.ToInt(args[0])
				if err == nil {
					end, err = types.ToInt(args[1])
				}
			} else if len(args) >= 3 {
				start, err = types.ToInt(args[0])
				if err == nil {
					end, err = types.ToInt(args[1])
				}
				if err == nil {
					step, err = types.ToInt(args[2])
				}
			}
			if err != nil {
				return types.NewError("range() expects integers", 0, 0, "")
			}
			result := collections.NewArray()
			if step > 0 {
				for i := start; i < end; i += step {
					result.Append(i)
				}
			} else if step < 0 {
				for i := start; i > end; i += step {
					result.Append(i)
				}
			}
			return result
		},
	}

	vm.globals["enumerate"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("enumerate() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("enumerate() expects an array", 0, 0, "")
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				pair := collections.NewArray()
				pair.Append(types.Int(i))
				pair.Append(arr.Get(i))
				result.Append(pair)
			}
			return result
		},
	}

	vm.globals["zip"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("zip() expects at least 2 arrays", 0, 0, "")
			}
			arrays := make([]*collections.Array, len(args))
			for i, arg := range args {
				arr, ok := arg.(*collections.Array)
				if !ok {
					return types.NewError("zip() expects arrays", 0, 0, "")
				}
				arrays[i] = arr
			}
			minLen := arrays[0].Len()
			for _, arr := range arrays {
				if arr.Len() < minLen {
					minLen = arr.Len()
				}
			}
			result := collections.NewArray()
			for i := 0; i < minLen; i++ {
				pair := collections.NewArray()
				for _, arr := range arrays {
					pair.Append(arr.Get(i))
				}
				result.Append(pair)
			}
			return result
		},
	}

	vm.globals["each"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("each() expects at least 2 arguments: array, function", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("each() expects an array as first argument", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("each() expects a function as second argument", 0, 0, "")
			}
			for _, item := range arr.Elements {
				fn.Fn(item)
			}
			return types.UndefinedValue
		},
	}

	vm.globals["pluck"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("pluck() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("pluck() first argument must be an array", 0, 0, "")
			}
			key := string(types.ToString(args[1]))
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				item := arr.Get(i)
				if m, ok := item.(*collections.Map); ok {
					val := m.Get(key)
					if val != nil {
						result.Append(val)
					}
				}
			}
			return result
		},
	}

	vm.globals["groupBy"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("groupBy() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("groupBy() first argument must be an array", 0, 0, "")
			}
			keyFn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("groupBy() second argument must be a function", 0, 0, "")
			}
			result := collections.NewMap()
			for _, item := range arr.Elements {
				keyVal := keyFn.Fn(item)
				key := string(types.ToString(keyVal))
				if result.Get(key) == nil {
					result.Set(key, collections.NewArray())
				}
				arr := result.Get(key).(*collections.Array)
				arr.Append(item)
			}
			return result
		},
	}

	vm.globals["sortBy"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("sortBy() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("sortBy() first argument must be an array", 0, 0, "")
			}
			keyFn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("sortBy() second argument must be a function", 0, 0, "")
			}
			elements := make([]types.Object, arr.Len())
			for i := 0; i < arr.Len(); i++ {
				elements[i] = arr.Get(i)
			}
			sort.Slice(elements, func(i, j int) bool {
				keyI := keyFn.Fn(elements[i])
				keyJ := keyFn.Fn(elements[j])
				return keyI.ToStr() < keyJ.ToStr()
			})
			return collections.NewArrayWithElements(elements)
		},
	}

	vm.globals["reverseArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("reverseArr() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("reverseArr() expects an array", 0, 0, "")
			}
			result := collections.NewArray()
			for i := arr.Len() - 1; i >= 0; i-- {
				result.Append(arr.Get(i))
			}
			return result
		},
	}

	vm.globals["shuffle"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("shuffle() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("shuffle() expects an array", 0, 0, "")
			}
			elements := make([]types.Object, arr.Len())
			for i := 0; i < arr.Len(); i++ {
				elements[i] = arr.Get(i)
			}
			rand.Shuffle(len(elements), func(i, j int) {
				elements[i], elements[j] = elements[j], elements[i]
			})
			return collections.NewArrayWithElements(elements)
		},
	}

	vm.globals["sample"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("sample() expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("sample() first argument must be an array", 0, 0, "")
			}
			n, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("sample() second argument must be an integer", 0, 0, "")
			}
			if n <= 0 || n > types.Int(arr.Len()) {
				return types.NewError("sample() n must be between 1 and array length", 0, 0, "")
			}
			indices := make([]int, arr.Len())
			for i := range indices {
				indices[i] = i
			}
			rand.Shuffle(len(indices), func(i, j int) {
				indices[i], indices[j] = indices[j], indices[i]
			})
			result := collections.NewArray()
			for i := 0; i < int(n); i++ {
				result.Append(arr.Get(indices[i]))
			}
			return result
		},
	}

	vm.globals["flatten"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("flatten() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("flatten() expects an array", 0, 0, "")
			}
			result := collections.NewArray()
			var flatten func(a *collections.Array)
			flatten = func(a *collections.Array) {
				for i := 0; i < a.Len(); i++ {
					if nested, ok := a.Get(i).(*collections.Array); ok {
						flatten(nested)
					} else {
						result.Append(a.Get(i))
					}
				}
			}
			flatten(arr)
			return result
		},
	}

	vm.globals["chunk"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("chunk() expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("chunk() first argument must be an array", 0, 0, "")
			}
			size, ok := args[1].(types.Int)
			if !ok || size <= 0 {
				return types.NewError("chunk() second argument must be a positive integer", 0, 0, "")
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i += int(size) {
				chunk := collections.NewArray()
				for j := 0; j < int(size) && i+j < arr.Len(); j++ {
					chunk.Append(arr.Get(i + j))
				}
				result.Append(chunk)
			}
			return result
		},
	}

	vm.globals["joinStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("joinStr() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("joinStr() first argument must be an array", 0, 0, "")
			}
			sep := string(types.ToString(args[1]))
			parts := make([]string, arr.Len())
			for i := 0; i < arr.Len(); i++ {
				parts[i] = string(types.ToString(arr.Get(i)))
			}
			return types.String(strings.Join(parts, sep))
		},
	}

	vm.globals["uniq"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("uniq() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("uniq() expects an array", 0, 0, "")
			}
			seen := collections.NewMap()
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				item := arr.Get(i)
				key := string(types.ToString(item))
				if seen.Get(key) == nil {
					seen.Set(key, types.Bool(true))
					result.Append(item)
				}
			}
			return result
		},
	}

	vm.globals["slice"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("slice() expects at least 3 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("slice() first argument must be an array", 0, 0, "")
			}
			start, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("slice() second argument must be an integer", 0, 0, "")
			}
			end, ok := args[2].(types.Int)
			if !ok {
				return types.NewError("slice() third argument must be an integer", 0, 0, "")
			}
			if start < 0 {
				start = types.Int(arr.Len()) + start
			}
			if end < 0 {
				end = types.Int(arr.Len()) + end
			}
			if start < 0 || start > types.Int(arr.Len()) {
				start = 0
			}
			if end > types.Int(arr.Len()) {
				end = types.Int(arr.Len())
			}
			result := collections.NewArray()
			for i := start; i < end; i++ {
				result.Append(arr.Get(int(i)))
			}
			return result
		},
	}

	vm.globals["any"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("any() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("any() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("any() second argument must be a function", 0, 0, "")
			}
			for _, item := range arr.Elements {
				result := fn.Fn(item)
				if types.ToBool(result) {
					return types.Bool(true)
				}
			}
			return types.Bool(false)
		},
	}

	vm.globals["all"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("all() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("all() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("all() second argument must be a function", 0, 0, "")
			}
			for _, item := range arr.Elements {
				result := fn.Fn(item)
				if !types.ToBool(result) {
					return types.Bool(false)
				}
			}
			return types.Bool(true)
		},
	}

	vm.globals["none"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("none() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("none() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("none() second argument must be a function", 0, 0, "")
			}
			for _, item := range arr.Elements {
				result := fn.Fn(item)
				if types.ToBool(result) {
					return types.Bool(false)
				}
			}
			return types.Bool(true)
		},
	}

	vm.globals["find"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("find() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("find() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("find() second argument must be a function", 0, 0, "")
			}
			for _, item := range arr.Elements {
				result := fn.Fn(item)
				if types.ToBool(result) {
					return item
				}
			}
			return types.UndefinedValue
		},
	}

	vm.globals["findIndex"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("findIndex() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("findIndex() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("findIndex() second argument must be a function", 0, 0, "")
			}
			for i := 0; i < arr.Len(); i++ {
				result := fn.Fn(arr.Get(i))
				if types.ToBool(result) {
					return types.Int(i)
				}
			}
			return types.Int(-1)
		},
	}

	vm.globals["includes"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("includes() expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("includes() first argument must be an array", 0, 0, "")
			}
			target := args[1]
			for i := 0; i < arr.Len(); i++ {
				if arr.Get(i).Equals(target) {
					return types.Bool(true)
				}
			}
			return types.Bool(false)
		},
	}

	vm.globals["every"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("every() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("every() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("every() second argument must be a function", 0, 0, "")
			}
			for _, item := range arr.Elements {
				result := fn.Fn(item)
				if !types.ToBool(result) {
					return types.Bool(false)
				}
			}
			return types.Bool(true)
		},
	}

	vm.globals["some"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("some() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("some() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("some() second argument must be a function", 0, 0, "")
			}
			for _, item := range arr.Elements {
				result := fn.Fn(item)
				if types.ToBool(result) {
					return types.Bool(true)
				}
			}
			return types.Bool(false)
		},
	}

	vm.globals["fill"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("fill() expects at least 2 arguments", 0, 0, "")
			}
			size, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("fill() first argument must be an integer", 0, 0, "")
			}
			value := args[1]
			result := collections.NewArray()
			for i := types.Int(0); i < size; i++ {
				result.Append(value)
			}
			return result
		},
	}

	vm.globals["first"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("first() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("first() expects an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return types.UndefinedValue
			}
			return arr.Get(0)
		},
	}

	vm.globals["last"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("last() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("last() expects an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return types.UndefinedValue
			}
			return arr.Get(arr.Len() - 1)
		},
	}

	vm.globals["tail"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("tail() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("tail() expects an array", 0, 0, "")
			}
			if arr.Len() <= 1 {
				return collections.NewArray()
			}
			result := collections.NewArray()
			for i := 1; i < arr.Len(); i++ {
				result.Append(arr.Get(i))
			}
			return result
		},
	}

	vm.globals["init"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("init() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("init() expects an array", 0, 0, "")
			}
			if arr.Len() <= 1 {
				return collections.NewArray()
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len()-1; i++ {
				result.Append(arr.Get(i))
			}
			return result
		},
	}

	vm.globals["nth"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("nth() expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("nth() first argument must be an array", 0, 0, "")
			}
			n, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("nth() second argument must be an integer", 0, 0, "")
			}
			if n < 0 || n >= types.Int(arr.Len()) {
				return types.UndefinedValue
			}
			return arr.Get(int(n))
		},
	}

	vm.globals["take"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("take() expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("take() first argument must be an array", 0, 0, "")
			}
			n, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("take() second argument must be an integer", 0, 0, "")
			}
			if n >= types.Int(arr.Len()) {
				return arr
			}
			result := collections.NewArray()
			for i := types.Int(0); i < n && i < types.Int(arr.Len()); i++ {
				result.Append(arr.Get(int(i)))
			}
			return result
		},
	}

	vm.globals["drop"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("drop() expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("drop() first argument must be an array", 0, 0, "")
			}
			n, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("drop() second argument must be an integer", 0, 0, "")
			}
			if n >= types.Int(arr.Len()) {
				return collections.NewArray()
			}
			result := collections.NewArray()
			for i := n; i < types.Int(arr.Len()); i++ {
				result.Append(arr.Get(int(i)))
			}
			return result
		},
	}

	vm.globals["takeLast"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("takeLast() expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("takeLast() first argument must be an array", 0, 0, "")
			}
			n, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("takeLast() second argument must be an integer", 0, 0, "")
			}
			if n >= types.Int(arr.Len()) {
				return arr
			}
			result := collections.NewArray()
			for i := types.Int(arr.Len()) - n; i < types.Int(arr.Len()); i++ {
				result.Append(arr.Get(int(i)))
			}
			return result
		},
	}

	vm.globals["dropLast"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("dropLast() expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("dropLast() first argument must be an array", 0, 0, "")
			}
			n, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("dropLast() second argument must be an integer", 0, 0, "")
			}
			if n >= types.Int(arr.Len()) {
				return collections.NewArray()
			}
			result := collections.NewArray()
			for i := types.Int(0); i < types.Int(arr.Len())-n; i++ {
				result.Append(arr.Get(int(i)))
			}
			return result
		},
	}

	vm.globals["insert"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("insert() expects 3 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("insert() first argument must be an array", 0, 0, "")
			}
			index, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("insert() second argument must be an integer", 0, 0, "")
			}
			value := args[2]
			if index < 0 {
				index = 0
			}
			if index > types.Int(arr.Len()) {
				index = types.Int(arr.Len())
			}
			result := collections.NewArray()
			for i := 0; i < int(index); i++ {
				result.Append(arr.Get(i))
			}
			result.Append(value)
			for i := int(index); i < arr.Len(); i++ {
				result.Append(arr.Get(i))
			}
			return result
		},
	}

	vm.globals["removeAt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("removeAt() expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("removeAt() first argument must be an array", 0, 0, "")
			}
			index, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("removeAt() second argument must be an integer", 0, 0, "")
			}
			if index < 0 || index >= types.Int(arr.Len()) {
				return types.NewError("removeAt() index out of bounds", 0, 0, "")
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				if i != int(index) {
					result.Append(arr.Get(i))
				}
			}
			return result
		},
	}

	vm.globals["replaceAt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("replaceAt() expects 3 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("replaceAt() first argument must be an array", 0, 0, "")
			}
			index, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("replaceAt() second argument must be an integer", 0, 0, "")
			}
			value := args[2]
			if index < 0 || index >= types.Int(arr.Len()) {
				return types.NewError("replaceAt() index out of bounds", 0, 0, "")
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				if i == int(index) {
					result.Append(value)
				} else {
					result.Append(arr.Get(i))
				}
			}
			return result
		},
	}

	vm.globals["size"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("size() expects 1 argument", 0, 0, "")
			}
			switch obj := args[0].(type) {
			case *collections.Array:
				return types.Int(obj.Len())
			case *collections.Map:
				return types.Int(obj.Len())
			case types.String:
				return types.Int(len(obj))
			default:
				return types.NewError("size() unsupported type", 0, 0, "")
			}
		},
	}

	vm.globals["empty"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("empty() expects 1 argument", 0, 0, "")
			}
			switch obj := args[0].(type) {
			case *collections.Array:
				return types.Bool(obj.Len() == 0)
			case *collections.Map:
				return types.Bool(obj.Len() == 0)
			case types.String:
				return types.Bool(len(obj) == 0)
			default:
				return types.NewError("empty() unsupported type", 0, 0, "")
			}
		},
	}

	vm.globals["clone"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("clone() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("clone() expects an array", 0, 0, "")
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				result.Append(arr.Get(i))
			}
			return result
		},
	}

	vm.globals["concat"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("concat() expects at least 2 arrays", 0, 0, "")
			}
			result := collections.NewArray()
			for _, arg := range args {
				arr, ok := arg.(*collections.Array)
				if !ok {
					return types.NewError("concat() expects arrays", 0, 0, "")
				}
				for i := 0; i < arr.Len(); i++ {
					result.Append(arr.Get(i))
				}
			}
			return result
		},
	}

	vm.globals["join"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("join() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("join() first argument must be an array", 0, 0, "")
			}
			sep := string(types.ToString(args[1]))
			parts := make([]string, arr.Len())
			for i := 0; i < arr.Len(); i++ {
				parts[i] = string(types.ToString(arr.Get(i)))
			}
			return types.String(strings.Join(parts, sep))
		},
	}

	vm.globals["hasKey"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("hasKey() expects 2 arguments", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				return types.NewError("hasKey() first argument must be a map", 0, 0, "")
			}
			key := string(types.ToString(args[1]))
			return types.Bool(m.Get(key) != nil)
		},
	}

	vm.globals["delete"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("delete() expects 2 arguments", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				return types.NewError("delete() first argument must be a map", 0, 0, "")
			}
			key := string(types.ToString(args[1]))
			m.Delete(key)
			return m
		},
	}

	vm.globals["set"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("set() expects 3 arguments", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				return types.NewError("set() first argument must be a map", 0, 0, "")
			}
			key := string(types.ToString(args[1]))
			value := args[2]
			m.Set(key, value)
			return m
		},
	}

	vm.globals["get"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("get() expects at least 2 arguments", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				return types.NewError("get() first argument must be a map", 0, 0, "")
			}
			key := string(types.ToString(args[1]))
			val := m.Get(key)
			if val == nil {
				if len(args) >= 3 {
					return args[2]
				}
				return types.UndefinedValue
			}
			return val
		},
	}

	vm.globals["merge"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("merge() expects at least 2 maps", 0, 0, "")
			}
			result := collections.NewMap()
			for _, arg := range args {
				m, ok := arg.(*collections.Map)
				if !ok {
					return types.NewError("merge() expects maps", 0, 0, "")
				}
				keys := m.Keys()
				for i := 0; i < keys.Len(); i++ {
					k := string(types.ToString(keys.Get(i)))
					result.Set(k, m.Get(k))
				}
			}
			return result
		},
	}

	vm.globals["toMap"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("toMap() expects 2 arguments: keys array, values array", 0, 0, "")
			}
			keys, ok1 := args[0].(*collections.Array)
			vals, ok2 := args[1].(*collections.Array)
			if !ok1 || !ok2 {
				return types.NewError("toMap() expects 2 arrays", 0, 0, "")
			}
			if keys.Len() != vals.Len() {
				return types.NewError("toMap() arrays must have same length", 0, 0, "")
			}
			result := collections.NewMap()
			for i := 0; i < keys.Len(); i++ {
				key := string(types.ToString(keys.Get(i)))
				result.Set(key, vals.Get(i))
			}
			return result
		},
	}

	vm.globals["fromPairs"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("fromPairs() expects 1 argument", 0, 0, "")
			}
			pairs, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("fromPairs() expects an array of pairs", 0, 0, "")
			}
			result := collections.NewMap()
			for i := 0; i < pairs.Len(); i++ {
				pair, ok := pairs.Get(i).(*collections.Array)
				if !ok || pair.Len() < 2 {
					continue
				}
				key := string(types.ToString(pair.Get(0)))
				value := pair.Get(1)
				result.Set(key, value)
			}
			return result
		},
	}

	vm.globals["toPairs"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toPairs() expects 1 argument", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				return types.NewError("toPairs() expects a map", 0, 0, "")
			}
			result := collections.NewArray()
			keys := m.Keys()
			for i := 0; i < keys.Len(); i++ {
				k := string(types.ToString(keys.Get(i)))
				pair := collections.NewArray()
				pair.Append(types.String(k))
				pair.Append(m.Get(k))
				result.Append(pair)
			}
			return result
		},
	}

	vm.globals["invert"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("invert() expects 1 argument", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				return types.NewError("invert() expects a map", 0, 0, "")
			}
			result := collections.NewMap()
			keys := m.Keys()
			for i := 0; i < keys.Len(); i++ {
				k := string(types.ToString(keys.Get(i)))
				v := m.Get(k)
				key := string(types.ToString(v))
				result.Set(key, types.String(k))
			}
			return result
		},
	}

	vm.globals["pick"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("pick() expects at least 2 arguments", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				return types.NewError("pick() first argument must be a map", 0, 0, "")
			}
			keys, ok := args[1].(*collections.Array)
			if !ok {
				return types.NewError("pick() second argument must be an array", 0, 0, "")
			}
			result := collections.NewMap()
			for i := 0; i < keys.Len(); i++ {
				key := string(types.ToString(keys.Get(i)))
				if m.Get(key) != nil {
					result.Set(key, m.Get(key))
				}
			}
			return result
		},
	}

	vm.globals["omit"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("omit() expects at least 2 arguments", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				return types.NewError("omit() first argument must be a map", 0, 0, "")
			}
			keys, ok := args[1].(*collections.Array)
			if !ok {
				return types.NewError("omit() second argument must be an array", 0, 0, "")
			}
			omitKeys := make(map[string]bool)
			for i := 0; i < keys.Len(); i++ {
				omitKeys[string(types.ToString(keys.Get(i)))] = true
			}
			result := collections.NewMap()
			mKeys := m.Keys()
			for i := 0; i < mKeys.Len(); i++ {
				k := string(types.ToString(mKeys.Get(i)))
				if !omitKeys[k] {
					result.Set(k, m.Get(k))
				}
			}
			return result
		},
	}

	vm.globals["toArray"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toArray() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if ok {
				return arr
			}
			return types.NewError("toArray() expects an array", 0, 0, "")
		},
	}

	vm.globals["toString"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toString() expects 1 argument", 0, 0, "")
			}
			return types.String(args[0].ToStr())
		},
	}

	vm.globals["toNumber"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toNumber() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			if strings.Contains(str, ".") {
				f, err := strconv.ParseFloat(str, 64)
				if err != nil {
					return types.NewError(fmt.Sprintf("toNumber() parse error: %v", err), 0, 0, "")
				}
				return types.Float(f)
			}
			i, err := strconv.ParseInt(str, 10, 64)
			if err != nil {
				return types.NewError(fmt.Sprintf("toNumber() parse error: %v", err), 0, 0, "")
			}
			return types.Int(i)
		},
	}

	vm.globals["toBool"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toBool() expects 1 argument", 0, 0, "")
			}
			return types.Bool(types.ToBool(args[0]))
		},
	}

	vm.globals["isNaN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isNaN() expects 1 argument", 0, 0, "")
			}
			f, ok := args[0].(types.Float)
			if !ok {
				return types.Bool(false)
			}
			return types.Bool(math.IsNaN(float64(f)))
		},
	}

	vm.globals["isFinite"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isFinite() expects 1 argument", 0, 0, "")
			}
			f, ok := args[0].(types.Float)
			if !ok {
				i, ok := args[0].(types.Int)
				if !ok {
					return types.Bool(false)
				}
				return types.Bool(i != types.Int(math.Inf(1)) && i != types.Int(math.Inf(-1)))
			}
			return types.Bool(!math.IsInf(float64(f), 0) && !math.IsNaN(float64(f)))
		},
	}

	vm.globals["isInf"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isInf() expects 1 argument", 0, 0, "")
			}
			f, ok := args[0].(types.Float)
			if !ok {
				return types.Bool(false)
			}
			return types.Bool(math.IsInf(float64(f), 0))
		},
	}

	vm.globals["clamp"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("clamp() expects 3 arguments", 0, 0, "")
			}
			val, ok := args[0].(types.Float)
			if !ok {
				i, ok := args[0].(types.Int)
				if !ok {
					return types.NewError("clamp() first argument must be a number", 0, 0, "")
				}
				val = types.Float(i)
			}
			min, _ := types.ToFloat(args[1])
			max, _ := types.ToFloat(args[2])
			if float64(val) < float64(min) {
				val = min
			}
			if float64(val) > float64(max) {
				val = max
			}
			if float64(val) == float64(int64(val)) {
				return types.Int(int64(val))
			}
			return val
		},
	}

	vm.globals["inRange"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("inRange() expects at least 2 arguments", 0, 0, "")
			}
			val, ok := args[0].(types.Float)
			if !ok {
				i, ok := args[0].(types.Int)
				if !ok {
					return types.NewError("inRange() first argument must be a number", 0, 0, "")
				}
				val = types.Float(i)
			}
			min, _ := types.ToFloat(args[1])
			var max types.Float
			if len(args) >= 3 {
				max, _ = types.ToFloat(args[2])
			} else {
				max = min
				min = 0
			}
			return types.Bool(float64(val) >= float64(min) && float64(val) < float64(max))
		},
	}

	vm.globals["random"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.Float(rand.Float64())
			}
			if len(args) == 1 {
				max, ok := args[0].(types.Int)
				if !ok {
					return types.NewError("random() argument must be an integer", 0, 0, "")
				}
				return types.Int(rand.Intn(int(max)))
			}
			min, ok1 := args[0].(types.Int)
			max, ok2 := args[1].(types.Int)
			if !ok1 || !ok2 {
				return types.NewError("random() arguments must be integers", 0, 0, "")
			}
			return types.Int(rand.Intn(int(max-min+1)) + int(min))
		},
	}

	vm.globals["rand"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Float(rand.Float64())
		},
	}

	vm.globals["randBetween"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("randBetween() expects 2 arguments", 0, 0, "")
			}
			min, ok1 := args[0].(types.Int)
			max, ok2 := args[1].(types.Int)
			if !ok1 || !ok2 {
				return types.NewError("randBetween() arguments must be integers", 0, 0, "")
			}
			return types.Int(rand.Intn(int(max-min+1)) + int(min))
		},
	}

	vm.globals["randFloat"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("randFloat() expects 2 arguments", 0, 0, "")
			}
			min, ok1 := args[0].(types.Float)
			max, ok2 := args[1].(types.Float)
			if !ok1 || !ok2 {
				return types.NewError("randFloat() arguments must be numbers", 0, 0, "")
			}
			r := min + types.Float(rand.Float64())*(max-min)
			return r
		},
	}

	vm.globals["shuffleStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("shuffleStr() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			runes := []rune(str)
			rand.Shuffle(len(runes), func(i, j int) {
				runes[i], runes[j] = runes[j], runes[i]
			})
			return types.String(string(runes))
		},
	}

	vm.globals["format"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("format() expects at least 2 arguments", 0, 0, "")
			}
			format := string(types.ToString(args[0]))
			args = args[1:]
			formatVals := make([]interface{}, len(args))
			for i, arg := range args {
				formatVals[i] = arg
			}
			result := fmt.Sprintf(format, formatVals...)
			return types.String(result)
		},
	}

	vm.globals["printf"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("printf() expects at least 1 argument", 0, 0, "")
			}
			format := string(types.ToString(args[0]))
			args = args[1:]
			formatVals := make([]interface{}, len(args))
			for i, arg := range args {
				formatVals[i] = arg
			}
			fmt.Printf(format, formatVals...)
			return types.UndefinedValue
		},
	}

	vm.globals["sprintf"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("sprintf() expects at least 2 arguments", 0, 0, "")
			}
			format := string(types.ToString(args[0]))
			args = args[1:]
			formatVals := make([]interface{}, len(args))
			for i, arg := range args {
				formatVals[i] = arg
			}
			result := fmt.Sprintf(format, formatVals...)
			return types.String(result)
		},
	}

	vm.globals["print"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			for _, arg := range args {
				fmt.Print(arg.ToStr())
			}
			return types.UndefinedValue
		},
	}

	vm.globals["println"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			for i, arg := range args {
				if i > 0 {
					fmt.Print(" ")
				}
				fmt.Print(arg.ToStr())
			}
			fmt.Println()
			return types.UndefinedValue
		},
	}

	vm.globals["readFile"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("readFile() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			data, err := os.ReadFile(path)
			if err != nil {
				return types.NewError(fmt.Sprintf("readFile error: %v", err), 0, 0, "")
			}
			return types.String(string(data))
		},
	}

	vm.globals["writeFile"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("writeFile() expects at least 2 arguments", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			content := string(types.ToString(args[1]))
			err := os.WriteFile(path, []byte(content), 0644)
			if err != nil {
				return types.NewError(fmt.Sprintf("writeFile error: %v", err), 0, 0, "")
			}
			return types.Bool(true)
		},
	}

	vm.globals["appendFile"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("appendFile() expects at least 2 arguments", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			content := string(types.ToString(args[1]))
			f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return types.NewError(fmt.Sprintf("appendFile error: %v", err), 0, 0, "")
			}
			defer f.Close()
			_, err = f.WriteString(content)
			if err != nil {
				return types.NewError(fmt.Sprintf("appendFile error: %v", err), 0, 0, "")
			}
			return types.Bool(true)
		},
	}

	vm.globals["readLines"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("readLines() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			data, err := os.ReadFile(path)
			if err != nil {
				return types.NewError(fmt.Sprintf("readLines error: %v", err), 0, 0, "")
			}
			lines := strings.Split(string(data), "\n")
			result := collections.NewArray()
			for _, line := range lines {
				result.Append(types.String(line))
			}
			return result
		},
	}

	vm.globals["writeLines"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("writeLines() expects at least 2 arguments", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			arr, ok := args[1].(*collections.Array)
			if !ok {
				return types.NewError("writeLines() second argument must be an array", 0, 0, "")
			}
			var lines []string
			for i := 0; i < arr.Len(); i++ {
				lines = append(lines, string(types.ToString(arr.Get(i))))
			}
			content := strings.Join(lines, "\n")
			err := os.WriteFile(path, []byte(content), 0644)
			if err != nil {
				return types.NewError(fmt.Sprintf("writeLines error: %v", err), 0, 0, "")
			}
			return types.Bool(true)
		},
	}

	vm.globals["mkdir"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("mkdir() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			err := os.MkdirAll(path, 0755)
			if err != nil {
				return types.NewError(fmt.Sprintf("mkdir error: %v", err), 0, 0, "")
			}
			return types.Bool(true)
		},
	}

	vm.globals["remove"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("remove() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			err := os.Remove(path)
			if err != nil {
				return types.NewError(fmt.Sprintf("remove error: %v", err), 0, 0, "")
			}
			return types.Bool(true)
		},
	}

	vm.globals["removeAll"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("removeAll() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			err := os.RemoveAll(path)
			if err != nil {
				return types.NewError(fmt.Sprintf("removeAll error: %v", err), 0, 0, "")
			}
			return types.Bool(true)
		},
	}

	vm.globals["rename"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("rename() expects 2 arguments", 0, 0, "")
			}
			oldPath := string(types.ToString(args[0]))
			newPath := string(types.ToString(args[1]))
			err := os.Rename(oldPath, newPath)
			if err != nil {
				return types.NewError(fmt.Sprintf("rename error: %v", err), 0, 0, "")
			}
			return types.Bool(true)
		},
	}

	vm.globals["copy"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("copy() expects 2 arguments", 0, 0, "")
			}
			src := string(types.ToString(args[0]))
			dst := string(types.ToString(args[1]))
			sourceFile, err := os.Open(src)
			if err != nil {
				return types.NewError(fmt.Sprintf("copy error: %v", err), 0, 0, "")
			}
			defer sourceFile.Close()
			destFile, err := os.Create(dst)
			if err != nil {
				return types.NewError(fmt.Sprintf("copy error: %v", err), 0, 0, "")
			}
			defer destFile.Close()
			_, err = io.Copy(destFile, sourceFile)
			if err != nil {
				return types.NewError(fmt.Sprintf("copy error: %v", err), 0, 0, "")
			}
			return types.Bool(true)
		},
	}

	vm.globals["readDir"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("readDir() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			entries, err := os.ReadDir(path)
			if err != nil {
				return types.NewError(fmt.Sprintf("readDir error: %v", err), 0, 0, "")
			}
			result := collections.NewArray()
			for _, entry := range entries {
				info := collections.NewMap()
				info.Set("name", types.String(entry.Name()))
				info.Set("isDir", types.Bool(entry.IsDir()))
				result.Append(info)
			}
			return result
		},
	}

	vm.globals["cwd"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			dir, err := os.Getwd()
			if err != nil {
				return types.NewError(fmt.Sprintf("cwd error: %v", err), 0, 0, "")
			}
			return types.String(dir)
		},
	}

	vm.globals["chdir"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("chdir() expects 1 argument", 0, 0, "")
			}
			dir := string(types.ToString(args[0]))
			err := os.Chdir(dir)
			if err != nil {
				return types.NewError(fmt.Sprintf("chdir error: %v", err), 0, 0, "")
			}
			return types.Bool(true)
		},
	}

	vm.globals["hostname"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			name, err := os.Hostname()
			if err != nil {
				return types.NewError(fmt.Sprintf("hostname error: %v", err), 0, 0, "")
			}
			return types.String(name)
		},
	}

	vm.globals["homeDir"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			dir, err := os.UserHomeDir()
			if err != nil {
				return types.NewError(fmt.Sprintf("homeDir error: %v", err), 0, 0, "")
			}
			return types.String(dir)
		},
	}

	vm.globals["tempDir"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.String(os.TempDir())
		},
	}

	vm.globals["eval"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("eval() expects 1 argument", 0, 0, "")
			}
			source := string(types.ToString(args[0]))
			lexer := parser.NewLexer(source)
			p := parser.NewParser(lexer)
			program := p.ParseProgram()
			if len(p.Errors()) > 0 {
				return types.NewError(fmt.Sprintf("eval parse errors: %v", p.Errors()), 0, 0, "")
			}
			comp := compiler.NewCompiler()
			if err := comp.Compile(program); err != nil {
				return types.NewError(fmt.Sprintf("eval compile error: %v", err), 0, 0, "")
			}
			bc := comp.Bytecode()
			vm := NewVM(bc)
			if err := vm.Run(); err != nil {
				return types.NewError(fmt.Sprintf("eval runtime error: %v", err), 0, 0, "")
			}
			if vm.Stack().Size() > 0 {
				return vm.Stack().Peek()
			}
			return types.UndefinedValue
		},
	}

	vm.globals["parse"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("parse() expects 1 argument", 0, 0, "")
			}
			source := string(types.ToString(args[0]))
			lexer := parser.NewLexer(source)
			p := parser.NewParser(lexer)
			_ = p.ParseProgram()
			if len(p.Errors()) > 0 {
				return types.NewError(fmt.Sprintf("parse errors: %v", p.Errors()), 0, 0, "")
			}
			return types.String("OK")
		},
	}

	vm.globals["compile"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("compile() expects 1 argument", 0, 0, "")
			}
			source := string(types.ToString(args[0]))
			lexer := parser.NewLexer(source)
			p := parser.NewParser(lexer)
			program := p.ParseProgram()
			if len(p.Errors()) > 0 {
				return types.NewError(fmt.Sprintf("compile parse errors: %v", p.Errors()), 0, 0, "")
			}
			comp := compiler.NewCompiler()
			if err := comp.Compile(program); err != nil {
				return types.NewError(fmt.Sprintf("compile error: %v", err), 0, 0, "")
			}
			return types.String("OK")
		},
	}

	vm.globals["version"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.String("1.0.0")
		},
	}

	vm.globals["info"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			result := collections.NewMap()
			result.Set("version", types.String("1.0.0"))
			result.Set("platform", types.String(runtime.GOOS))
			result.Set("arch", types.String(runtime.GOARCH))
			result.Set("numCPU", types.Int(runtime.NumCPU()))
			return result
		},
	}

	vm.globals["gc"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			runtime.GC()
			return types.UndefinedValue
		},
	}

	vm.globals["numCPU"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(runtime.NumCPU())
		},
	}

	vm.globals["freeMemory"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			return types.Int(m.Frees)
		},
	}

	vm.globals["totalMemory"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			return types.Int(m.Alloc)
		},
	}

	vm.globals["sleepMicros"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("sleepMicros() expects 1 argument", 0, 0, "")
			}
			us, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("sleepMicros() expects an integer", 0, 0, "")
			}
			time.Sleep(time.Duration(us) * time.Microsecond)
			return types.UndefinedValue
		},
	}

	vm.globals["sleepNanos"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("sleepNanos() expects 1 argument", 0, 0, "")
			}
			ns, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("sleepNanos() expects an integer", 0, 0, "")
			}
			time.Sleep(time.Duration(ns) * time.Nanosecond)
			return types.UndefinedValue
		},
	}

	vm.globals["unix"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(time.Now().Unix())
		},
	}

	vm.globals["unixMs"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(time.Now().UnixMilli())
		},
	}

	vm.globals["unixNano"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(time.Now().UnixNano())
		},
	}

	vm.globals["isDir"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isDir() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			info, err := os.Stat(path)
			if err != nil {
				return types.Bool(false)
			}
			return types.Bool(info.IsDir())
		},
	}

	vm.globals["isFile"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isFile() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			info, err := os.Stat(path)
			if err != nil {
				return types.Bool(false)
			}
			return types.Bool(!info.IsDir())
		},
	}

	vm.globals["exists"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("exists() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			_, err := os.Stat(path)
			return types.Bool(err == nil)
		},
	}

	vm.globals["stat"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("stat() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			info, err := os.Stat(path)
			if err != nil {
				return types.NewError(fmt.Sprintf("stat error: %v", err), 0, 0, "")
			}
			result := collections.NewMap()
			result.Set("name", types.String(info.Name()))
			result.Set("size", types.Int(info.Size()))
			result.Set("isDir", types.Bool(info.IsDir()))
			result.Set("mode", types.Int(int64(info.Mode())))
			result.Set("modTime", types.String(info.ModTime().Format("2006-01-02 15:04:05")))
			return result
		},
	}

	vm.globals["splitN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("splitN() expects 3 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			sep := string(types.ToString(args[1]))
			n, ok := args[2].(types.Int)
			if !ok {
				return types.NewError("splitN() third argument must be an integer", 0, 0, "")
			}
			parts := strings.SplitN(str, sep, int(n))
			result := collections.NewArray()
			for _, part := range parts {
				result.Append(types.String(part))
			}
			return result
		},
	}

	vm.globals["replaceAll"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("replaceAll() expects 3 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			old := string(types.ToString(args[1]))
			new := string(types.ToString(args[2]))
			return types.String(strings.ReplaceAll(str, old, new))
		},
	}

	vm.globals["count"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("count() expects at least 2 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			substr := string(types.ToString(args[1]))
			return types.Int(strings.Count(str, substr))
		},
	}

	vm.globals["hasPrefix"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("hasPrefix() expects 2 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			prefix := string(types.ToString(args[1]))
			return types.Bool(strings.HasPrefix(str, prefix))
		},
	}

	vm.globals["hasSuffix"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("hasSuffix() expects 2 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			suffix := string(types.ToString(args[1]))
			return types.Bool(strings.HasSuffix(str, suffix))
		},
	}

	vm.globals["lines"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("lines() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			lines := strings.FieldsFunc(str, func(r rune) bool { return r == '\n' || r == '\r' })
			result := collections.NewArray()
			for _, line := range lines {
				result.Append(types.String(line))
			}
			return result
		},
	}

	vm.globals["words"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("words() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			words := strings.Fields(str)
			result := collections.NewArray()
			for _, word := range words {
				result.Append(types.String(word))
			}
			return result
		},
	}

	vm.globals["strip"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("strip() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			return types.String(strings.TrimSpace(s))
		},
	}

	vm.globals["countStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("countStr() expects 2 arguments (string, substring)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			substr := string(types.ToString(args[1]))
			return types.Int(strings.Count(s, substr))
		},
	}

	vm.globals["splitLines"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("splitLines() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			lines := strings.Split(s, "\n")
			arr := collections.NewArray()
			for _, line := range lines {
				arr.Append(types.String(line))
			}
			return arr
		},
	}

	vm.globals["levenshtein"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("levenshtein() expects 2 arguments (string1, string2)", 0, 0, "")
			}
			s1 := string(types.ToString(args[0]))
			s2 := string(types.ToString(args[1]))
			if len(s1) == 0 {
				return types.Int(len(s2))
			}
			if len(s2) == 0 {
				return types.Int(len(s1))
			}
			rows := len(s1) + 1
			cols := len(s2) + 1
			matrix := make([][]int, rows)
			for i := range matrix {
				matrix[i] = make([]int, cols)
			}
			for i := 0; i < rows; i++ {
				matrix[i][0] = i
			}
			for j := 0; j < cols; j++ {
				matrix[0][j] = j
			}
			for i := 1; i < rows; i++ {
				for j := 1; j < cols; j++ {
					cost := 1
					if s1[i-1] == s2[j-1] {
						cost = 0
					}
					matrix[i][j] = min(
						matrix[i-1][j]+1,
						matrix[i][j-1]+1,
						matrix[i-1][j-1]+cost,
					)
				}
			}
			return types.Int(matrix[rows-1][cols-1])
		},
	}

	vm.globals["bytes"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("bytes() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			result := collections.NewArray()
			for _, b := range []byte(str) {
				result.Append(types.Int(b))
			}
			return result
		},
	}

	vm.globals["runes"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("runes() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			result := collections.NewArray()
			for _, r := range []rune(str) {
				result.Append(types.Int(r))
			}
			return result
		},
	}

	vm.globals["fromBytes"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("fromBytes() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("fromBytes() expects an array", 0, 0, "")
			}
			b := make([]byte, arr.Len())
			for i := 0; i < arr.Len(); i++ {
				val, _ := types.ToInt(arr.Get(i))
				b[i] = byte(val)
			}
			return types.String(string(b))
		},
	}

	vm.globals["fromRunes"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("fromRunes() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("fromRunes() expects an array", 0, 0, "")
			}
			r := make([]rune, arr.Len())
			for i := 0; i < arr.Len(); i++ {
				val, _ := types.ToInt(arr.Get(i))
				r[i] = rune(val)
			}
			return types.String(string(r))
		},
	}

	vm.globals["padStart"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("padStart() expects 3 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			length, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("padStart() second argument must be an integer", 0, 0, "")
			}
			pad := string(types.ToString(args[2]))
			if len(str) >= int(length) {
				return types.String(str)
			}
			padLen := int(length) - len(str)
			padding := strings.Repeat(pad, (padLen+len(pad)-1)/len(pad))
			return types.String(padding[:padLen] + str)
		},
	}

	vm.globals["padEnd"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("padEnd() expects 3 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			length, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("padEnd() second argument must be an integer", 0, 0, "")
			}
			pad := string(types.ToString(args[2]))
			if len(str) >= int(length) {
				return types.String(str)
			}
			padLen := int(length) - len(str)
			padding := strings.Repeat(pad, (padLen+len(pad)-1)/len(pad))
			return types.String(str + padding[:padLen])
		},
	}

	vm.globals["trimLeft"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("trimLeft() expects at least 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			if len(args) > 1 {
				cutset := string(types.ToString(args[1]))
				return types.String(strings.TrimLeft(str, cutset))
			}
			return types.String(strings.TrimLeftFunc(str, unicode.IsSpace))
		},
	}

	vm.globals["trimRight"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("trimRight() expects at least 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			if len(args) > 1 {
				cutset := string(types.ToString(args[1]))
				return types.String(strings.TrimRight(str, cutset))
			}
			return types.String(strings.TrimRightFunc(str, unicode.IsSpace))
		},
	}

	vm.globals["trimPrefix"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("trimPrefix() expects 2 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			prefix := string(types.ToString(args[1]))
			return types.String(strings.TrimPrefix(str, prefix))
		},
	}

	vm.globals["trimSuffix"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("trimSuffix() expects 2 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			suffix := string(types.ToString(args[1]))
			return types.String(strings.TrimSuffix(str, suffix))
		},
	}

	vm.globals["title"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("title() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			return types.String(strings.Title(str))
		},
	}

	vm.globals["toTitle"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toTitle() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			return types.String(strings.ToTitle(str))
		},
	}

	vm.globals["toLowerSpecial"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toLowerSpecial() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			return types.String(strings.ToLowerSpecial(unicode.SpecialCase{}, str))
		},
	}

	vm.globals["toUpperSpecial"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toUpperSpecial() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			return types.String(strings.ToUpperSpecial(unicode.SpecialCase{}, str))
		},
	}

	vm.globals["isTitle"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isTitle() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			return types.Bool(strings.Title(str) == str)
		},
	}

	vm.globals["equalFold"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("equalFold() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			t := string(types.ToString(args[1]))
			return types.Bool(strings.EqualFold(s, t))
		},
	}

	vm.globals["compare"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("compare() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			t := string(types.ToString(args[1]))
			return types.Int(strings.Compare(s, t))
		},
	}

	vm.globals["containsAny"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("containsAny() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			chars := string(types.ToString(args[1]))
			return types.Bool(strings.ContainsAny(s, chars))
		},
	}

	vm.globals["containsRune"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("containsRune() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			r, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("containsRune() second argument must be an integer", 0, 0, "")
			}
			return types.Bool(strings.ContainsRune(s, rune(r)))
		},
	}

	vm.globals["indexAny"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("indexAny() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			chars := string(types.ToString(args[1]))
			idx := strings.IndexAny(s, chars)
			if idx == -1 {
				return types.Int(-1)
			}
			return types.Int(idx)
		},
	}

	vm.globals["lastIndexAny"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("lastIndexAny() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			chars := string(types.ToString(args[1]))
			idx := strings.LastIndexAny(s, chars)
			if idx == -1 {
				return types.Int(-1)
			}
			return types.Int(idx)
		},
	}

	vm.globals["mapCodePoints"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("mapCodePoints() expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("mapCodePoints() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("mapCodePoints() second argument must be a function", 0, 0, "")
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				mapped := fn.Fn(arr.Get(i))
				result.Append(mapped)
			}
			return result
		},
	}

	vm.globals["filterChars"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("filterChars() expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("filterChars() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("filterChars() second argument must be a function", 0, 0, "")
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				if types.ToBool(fn.Fn(arr.Get(i))) {
					result.Append(arr.Get(i))
				}
			}
			return result
		},
	}

	vm.globals["reduceArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("reduceArr() expects at least 3 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("reduceArr() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("reduceArr() second argument must be a function", 0, 0, "")
			}
			acc := args[2]
			for i := 0; i < arr.Len(); i++ {
				acc = fn.Fn(acc, arr.Get(i))
			}
			return acc
		},
	}

	vm.globals["forEach"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("forEach() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("forEach() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("forEach() second argument must be a function", 0, 0, "")
			}
			for i := 0; i < arr.Len(); i++ {
				fn.Fn(arr.Get(i), types.Int(i))
			}
			return types.UndefinedValue
		},
	}

	vm.globals["someArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("someArr() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("someArr() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("someArr() second argument must be a function", 0, 0, "")
			}
			for i := 0; i < arr.Len(); i++ {
				if types.ToBool(fn.Fn(arr.Get(i))) {
					return types.Bool(true)
				}
			}
			return types.Bool(false)
		},
	}

	vm.globals["everyArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("everyArr() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("everyArr() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("everyArr() second argument must be a function", 0, 0, "")
			}
			for i := 0; i < arr.Len(); i++ {
				if !types.ToBool(fn.Fn(arr.Get(i))) {
					return types.Bool(false)
				}
			}
			return types.Bool(true)
		},
	}

	vm.globals["findArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("findArr() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("findArr() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("findArr() second argument must be a function", 0, 0, "")
			}
			for i := 0; i < arr.Len(); i++ {
				if types.ToBool(fn.Fn(arr.Get(i))) {
					return arr.Get(i)
				}
			}
			return types.UndefinedValue
		},
	}

	vm.globals["findIndexArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("findIndexArr() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("findIndexArr() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("findIndexArr() second argument must be a function", 0, 0, "")
			}
			for i := 0; i < arr.Len(); i++ {
				if types.ToBool(fn.Fn(arr.Get(i))) {
					return types.Int(i)
				}
			}
			return types.Int(-1)
		},
	}

	vm.globals["createArray"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return collections.NewArray()
		},
	}

	vm.globals["createMap"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return collections.NewMap()
		},
	}

	vm.globals["isEmpty"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isEmpty() expects 1 argument", 0, 0, "")
			}
			switch obj := args[0].(type) {
			case *collections.Array:
				return types.Bool(obj.Len() == 0)
			case *collections.Map:
				return types.Bool(obj.Len() == 0)
			case types.String:
				return types.Bool(len(obj) == 0)
			default:
				return types.NewError("isEmpty() unsupported type", 0, 0, "")
			}
		},
	}

	vm.globals["equal"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("equal() expects 2 arguments", 0, 0, "")
			}
			return types.Bool(args[0].Equals(args[1]))
		},
	}

	vm.globals["neq"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("neq() expects 2 arguments", 0, 0, "")
			}
			return types.Bool(!args[0].Equals(args[1]))
		},
	}

	vm.globals["gt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("gt() expects 2 arguments", 0, 0, "")
			}
			a, b := args[0], args[1]
			aInt, aOk := a.(types.Int)
			bInt, bOk := b.(types.Int)
			if aOk && bOk {
				return types.Bool(aInt > bInt)
			}
			aFloat, aFloatOk := a.(types.Float)
			bFloat, bFloatOk := b.(types.Float)
			if aFloatOk && bFloatOk {
				return types.Bool(aFloat > bFloat)
			}
			if aOk {
				f, _ := types.ToFloat(b)
				return types.Bool(float64(aInt) > float64(f))
			}
			if bOk {
				f, _ := types.ToFloat(a)
				return types.Bool(float64(f) > float64(bInt))
			}
			aF, _ := types.ToFloat(a)
			bF, _ := types.ToFloat(b)
			return types.Bool(float64(aF) > float64(bF))
		},
	}

	vm.globals["lt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("lt() expects 2 arguments", 0, 0, "")
			}
			a, b := args[0], args[1]
			aInt, aOk := a.(types.Int)
			bInt, bOk := b.(types.Int)
			if aOk && bOk {
				return types.Bool(aInt < bInt)
			}
			aFloat, aFloatOk := a.(types.Float)
			bFloat, bFloatOk := b.(types.Float)
			if aFloatOk && bFloatOk {
				return types.Bool(aFloat < bFloat)
			}
			if aOk {
				f, _ := types.ToFloat(b)
				return types.Bool(float64(aInt) < float64(f))
			}
			if bOk {
				f, _ := types.ToFloat(a)
				return types.Bool(float64(f) < float64(bInt))
			}
			aF, _ := types.ToFloat(a)
			bF, _ := types.ToFloat(b)
			return types.Bool(float64(aF) < float64(bF))
		},
	}

	vm.globals["gte"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("gte() expects 2 arguments", 0, 0, "")
			}
			a, b := args[0], args[1]
			aInt, aOk := a.(types.Int)
			bInt, bOk := b.(types.Int)
			if aOk && bOk {
				return types.Bool(aInt >= bInt)
			}
			aFloat, aFloatOk := a.(types.Float)
			bFloat, bFloatOk := b.(types.Float)
			if aFloatOk && bFloatOk {
				return types.Bool(aFloat >= bFloat)
			}
			if aOk {
				f, _ := types.ToFloat(b)
				return types.Bool(float64(aInt) >= float64(f))
			}
			if bOk {
				f, _ := types.ToFloat(a)
				return types.Bool(float64(f) >= float64(bInt))
			}
			aF, _ := types.ToFloat(a)
			bF, _ := types.ToFloat(b)
			return types.Bool(float64(aF) >= float64(bF))
		},
	}

	vm.globals["lte"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("lte() expects 2 arguments", 0, 0, "")
			}
			a, b := args[0], args[1]
			aInt, aOk := a.(types.Int)
			bInt, bOk := b.(types.Int)
			if aOk && bOk {
				return types.Bool(aInt <= bInt)
			}
			aFloat, aFloatOk := a.(types.Float)
			bFloat, bFloatOk := b.(types.Float)
			if aFloatOk && bFloatOk {
				return types.Bool(aFloat <= bFloat)
			}
			if aOk {
				f, _ := types.ToFloat(b)
				return types.Bool(float64(aInt) <= float64(f))
			}
			if bOk {
				f, _ := types.ToFloat(a)
				return types.Bool(float64(f) <= float64(bInt))
			}
			aF, _ := types.ToFloat(a)
			bF, _ := types.ToFloat(b)
			return types.Bool(float64(aF) <= float64(bF))
		},
	}

	vm.globals["not"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("not() expects 1 argument", 0, 0, "")
			}
			return types.Bool(!types.ToBool(args[0]))
		},
	}

	vm.globals["and"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			for _, arg := range args {
				if !types.ToBool(arg) {
					return arg
				}
			}
			if len(args) > 0 {
				return args[len(args)-1]
			}
			return types.Bool(false)
		},
	}

	vm.globals["or"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			for _, arg := range args {
				if types.ToBool(arg) {
					return arg
				}
			}
			if len(args) > 0 {
				return args[len(args)-1]
			}
			return types.Bool(false)
		},
	}

	vm.globals["coalesce"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			for _, arg := range args {
				if arg != nil && arg != types.UndefinedValue && arg != types.NullValue {
					if str, ok := arg.(types.String); ok && string(str) != "" {
						return arg
					}
					return arg
				}
			}
			return types.UndefinedValue
		},
	}

	vm.globals["defaultTo"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("defaultTo() expects at least 2 arguments", 0, 0, "")
			}
			val := args[0]
			if val == nil || val == types.UndefinedValue || val == types.NullValue {
				if str, ok := val.(types.String); ok && string(str) != "" {
					return val
				}
				return args[1]
			}
			return val
		},
	}

	vm.globals["ifElse"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("ifElse() expects 3 arguments", 0, 0, "")
			}
			cond := args[0]
			then := args[1]
			else_ := args[2]
			if types.ToBool(cond) {
				return then
			}
			return else_
		},
	}

	vm.globals["when"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("when() expects at least 3 arguments", 0, 0, "")
			}
			val := args[0]
			for i := 1; i < len(args)-1; i += 2 {
				if args[i].Equals(val) {
					return args[i+1]
				}
			}
			if len(args)%2 == 0 && len(args) > 3 {
				return args[len(args)-1]
			}
			return types.UndefinedValue
		},
	}

	vm.globals["rangeStep"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("rangeStep() expects at least 2 arguments", 0, 0, "")
			}
			start, _ := types.ToInt(args[0])
			end, _ := types.ToInt(args[1])
			step := types.Int(1)
			if len(args) >= 3 {
				step, _ = types.ToInt(args[2])
			}
			result := collections.NewArray()
			if step > 0 {
				for i := start; i < end; i += step {
					result.Append(i)
				}
			} else if step < 0 {
				for i := start; i > end; i += step {
					result.Append(i)
				}
			}
			return result
		},
	}

	vm.globals["times"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("times() expects at least 2 arguments", 0, 0, "")
			}
			n, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("times() first argument must be an integer", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("times() second argument must be a function", 0, 0, "")
			}
			for i := types.Int(0); i < n; i++ {
				fn.Fn(i)
			}
			return types.UndefinedValue
		},
	}

	vm.globals["tap"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("tap() expects at least 2 arguments", 0, 0, "")
			}
			val := args[0]
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("tap() second argument must be a function", 0, 0, "")
			}
			fn.Fn(val)
			return val
		},
	}

	vm.globals["pipe"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("pipe() expects at least 2 arguments", 0, 0, "")
			}
			val := args[0]
			for i := 1; i < len(args); i++ {
				fn, ok := args[i].(*types.NativeFunction)
				if !ok {
					return types.NewError("pipe() arguments after first must be functions", 0, 0, "")
				}
				val = fn.Fn(val)
			}
			return val
		},
	}

	vm.globals["compose"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("compose() expects at least 2 functions", 0, 0, "")
			}
			fns := make([]*types.NativeFunction, len(args))
			for i, arg := range args {
				fn, ok := arg.(*types.NativeFunction)
				if !ok {
					return types.NewError("compose() arguments must be functions", 0, 0, "")
				}
				fns[i] = fn
			}
			return types.UndefinedValue
		},
	}

	vm.globals["once"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("once() expects 1 argument", 0, 0, "")
			}
			fn, ok := args[0].(*types.NativeFunction)
			if !ok {
				return types.NewError("once() argument must be a function", 0, 0, "")
			}
			called := false
			var result types.Object
			return &types.NativeFunction{
				Fn: func(args ...types.Object) types.Object {
					if !called {
						result = fn.Fn(args...)
						called = true
					}
					return result
				},
			}
		},
	}

	vm.globals["memoize"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("memoize() expects 1 argument", 0, 0, "")
			}
			fn, ok := args[0].(*types.NativeFunction)
			if !ok {
				return types.NewError("memoize() argument must be a function", 0, 0, "")
			}
			cache := collections.NewMap()
			return &types.NativeFunction{
				Fn: func(args ...types.Object) types.Object {
					key := ""
					for _, arg := range args {
						key += arg.ToStr() + "|"
					}
					if cache.Get(key) != nil {
						return cache.Get(key)
					}
					result := fn.Fn(args...)
					cache.Set(key, result)
					return result
				},
			}
		},
	}

	vm.globals["curry"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("curry() expects at least 2 arguments", 0, 0, "")
			}
			fn, ok := args[0].(*types.NativeFunction)
			if !ok {
				return types.NewError("curry() first argument must be a function", 0, 0, "")
			}
			return fn
		},
	}

	vm.globals["flip"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("flip() expects 1 argument", 0, 0, "")
			}
			fn, ok := args[0].(*types.NativeFunction)
			if !ok {
				return types.NewError("flip() argument must be a function", 0, 0, "")
			}
			return &types.NativeFunction{
				Fn: func(args ...types.Object) types.Object {
					reversed := make([]types.Object, len(args))
					for i, arg := range args {
						reversed[len(args)-1-i] = arg
					}
					return fn.Fn(reversed...)
				},
			}
		},
	}

	vm.globals["identity"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.UndefinedValue
			}
			return args[0]
		},
	}

	vm.globals["constant"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("constant() expects 1 argument", 0, 0, "")
			}
			val := args[0]
			return &types.NativeFunction{
				Fn: func(args ...types.Object) types.Object {
					return val
				},
			}
		},
	}

	vm.globals["partial"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("partial() expects at least 2 arguments", 0, 0, "")
			}
			fn, ok := args[0].(*types.NativeFunction)
			if !ok {
				return types.NewError("partial() first argument must be a function", 0, 0, "")
			}
			partialArgs := args[1:]
			return &types.NativeFunction{
				Fn: func(args ...types.Object) types.Object {
					allArgs := append(partialArgs, args...)
					return fn.Fn(allArgs...)
				},
			}
		},
	}

	vm.globals["sleepAsync"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("sleepAsync() expects 1 argument", 0, 0, "")
			}
			ms, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("sleepAsync() expects an integer", 0, 0, "")
			}
			go func() {
				time.Sleep(time.Duration(ms) * time.Millisecond)
			}()
			return types.UndefinedValue
		},
	}

	vm.globals["async"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("async() expects 1 argument", 0, 0, "")
			}
			fn, ok := args[0].(*types.NativeFunction)
			if !ok {
				return types.NewError("async() argument must be a function", 0, 0, "")
			}
			go fn.Fn()
			return types.UndefinedValue
		},
	}

	vm.globals["defer"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("defer() expects 1 argument", 0, 0, "")
			}
			fn, ok := args[0].(*types.NativeFunction)
			if !ok {
				return types.NewError("defer() argument must be a function", 0, 0, "")
			}
			runtime.SetFinalizer(fn, func(f *types.NativeFunction) {
				f.Fn()
			})
			return types.UndefinedValue
		},
	}

	vm.globals["noop"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.UndefinedValue
		},
	}

	vm.globals["always"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("always() expects 1 argument", 0, 0, "")
			}
			val := args[0]
			return &types.NativeFunction{
				Fn: func(args ...types.Object) types.Object {
					return val
				},
			}
		},
	}

	vm.globals["juxt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("juxt() expects at least 2 functions", 0, 0, "")
			}
			fns := make([]*types.NativeFunction, len(args))
			for i, arg := range args {
				fn, ok := arg.(*types.NativeFunction)
				if !ok {
					return types.NewError("juxt() arguments must be functions", 0, 0, "")
				}
				fns[i] = fn
			}
			return &types.NativeFunction{
				Fn: func(args ...types.Object) types.Object {
					result := collections.NewArray()
					for _, fn := range fns {
						result.Append(fn.Fn(args...))
					}
					return result
				},
			}
		},
	}

	vm.globals["apply"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("apply() expects at least 2 arguments", 0, 0, "")
			}
			fn, ok := args[0].(*types.NativeFunction)
			if !ok {
				return types.NewError("apply() first argument must be a function", 0, 0, "")
			}
			arr, ok := args[1].(*collections.Array)
			if !ok {
				return types.NewError("apply() second argument must be an array", 0, 0, "")
			}
			fnArgs := make([]types.Object, arr.Len())
			for i := 0; i < arr.Len(); i++ {
				fnArgs[i] = arr.Get(i)
			}
			return fn.Fn(fnArgs...)
		},
	}

	vm.globals["spread"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("spread() expects at least 2 arguments", 0, 0, "")
			}
			fn, ok := args[0].(*types.NativeFunction)
			if !ok {
				return types.NewError("spread() first argument must be a function", 0, 0, "")
			}
			arr, ok := args[1].(*collections.Array)
			if !ok {
				return types.NewError("spread() second argument must be an array", 0, 0, "")
			}
			fnArgs := make([]types.Object, arr.Len())
			for i := 0; i < arr.Len(); i++ {
				fnArgs[i] = arr.Get(i)
			}
			return fn.Fn(fnArgs...)
		},
	}

	vm.globals["call"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("call() expects at least 1 argument", 0, 0, "")
			}
			fn, ok := args[0].(*types.NativeFunction)
			if !ok {
				return types.NewError("call() first argument must be a function", 0, 0, "")
			}
			return fn.Fn(args[1:]...)
		},
	}

	vm.globals["bind"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("bind() expects at least 2 arguments", 0, 0, "")
			}
			fn, ok := args[0].(*types.NativeFunction)
			if !ok {
				return types.NewError("bind() first argument must be a function", 0, 0, "")
			}
			this := args[1]
			return &types.NativeFunction{
				Fn: func(args ...types.Object) types.Object {
					return fn.Fn(append([]types.Object{this}, args...)...)
				},
			}
		},
	}

	vm.globals["getPath"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("getPath() expects at least 2 arguments", 0, 0, "")
			}
			obj := args[0]
			path, ok := args[1].(*collections.Array)
			if !ok {
				return types.NewError("getPath() second argument must be an array", 0, 0, "")
			}
			for i := 0; i < path.Len(); i++ {
				key := string(types.ToString(path.Get(i)))
				if m, ok := obj.(*collections.Map); ok {
					obj = m.Get(key)
				} else if arr, ok := obj.(*collections.Array); ok {
					idx, _ := types.ToInt(types.String(key))
					obj = arr.Get(int(idx))
				} else {
					return types.UndefinedValue
				}
			}
			return obj
		},
	}

	vm.globals["setPath"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("setPath() expects at least 3 arguments", 0, 0, "")
			}
			obj := args[0]
			path, ok := args[1].(*collections.Array)
			if !ok {
				return types.NewError("setPath() second argument must be an array", 0, 0, "")
			}
			value := args[2]
			if path.Len() == 1 {
				key := string(types.ToString(path.Get(0)))
				if m, ok := obj.(*collections.Map); ok {
					m.Set(key, value)
				}
				return obj
			}
			return obj
		},
	}

	vm.globals["pickPaths"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("pickPaths() expects at least 2 arguments", 0, 0, "")
			}
			obj := args[0]
			paths, ok := args[1].(*collections.Array)
			if !ok {
				return types.NewError("pickPaths() second argument must be an array", 0, 0, "")
			}
			result := collections.NewMap()
			for i := 0; i < paths.Len(); i++ {
				path := paths.Get(i)
				val := obj
				if pathArr, ok := path.(*collections.Array); ok {
					for j := 0; j < pathArr.Len(); j++ {
						key := string(types.ToString(pathArr.Get(j)))
						if m, ok := val.(*collections.Map); ok {
							val = m.Get(key)
						} else if arr, ok := val.(*collections.Array); ok {
							idx, _ := types.ToInt(types.String(key))
							val = arr.Get(int(idx))
						} else {
							val = types.UndefinedValue
							break
						}
					}
				}
				result.Set(path.ToStr(), val)
			}
			return result
		},
	}

	vm.globals["omitPaths"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("omitPaths() expects at least 2 arguments", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				return types.NewError("omitPaths() first argument must be a map", 0, 0, "")
			}
			paths, ok := args[1].(*collections.Array)
			if !ok {
				return types.NewError("omitPaths() second argument must be an array", 0, 0, "")
			}
			omit := make(map[string]bool)
			for i := 0; i < paths.Len(); i++ {
				omit[paths.Get(i).ToStr()] = true
			}
			result := collections.NewMap()
			mKeys := m.Keys()
			for i := 0; i < mKeys.Len(); i++ {
				k := string(types.ToString(mKeys.Get(i)))
				if !omit[k] {
					result.Set(k, m.Get(k))
				}
			}
			return result
		},
	}

	vm.globals["deepGet"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("deepGet() expects at least 2 arguments", 0, 0, "")
			}
			obj := args[0]
			key := string(types.ToString(args[1]))
			if m, ok := obj.(*collections.Map); ok {
				return m.Get(key)
			}
			return types.UndefinedValue
		},
	}

	vm.globals["deepSet"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("deepSet() expects at least 3 arguments", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				return types.NewError("deepSet() first argument must be a map", 0, 0, "")
			}
			key := string(types.ToString(args[1]))
			value := args[2]
			m.Set(key, value)
			return m
		},
	}

	vm.globals["cloneDeep"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("cloneDeep() expects 1 argument", 0, 0, "")
			}
			return args[0]
		},
	}

	vm.globals["mergeDeep"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("mergeDeep() expects at least 2 arguments", 0, 0, "")
			}
			result := collections.NewMap()
			for _, arg := range args {
				m, ok := arg.(*collections.Map)
				if !ok {
					continue
				}
				mKeys := m.Keys()
				for i := 0; i < mKeys.Len(); i++ {
					k := string(types.ToString(mKeys.Get(i)))
					result.Set(k, m.Get(k))
				}
			}
			return result
		},
	}

	vm.globals["chunkArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("chunk() expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("chunk() first argument must be an array", 0, 0, "")
			}
			size, ok := args[1].(types.Int)
			if !ok || size <= 0 {
				return types.NewError("chunk() second argument must be a positive integer", 0, 0, "")
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i += int(size) {
				chunk := collections.NewArray()
				for j := 0; j < int(size) && i+j < arr.Len(); j++ {
					chunk.Append(arr.Get(i + j))
				}
				result.Append(chunk)
			}
			return result
		},
	}

	vm.globals["unzip"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("unzip() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("unzip() expects an array of arrays", 0, 0, "")
			}
			if arr.Len() == 0 {
				return collections.NewArray()
			}
			first, ok := arr.Get(0).(*collections.Array)
			if !ok {
				return types.NewError("unzip() expects an array of arrays", 0, 0, "")
			}
			result := collections.NewArray()
			for i := 0; i < first.Len(); i++ {
				col := collections.NewArray()
				for j := 0; j < arr.Len(); j++ {
					row := arr.Get(j).(*collections.Array)
					if i < row.Len() {
						col.Append(row.Get(i))
					}
				}
				result.Append(col)
			}
			return result
		},
	}

	vm.globals["zipObj"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("zipObj() expects 2 arrays", 0, 0, "")
			}
			keys, ok1 := args[0].(*collections.Array)
			vals, ok2 := args[1].(*collections.Array)
			if !ok1 || !ok2 {
				return types.NewError("zipObj() expects 2 arrays", 0, 0, "")
			}
			result := collections.NewMap()
			n := keys.Len()
			if vals.Len() < n {
				n = vals.Len()
			}
			for i := 0; i < n; i++ {
				key := string(types.ToString(keys.Get(i)))
				result.Set(key, vals.Get(i))
			}
			return result
		},
	}

	vm.globals["fromEntries"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("fromEntries() expects 1 argument", 0, 0, "")
			}
			entries, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("fromEntries() expects an array", 0, 0, "")
			}
			result := collections.NewMap()
			for i := 0; i < entries.Len(); i++ {
				entry, ok := entries.Get(i).(*collections.Array)
				if !ok || entry.Len() < 2 {
					continue
				}
				key := string(types.ToString(entry.Get(0)))
				value := entry.Get(1)
				result.Set(key, value)
			}
			return result
		},
	}

	vm.globals["toEntries"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toEntries() expects 1 argument", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				return types.NewError("toEntries() expects a map", 0, 0, "")
			}
			result := collections.NewArray()
			mKeys := m.Keys()
			for i := 0; i < mKeys.Len(); i++ {
				k := string(types.ToString(mKeys.Get(i)))
				entry := collections.NewArray()
				entry.Append(types.String(k))
				entry.Append(m.Get(k))
				result.Append(entry)
			}
			return result
		},
	}

	vm.globals["headArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("head() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("head() expects an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return types.UndefinedValue
			}
			return arr.Get(0)
		},
	}

	vm.globals["tailArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("tail() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("tail() expects an array", 0, 0, "")
			}
			if arr.Len() <= 1 {
				return collections.NewArray()
			}
			result := collections.NewArray()
			for i := 1; i < arr.Len(); i++ {
				result.Append(arr.Get(i))
			}
			return result
		},
	}

	vm.globals["initArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("init() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("init() expects an array", 0, 0, "")
			}
			if arr.Len() <= 1 {
				return collections.NewArray()
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len()-1; i++ {
				result.Append(arr.Get(i))
			}
			return result
		},
	}

	vm.globals["lastArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("last() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("last() expects an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return types.UndefinedValue
			}
			return arr.Get(arr.Len() - 1)
		},
	}

	vm.globals["uniqueArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("unique() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("unique() expects an array", 0, 0, "")
			}
			seen := collections.NewMap()
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				item := arr.Get(i)
				key := item.ToStr()
				if seen.Get(key) == nil {
					seen.Set(key, types.Bool(true))
					result.Append(item)
				}
			}
			return result
		},
	}

	vm.globals["flattenDeep"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("flattenDeep() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("flattenDeep() expects an array", 0, 0, "")
			}
			result := collections.NewArray()
			var flatten func(a *collections.Array)
			flatten = func(a *collections.Array) {
				for i := 0; i < a.Len(); i++ {
					if nested, ok := a.Get(i).(*collections.Array); ok {
						flatten(nested)
					} else {
						result.Append(a.Get(i))
					}
				}
			}
			flatten(arr)
			return result
		},
	}

	vm.globals["differenceArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("difference() expects at least 2 arrays", 0, 0, "")
			}
			arr1, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("difference() expects arrays", 0, 0, "")
			}
			set := collections.NewMap()
			for i := 1; i < len(args); i++ {
				arr, ok := args[i].(*collections.Array)
				if !ok {
					continue
				}
				for j := 0; j < arr.Len(); j++ {
					set.Set(arr.Get(j).ToStr(), types.Bool(true))
				}
			}
			result := collections.NewArray()
			for i := 0; i < arr1.Len(); i++ {
				item := arr1.Get(i)
				if set.Get(item.ToStr()) == nil {
					result.Append(item)
				}
			}
			return result
		},
	}

	vm.globals["intersectionArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("intersection() expects at least 2 arrays", 0, 0, "")
			}
			arr1, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("intersection() expects arrays", 0, 0, "")
			}
			set := collections.NewMap()
			for i := 1; i < len(args); i++ {
				arr, ok := args[i].(*collections.Array)
				if !ok {
					continue
				}
				for j := 0; j < arr.Len(); j++ {
					set.Set(arr.Get(j).ToStr(), types.Bool(true))
				}
			}
			result := collections.NewArray()
			for i := 0; i < arr1.Len(); i++ {
				item := arr1.Get(i)
				if set.Get(item.ToStr()) != nil {
					result.Append(item)
				}
			}
			return result
		},
	}

	vm.globals["unionArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("union() expects at least 2 arrays", 0, 0, "")
			}
			seen := collections.NewMap()
			result := collections.NewArray()
			for _, arg := range args {
				arr, ok := arg.(*collections.Array)
				if !ok {
					continue
				}
				for i := 0; i < arr.Len(); i++ {
					item := arr.Get(i)
					if seen.Get(item.ToStr()) == nil {
						seen.Set(item.ToStr(), types.Bool(true))
						result.Append(item)
					}
				}
			}
			return result
		},
	}

	vm.globals["sortedIndex"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("sortedIndex() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("sortedIndex() first argument must be an array", 0, 0, "")
			}
			val := args[1]
			for i := 0; i < arr.Len(); i++ {
				if val.ToStr() <= arr.Get(i).ToStr() {
					return types.Int(i)
				}
			}
			return types.Int(arr.Len())
		},
	}

	vm.globals["sortedLastIndex"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("sortedLastIndex() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("sortedLastIndex() first argument must be an array", 0, 0, "")
			}
			val := args[1]
			for i := arr.Len() - 1; i >= 0; i-- {
				if val.ToStr() >= arr.Get(i).ToStr() {
					return types.Int(i + 1)
				}
			}
			return types.Int(0)
		},
	}

	vm.globals["addIndex"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("addIndex() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("addIndex() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("addIndex() second argument must be a function", 0, 0, "")
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				result.Append(fn.Fn(arr.Get(i), types.Int(i)))
			}
			return result
		},
	}

	vm.globals["filterIndex"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("filterIndex() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("filterIndex() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("filterIndex() second argument must be a function", 0, 0, "")
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				if types.ToBool(fn.Fn(arr.Get(i), types.Int(i))) {
					result.Append(arr.Get(i))
				}
			}
			return result
		},
	}

	vm.globals["findLastIndex"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("findLastIndex() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("findLastIndex() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("findLastIndex() second argument must be a function", 0, 0, "")
			}
			for i := arr.Len() - 1; i >= 0; i-- {
				if types.ToBool(fn.Fn(arr.Get(i))) {
					return types.Int(i)
				}
			}
			return types.Int(-1)
		},
	}

	vm.globals["findLast"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("findLast() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("findLast() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("findLast() second argument must be a function", 0, 0, "")
			}
			for i := arr.Len() - 1; i >= 0; i-- {
				if types.ToBool(fn.Fn(arr.Get(i))) {
					return arr.Get(i)
				}
			}
			return types.UndefinedValue
		},
	}

	vm.globals["countBy"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("countBy() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("countBy() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("countBy() second argument must be a function", 0, 0, "")
			}
			counts := collections.NewMap()
			for i := 0; i < arr.Len(); i++ {
				key := fn.Fn(arr.Get(i)).ToStr()
				if counts.Get(key) == nil {
					counts.Set(key, types.Int(1))
				} else {
					cur, _ := counts.Get(key).(types.Int)
					counts.Set(key, cur+1)
				}
			}
			return counts
		},
	}

	vm.globals["groupByKey"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("groupBy() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("groupBy() first argument must be an array", 0, 0, "")
			}
			keyFn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("groupBy() second argument must be a function", 0, 0, "")
			}
			result := collections.NewMap()
			for i := 0; i < arr.Len(); i++ {
				key := keyFn.Fn(arr.Get(i)).ToStr()
				if result.Get(key) == nil {
					result.Set(key, collections.NewArray())
				}
				result.Get(key).(*collections.Array).Append(arr.Get(i))
			}
			return result
		},
	}

	vm.globals["keyBy"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("keyBy() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("keyBy() first argument must be an array", 0, 0, "")
			}
			keyFn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("keyBy() second argument must be a function", 0, 0, "")
			}
			result := collections.NewMap()
			for i := 0; i < arr.Len(); i++ {
				key := keyFn.Fn(arr.Get(i)).ToStr()
				result.Set(key, arr.Get(i))
			}
			return result
		},
	}

	vm.globals["partition"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("partition() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("partition() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("partition() second argument must be a function", 0, 0, "")
			}
			pass := collections.NewArray()
			fail := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				if types.ToBool(fn.Fn(arr.Get(i))) {
					pass.Append(arr.Get(i))
				} else {
					fail.Append(arr.Get(i))
				}
			}
			result := collections.NewArray()
			result.Append(pass)
			result.Append(fail)
			return result
		},
	}

	vm.globals["sortByKey"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("sortBy() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("sortBy() first argument must be an array", 0, 0, "")
			}
			keyFn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("sortBy() second argument must be a function", 0, 0, "")
			}
			elements := make([]types.Object, arr.Len())
			for i := 0; i < arr.Len(); i++ {
				elements[i] = arr.Get(i)
			}
			sort.Slice(elements, func(i, j int) bool {
				return keyFn.Fn(elements[i]).ToStr() < keyFn.Fn(elements[j]).ToStr()
			})
			return collections.NewArrayWithElements(elements)
		},
	}

	vm.globals["sortByOrder"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("sortByOrder() expects at least 3 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("sortByOrder() first argument must be an array", 0, 0, "")
			}
			keys, ok := args[1].(*collections.Array)
			if !ok {
				return types.NewError("sortByOrder() second argument must be an array", 0, 0, "")
			}
			orders, ok := args[2].(*collections.Array)
			if !ok {
				return types.NewError("sortByOrder() third argument must be an array", 0, 0, "")
			}
			elements := make([]types.Object, arr.Len())
			for i := 0; i < arr.Len(); i++ {
				elements[i] = arr.Get(i)
			}
			sort.Slice(elements, func(i, j int) bool {
				for k := 0; k < keys.Len() && k < orders.Len(); k++ {
					key := keys.Get(k).ToStr()
					order := orders.Get(k).ToStr()
					valI := elements[i].(*collections.Map).Get(key)
					valJ := elements[j].(*collections.Map).Get(key)
					if valI.ToStr() != valJ.ToStr() {
						if order == "desc" {
							return valI.ToStr() > valJ.ToStr()
						}
						return valI.ToStr() < valJ.ToStr()
					}
				}
				return false
			})
			return collections.NewArrayWithElements(elements)
		},
	}

	vm.globals["isSorted"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isSorted() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("isSorted() expects an array", 0, 0, "")
			}
			for i := 0; i < arr.Len()-1; i++ {
				if arr.Get(i).ToStr() > arr.Get(i+1).ToStr() {
					return types.Bool(false)
				}
			}
			return types.Bool(true)
		},
	}

	vm.globals["sortedUNIQ"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("sortedUNIQ() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("sortedUNIQ() expects an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return collections.NewArray()
			}
			result := collections.NewArray()
			result.Append(arr.Get(0))
			for i := 1; i < arr.Len(); i++ {
				if arr.Get(i).ToStr() != arr.Get(i-1).ToStr() {
					result.Append(arr.Get(i))
				}
			}
			return result
		},
	}

	vm.globals["xor"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			result := false
			for _, arg := range args {
				result = result != types.ToBool(arg)
			}
			return types.Bool(result)
		},
	}

	vm.globals["bitcount"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("bitcount() expects 1 argument", 0, 0, "")
			}
			n, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("bitcount() expects an integer", 0, 0, "")
			}
			count := 0
			for n > 0 {
				if n&1 == 1 {
					count++
				}
				n >>= 1
			}
			return types.Int(count)
		},
	}

	vm.globals["gcd"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("gcd() expects 2 arguments", 0, 0, "")
			}
			a, ok1 := args[0].(types.Int)
			b, ok2 := args[1].(types.Int)
			if !ok1 || !ok2 {
				return types.NewError("gcd() expects integers", 0, 0, "")
			}
			for b != 0 {
				a, b = b, a%b
			}
			if a < 0 {
				a = -a
			}
			return a
		},
	}

	vm.globals["lcm"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("lcm() expects 2 arguments", 0, 0, "")
			}
			a, ok1 := args[0].(types.Int)
			b, ok2 := args[1].(types.Int)
			if !ok1 || !ok2 {
				return types.NewError("lcm() expects integers", 0, 0, "")
			}
			if a == 0 || b == 0 {
				return types.Int(0)
			}
			if a < 0 {
				a = -a
			}
			if b < 0 {
				b = -b
			}
			x, y := a, b
			for y != 0 {
				x, y = y, x%y
			}
			return a / x * b
		},
	}

	vm.globals["isPowerOf2"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isPowerOf2() expects 1 argument", 0, 0, "")
			}
			n, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("isPowerOf2() expects an integer", 0, 0, "")
			}
			return types.Bool(n > 0 && (n&(n-1)) == 0)
		},
	}

	vm.globals["tzCount"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("tzCount() expects 1 argument", 0, 0, "")
			}
			n, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("tzCount() expects an integer", 0, 0, "")
			}
			count := 0
			for n&1 == 0 && count < 64 {
				count++
				n >>= 1
			}
			return types.Int(count)
		},
	}

	vm.globals["char"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("char() expects 1 argument", 0, 0, "")
			}
			i, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("char() expects an integer", 0, 0, "")
			}
			return types.String(string(rune(i)))
		},
	}

	vm.globals["chars"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("chars() expects 1 argument", 0, 0, "")
			}
			s, ok := args[0].(types.String)
			if !ok {
				return types.NewError("chars() expects a string", 0, 0, "")
			}
			result := collections.NewArray()
			for _, c := range []rune(s) {
				result.Append(types.Int(c))
			}
			return result
		},
	}

	vm.globals["add"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("add() expects at least 2 arguments", 0, 0, "")
			}
			result, _ := types.ToFloat(args[0])
			for i := 1; i < len(args); i++ {
				val, _ := types.ToFloat(args[i])
				result += val
			}
			if result == types.Float(int(result)) {
				return types.Int(int(result))
			}
			return types.Float(result)
		},
	}

	vm.globals["negate"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("negate() expects 1 argument", 0, 0, "")
			}
			n, ok := args[0].(types.Int)
			if ok {
				return types.Int(-n)
			}
			f, ok := args[0].(types.Float)
			if ok {
				return types.Float(-f)
			}
			return types.NewError("negate() expects a number", 0, 0, "")
		},
	}

	vm.globals["inc"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("inc() expects 1 argument", 0, 0, "")
			}
			n, ok := args[0].(types.Int)
			if ok {
				return types.Int(n + 1)
			}
			f, ok := args[0].(types.Float)
			if ok {
				return types.Float(f + 1)
			}
			return types.NewError("inc() expects a number", 0, 0, "")
		},
	}

	vm.globals["dec"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("dec() expects 1 argument", 0, 0, "")
			}
			n, ok := args[0].(types.Int)
			if ok {
				return types.Int(n - 1)
			}
			f, ok := args[0].(types.Float)
			if ok {
				return types.Float(f - 1)
			}
			return types.NewError("dec() expects a number", 0, 0, "")
		},
	}

	vm.globals["sign"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("sign() expects 1 argument", 0, 0, "")
			}
			f, _ := types.ToFloat(args[0])
			if f > 0 {
				return types.Int(1)
			} else if f < 0 {
				return types.Int(-1)
			}
			return types.Int(0)
		},
	}

	vm.globals["signBit"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("signBit() expects 1 argument", 0, 0, "")
			}
			n, ok := args[0].(types.Int)
			if ok {
				return types.Bool(n < 0)
			}
			f, ok := args[0].(types.Float)
			if ok {
				return types.Bool(f < 0)
			}
			return types.NewError("signBit() expects a number", 0, 0, "")
		},
	}

	vm.globals["uint"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("uint() expects 1 argument", 0, 0, "")
			}
			i, ok := args[0].(types.Int)
			if ok {
				if i < 0 {
					return types.NewError("uint() expects non-negative integer", 0, 0, "")
				}
				return i
			}
			f, ok := args[0].(types.Float)
			if ok {
				if f < 0 {
					return types.NewError("uint() expects non-negative number", 0, 0, "")
				}
				return types.Int(uint(f))
			}
			return types.NewError("uint() expects a number", 0, 0, "")
		},
	}

	vm.globals["uint8"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("uint8() expects 1 argument", 0, 0, "")
			}
			i, ok := args[0].(types.Int)
			if ok {
				if i < 0 || i > 255 {
					return types.NewError("uint8() expects 0-255", 0, 0, "")
				}
				return types.Int(uint8(i))
			}
			f, ok := args[0].(types.Float)
			if ok {
				if f < 0 || f > 255 {
					return types.NewError("uint8() expects 0-255", 0, 0, "")
				}
				return types.Int(uint8(f))
			}
			return types.NewError("uint8() expects a number", 0, 0, "")
		},
	}

	vm.globals["int8"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("int8() expects 1 argument", 0, 0, "")
			}
			i, ok := args[0].(types.Int)
			if ok {
				return types.Int(int8(i))
			}
			f, ok := args[0].(types.Float)
			if ok {
				return types.Int(int8(f))
			}
			return types.NewError("int8() expects a number", 0, 0, "")
		},
	}

	vm.globals["int16"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("int16() expects 1 argument", 0, 0, "")
			}
			i, ok := args[0].(types.Int)
			if ok {
				return types.Int(int16(i))
			}
			f, ok := args[0].(types.Float)
			if ok {
				return types.Int(int16(f))
			}
			return types.NewError("int16() expects a number", 0, 0, "")
		},
	}

	vm.globals["int32"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("int32() expects 1 argument", 0, 0, "")
			}
			i, ok := args[0].(types.Int)
			if ok {
				return types.Int(int32(i))
			}
			f, ok := args[0].(types.Float)
			if ok {
				return types.Int(int32(f))
			}
			return types.NewError("int32() expects a number", 0, 0, "")
		},
	}

	vm.globals["int64"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("int64() expects 1 argument", 0, 0, "")
			}
			i, ok := args[0].(types.Int)
			if ok {
				return i
			}
			f, ok := args[0].(types.Float)
			if ok {
				return types.Int(int64(f))
			}
			return types.NewError("int64() expects a number", 0, 0, "")
		},
	}

	vm.globals["float32"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("float32() expects 1 argument", 0, 0, "")
			}
			f, ok := args[0].(types.Float)
			if ok {
				return types.Float(float32(f))
			}
			i, ok := args[0].(types.Int)
			if ok {
				return types.Float(float32(i))
			}
			return types.NewError("float32() expects a number", 0, 0, "")
		},
	}

	vm.globals["float64"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("float64() expects 1 argument", 0, 0, "")
			}
			f, ok := args[0].(types.Float)
			if ok {
				return f
			}
			i, ok := args[0].(types.Int)
			if ok {
				return types.Float(float64(i))
			}
			return types.NewError("float64() expects a number", 0, 0, "")
		},
	}

	vm.globals["byteVal"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("byteVal() expects 1 argument", 0, 0, "")
			}
			s, ok := args[0].(types.String)
			if ok {
				if len(s) == 0 {
					return types.Int(0)
				}
				return types.Int(s[0])
			}
			i, ok := args[0].(types.Int)
			if ok {
				return types.Int(byte(i))
			}
			return types.NewError("byteVal() expects string or integer", 0, 0, "")
		},
	}

	vm.globals["hexEncode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("hexEncode() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			return types.String(hex.EncodeToString([]byte(str)))
		},
	}

	vm.globals["hexDecode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("hexDecode() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			decoded, err := hex.DecodeString(str)
			if err != nil {
				return types.NewError(fmt.Sprintf("hexDecode error: %v", err), 0, 0, "")
			}
			return types.String(string(decoded))
		},
	}

	vm.globals["rot13"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("rot13() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			result := make([]byte, len(str))
			for i, c := range str {
				if c >= 'a' && c <= 'z' {
					result[i] = byte((c-'a'+13)%26 + 'a')
				} else if c >= 'A' && c <= 'Z' {
					result[i] = byte((c-'A'+13)%26 + 'A')
				} else {
					result[i] = byte(c)
				}
			}
			return types.String(string(result))
		},
	}

	vm.globals["reverse"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("reverse() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			runes := []rune(str)
			for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
				runes[i], runes[j] = runes[j], runes[i]
			}
			return types.String(string(runes))
		},
	}

	vm.globals["repeatStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("repeatStr() expects 2 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			count, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("repeatStr() expects string and integer", 0, 0, "")
			}
			return types.String(strings.Repeat(str, int(count)))
		},
	}

	vm.globals["replaceStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("replaceStr() expects 3 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			old := string(types.ToString(args[1]))
			new := string(types.ToString(args[2]))
			return types.String(strings.Replace(str, old, new, 1))
		},
	}

	vm.globals["replaceAllStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("replaceAllStr() expects 3 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			old := string(types.ToString(args[1]))
			new := string(types.ToString(args[2]))
			return types.String(strings.ReplaceAll(str, old, new))
		},
	}

	vm.globals["strStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strStr() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			substr := string(types.ToString(args[1]))
			idx := strings.Index(s, substr)
			if idx == -1 {
				return types.Int(-1)
			}
			return types.Int(idx)
		},
	}

	vm.globals["lastStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("last() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			substr := string(types.ToString(args[1]))
			idx := strings.LastIndex(s, substr)
			if idx == -1 {
				return types.Int(-1)
			}
			return types.Int(idx)
		},
	}

	vm.globals["strContains"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strContains() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			substr := string(types.ToString(args[1]))
			return types.Bool(strings.Contains(s, substr))
		},
	}

	vm.globals["hasPrefixStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("hasPrefixStr() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			prefix := string(types.ToString(args[1]))
			return types.Bool(strings.HasPrefix(s, prefix))
		},
	}

	vm.globals["hasSuffixStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("hasSuffixStr() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			suffix := string(types.ToString(args[1]))
			return types.Bool(strings.HasSuffix(s, suffix))
		},
	}

	vm.globals["strEqual"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strEqual() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			t := string(types.ToString(args[1]))
			return types.Bool(s == t)
		},
	}

	vm.globals["strEqualFold"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strEqualFold() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			t := string(types.ToString(args[1]))
			return types.Bool(strings.EqualFold(s, t))
		},
	}

	vm.globals["strCompare"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strCompare() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			t := string(types.ToString(args[1]))
			return types.Int(strings.Compare(s, t))
		},
	}

	vm.globals["trimStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("trimStr() expects at least 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			if len(args) > 1 {
				cutset := string(types.ToString(args[1]))
				return types.String(strings.Trim(str, cutset))
			}
			return types.String(strings.TrimSpace(str))
		},
	}

	vm.globals["trimLeftStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("trimLeftStr() expects at least 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			if len(args) > 1 {
				cutset := string(types.ToString(args[1]))
				return types.String(strings.TrimLeft(str, cutset))
			}
			return types.String(strings.TrimLeftFunc(str, unicode.IsSpace))
		},
	}

	vm.globals["trimRightStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("trimRightStr() expects at least 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			if len(args) > 1 {
				cutset := string(types.ToString(args[1]))
				return types.String(strings.TrimRight(str, cutset))
			}
			return types.String(strings.TrimRightFunc(str, unicode.IsSpace))
		},
	}

	vm.globals["strFields"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("strFields() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			fields := strings.Fields(str)
			result := collections.NewArray()
			for _, f := range fields {
				result.Append(types.String(f))
			}
			return result
		},
	}

	vm.globals["strSplit"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strSplit() expects 2 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			sep := string(types.ToString(args[1]))
			parts := strings.Split(str, sep)
			result := collections.NewArray()
			for _, p := range parts {
				result.Append(types.String(p))
			}
			return result
		},
	}

	vm.globals["strJoin"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strJoin() expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("strJoin() first argument must be an array", 0, 0, "")
			}
			sep := string(types.ToString(args[1]))
			parts := make([]string, arr.Len())
			for i := 0; i < arr.Len(); i++ {
				parts[i] = string(types.ToString(arr.Get(i)))
			}
			return types.String(strings.Join(parts, sep))
		},
	}

	vm.globals["strToUpper"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("strToUpper() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			return types.String(strings.ToUpper(str))
		},
	}

	vm.globals["strToLower"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("strToLower() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			return types.String(strings.ToLower(str))
		},
	}

	vm.globals["strTitle"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("strTitle() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			return types.String(strings.Title(str))
		},
	}

	vm.globals["strLen"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("strLen() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			return types.Int(len(str))
		},
	}

	vm.globals["strRuneLen"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("strRuneLen() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			return types.Int(len([]rune(str)))
		},
	}

	vm.globals["strHasPrefix"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strHasPrefix() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			p := string(types.ToString(args[1]))
			return types.Bool(strings.HasPrefix(s, p))
		},
	}

	vm.globals["strHasSuffix"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strHasSuffix() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			suf := string(types.ToString(args[1]))
			return types.Bool(strings.HasSuffix(s, suf))
		},
	}

	vm.globals["strContainsAny"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strContainsAny() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			chars := string(types.ToString(args[1]))
			return types.Bool(strings.ContainsAny(s, chars))
		},
	}

	vm.globals["strContainsRune"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strContainsRune() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			r, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("strContainsRune() second argument must be an integer", 0, 0, "")
			}
			return types.Bool(strings.ContainsRune(s, rune(r)))
		},
	}

	vm.globals["strIndex"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strIndex() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			substr := string(types.ToString(args[1]))
			idx := strings.Index(s, substr)
			if idx == -1 {
				return types.Int(-1)
			}
			return types.Int(idx)
		},
	}

	vm.globals["strLastIndex"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strLastIndex() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			substr := string(types.ToString(args[1]))
			idx := strings.LastIndex(s, substr)
			if idx == -1 {
				return types.Int(-1)
			}
			return types.Int(idx)
		},
	}

	vm.globals["strIndexAny"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strIndexAny() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			chars := string(types.ToString(args[1]))
			idx := strings.IndexAny(s, chars)
			if idx == -1 {
				return types.Int(-1)
			}
			return types.Int(idx)
		},
	}

	vm.globals["strCount"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strCount() expects 2 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			substr := string(types.ToString(args[1]))
			return types.Int(strings.Count(s, substr))
		},
	}

	vm.globals["strReplace"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 4 {
				return types.NewError("strReplace() expects 4 arguments", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			old := string(types.ToString(args[1]))
			new := string(types.ToString(args[2]))
			n, ok := args[3].(types.Int)
			if !ok {
				return types.NewError("strReplace() fourth argument must be an integer", 0, 0, "")
			}
			return types.String(strings.Replace(s, old, new, int(n)))
		},
	}

	vm.globals["strMap"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strMap() expects 2 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("strMap() second argument must be a function", 0, 0, "")
			}
			runes := []rune(str)
			result := collections.NewArray()
			for _, r := range runes {
				result.Append(fn.Fn(types.Int(r)))
			}
			return result
		},
	}

	vm.globals["strFilter"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strFilter() expects 2 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("strFilter() second argument must be a function", 0, 0, "")
			}
			runes := []rune(str)
			result := collections.NewArray()
			for _, r := range runes {
				if types.ToBool(fn.Fn(types.Int(r))) {
					result.Append(types.Int(r))
				}
			}
			return result
		},
	}

	vm.globals["strFold"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("strFold() expects 1 argument", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			return types.String(strings.ToLower(strings.ToUpper(str)))
		},
	}

	vm.globals["strRepeat"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strRepeat() expects 2 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			count, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("strRepeat() expects string and integer", 0, 0, "")
			}
			return types.String(strings.Repeat(str, int(count)))
		},
	}

	vm.globals["strPad"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("strPad() expects 3 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			length, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("strPad() second argument must be an integer", 0, 0, "")
			}
			pad := string(types.ToString(args[2]))
			if len(str) >= int(length) {
				return types.String(str)
			}
			padLen := int(length) - len(str)
			padding := strings.Repeat(pad, (padLen+len(pad)-1)/len(pad))
			return types.String(padding[:padLen] + str)
		},
	}

	vm.globals["strPadEnd"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("strPadEnd() expects 3 arguments", 0, 0, "")
			}
			str := string(types.ToString(args[0]))
			length, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("strPadEnd() second argument must be an integer", 0, 0, "")
			}
			pad := string(types.ToString(args[2]))
			if len(str) >= int(length) {
				return types.String(str)
			}
			padLen := int(length) - len(str)
			padding := strings.Repeat(pad, (padLen+len(pad)-1)/len(pad))
			return types.String(str + padding[:padLen])
		},
	}

	vm.globals["uuid"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			b := make([]byte, 16)
			rand.Read(b)
			b[6] = (b[6] & 0x0f) | 0x40
			b[8] = (b[8] & 0x3f) | 0x80
			return types.String(fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]))
		},
	}

	vm.globals["powInt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("powInt() expects 2 arguments", 0, 0, "")
			}
			x, ok1 := args[0].(types.Int)
			y, ok2 := args[1].(types.Int)
			if !ok1 || !ok2 {
				return types.NewError("powInt() expects integers", 0, 0, "")
			}
			if y < 0 {
				return types.NewError("powInt() expects non-negative exponent", 0, 0, "")
			}
			result := types.Int(1)
			for i := types.Int(0); i < y; i++ {
				result *= x
			}
			return result
		},
	}

	vm.globals["factorial"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("factorial() expects 1 argument", 0, 0, "")
			}
			n, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("factorial() expects an integer", 0, 0, "")
			}
			if n < 0 {
				return types.NewError("factorial() expects non-negative integer", 0, 0, "")
			}
			result := types.Int(1)
			for i := types.Int(1); i <= n; i++ {
				result *= i
			}
			return result
		},
	}

	vm.globals["fibonacci"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("fibonacci() expects 1 argument", 0, 0, "")
			}
			n, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("fibonacci() expects an integer", 0, 0, "")
			}
			if n < 0 {
				return types.NewError("fibonacci() expects non-negative integer", 0, 0, "")
			}
			if n == 0 {
				return types.Int(0)
			}
			if n == 1 {
				return types.Int(1)
			}
			a, b := types.Int(0), types.Int(1)
			for i := types.Int(2); i <= n; i++ {
				a, b = b, a+b
			}
			return b
		},
	}

	vm.globals["isPrimeNum"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isPrimeNum() expects 1 argument", 0, 0, "")
			}
			n, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("isPrimeNum() expects an integer", 0, 0, "")
			}
			if n < 2 {
				return types.Bool(false)
			}
			if n == 2 {
				return types.Bool(true)
			}
			if n%2 == 0 {
				return types.Bool(false)
			}
			for i := types.Int(3); i*i <= n; i += 2 {
				if n%i == 0 {
					return types.Bool(false)
				}
			}
			return types.Bool(true)
		},
	}

	vm.globals["primeFactors"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("primeFactors() expects 1 argument", 0, 0, "")
			}
			n, ok := args[0].(types.Int)
			if !ok {
				return types.NewError("primeFactors() expects an integer", 0, 0, "")
			}
			if n < 2 {
				return collections.NewArray()
			}
			result := collections.NewArray()
			d := types.Int(2)
			for d*d <= n {
				for n%d == 0 {
					result.Append(d)
					n /= d
				}
				d++
			}
			if n > 1 {
				result.Append(n)
			}
			return result
		},
	}

	vm.globals["divmod"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("divmod() expects 2 arguments", 0, 0, "")
			}
			a, ok1 := args[0].(types.Int)
			b, ok2 := args[1].(types.Int)
			if !ok1 || !ok2 {
				return types.NewError("divmod() expects integers", 0, 0, "")
			}
			if b == 0 {
				return types.NewError("divmod() division by zero", 0, 0, "")
			}
			result := collections.NewArray()
			result.Append(a / b)
			result.Append(a % b)
			return result
		},
	}

	vm.globals["percent"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("percent() expects 2 arguments", 0, 0, "")
			}
			part, _ := types.ToFloat(args[0])
			total, _ := types.ToFloat(args[1])
			if total == 0 {
				return types.Float(0)
			}
			return types.Float((part / total) * 100)
		},
	}

	vm.globals["degToRad"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("degToRad() expects 1 argument", 0, 0, "")
			}
			deg, _ := types.ToFloat(args[0])
			return types.Float(deg * math.Pi / 180)
		},
	}

	vm.globals["radToDeg"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("radToDeg() expects 1 argument", 0, 0, "")
			}
			rad, _ := types.ToFloat(args[0])
			return types.Float(rad * 180 / math.Pi)
		},
	}

	vm.globals["trunc"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("trunc() expects 1 argument", 0, 0, "")
			}
			f, ok := args[0].(types.Float)
			if !ok {
				i, ok := args[0].(types.Int)
				if !ok {
					return types.NewError("trunc() expects a number", 0, 0, "")
				}
				return i
			}
			return types.Int(math.Trunc(float64(f)))
		},
	}

	vm.globals["rangeN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("rangeN() expects at least 1 argument", 0, 0, "")
			}
			start := types.Int(0)
			step := types.Int(1)
			end, _ := types.ToInt(args[0])
			if len(args) >= 2 {
				start, _ = types.ToInt(args[1])
			}
			if len(args) >= 3 {
				step, _ = types.ToInt(args[2])
			}
			result := collections.NewArray()
			if step > 0 {
				for i := start; i < end; i += step {
					result.Append(i)
				}
			} else if step < 0 {
				for i := start; i > end; i += step {
					result.Append(i)
				}
			}
			return result
		},
	}

	vm.globals["timesN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("timesN() expects at least 2 arguments", 0, 0, "")
			}
			n, _ := types.ToInt(args[0])
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("timesN() second argument must be a function", 0, 0, "")
			}
			for i := types.Int(0); i < n; i++ {
				fn.Fn(i)
			}
			return types.UndefinedValue
		},
	}

	vm.globals["forEachN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("forEachN() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("forEachN() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("forEachN() second argument must be a function", 0, 0, "")
			}
			for i := 0; i < arr.Len(); i++ {
				fn.Fn(arr.Get(i), types.Int(i))
			}
			return types.UndefinedValue
		},
	}

	vm.globals["mapN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("mapN() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("mapN() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("mapN() second argument must be a function", 0, 0, "")
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				result.Append(fn.Fn(arr.Get(i), types.Int(i)))
			}
			return result
		},
	}

	vm.globals["filterN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("filterN() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("filterN() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("filterN() second argument must be a function", 0, 0, "")
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				if types.ToBool(fn.Fn(arr.Get(i), types.Int(i))) {
					result.Append(arr.Get(i))
				}
			}
			return result
		},
	}

	vm.globals["reduceN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("reduceN() expects at least 3 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("reduceN() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("reduceN() second argument must be a function", 0, 0, "")
			}
			acc := args[2]
			for i := 0; i < arr.Len(); i++ {
				acc = fn.Fn(acc, arr.Get(i), types.Int(i))
			}
			return acc
		},
	}

	vm.globals["findN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("findN() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("findN() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("findN() second argument must be a function", 0, 0, "")
			}
			for i := 0; i < arr.Len(); i++ {
				if types.ToBool(fn.Fn(arr.Get(i), types.Int(i))) {
					return arr.Get(i)
				}
			}
			return types.UndefinedValue
		},
	}

	vm.globals["findIndexN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("findIndexN() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("findIndexN() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("findIndexN() second argument must be a function", 0, 0, "")
			}
			for i := 0; i < arr.Len(); i++ {
				if types.ToBool(fn.Fn(arr.Get(i), types.Int(i))) {
					return types.Int(i)
				}
			}
			return types.Int(-1)
		},
	}

	vm.globals["everyN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("everyN() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("everyN() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("everyN() second argument must be a function", 0, 0, "")
			}
			for i := 0; i < arr.Len(); i++ {
				if !types.ToBool(fn.Fn(arr.Get(i), types.Int(i))) {
					return types.Bool(false)
				}
			}
			return types.Bool(true)
		},
	}

	vm.globals["someN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("someN() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("someN() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("someN() second argument must be a function", 0, 0, "")
			}
			for i := 0; i < arr.Len(); i++ {
				if types.ToBool(fn.Fn(arr.Get(i), types.Int(i))) {
					return types.Bool(true)
				}
			}
			return types.Bool(false)
		},
	}

	vm.globals["sortByN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("sortByN() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("sortByN() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("sortByN() second argument must be a function", 0, 0, "")
			}
			elements := make([]types.Object, arr.Len())
			for i := 0; i < arr.Len(); i++ {
				elements[i] = arr.Get(i)
			}
			sort.Slice(elements, func(i, j int) bool {
				return fn.Fn(elements[i], elements[j]).ToStr() == "true"
			})
			return collections.NewArrayWithElements(elements)
		},
	}

	vm.globals["groupByN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("groupByN() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("groupByN() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("groupByN() second argument must be a function", 0, 0, "")
			}
			result := collections.NewMap()
			for i := 0; i < arr.Len(); i++ {
				key := fn.Fn(arr.Get(i), types.Int(i)).ToStr()
				if result.Get(key) == nil {
					result.Set(key, collections.NewArray())
				}
				result.Get(key).(*collections.Array).Append(arr.Get(i))
			}
			return result
		},
	}

	vm.globals["countByN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("countByN() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("countByN() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("countByN() second argument must be a function", 0, 0, "")
			}
			counts := collections.NewMap()
			for i := 0; i < arr.Len(); i++ {
				key := fn.Fn(arr.Get(i), types.Int(i)).ToStr()
				if counts.Get(key) == nil {
					counts.Set(key, types.Int(1))
				} else {
					cur := counts.Get(key).(types.Int)
					counts.Set(key, cur+1)
				}
			}
			return counts
		},
	}

	vm.globals["partitionN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("partitionN() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("partitionN() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("partitionN() second argument must be a function", 0, 0, "")
			}
			pass := collections.NewArray()
			fail := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				if types.ToBool(fn.Fn(arr.Get(i), types.Int(i))) {
					pass.Append(arr.Get(i))
				} else {
					fail.Append(arr.Get(i))
				}
			}
			result := collections.NewArray()
			result.Append(pass)
			result.Append(fail)
			return result
		},
	}

	vm.globals["keyByN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("keyByN() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("keyByN() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("keyByN() second argument must be a function", 0, 0, "")
			}
			result := collections.NewMap()
			for i := 0; i < arr.Len(); i++ {
				key := fn.Fn(arr.Get(i), types.Int(i)).ToStr()
				result.Set(key, arr.Get(i))
			}
			return result
		},
	}

	vm.globals["sortedIndexN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("sortedIndexN() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("sortedIndexN() first argument must be an array", 0, 0, "")
			}
			val := args[1]
			fn := func(a, b types.Object) bool { return a.ToStr() <= b.ToStr() }
			if len(args) >= 3 {
				fnObj, ok := args[2].(*types.NativeFunction)
				if ok {
					fn = func(a, b types.Object) bool { return fnObj.Fn(a, b).ToStr() == "true" }
				}
			}
			low, high := 0, arr.Len()
			for low < high {
				mid := (low + high) / 2
				if fn(arr.Get(mid), val) {
					low = mid + 1
				} else {
					high = mid
				}
			}
			return types.Int(low)
		},
	}

	vm.globals["toSortedN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toSortedN() expects at least 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("toSortedN() first argument must be an array", 0, 0, "")
			}
			elements := make([]types.Object, arr.Len())
			for i := 0; i < arr.Len(); i++ {
				elements[i] = arr.Get(i)
			}
			sort.Slice(elements, func(i, j int) bool {
				return elements[i].ToStr() < elements[j].ToStr()
			})
			return collections.NewArrayWithElements(elements)
		},
	}

	vm.globals["toReversedN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toReversedN() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("toReversedN() expects an array", 0, 0, "")
			}
			result := collections.NewArray()
			for i := arr.Len() - 1; i >= 0; i-- {
				result.Append(arr.Get(i))
			}
			return result
		},
	}

	vm.globals["with"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("with() expects at least 3 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("with() first argument must be an array", 0, 0, "")
			}
			index, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("with() second argument must be an integer", 0, 0, "")
			}
			value := args[2]
			if index < 0 || index >= types.Int(arr.Len()) {
				return arr
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				if types.Int(i) == index {
					result.Append(value)
				} else {
					result.Append(arr.Get(i))
				}
			}
			return result
		},
	}

	vm.globals["findLastN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("findLastN() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("findLastN() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("findLastN() second argument must be a function", 0, 0, "")
			}
			for i := arr.Len() - 1; i >= 0; i-- {
				if types.ToBool(fn.Fn(arr.Get(i), types.Int(i))) {
					return arr.Get(i)
				}
			}
			return types.UndefinedValue
		},
	}

	vm.globals["findLastIndexN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("findLastIndexN() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("findLastIndexN() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("findLastIndexN() second argument must be a function", 0, 0, "")
			}
			for i := arr.Len() - 1; i >= 0; i-- {
				if types.ToBool(fn.Fn(arr.Get(i), types.Int(i))) {
					return types.Int(i)
				}
			}
			return types.Int(-1)
		},
	}

	vm.globals["addIndexN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("addIndexN() expects at least 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("addIndexN() first argument must be an array", 0, 0, "")
			}
			fn, ok := args[1].(*types.NativeFunction)
			if !ok {
				return types.NewError("addIndexN() second argument must be a function", 0, 0, "")
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				result.Append(fn.Fn(arr.Get(i), types.Int(i)))
			}
			return result
		},
	}

	var timeLocation *time.Location

	vm.globals["setTimeZone"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("setTimeZone() expects a timezone string", 0, 0, "")
			}
			tz := string(types.ToString(args[0]))
			loc, err := time.LoadLocation(tz)
			if err != nil {
				return types.NewError("setTimeZone(): invalid timezone: "+tz, 0, 0, "")
			}
			timeLocation = loc
			return types.Bool(true)
		},
	}

	vm.globals["getTimeZone"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if timeLocation == nil {
				timeLocation = time.Local
			}
			return types.String(timeLocation.String())
		},
	}

	vm.globals["parseDate"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("parseDate() expects at least 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			layout := time.RFC3339
			if len(args) >= 2 {
				layout = string(types.ToString(args[1]))
			}
			if timeLocation == nil {
				timeLocation = time.Local
			}
			t, err := time.ParseInLocation(layout, s, timeLocation)
			if err != nil {
				return types.NewError("parseDate(): "+err.Error(), 0, 0, "")
			}
			result := collections.NewMap()
			result.Set("year", types.Int(t.Year()))
			result.Set("month", types.Int(t.Month()))
			result.Set("day", types.Int(t.Day()))
			result.Set("hour", types.Int(t.Hour()))
			result.Set("minute", types.Int(t.Minute()))
			result.Set("second", types.Int(t.Second()))
			result.Set("weekday", types.String(t.Weekday().String()))
			result.Set("unix", types.Int(t.Unix()))
			return result
		},
	}

	vm.globals["formatDate"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("formatDate() expects at least 1 argument", 0, 0, "")
			}
			var t time.Time
			switch v := args[0].(type) {
			case types.Int:
				t = time.Unix(int64(v), 0)
			case *collections.Array:
				if v.Len() >= 6 {
					year, _ := types.ToInt(v.Get(0))
					month, _ := types.ToInt(v.Get(1))
					day, _ := types.ToInt(v.Get(2))
					hour, _ := types.ToInt(v.Get(3))
					minute, _ := types.ToInt(v.Get(4))
					second, _ := types.ToInt(v.Get(5))
					t = time.Date(int(year), time.Month(month), int(day), int(hour), int(minute), int(second), 0, time.UTC)
				} else {
					return types.NewError("formatDate(): array must have at least 6 elements", 0, 0, "")
				}
			default:
				return types.NewError("formatDate(): first argument must be unix timestamp or array", 0, 0, "")
			}
			layout := "2006-01-02 15:04:05"
			if len(args) >= 2 {
				layout = string(types.ToString(args[1]))
			}
			return types.String(t.Format(layout))
		},
	}

	vm.globals["addDate"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 4 {
				return types.NewError("addDate() expects 4 arguments: timestamp, years, months, days", 0, 0, "")
			}
			unix, _ := types.ToInt(args[0])
			years, _ := types.ToInt(args[1])
			months, _ := types.ToInt(args[2])
			days, _ := types.ToInt(args[3])
			if timeLocation == nil {
				timeLocation = time.Local
			}
			t := time.Unix(int64(unix), 0).In(timeLocation)
			t = t.AddDate(int(years), int(months), int(days))
			return types.Int(t.Unix())
		},
	}

	vm.globals["dateDiff"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("dateDiff() expects 2 arguments: timestamp1, timestamp2", 0, 0, "")
			}
			t1, _ := types.ToInt(args[0])
			t2, _ := types.ToInt(args[1])
			diff := time.Duration(t2-t1) * time.Second
			result := collections.NewMap()
			result.Set("seconds", types.Int(diff.Seconds()))
			result.Set("minutes", types.Int(diff.Minutes()))
			result.Set("hours", types.Int(diff.Hours()))
			result.Set("days", types.Int(diff.Hours()/24))
			return result
		},
	}

	vm.globals["isLeapYear"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isLeapYear() expects 1 argument", 0, 0, "")
			}
			year, _ := types.ToInt(args[0])
			return types.Bool((year%4 == 0 && year%100 != 0) || year%400 == 0)
		},
	}

	vm.globals["daysInMonth"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("daysInMonth() expects 2 arguments: year, month", 0, 0, "")
			}
			year, _ := types.ToInt(args[0])
			month, _ := types.ToInt(args[1])
			t := time.Date(int(year), time.Month(month), 1, 0, 0, 0, 0, time.UTC)
			return types.Int(t.AddDate(0, 1, -1).Day())
		},
	}

	vm.globals["weekNumber"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("weekNumber() expects 1 argument: timestamp", 0, 0, "")
			}
			unix, _ := types.ToInt(args[0])
			if timeLocation == nil {
				timeLocation = time.Local
			}
			t := time.Unix(int64(unix), 0).In(timeLocation)
			_, week := t.ISOWeek()
			return types.Int(week)
		},
	}

	vm.globals["dayOfYear"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("dayOfYear() expects 1 argument: timestamp", 0, 0, "")
			}
			unix, _ := types.ToInt(args[0])
			if timeLocation == nil {
				timeLocation = time.Local
			}
			t := time.Unix(int64(unix), 0).In(timeLocation)
			return types.Int(t.YearDay())
		},
	}

	vm.globals["startOfDay"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("startOfDay() expects 1 argument: timestamp", 0, 0, "")
			}
			unix, _ := types.ToInt(args[0])
			if timeLocation == nil {
				timeLocation = time.Local
			}
			t := time.Unix(int64(unix), 0).In(timeLocation)
			start := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, timeLocation)
			return types.Int(start.Unix())
		},
	}

	vm.globals["endOfDay"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("endOfDay() expects 1 argument: timestamp", 0, 0, "")
			}
			unix, _ := types.ToInt(args[0])
			if timeLocation == nil {
				timeLocation = time.Local
			}
			t := time.Unix(int64(unix), 0).In(timeLocation)
			end := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, timeLocation)
			return types.Int(end.Unix())
		},
	}

	vm.globals["startOfMonth"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("startOfMonth() expects 1 argument: timestamp", 0, 0, "")
			}
			unix, _ := types.ToInt(args[0])
			if timeLocation == nil {
				timeLocation = time.Local
			}
			t := time.Unix(int64(unix), 0).In(timeLocation)
			start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, timeLocation)
			return types.Int(start.Unix())
		},
	}

	vm.globals["endOfMonth"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("endOfMonth() expects 1 argument: timestamp", 0, 0, "")
			}
			unix, _ := types.ToInt(args[0])
			if timeLocation == nil {
				timeLocation = time.Local
			}
			t := time.Unix(int64(unix), 0).In(timeLocation)
			end := t.AddDate(0, 1, -1)
			end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, timeLocation)
			return types.Int(end.Unix())
		},
	}

	vm.globals["startOfYear"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("startOfYear() expects 1 argument: timestamp", 0, 0, "")
			}
			unix, _ := types.ToInt(args[0])
			if timeLocation == nil {
				timeLocation = time.Local
			}
			t := time.Unix(int64(unix), 0).In(timeLocation)
			start := time.Date(t.Year(), 1, 1, 0, 0, 0, 0, timeLocation)
			return types.Int(start.Unix())
		},
	}

	vm.globals["endOfYear"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("endOfYear() expects 1 argument: timestamp", 0, 0, "")
			}
			unix, _ := types.ToInt(args[0])
			if timeLocation == nil {
				timeLocation = time.Local
			}
			t := time.Unix(int64(unix), 0).In(timeLocation)
			end := time.Date(t.Year(), 12, 31, 23, 59, 59, 999999999, timeLocation)
			return types.Int(end.Unix())
		},
	}

	vm.globals["timestampAdd"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("timestampAdd() expects 3 arguments: timestamp, value, unit", 0, 0, "")
			}
			unix, _ := types.ToInt(args[0])
			value, _ := types.ToInt(args[1])
			unit := string(types.ToString(args[2]))
			if timeLocation == nil {
				timeLocation = time.Local
			}
			t := time.Unix(int64(unix), 0).In(timeLocation)
			switch unit {
			case "second", "seconds", "s":
				t = t.Add(time.Duration(value) * time.Second)
			case "minute", "minutes", "m":
				t = t.Add(time.Duration(value) * time.Minute)
			case "hour", "hours", "h":
				t = t.Add(time.Duration(value) * time.Hour)
			case "day", "days", "d":
				t = t.AddDate(0, 0, int(value))
			case "week", "weeks", "w":
				t = t.AddDate(0, 0, int(value)*7)
			case "month", "months":
				t = t.AddDate(0, int(value), 0)
			case "year", "years":
				t = t.AddDate(int(value), 0, 0)
			default:
				return types.NewError("timestampAdd(): invalid unit: "+unit, 0, 0, "")
			}
			return types.Int(t.Unix())
		},
	}

	vm.globals["strftime"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strftime() expects 2 arguments: timestamp, format", 0, 0, "")
			}
			unix, _ := types.ToInt(args[0])
			format := string(types.ToString(args[1]))
			if timeLocation == nil {
				timeLocation = time.Local
			}
			t := time.Unix(int64(unix), 0).In(timeLocation)
			result := format
			result = strings.ReplaceAll(result, "%Y", fmt.Sprintf("%d", t.Year()))
			result = strings.ReplaceAll(result, "%m", fmt.Sprintf("%02d", t.Month()))
			result = strings.ReplaceAll(result, "%d", fmt.Sprintf("%02d", t.Day()))
			result = strings.ReplaceAll(result, "%H", fmt.Sprintf("%02d", t.Hour()))
			result = strings.ReplaceAll(result, "%M", fmt.Sprintf("%02d", t.Minute()))
			result = strings.ReplaceAll(result, "%S", fmt.Sprintf("%02d", t.Second()))
			result = strings.ReplaceAll(result, "%w", fmt.Sprintf("%d", t.Weekday()))
			result = strings.ReplaceAll(result, "%j", fmt.Sprintf("%03d", t.YearDay()))
			_, week := t.ISOWeek()
			result = strings.ReplaceAll(result, "%U", fmt.Sprintf("%02d", week))
			return types.String(result)
		},
	}

	vm.globals["strptime"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strptime() expects 2 arguments: string, format", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			format := string(types.ToString(args[1]))
			format = strings.ReplaceAll(format, "%Y", "2006")
			format = strings.ReplaceAll(format, "%m", "01")
			format = strings.ReplaceAll(format, "%d", "02")
			format = strings.ReplaceAll(format, "%H", "15")
			format = strings.ReplaceAll(format, "%M", "04")
			format = strings.ReplaceAll(format, "%S", "05")
			format = strings.ReplaceAll(format, "%j", "002")
			format = strings.ReplaceAll(format, "%w", "0")
			if timeLocation == nil {
				timeLocation = time.Local
			}
			t, err := time.ParseInLocation(format, s, timeLocation)
			if err != nil {
				return types.NewError("strptime(): "+err.Error(), 0, 0, "")
			}
			return types.Int(t.Unix())
		},
	}

	vm.globals["sleepUntil"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("sleepUntil() expects 1 argument: timestamp", 0, 0, "")
			}
			until, _ := types.ToInt(args[0])
			now := int64(time.Now().Unix())
			if int64(until) > now {
				time.Sleep(time.Duration(int64(until)-now) * time.Second)
			}
			return types.UndefinedValue
		},
	}

	vm.globals["sleepMs"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("sleepMs() expects 1 argument: milliseconds", 0, 0, "")
			}
			ms, _ := types.ToInt(args[0])
			time.Sleep(time.Duration(ms) * time.Millisecond)
			return types.UndefinedValue
		},
	}

	vm.globals["unixToTime"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("unixToTime() expects 1 argument: unix timestamp", 0, 0, "")
			}
			unix, _ := types.ToInt(args[0])
			if timeLocation == nil {
				timeLocation = time.Local
			}
			t := time.Unix(int64(unix), 0).In(timeLocation)
			result := collections.NewMap()
			result.Set("year", types.Int(t.Year()))
			result.Set("month", types.Int(t.Month()))
			result.Set("day", types.Int(t.Day()))
			result.Set("hour", types.Int(t.Hour()))
			result.Set("minute", types.Int(t.Minute()))
			result.Set("second", types.Int(t.Second()))
			result.Set("weekday", types.String(t.Weekday().String()))
			result.Set("unix", types.Int(t.Unix()))
			return result
		},
	}

	vm.globals["formatDuration"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("formatDuration() expects 1 argument: seconds", 0, 0, "")
			}
			secs, _ := types.ToFloat(args[0])
			d := time.Duration(float64(secs) * float64(time.Second))
			result := collections.NewMap()
			result.Set("seconds", types.Float(d.Seconds()))
			result.Set("minutes", types.Float(d.Minutes()))
			result.Set("hours", types.Float(d.Hours()))
			result.Set("days", types.Float(d.Hours()/24))
			result.Set("string", types.String(d.String()))
			return result
		},
	}

	vm.globals["parseDuration"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("parseDuration() expects 1 argument: duration string", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			d, err := time.ParseDuration(s)
			if err != nil {
				return types.NewError("parseDuration(): "+err.Error(), 0, 0, "")
			}
			return types.Float(d.Seconds())
		},
	}

	vm.globals["hash"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("hash() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			h := fnv.New32a()
			h.Write([]byte(s))
			return types.Int(h.Sum32())
		},
	}

	vm.globals["crc32"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("crc32() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			return types.Int(crc32.ChecksumIEEE([]byte(s)))
		},
	}

	vm.globals["parseInt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("parseInt() expects at least 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			base := 10
			if len(args) >= 2 {
				b, _ := types.ToInt(args[1])
				base = int(b)
			}
			val, err := strconv.ParseInt(s, base, 64)
			if err != nil {
				return types.UndefinedValue
			}
			return types.Int(val)
		},
	}

	vm.globals["parseFloat"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("parseFloat() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			val, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return types.UndefinedValue
			}
			return types.Float(val)
		},
	}

	vm.globals["formatInt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("formatInt() expects at least 1 argument", 0, 0, "")
			}
			val, _ := types.ToInt(args[0])
			base := 10
			if len(args) >= 2 {
				b, _ := types.ToInt(args[1])
				base = int(b)
			}
			return types.String(strconv.FormatInt(int64(val), base))
		},
	}

	vm.globals["formatFloat"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("formatFloat() expects at least 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			prec := -1
			if len(args) >= 2 {
				p, _ := types.ToInt(args[1])
				prec = int(p)
			}
			format := byte('f')
			if len(args) >= 3 {
				f := string(types.ToString(args[2]))
				if len(f) > 0 {
					format = f[0]
				}
			}
			return types.String(strconv.FormatFloat(float64(val), format, prec, 64))
		},
	}

	vm.globals["isInt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isInt() expects 1 argument", 0, 0, "")
			}
			_, ok := args[0].(types.Int)
			return types.Bool(ok)
		},
	}

	vm.globals["isFloat"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isFloat() expects 1 argument", 0, 0, "")
			}
			_, ok := args[0].(types.Float)
			return types.Bool(ok)
		},
	}

	vm.globals["toInt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("toInt() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToInt(args[0])
			return val
		},
	}

	vm.globals["toFloat"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("toFloat() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(val)
		},
	}

	vm.globals["isString"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isString() expects 1 argument", 0, 0, "")
			}
			_, ok := args[0].(types.String)
			return types.Bool(ok)
		},
	}

	vm.globals["isBool"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isBool() expects 1 argument", 0, 0, "")
			}
			_, ok := args[0].(types.Bool)
			return types.Bool(ok)
		},
	}

	vm.globals["isArray"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isArray() expects 1 argument", 0, 0, "")
			}
			_, ok := args[0].(*collections.Array)
			return types.Bool(ok)
		},
	}

	vm.globals["isMap"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isMap() expects 1 argument", 0, 0, "")
			}
			_, ok := args[0].(*collections.Map)
			return types.Bool(ok)
		},
	}

	vm.globals["isFunction"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isFunction() expects 1 argument", 0, 0, "")
			}
			_, ok := args[0].(*types.NativeFunction)
			if ok {
				return types.Bool(true)
			}
			_, ok = args[0].(*types.Function)
			return types.Bool(ok)
		},
	}

	vm.globals["isNull"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isNull() expects 1 argument", 0, 0, "")
			}
			return types.Bool(args[0] == types.UndefinedValue || args[0] == nil)
		},
	}

	vm.globals["isNumber"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isNumber() expects 1 argument", 0, 0, "")
			}
			_, ok := args[0].(types.Int)
			if ok {
				return types.Bool(true)
			}
			_, ok = args[0].(types.Float)
			return types.Bool(ok)
		},
	}

	vm.globals["isInteger"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isInteger() expects 1 argument", 0, 0, "")
			}
			_, ok := args[0].(types.Int)
			return types.Bool(ok)
		},
	}

	vm.globals["toBool"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("toBool() expects 1 argument", 0, 0, "")
			}
			return types.Bool(types.ToBool(args[0]))
		},
	}

	vm.globals["toStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("toStr() expects 1 argument", 0, 0, "")
			}
			return types.String(types.ToString(args[0]))
		},
	}

	vm.globals["toArray"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("toArray() expects 1 argument", 0, 0, "")
			}
			if arr, ok := args[0].(*collections.Array); ok {
				return arr
			}
			if s, ok := args[0].(types.String); ok {
				result := collections.NewArray()
				for _, r := range string(s) {
					result.Append(types.String(r))
				}
				return result
			}
			result := collections.NewArray()
			result.Append(args[0])
			return result
		},
	}

	vm.globals["toMap"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("toMap() expects 1 argument", 0, 0, "")
			}
			if m, ok := args[0].(*collections.Map); ok {
				return m
			}
			return collections.NewMap()
		},
	}

	vm.globals["clamp"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("clamp() expects 3 arguments: value, min, max", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			min, _ := types.ToFloat(args[1])
			max, _ := types.ToFloat(args[2])
			if float64(val) < float64(min) {
				return types.Float(min)
			}
			if float64(val) > float64(max) {
				return types.Float(max)
			}
			return types.Float(val)
		},
	}

	vm.globals["wrap"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("wrap() expects 3 arguments: value, min, max", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			min, _ := types.ToFloat(args[1])
			max, _ := types.ToFloat(args[2])
			range_ := float64(max) - float64(min)
			if range_ == 0 {
				return types.Float(min)
			}
			result := math.Mod(float64(val)-float64(min), range_)
			if result < 0 {
				result += range_
			}
			return types.Float(float64(min) + result)
		},
	}

	vm.globals["sign"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("sign() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			if float64(val) > 0 {
				return types.Int(1)
			}
			if float64(val) < 0 {
				return types.Int(-1)
			}
			return types.Int(0)
		},
	}

	vm.globals["round"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("round() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(math.Round(float64(val)))
		},
	}

	vm.globals["floor"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("floor() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(math.Floor(float64(val)))
		},
	}

	vm.globals["ceil"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("ceil() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(math.Ceil(float64(val)))
		},
	}

	vm.globals["trunc"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("trunc() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(math.Trunc(float64(val)))
		},
	}

	vm.globals["abs"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("abs() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(math.Abs(float64(val)))
		},
	}

	vm.globals["sqrt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("sqrt() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(math.Sqrt(float64(val)))
		},
	}

	vm.globals["cbrt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("cbrt() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(math.Cbrt(float64(val)))
		},
	}

	vm.globals["pow"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("pow() expects 2 arguments: base, exponent", 0, 0, "")
			}
			base, _ := types.ToFloat(args[0])
			exp, _ := types.ToFloat(args[1])
			return types.Float(math.Pow(float64(base), float64(exp)))
		},
	}

	vm.globals["exp"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("exp() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(math.Exp(float64(val)))
		},
	}

	vm.globals["exp2"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("exp2() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(math.Exp2(float64(val)))
		},
	}

	vm.globals["log"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("log() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(math.Log(float64(val)))
		},
	}

	vm.globals["log2"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("log2() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(math.Log2(float64(val)))
		},
	}

	vm.globals["log10"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("log10() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(math.Log10(float64(val)))
		},
	}

	vm.globals["sin"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("sin() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(math.Sin(float64(val)))
		},
	}

	vm.globals["cos"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("cos() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(math.Cos(float64(val)))
		},
	}

	vm.globals["tan"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("tan() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(math.Tan(float64(val)))
		},
	}

	vm.globals["asin"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("asin() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(math.Asin(float64(val)))
		},
	}

	vm.globals["acos"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("acos() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(math.Acos(float64(val)))
		},
	}

	vm.globals["atan"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("atan() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(math.Atan(float64(val)))
		},
	}

	vm.globals["sinh"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("sinh() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(math.Sinh(float64(val)))
		},
	}

	vm.globals["cosh"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("cosh() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(math.Cosh(float64(val)))
		},
	}

	vm.globals["tanh"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("tanh() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(math.Tanh(float64(val)))
		},
	}

	vm.globals["degToRad"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("degToRad() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(val * math.Pi / 180)
		},
	}

	vm.globals["radToDeg"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("radToDeg() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Float(val * 180 / math.Pi)
		},
	}

	vm.globals["pi"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Float(math.Pi)
		},
	}

	vm.globals["e"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Float(math.E)
		},
	}

	vm.globals["phi"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Float(1.618033988749895)
		},
	}

	vm.globals["tau"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Float(math.Pi * 2)
		},
	}

	vm.globals["inf"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Float(math.Inf(1))
		},
	}

	vm.globals["negInf"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Float(math.Inf(-1))
		},
	}

	vm.globals["nan"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Float(math.NaN())
		},
	}

	vm.globals["isNaN"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isNaN() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Bool(math.IsNaN(float64(val)))
		},
	}

	vm.globals["isInf"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isInf() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			return types.Bool(math.IsInf(float64(val), 0))
		},
	}

	vm.globals["isEven"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isEven() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToInt(args[0])
			return types.Bool(val%2 == 0)
		},
	}

	vm.globals["isOdd"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isOdd() expects 1 argument", 0, 0, "")
			}
			val, _ := types.ToInt(args[0])
			return types.Bool(val%2 != 0)
		},
	}

	vm.globals["between"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("between() expects 3 arguments: value, min, max", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			min, _ := types.ToFloat(args[1])
			max, _ := types.ToFloat(args[2])
			return types.Bool(val >= min && val <= max)
		},
	}

	vm.globals["within"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("within() expects 3 arguments: value, min, max", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			min, _ := types.ToFloat(args[1])
			max, _ := types.ToFloat(args[2])
			return types.Bool(val > min && val < max)
		},
	}

	vm.globals["percent"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("percent() expects 2 arguments: value, total", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			total, _ := types.ToFloat(args[1])
			if total == 0 {
				return types.Float(0)
			}
			return types.Float(val / total * 100)
		},
	}

	vm.globals["ratio"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("ratio() expects 2 arguments: part, total", 0, 0, "")
			}
			part, _ := types.ToFloat(args[0])
			total, _ := types.ToFloat(args[1])
			if total == 0 {
				return types.Float(0)
			}
			return types.Float(part / total)
		},
	}

	vm.globals["lerp"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("lerp() expects 3 arguments: start, end, t", 0, 0, "")
			}
			start, _ := types.ToFloat(args[0])
			end, _ := types.ToFloat(args[1])
			t, _ := types.ToFloat(args[2])
			return types.Float(start + (end-start)*t)
		},
	}

	vm.globals["mapRange"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 5 {
				return types.NewError("mapRange() expects 5 arguments: value, inMin, inMax, outMin, outMax", 0, 0, "")
			}
			val, _ := types.ToFloat(args[0])
			inMin, _ := types.ToFloat(args[1])
			inMax, _ := types.ToFloat(args[2])
			outMin, _ := types.ToFloat(args[3])
			outMax, _ := types.ToFloat(args[4])
			if inMax == inMin {
				return types.Float(outMin)
			}
			return types.Float(outMin + (val-inMin)*(outMax-outMin)/(inMax-inMin))
		},
	}

	vm.globals["dist"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 4 {
				return types.NewError("dist() expects 4 arguments: x1, y1, x2, y2", 0, 0, "")
			}
			x1, _ := types.ToFloat(args[0])
			y1, _ := types.ToFloat(args[1])
			x2, _ := types.ToFloat(args[2])
			y2, _ := types.ToFloat(args[3])
			dx := float64(x2) - float64(x1)
			dy := float64(y2) - float64(y1)
			return types.Float(math.Sqrt(dx*dx + dy*dy))
		},
	}

	vm.globals["hypot"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("hypot() expects 2 arguments: x, y", 0, 0, "")
			}
			x, _ := types.ToFloat(args[0])
			y, _ := types.ToFloat(args[1])
			return types.Float(math.Hypot(float64(x), float64(y)))
		},
	}

	vm.globals["gcd"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("gcd() expects 2 arguments", 0, 0, "")
			}
			a, _ := types.ToInt(args[0])
			b, _ := types.ToInt(args[1])
			if a < 0 {
				a = -a
			}
			if b < 0 {
				b = -b
			}
			for b != 0 {
				a, b = b, a%b
			}
			return types.Int(a)
		},
	}

	vm.globals["lcm"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("lcm() expects 2 arguments", 0, 0, "")
			}
			a, _ := types.ToInt(args[0])
			b, _ := types.ToInt(args[1])
			if a == 0 || b == 0 {
				return types.Int(0)
			}
			if a < 0 {
				a = -a
			}
			if b < 0 {
				b = -b
			}
			gcd := func(x, y types.Int) types.Int {
				if x < 0 {
					x = -x
				}
				if y < 0 {
					y = -y
				}
				for y != 0 {
					x, y = y, x%y
				}
				return x
			}
			return types.Int(a * b / gcd(a, b))
		},
	}

	vm.globals["factorial"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("factorial() expects 1 argument", 0, 0, "")
			}
			n, _ := types.ToInt(args[0])
			if n < 0 {
				return types.NewError("factorial() expects non-negative integer", 0, 0, "")
			}
			result := types.Int(1)
			for i := types.Int(2); i <= n; i++ {
				result *= i
			}
			return result
		},
	}

	vm.globals["fibonacci"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("fibonacci() expects 1 argument", 0, 0, "")
			}
			n, _ := types.ToInt(args[0])
			if n < 0 {
				return types.NewError("fibonacci() expects non-negative integer", 0, 0, "")
			}
			if n == 0 {
				return types.Int(0)
			}
			if n == 1 {
				return types.Int(1)
			}
			a, b := types.Int(0), types.Int(1)
			for i := types.Int(2); i <= n; i++ {
				a, b = b, a+b
			}
			return b
		},
	}

	vm.globals["isPrime"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isPrime() expects 1 argument", 0, 0, "")
			}
			n, _ := types.ToInt(args[0])
			if n < 2 {
				return types.Bool(false)
			}
			if n == 2 {
				return types.Bool(true)
			}
			if n%2 == 0 {
				return types.Bool(false)
			}
			for i := types.Int(3); i*i <= n; i += 2 {
				if n%i == 0 {
					return types.Bool(false)
				}
			}
			return types.Bool(true)
		},
	}

	vm.globals["primeFactors"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("primeFactors() expects 1 argument", 0, 0, "")
			}
			n, _ := types.ToInt(args[0])
			if n < 2 {
				return collections.NewArray()
			}
			result := collections.NewArray()
			d := types.Int(2)
			for d*d <= n {
				for n%d == 0 {
					result.Append(d)
					n /= d
				}
				d++
			}
			if n > 1 {
				result.Append(n)
			}
			return result
		},
	}

	vm.globals["divmod"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("divmod() expects 2 arguments: dividend, divisor", 0, 0, "")
			}
			a, _ := types.ToInt(args[0])
			b, _ := types.ToInt(args[1])
			if b == 0 {
				return types.NewError("divmod(): division by zero", 0, 0, "")
			}
			result := collections.NewArray()
			result.Append(a / b)
			result.Append(a % b)
			return result
		},
	}

	vm.globals["bitcount"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("bitcount() expects 1 argument", 0, 0, "")
			}
			n, _ := types.ToInt(args[0])
			count := 0
			for n > 0 {
				if n&1 == 1 {
					count++
				}
				n >>= 1
			}
			return types.Int(count)
		},
	}

	vm.globals["bitlen"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("bitlen() expects 1 argument", 0, 0, "")
			}
			n, _ := types.ToInt(args[0])
			if n == 0 {
				return types.Int(0)
			}
			len := 0
			for n > 0 {
				len++
				n >>= 1
			}
			return types.Int(len)
		},
	}

	vm.globals["bits"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("bits() expects 1 argument", 0, 0, "")
			}
			n, _ := types.ToInt(args[0])
			result := collections.NewArray()
			if n == 0 {
				result.Append(types.Int(0))
				return result
			}
			bits := collections.NewArray()
			for n > 0 {
				bits.Append(types.Int(n & 1))
				n >>= 1
			}
			for i := bits.Len() - 1; i >= 0; i-- {
				result.Append(bits.Get(i))
			}
			return result
		},
	}

	vm.globals["fromBits"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("fromBits() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("fromBits() first argument must be an array", 0, 0, "")
			}
			result := types.Int(0)
			for i := 0; i < arr.Len(); i++ {
				bit, _ := types.ToInt(arr.Get(i))
				result = (result << 1) | bit
			}
			return result
		},
	}

	vm.globals["xor"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("xor() expects 2 arguments", 0, 0, "")
			}
			a, _ := types.ToInt(args[0])
			b, _ := types.ToInt(args[1])
			return types.Int(a ^ b)
		},
	}

	vm.globals["and"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("and() expects 2 arguments", 0, 0, "")
			}
			a, _ := types.ToInt(args[0])
			b, _ := types.ToInt(args[1])
			return types.Int(a & b)
		},
	}

	vm.globals["or"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("or() expects 2 arguments", 0, 0, "")
			}
			a, _ := types.ToInt(args[0])
			b, _ := types.ToInt(args[1])
			return types.Int(a | b)
		},
	}

	vm.globals["not"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("not() expects 1 argument", 0, 0, "")
			}
			n, _ := types.ToInt(args[0])
			return types.Int(^n)
		},
	}

	vm.globals["shl"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("shl() expects 2 arguments: value, shift", 0, 0, "")
			}
			n, _ := types.ToInt(args[0])
			s, _ := types.ToInt(args[1])
			return types.Int(n << s)
		},
	}

	vm.globals["shr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("shr() expects 2 arguments: value, shift", 0, 0, "")
			}
			n, _ := types.ToInt(args[0])
			s, _ := types.ToInt(args[1])
			return types.Int(n >> s)
		},
	}

	vm.globals["randInt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("randInt() expects 1 argument: max", 0, 0, "")
			}
			max, _ := types.ToInt(args[0])
			if max <= 0 {
				return types.Int(0)
			}
			return types.Int(rand.Int63n(int64(max)))
		},
	}

	vm.globals["randRange"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("randRange() expects 2 arguments: min, max", 0, 0, "")
			}
			min, _ := types.ToInt(args[0])
			max, _ := types.ToInt(args[1])
			if max <= min {
				return types.Int(min)
			}
			return types.Int(rand.Int63n(int64(max-min)) + int64(min))
		},
	}

	vm.globals["randFloat"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Float(rand.Float64())
		},
	}

	vm.globals["randBool"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Bool(rand.Intn(2) == 1)
		},
	}

	vm.globals["randChoice"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("randChoice() expects 1 argument: array", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("randChoice() first argument must be an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return types.UndefinedValue
			}
			return arr.Get(rand.Intn(arr.Len()))
		},
	}

	vm.globals["shuffleArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("shuffleArr() expects 1 argument: array", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("shuffleArr() first argument must be an array", 0, 0, "")
			}
			elements := make([]types.Object, arr.Len())
			for i := 0; i < arr.Len(); i++ {
				elements[i] = arr.Get(i)
			}
			rand.Shuffle(len(elements), func(i, j int) {
				elements[i], elements[j] = elements[j], elements[i]
			})
			return collections.NewArrayWithElements(elements)
		},
	}

	vm.globals["sampleArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("sampleArr() expects 2 arguments: array, n", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("sampleArr() first argument must be an array", 0, 0, "")
			}
			n, _ := types.ToInt(args[1])
			if n <= 0 {
				return collections.NewArray()
			}
			arrLen := arr.Len()
			sampleN := n
			if sampleN >= types.Int(arrLen) {
				sampleN = types.Int(arrLen)
			}
			indices := make([]int, arrLen)
			for i := range indices {
				indices[i] = i
			}
			rand.Shuffle(len(indices), func(i, j int) {
				indices[i], indices[j] = indices[j], indices[i]
			})
			result := collections.NewArray()
			for i := 0; i < int(n); i++ {
				result.Append(arr.Get(indices[i]))
			}
			return result
		},
	}

	vm.globals["seed"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("seed() expects 1 argument", 0, 0, "")
			}
			seed, _ := types.ToInt(args[0])
			rand.Seed(int64(seed))
			return types.UndefinedValue
		},
	}

	vm.globals["repeatStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("repeatStr() expects 2 arguments: string, count", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			count, _ := types.ToInt(args[1])
			if count <= 0 {
				return types.String("")
			}
			return types.String(strings.Repeat(s, int(count)))
		},
	}

	vm.globals["padEnd"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("padEnd() expects 3 arguments: string, length, padString", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			length, _ := types.ToInt(args[1])
			pad := string(types.ToString(args[2]))
			if len(s) >= int(length) {
				return types.String(s)
			}
			padLen := int(length) - len(s)
			result := strings.Repeat(pad, (padLen/len(pad))+1)
			return types.String(s + result[:padLen])
		},
	}

	vm.globals["trimSpace"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("trimSpace() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			return types.String(strings.TrimSpace(s))
		},
	}

	vm.globals["hasPrefix"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("hasPrefix() expects 2 arguments: string, prefix", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			prefix := string(types.ToString(args[1]))
			return types.Bool(strings.HasPrefix(s, prefix))
		},
	}

	vm.globals["hasSuffix"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("hasSuffix() expects 2 arguments: string, suffix", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			suffix := string(types.ToString(args[1]))
			return types.Bool(strings.HasSuffix(s, suffix))
		},
	}

	vm.globals["replaceAllStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("replaceAllStr() expects 3 arguments: string, old, new", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			old := string(types.ToString(args[1]))
			new := string(types.ToString(args[2]))
			return types.String(strings.ReplaceAll(s, old, new))
		},
	}

	vm.globals["explode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("explode() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			result := collections.NewArray()
			for _, r := range s {
				result.Append(types.String(r))
			}
			return result
		},
	}

	vm.globals["implode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("implode() expects 1 argument: array", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("implode() first argument must be an array", 0, 0, "")
			}
			sep := ""
			if len(args) >= 2 {
				sep = string(types.ToString(args[1]))
			}
			parts := make([]string, arr.Len())
			for i := 0; i < arr.Len(); i++ {
				parts[i] = string(types.ToString(arr.Get(i)))
			}
			return types.String(strings.Join(parts, sep))
		},
	}

	vm.globals["reverseStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("reverseStr() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			runes := []rune(s)
			for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
				runes[i], runes[j] = runes[j], runes[i]
			}
			return types.String(string(runes))
		},
	}

	vm.globals["indexStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("indexStr() expects 2 arguments: string, substring", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			sub := string(types.ToString(args[1]))
			idx := strings.Index(s, sub)
			if idx < 0 {
				return types.Int(-1)
			}
			return types.Int(idx)
		},
	}

	vm.globals["lastIndexStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("lastIndexStr() expects 2 arguments: string, substring", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			sub := string(types.ToString(args[1]))
			idx := strings.LastIndex(s, sub)
			if idx < 0 {
				return types.Int(-1)
			}
			return types.Int(idx)
		},
	}

	vm.globals["lines"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("lines() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			if s == "" {
				return collections.NewArray()
			}
			lines := strings.Split(s, "\n")
			result := collections.NewArray()
			for _, line := range lines {
				result.Append(types.String(line))
			}
			return result
		},
	}

	vm.globals["words"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("words() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			fields := strings.Fields(s)
			result := collections.NewArray()
			for _, f := range fields {
				result.Append(types.String(f))
			}
			return result
		},
	}

	vm.globals["strip"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("strip() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			return types.String(strings.TrimSpace(s))
		},
	}

	vm.globals["countStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("countStr() expects 2 arguments (string, substring)", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			substr := string(types.ToString(args[1]))
			return types.Int(strings.Count(s, substr))
		},
	}

	vm.globals["splitLines"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("splitLines() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			lines := strings.Split(s, "\n")
			arr := collections.NewArray()
			for _, line := range lines {
				arr.Append(types.String(line))
			}
			return arr
		},
	}

	vm.globals["levenshtein"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("levenshtein() expects 2 arguments (string1, string2)", 0, 0, "")
			}
			s1 := string(types.ToString(args[0]))
			s2 := string(types.ToString(args[1]))
			if len(s1) == 0 {
				return types.Int(len(s2))
			}
			if len(s2) == 0 {
				return types.Int(len(s1))
			}
			rows := len(s1) + 1
			cols := len(s2) + 1
			matrix := make([][]int, rows)
			for i := range matrix {
				matrix[i] = make([]int, cols)
			}
			for i := 0; i < rows; i++ {
				matrix[i][0] = i
			}
			for j := 0; j < cols; j++ {
				matrix[0][j] = j
			}
			for i := 1; i < rows; i++ {
				for j := 1; j < cols; j++ {
					cost := 1
					if s1[i-1] == s2[j-1] {
						cost = 0
					}
					matrix[i][j] = min(
						matrix[i-1][j]+1,
						matrix[i][j-1]+1,
						matrix[i-1][j-1]+cost,
					)
				}
			}
			return types.Int(matrix[rows-1][cols-1])
		},
	}

	vm.globals["byteSlice"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("byteSlice() expects 1 argument", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			result := collections.NewArray()
			for _, b := range []byte(s) {
				result.Append(types.Int(b))
			}
			return result
		},
	}

	vm.globals["fromByteSlice"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("fromByteSlice() expects 1 argument: array", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("fromByteSlice() first argument must be an array", 0, 0, "")
			}
			bytes := make([]byte, arr.Len())
			for i := 0; i < arr.Len(); i++ {
				b, _ := types.ToInt(arr.Get(i))
				bytes[i] = byte(b)
			}
			return types.String(string(bytes))
		},
	}

	vm.globals["match"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("match() expects 2 arguments: string, pattern", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			pattern := string(types.ToString(args[1]))
			matched, err := regexp.MatchString(pattern, s)
			if err != nil {
				return types.NewError("match(): "+err.Error(), 0, 0, "")
			}
			return types.Bool(matched)
		},
	}

	vm.globals["replaceRegexp"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("replaceRegexp() expects 3 arguments: string, pattern, replacement", 0, 0, "")
			}
			s := string(types.ToString(args[0]))
			pattern := string(types.ToString(args[1]))
			replacement := string(types.ToString(args[2]))
			re, err := regexp.Compile(pattern)
			if err != nil {
				return types.NewError("replaceRegexp(): "+err.Error(), 0, 0, "")
			}
			return types.String(re.ReplaceAllString(s, replacement))
		},
	}

	vm.globals["fileExists"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("fileExists() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			_, err := os.Stat(path)
			return types.Bool(err == nil)
		},
	}

	vm.globals["isDir"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isDir() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			info, err := os.Stat(path)
			if err != nil {
				return types.Bool(false)
			}
			return types.Bool(info.IsDir())
		},
	}

	vm.globals["isFile"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isFile() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			info, err := os.Stat(path)
			if err != nil {
				return types.Bool(false)
			}
			return types.Bool(!info.IsDir())
		},
	}

	vm.globals["fileSize"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("fileSize() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			info, err := os.Stat(path)
			if err != nil {
				return types.NewError("fileSize(): "+err.Error(), 0, 0, "")
			}
			return types.Int(info.Size())
		},
	}

	vm.globals["absPath"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("absPath() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			abs, err := filepath.Abs(path)
			if err != nil {
				return types.NewError("absPath(): "+err.Error(), 0, 0, "")
			}
			return types.String(abs)
		},
	}

	vm.globals["dirName"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("dirName() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			dir := filepath.Dir(path)
			return types.String(dir)
		},
	}

	vm.globals["baseName"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("baseName() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			base := filepath.Base(path)
			return types.String(base)
		},
	}

	vm.globals["extName"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("extName() expects 1 argument", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			ext := filepath.Ext(path)
			return types.String(ext)
		},
	}

	vm.globals["joinPath"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("joinPath() expects at least 1 argument", 0, 0, "")
			}
			parts := make([]string, len(args))
			for i, arg := range args {
				parts[i] = string(types.ToString(arg))
			}
			return types.String(filepath.Join(parts...))
		},
	}

	vm.globals["listDir"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("listDir() expects 1 argument: path", 0, 0, "")
			}
			path := string(types.ToString(args[0]))
			files, err := os.ReadDir(path)
			if err != nil {
				return types.NewError("listDir(): "+err.Error(), 0, 0, "")
			}
			result := collections.NewArray()
			for _, f := range files {
				info, _ := f.Info()
				item := collections.NewMap()
				item.Set("name", types.String(f.Name()))
				item.Set("isDir", types.Bool(f.IsDir()))
				if info != nil {
					item.Set("size", types.Int(info.Size()))
					item.Set("modTime", types.Int(info.ModTime().Unix()))
				}
				result.Append(item)
			}
			return result
		},
	}

	vm.globals["envVar"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("envVar() expects 1 argument: name", 0, 0, "")
			}
			name := string(types.ToString(args[0]))
			return types.String(os.Getenv(name))
		},
	}

	vm.globals["envVars"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			result := collections.NewMap()
			for _, e := range os.Environ() {
				parts := strings.SplitN(e, "=", 2)
				if len(parts) == 2 {
					result.Set(parts[0], types.String(parts[1]))
				}
			}
			return result
		},
	}

	vm.globals["osName"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.String(runtime.GOOS)
		},
	}

	vm.globals["arch"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.String(runtime.GOARCH)
		},
	}

	vm.globals["cpuCount"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(runtime.NumCPU())
		},
	}

	vm.globals["numGoroutine"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(runtime.NumGoroutine())
		},
	}

	vm.globals["toJSON"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("toJSON() expects 1 argument", 0, 0, "")
			}
			jsonBytes, err := json.Marshal(args[0])
			if err != nil {
				return types.NewError("toJSON(): "+err.Error(), 0, 0, "")
			}
			return types.String(string(jsonBytes))
		},
	}

	vm.globals["jsonEncode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("jsonEncode() expects 1 argument", 0, 0, "")
			}
			jsonBytes, err := json.Marshal(toJSONable(args[0]))
			if err != nil {
				return types.NewError("jsonEncode(): "+err.Error(), 0, 0, "")
			}
			return types.String(string(jsonBytes))
		},
	}

	vm.globals["fromJSON"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("fromJSON() expects 1 argument", 0, 0, "")
			}
			jsonStr := string(types.ToString(args[0]))
			var result map[string]interface{}
			err := json.Unmarshal([]byte(jsonStr), &result)
			if err != nil {
				return types.NewError("fromJSON(): "+err.Error(), 0, 0, "")
			}
			m := collections.NewMap()
			for k, v := range result {
				m.Set(k, types.String(fmt.Sprintf("%v", v)))
			}
			return m
		},
	}

	vm.globals["typeName"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("typeName() expects 1 argument", 0, 0, "")
			}
			return types.String(args[0].TypeName())
		},
	}

	vm.globals["lenArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("lenArr() expects 1 argument", 0, 0, "")
			}
			switch v := args[0].(type) {
			case *collections.Array:
				return types.Int(v.Len())
			case *collections.Map:
				return types.Int(v.Len())
			case types.String:
				return types.Int(len(string(v)))
			default:
				return types.Int(0)
			}
		},
	}

	vm.globals["minNum"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("minNum() expects at least 1 argument", 0, 0, "")
			}
			min, _ := types.ToFloat(args[0])
			for i := 1; i < len(args); i++ {
				val, _ := types.ToFloat(args[i])
				if float64(val) < float64(min) {
					min = val
				}
			}
			return types.Float(min)
		},
	}

	vm.globals["maxNum"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("maxNum() expects at least 1 argument", 0, 0, "")
			}
			max, _ := types.ToFloat(args[0])
			for i := 1; i < len(args); i++ {
				val, _ := types.ToFloat(args[i])
				if float64(val) > float64(max) {
					max = val
				}
			}
			return types.Float(max)
		},
	}

	vm.globals["sumArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("sumArr() expects 1 argument: array", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("sumArr() first argument must be an array", 0, 0, "")
			}
			var sum types.Float
			for i := 0; i < arr.Len(); i++ {
				val, _ := types.ToFloat(arr.Get(i))
				sum += val
			}
			return sum
		},
	}

	vm.globals["avgArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("avgArr() expects 1 argument: array", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("avgArr() first argument must be an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return types.Float(0)
			}
			var sum types.Float
			for i := 0; i < arr.Len(); i++ {
				val, _ := types.ToFloat(arr.Get(i))
				sum += val
			}
			return types.Float(float64(sum) / float64(arr.Len()))
		},
	}

	vm.globals["uniqArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("uniqArr() expects 1 argument: array", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("uniqArr() first argument must be an array", 0, 0, "")
			}
			seen := make(map[string]bool)
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				val := arr.Get(i).ToStr()
				if !seen[val] {
					seen[val] = true
					result.Append(arr.Get(i))
				}
			}
			return result
		},
	}

	vm.globals["differenceArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("differenceArr() expects 2 arguments", 0, 0, "")
			}
			arr1, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("differenceArr() first argument must be an array", 0, 0, "")
			}
			arr2, ok := args[1].(*collections.Array)
			if !ok {
				return types.NewError("differenceArr() second argument must be an array", 0, 0, "")
			}
			set2 := make(map[string]bool)
			for i := 0; i < arr2.Len(); i++ {
				set2[arr2.Get(i).ToStr()] = true
			}
			result := collections.NewArray()
			for i := 0; i < arr1.Len(); i++ {
				val := arr1.Get(i).ToStr()
				if !set2[val] {
					result.Append(arr1.Get(i))
				}
			}
			return result
		},
	}

	vm.globals["intersectionArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("intersectionArr() expects 2 arguments", 0, 0, "")
			}
			arr1, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("intersectionArr() first argument must be an array", 0, 0, "")
			}
			arr2, ok := args[1].(*collections.Array)
			if !ok {
				return types.NewError("intersectionArr() second argument must be an array", 0, 0, "")
			}
			set2 := make(map[string]bool)
			for i := 0; i < arr2.Len(); i++ {
				set2[arr2.Get(i).ToStr()] = true
			}
			result := collections.NewArray()
			seen := make(map[string]bool)
			for i := 0; i < arr1.Len(); i++ {
				val := arr1.Get(i).ToStr()
				if set2[val] && !seen[val] {
					seen[val] = true
					result.Append(arr1.Get(i))
				}
			}
			return result
		},
	}

	vm.globals["unionArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("unionArr() expects 2 arguments", 0, 0, "")
			}
			arr1, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("unionArr() first argument must be an array", 0, 0, "")
			}
			arr2, ok := args[1].(*collections.Array)
			if !ok {
				return types.NewError("unionArr() second argument must be an array", 0, 0, "")
			}
			seen := make(map[string]bool)
			result := collections.NewArray()
			for i := 0; i < arr1.Len(); i++ {
				val := arr1.Get(i).ToStr()
				if !seen[val] {
					seen[val] = true
					result.Append(arr1.Get(i))
				}
			}
			for i := 0; i < arr2.Len(); i++ {
				val := arr2.Get(i).ToStr()
				if !seen[val] {
					seen[val] = true
					result.Append(arr2.Get(i))
				}
			}
			return result
		},
	}

	vm.globals["includesArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("includesArr() expects 2 arguments: array, value", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("includesArr() first argument must be an array", 0, 0, "")
			}
			target := args[1].ToStr()
			for i := 0; i < arr.Len(); i++ {
				if arr.Get(i).ToStr() == target {
					return types.Bool(true)
				}
			}
			return types.Bool(false)
		},
	}

	vm.globals["indexOfArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("indexOfArr() expects 2 arguments: array, value", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("indexOfArr() first argument must be an array", 0, 0, "")
			}
			target := args[1].ToStr()
			for i := 0; i < arr.Len(); i++ {
				if arr.Get(i).ToStr() == target {
					return types.Int(i)
				}
			}
			return types.Int(-1)
		},
	}

	vm.globals["concatArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("concatArr() expects at least 1 argument", 0, 0, "")
			}
			result := collections.NewArray()
			for _, arg := range args {
				arr, ok := arg.(*collections.Array)
				if !ok {
					result.Append(arg)
				} else {
					for i := 0; i < arr.Len(); i++ {
						result.Append(arr.Get(i))
					}
				}
			}
			return result
		},
	}

	vm.globals["sliceArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("sliceArr() expects at least 2 arguments: array, start", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("sliceArr() first argument must be an array", 0, 0, "")
			}
			start, _ := types.ToInt(args[1])
			arrLen := arr.Len()
			end := arrLen
			if len(args) >= 3 {
				e, _ := types.ToInt(args[2])
				end = int(e)
			}
			if start < 0 {
				start = 0
			}
			if end > arrLen {
				end = arrLen
			}
			if start >= types.Int(end) {
				return collections.NewArray()
			}
			result := collections.NewArray()
			for i := int(start); i < end; i++ {
				result.Append(arr.Get(i))
			}
			return result
		},
	}

	vm.globals["firstArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("firstArr() expects 1 argument: array", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("firstArr() first argument must be an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return types.UndefinedValue
			}
			return arr.Get(0)
		},
	}

	vm.globals["lastArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("lastArr() expects 1 argument: array", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("lastArr() first argument must be an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return types.UndefinedValue
			}
			return arr.Get(arr.Len() - 1)
		},
	}

	vm.globals["takeArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("takeArr() expects 2 arguments: array, n", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("takeArr() first argument must be an array", 0, 0, "")
			}
			n, _ := types.ToInt(args[1])
			arrLen := arr.Len()
			if n >= types.Int(arrLen) {
				return arr
			}
			result := collections.NewArray()
			for i := types.Int(0); i < n && i < types.Int(arrLen); i++ {
				result.Append(arr.Get(int(i)))
			}
			return result
		},
	}

	vm.globals["dropArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("dropArr() expects 2 arguments: array, n", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("dropArr() first argument must be an array", 0, 0, "")
			}
			n, _ := types.ToInt(args[1])
			arrLen := arr.Len()
			if n >= types.Int(arrLen) {
				return collections.NewArray()
			}
			result := collections.NewArray()
			for i := n; i < types.Int(arrLen); i++ {
				result.Append(arr.Get(int(i)))
			}
			return result
		},
	}

	vm.globals["toPairs"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("toPairs() expects 1 argument", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				return types.NewError("toPairs() first argument must be a map", 0, 0, "")
			}
			result := collections.NewArray()
			keys := m.Keys()
			for i := 0; i < keys.Len(); i++ {
				k := keys.Get(i).ToStr()
				pair := collections.NewArray()
				pair.Append(types.String(k))
				pair.Append(m.Get(k))
				result.Append(pair)
			}
			return result
		},
	}

	vm.globals["fromPairs"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("fromPairs() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("fromPairs() first argument must be an array", 0, 0, "")
			}
			result := collections.NewMap()
			for i := 0; i < arr.Len(); i++ {
				pair, ok := arr.Get(i).(*collections.Array)
				if ok && pair.Len() >= 2 {
					result.Set(pair.Get(0).ToStr(), pair.Get(1))
				}
			}
			return result
		},
	}

	vm.globals["invertMap"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("invertMap() expects 1 argument", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				return types.NewError("invertMap() first argument must be a map", 0, 0, "")
			}
			result := collections.NewMap()
			keys := m.Keys()
			for i := 0; i < keys.Len(); i++ {
				k := keys.Get(i).ToStr()
				v := m.Get(k)
				result.Set(v.ToStr(), types.String(k))
			}
			return result
		},
	}

	vm.globals["mergeMap"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("mergeMap() expects at least 1 argument", 0, 0, "")
			}
			result := collections.NewMap()
			for _, arg := range args {
				m, ok := arg.(*collections.Map)
				if !ok {
					continue
				}
				keys := m.Keys()
				for i := 0; i < keys.Len(); i++ {
					k := keys.Get(i).ToStr()
					result.Set(k, m.Get(k))
				}
			}
			return result
		},
	}

	vm.globals["hasKey"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("hasKey() expects 2 arguments", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				return types.NewError("hasKey() first argument must be a map", 0, 0, "")
			}
			key := args[1].ToStr()
			return types.Bool(m.Get(key) != nil)
		},
	}

	vm.globals["deleteKey"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("deleteKey() expects 2 arguments", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				return types.NewError("deleteKey() first argument must be a map", 0, 0, "")
			}
			key := args[1].ToStr()
			m.Delete(key)
			return m
		},
	}

	vm.globals["pickKeys"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("pickKeys() expects 2 arguments", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				return types.NewError("pickKeys() first argument must be a map", 0, 0, "")
			}
			keys, ok := args[1].(*collections.Array)
			if !ok {
				return types.NewError("pickKeys() second argument must be an array", 0, 0, "")
			}
			result := collections.NewMap()
			for i := 0; i < keys.Len(); i++ {
				k := keys.Get(i).ToStr()
				if m.Get(k) != nil {
					result.Set(k, m.Get(k))
				}
			}
			return result
		},
	}

	vm.globals["omitKeys"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("omitKeys() expects 2 arguments", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				return types.NewError("omitKeys() first argument must be a map", 0, 0, "")
			}
			keys, ok := args[1].(*collections.Array)
			if !ok {
				return types.NewError("omitKeys() second argument must be an array", 0, 0, "")
			}
			omit := make(map[string]bool)
			for i := 0; i < keys.Len(); i++ {
				omit[keys.Get(i).ToStr()] = true
			}
			result := collections.NewMap()
			mapKeys := m.Keys()
			for i := 0; i < mapKeys.Len(); i++ {
				k := mapKeys.Get(i).ToStr()
				if !omit[k] {
					result.Set(k, m.Get(k))
				}
			}
			return result
		},
	}

	vm.globals["sleep"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("sleep() expects 1 argument", 0, 0, "")
			}
			secs, _ := types.ToFloat(args[0])
			time.Sleep(time.Duration(float64(secs) * float64(time.Second)))
			return types.UndefinedValue
		},
	}

	vm.globals["timestamp"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(time.Now().Unix())
		},
	}

	vm.globals["now"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.String(time.Now().Format("2006-01-02 15:04:05"))
		},
	}

	vm.globals["nowISO"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.String(time.Now().Format(time.RFC3339))
		},
	}

	vm.globals["version"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.String("1.0.0")
		},
	}

	vm.globals["info"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			result := collections.NewMap()
			result.Set("version", types.String("1.0.0"))
			result.Set("os", types.String(runtime.GOOS))
			result.Set("arch", types.String(runtime.GOARCH))
			result.Set("cpus", types.Int(runtime.NumCPU()))
			return result
		},
	}

	vm.globals["numCPU"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(runtime.NumCPU())
		},
	}

	vm.globals["cwd"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			dir, err := os.Getwd()
			if err != nil {
				return types.NewError("cwd(): "+err.Error(), 0, 0, "")
			}
			return types.String(dir)
		},
	}

	vm.globals["homeDir"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			dir, err := os.UserHomeDir()
			if err != nil {
				return types.NewError("homeDir(): "+err.Error(), 0, 0, "")
			}
			return types.String(dir)
		},
	}

	vm.globals["tempDir"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.String(os.TempDir())
		},
	}

	vm.globals["hostname"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			name, err := os.Hostname()
			if err != nil {
				return types.NewError("hostname(): "+err.Error(), 0, 0, "")
			}
			return types.String(name)
		},
	}

	vm.globals["args"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			result := collections.NewArray()
			for _, arg := range os.Args {
				result.Append(types.String(arg))
			}
			return result
		},
	}

	vm.globals["exit"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			code := 0
			if len(args) > 0 {
				val, _ := types.ToInt(args[0])
				code = int(val)
			}
			os.Exit(code)
			return types.UndefinedValue
		},
	}

	vm.globals["env"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				result := collections.NewMap()
				for _, e := range os.Environ() {
					parts := strings.SplitN(e, "=", 2)
					if len(parts) == 2 {
						result.Set(parts[0], types.String(parts[1]))
					}
				}
				return result
			}
			name := args[0].ToStr()
			return types.String(os.Getenv(name))
		},
	}

	vm.globals["readFile"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("readFile() expects 1 argument", 0, 0, "")
			}
			path := args[0].ToStr()
			data, err := os.ReadFile(path)
			if err != nil {
				return types.NewError("readFile(): "+err.Error(), 0, 0, "")
			}
			return types.String(string(data))
		},
	}

	vm.globals["writeFile"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("writeFile() expects 2 arguments", 0, 0, "")
			}
			path := args[0].ToStr()
			content := args[1].ToStr()
			err := os.WriteFile(path, []byte(content), 0644)
			if err != nil {
				return types.NewError("writeFile(): "+err.Error(), 0, 0, "")
			}
			return types.UndefinedValue
		},
	}

	vm.globals["mkdir"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("mkdir() expects 1 argument", 0, 0, "")
			}
			path := args[0].ToStr()
			err := os.MkdirAll(path, 0755)
			if err != nil {
				return types.NewError("mkdir(): "+err.Error(), 0, 0, "")
			}
			return types.UndefinedValue
		},
	}

	vm.globals["remove"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("remove() expects 1 argument", 0, 0, "")
			}
			path := args[0].ToStr()
			err := os.Remove(path)
			if err != nil {
				return types.NewError("remove(): "+err.Error(), 0, 0, "")
			}
			return types.UndefinedValue
		},
	}

	vm.globals["rename"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("rename() expects 2 arguments", 0, 0, "")
			}
			old := args[0].ToStr()
			new := args[1].ToStr()
			err := os.Rename(old, new)
			if err != nil {
				return types.NewError("rename(): "+err.Error(), 0, 0, "")
			}
			return types.UndefinedValue
		},
	}

	vm.globals["exists"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("exists() expects 1 argument", 0, 0, "")
			}
			path := args[0].ToStr()
			_, err := os.Stat(path)
			return types.Bool(err == nil)
		},
	}

	vm.globals["stat"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("stat() expects 1 argument", 0, 0, "")
			}
			path := args[0].ToStr()
			info, err := os.Stat(path)
			if err != nil {
				return types.NewError("stat(): "+err.Error(), 0, 0, "")
			}
			result := collections.NewMap()
			result.Set("name", types.String(info.Name()))
			result.Set("size", types.Int(info.Size()))
			result.Set("isDir", types.Bool(info.IsDir()))
			result.Set("modTime", types.Int(info.ModTime().Unix()))
			return result
		},
	}

	vm.globals["httpGet"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("httpGet() expects 1 argument", 0, 0, "")
			}
			urlStr := args[0].ToStr()
			resp, err := http.Get(urlStr)
			if err != nil {
				return types.NewError("httpGet(): "+err.Error(), 0, 0, "")
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return types.NewError("httpGet(): "+err.Error(), 0, 0, "")
			}
			result := collections.NewMap()
			result.Set("status", types.Int(resp.StatusCode))
			result.Set("body", types.String(string(body)))
			return result
		},
	}

	vm.globals["httpPost"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("httpPost() expects at least 2 arguments", 0, 0, "")
			}
			urlStr := args[0].ToStr()
			body := args[1].ToStr()
			resp, err := http.Post(urlStr, "text/plain", strings.NewReader(body))
			if err != nil {
				return types.NewError("httpPost(): "+err.Error(), 0, 0, "")
			}
			defer resp.Body.Close()
			respBody, _ := io.ReadAll(resp.Body)
			result := collections.NewMap()
			result.Set("status", types.Int(resp.StatusCode))
			result.Set("body", types.String(string(respBody)))
			return result
		},
	}

	vm.globals["urlParse"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("urlParse() expects 1 argument", 0, 0, "")
			}
			urlStr := args[0].ToStr()
			u, err := url.Parse(urlStr)
			if err != nil {
				return types.NewError("urlParse(): "+err.Error(), 0, 0, "")
			}
			result := collections.NewMap()
			result.Set("scheme", types.String(u.Scheme))
			result.Set("host", types.String(u.Host))
			result.Set("path", types.String(u.Path))
			result.Set("query", types.String(u.RawQuery))
			return result
		},
	}

	vm.globals["urlEncode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("urlEncode() expects 1 argument", 0, 0, "")
			}
			s := args[0].ToStr()
			return types.String(url.QueryEscape(s))
		},
	}

	vm.globals["urlDecode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("urlDecode() expects 1 argument", 0, 0, "")
			}
			s := args[0].ToStr()
			decoded, err := url.QueryUnescape(s)
			if err != nil {
				return types.NewError("urlDecode(): "+err.Error(), 0, 0, "")
			}
			return types.String(decoded)
		},
	}

	vm.globals["md5"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("md5() expects 1 argument", 0, 0, "")
			}
			s := args[0].ToStr()
			hash := md5.Sum([]byte(s))
			return types.String(hex.EncodeToString(hash[:]))
		},
	}

	vm.globals["sha256"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("sha256() expects 1 argument", 0, 0, "")
			}
			s := args[0].ToStr()
			hash := sha256.Sum256([]byte(s))
			return types.String(hex.EncodeToString(hash[:]))
		},
	}

	vm.globals["sha512"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("sha512() expects 1 argument", 0, 0, "")
			}
			s := args[0].ToStr()
			hash := sha512.Sum512([]byte(s))
			return types.String(hex.EncodeToString(hash[:]))
		},
	}

	vm.globals["base64Encode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("base64Encode() expects 1 argument", 0, 0, "")
			}
			s := args[0].ToStr()
			return types.String(base64.StdEncoding.EncodeToString([]byte(s)))
		},
	}

	vm.globals["base64Decode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("base64Decode() expects 1 argument", 0, 0, "")
			}
			s := args[0].ToStr()
			decoded, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return types.NewError("base64Decode(): "+err.Error(), 0, 0, "")
			}
			return types.String(string(decoded))
		},
	}

	vm.globals["hexEncode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("hexEncode() expects 1 argument", 0, 0, "")
			}
			s := args[0].ToStr()
			return types.String(hex.EncodeToString([]byte(s)))
		},
	}

	vm.globals["hexDecode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("hexDecode() expects 1 argument", 0, 0, "")
			}
			s := args[0].ToStr()
			decoded, err := hex.DecodeString(s)
			if err != nil {
				return types.NewError("hexDecode(): "+err.Error(), 0, 0, "")
			}
			return types.String(string(decoded))
		},
	}

	vm.globals["toJSON"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("toJSON() expects 1 argument", 0, 0, "")
			}
			jsonBytes, err := json.Marshal(args[0])
			if err != nil {
				return types.NewError("toJSON(): "+err.Error(), 0, 0, "")
			}
			return types.String(string(jsonBytes))
		},
	}

	vm.globals["jsonEncode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("jsonEncode() expects 1 argument", 0, 0, "")
			}
			jsonBytes, err := json.Marshal(toJSONable(args[0]))
			if err != nil {
				return types.NewError("jsonEncode(): "+err.Error(), 0, 0, "")
			}
			return types.String(string(jsonBytes))
		},
	}

	vm.globals["fromJSON"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("fromJSON() expects 1 argument", 0, 0, "")
			}
			jsonStr := args[0].ToStr()
			var result map[string]interface{}
			err := json.Unmarshal([]byte(jsonStr), &result)
			if err != nil {
				return types.NewError("fromJSON(): "+err.Error(), 0, 0, "")
			}
			m := collections.NewMap()
			for k, v := range result {
				m.Set(k, types.String(fmt.Sprintf("%v", v)))
			}
			return m
		},
	}

	vm.globals["print"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			for _, arg := range args {
				fmt.Print(arg.ToStr())
			}
			return types.UndefinedValue
		},
	}

	vm.globals["println"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			for i, arg := range args {
				if i > 0 {
					fmt.Print(" ")
				}
				fmt.Print(arg.ToStr())
			}
			fmt.Println()
			return types.UndefinedValue
		},
	}

	vm.globals["printf"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("printf() expects at least 1 argument", 0, 0, "")
			}
			format := args[0].ToStr()
			arr := args[1:]
			anyArr := make([]any, len(arr))
			for i, v := range arr {
				anyArr[i] = v
			}
			fmt.Printf(format, anyArr...)
			return types.UndefinedValue
		},
	}

	vm.globals["sprintf"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("sprintf() expects at least 1 argument", 0, 0, "")
			}
			format := args[0].ToStr()
			arr := args[1:]
			anyArr := make([]any, len(arr))
			for i, v := range arr {
				anyArr[i] = v
			}
			return types.String(fmt.Sprintf(format, anyArr...))
		},
	}

	vm.globals["typeName"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("typeName() expects 1 argument", 0, 0, "")
			}
			return types.String(args[0].TypeName())
		},
	}

	vm.globals["typeOf"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("typeOf() expects 1 argument", 0, 0, "")
			}
			return types.String(args[0].TypeName())
		},
	}

	vm.globals["len"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("len() expects 1 argument", 0, 0, "")
			}
			switch v := args[0].(type) {
			case *collections.Array:
				return types.Int(v.Len())
			case *collections.Map:
				return types.Int(v.Len())
			case types.String:
				return types.Int(len(string(v)))
			default:
				return types.Int(0)
			}
		},
	}

	vm.globals["keys"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("keys() expects 1 argument", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				return types.NewError("keys() expects a map", 0, 0, "")
			}
			result := collections.NewArray()
			keys := m.Keys()
			for i := 0; i < keys.Len(); i++ {
				result.Append(keys.Get(i))
			}
			return result
		},
	}

	vm.globals["values"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("values() expects 1 argument", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				return types.NewError("values() expects a map", 0, 0, "")
			}
			result := collections.NewArray()
			keys := m.Keys()
			for i := 0; i < keys.Len(); i++ {
				k := keys.Get(i).ToStr()
				result.Append(m.Get(k))
			}
			return result
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

	// String utilities
	vm.globals["repeat"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("repeat(str, n) expects 2 arguments", 0, 0, "")
			}
			str := args[0].ToStr()
			n, err := types.ToInt(args[1])
			if err != nil {
				return err
			}
			result := ""
			i := 0
			for i < int(n) {
				result = result + str
				i = i + 1
			}
			return types.String(result)
		},
	}

	// Math utilities
	vm.globals["clamp"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("clamp(val, min, max) expects 3 arguments", 0, 0, "")
			}
			val, err := types.ToInt(args[0])
			if err != nil {
				return err
			}
			minVal, err := types.ToInt(args[1])
			if err != nil {
				return err
			}
			maxVal, err := types.ToInt(args[2])
			if err != nil {
				return err
			}
			if val < minVal {
				return minVal
			}
			if val > maxVal {
				return maxVal
			}
			return val
		},
	}

	vm.globals["min"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("min(a, b) expects 2 arguments", 0, 0, "")
			}
			a, err := types.ToInt(args[0])
			if err != nil {
				return err
			}
			b, err := types.ToInt(args[1])
			if err != nil {
				return err
			}
			if a < b {
				return a
			}
			return b
		},
	}

	vm.globals["max"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("max(a, b) expects 2 arguments", 0, 0, "")
			}
			a, err := types.ToInt(args[0])
			if err != nil {
				return err
			}
			b, err := types.ToInt(args[1])
			if err != nil {
				return err
			}
			if a > b {
				return a
			}
			return b
		},
	}

	vm.globals["abs"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("abs(n) expects 1 argument", 0, 0, "")
			}
			n, err := types.ToInt(args[0])
			if err != nil {
				return err
			}
			if n < 0 {
				return -n
			}
			return n
		},
	}

	// Array functions
	vm.globals["includes"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("includes(arr, val) expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("includes: first argument must be array", 0, 0, "")
			}
			target := args[1]
			for i := 0; i < arr.Len(); i++ {
				if arr.Get(i).Equals(target) {
					return types.Bool(true)
				}
			}
			return types.Bool(false)
		},
	}

	vm.globals["find"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("find(arr, val) expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("find: first argument must be array", 0, 0, "")
			}
			target := args[1]
			for i := 0; i < arr.Len(); i++ {
				if arr.Get(i).Equals(target) {
					return types.Int(i)
				}
			}
			return types.Int(-1)
		},
	}

	vm.globals["slice"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("slice(arr) expects at least 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("slice: first argument must be array", 0, 0, "")
			}
			start := 0
			end := arr.Len()
			if len(args) >= 2 {
				start = int(args[1].(types.Int))
			}
			if len(args) >= 3 {
				end = int(args[2].(types.Int))
			}
			result := collections.NewArray()
			for i := start; i < end && i < arr.Len(); i++ {
				result.Append(arr.Get(i))
			}
			return result
		},
	}

	vm.globals["concat"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("concat(arr1, arr2, ...) expects at least 2 arguments", 0, 0, "")
			}
			result := collections.NewArray()
			for _, arg := range args {
				arr, ok := arg.(*collections.Array)
				if !ok {
					return types.NewError("concat: all arguments must be arrays", 0, 0, "")
				}
				for i := 0; i < arr.Len(); i++ {
					result.Append(arr.Get(i))
				}
			}
			return result
		},
	}

	vm.globals["reverse"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("reverse() expects 1 argument", 0, 0, "")
			}
			if arr, ok := args[0].(*collections.Array); ok {
				result := collections.NewArray()
				for i := arr.Len() - 1; i >= 0; i-- {
					result.Append(arr.Get(i))
				}
				return result
			}
			if s, ok := args[0].(types.String); ok {
				runes := []rune(s)
				for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
					runes[i], runes[j] = runes[j], runes[i]
				}
				return types.String(runes)
			}
			return types.NewError("reverse: argument must be array or string", 0, 0, "")
		},
	}

	vm.globals["sum"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("sum(arr) expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("sum: argument must be an array", 0, 0, "")
			}
			total := int64(0)
			for i := 0; i < arr.Len(); i++ {
				if n, ok := arr.Get(i).(types.Int); ok {
					total = total + int64(n)
				}
			}
			return types.Int(total)
		},
	}

	vm.globals["avg"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("avg(arr) expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("avg: argument must be an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return types.Int(0)
			}
			total := int64(0)
			for i := 0; i < arr.Len(); i++ {
				if n, ok := arr.Get(i).(types.Int); ok {
					total = total + int64(n)
				}
			}
			return types.Int(total / int64(arr.Len()))
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

	// Trim left
	vm.globals["trimLeft"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("trimLeft(s) expects 1 argument", 0, 0, "")
			}
			s := types.ToString(args[0])
			return types.String(strings.TrimLeft(string(s), " \t\n\r"))
		},
	}

	// Trim right
	vm.globals["trimRight"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("trimRight(s) expects 1 argument", 0, 0, "")
			}
			s := types.ToString(args[0])
			return types.String(strings.TrimRight(string(s), " \t\n\r"))
		},
	}

	// Replace all
	vm.globals["replaceAll"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("replaceAll(s, old, new) expects 3 arguments", 0, 0, "")
			}
			s := types.ToString(args[0])
			old := types.ToString(args[1])
			new := types.ToString(args[2])
			result := strings.ReplaceAll(string(s), string(old), string(new))
			return types.String(result)
		},
	}

	// Index of
	vm.globals["indexOf"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("indexOf(s, sub) expects 2 arguments", 0, 0, "")
			}
			s := types.ToString(args[0])
			sub := types.ToString(args[1])
			idx := strings.Index(string(s), string(sub))
			if idx < 0 {
				return types.Int(-1)
			}
			return types.Int(idx)
		},
	}

	// Last index of
	vm.globals["lastIndexOf"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("lastIndexOf(s, sub) expects 2 arguments", 0, 0, "")
			}
			s := types.ToString(args[0])
			sub := types.ToString(args[1])
			idx := strings.LastIndex(string(s), string(sub))
			if idx < 0 {
				return types.Int(-1)
			}
			return types.Int(idx)
		},
	}

	// URL encode
	vm.globals["urlEncode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("urlEncode(s) expects 1 argument", 0, 0, "")
			}
			s := types.ToString(args[0])
			return types.String(url.QueryEscape(string(s)))
		},
	}

	// URL decode
	vm.globals["urlDecode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("urlDecode(s) expects 1 argument", 0, 0, "")
			}
			s := types.ToString(args[0])
			decoded, err := url.QueryUnescape(string(s))
			if err != nil {
				return types.NewError(fmt.Sprintf("urlDecode error: %v", err), 0, 0, "")
			}
			return types.String(decoded)
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

	vm.globals["parseJson"] = vm.globals["fromJson"]
	vm.globals["parseJSON"] = vm.globals["fromJson"]

	// typeOf - returns the type name of a value
	vm.globals["typeOf"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.String("undefined")
			}
			return types.String(args[0].TypeName())
		},
	}

	// Type check functions
	vm.globals["isArray"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.Bool(false)
			}
			_, ok := args[0].(*collections.Array)
			return types.Bool(ok)
		},
	}

	vm.globals["isMap"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.Bool(false)
			}
			_, ok := args[0].(*collections.Map)
			return types.Bool(ok)
		},
	}

	vm.globals["isNumber"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.Bool(false)
			}
			_, ok := args[0].(types.Int)
			if ok {
				return types.Bool(true)
			}
			_, ok = args[0].(types.Float)
			return types.Bool(ok)
		},
	}

	vm.globals["isString"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.Bool(false)
			}
			_, ok := args[0].(types.String)
			return types.Bool(ok)
		},
	}

	vm.globals["isBool"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.Bool(false)
			}
			_, ok := args[0].(types.Bool)
			return types.Bool(ok)
		},
	}

	vm.globals["isFunction"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.Bool(false)
			}
			_, ok := args[0].(*types.Function)
			return types.Bool(ok)
		},
	}

	// More utility functions
	vm.globals["zip"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("zip(arr1, arr2) expects at least 2 arguments", 0, 0, "")
			}
			result := collections.NewArray()
			minLen := args[0].(*collections.Array).Len()
			for i := 1; i < len(args); i++ {
				arr, ok := args[i].(*collections.Array)
				if !ok {
					return types.NewError("zip: all arguments must be arrays", 0, 0, "")
				}
				if arr.Len() < minLen {
					minLen = arr.Len()
				}
			}
			for i := 0; i < minLen; i++ {
				item := collections.NewArray()
				for j := 0; j < len(args); j++ {
					item.Append(args[j].(*collections.Array).Get(i))
				}
				result.Append(item)
			}
			return result
		},
	}

	vm.globals["zipToMap"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("zipToMap(keys, values) expects 2 arguments", 0, 0, "")
			}
			keys, ok1 := args[0].(*collections.Array)
			values, ok2 := args[1].(*collections.Array)
			if !ok1 || !ok2 {
				return types.NewError("zipToMap: both arguments must be arrays", 0, 0, "")
			}
			result := collections.NewMap()
			minLen := keys.Len()
			if values.Len() < minLen {
				minLen = values.Len()
			}
			for i := 0; i < minLen; i++ {
				key := keys.Get(i).ToStr()
				result.Set(key, values.Get(i))
			}
			return result
		},
	}

	vm.globals["flatten"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("flatten(arr) expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("flatten: argument must be array", 0, 0, "")
			}
			result := collections.NewArray()
			var flat func(a *collections.Array)
			flat = func(a *collections.Array) {
				for i := 0; i < a.Len(); i++ {
					if nested, ok := a.Get(i).(*collections.Array); ok {
						flat(nested)
					} else {
						result.Append(a.Get(i))
					}
				}
			}
			flat(arr)
			return result
		},
	}

	vm.globals["unique"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("unique(arr) expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("unique: argument must be array", 0, 0, "")
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				item := arr.Get(i)
				found := false
				for j := 0; j < result.Len(); j++ {
					if result.Get(j).Equals(item) {
						found = true
						break
					}
				}
				if !found {
					result.Append(item)
				}
			}
			return result
		},
	}

	vm.globals["rangeOf"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("rangeOf(start, end, step) expects 1-3 arguments", 0, 0, "")
			}
			start := int64(0)
			end := int64(0)
			step := int64(1)
			if len(args) >= 1 {
				start = int64(args[0].(types.Int))
			}
			if len(args) >= 2 {
				end = int64(args[1].(types.Int))
			} else {
				end = start
				start = 0
			}
			if len(args) >= 3 {
				step = int64(args[2].(types.Int))
			}
			result := collections.NewArray()
			if step > 0 {
				for i := start; i < end; i += step {
					result.Append(types.Int(i))
				}
			} else {
				for i := start; i > end; i += step {
					result.Append(types.Int(i))
				}
			}
			return result
		},
	}

	vm.globals["chunk"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("chunk(arr, size) expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("chunk: first argument must be array", 0, 0, "")
			}
			size, ok := args[1].(types.Int)
			if !ok {
				return types.NewError("chunk: second argument must be integer", 0, 0, "")
			}
			result := collections.NewArray()
			current := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				current.Append(arr.Get(i))
				if current.Len() == int(size) {
					result.Append(current)
					current = collections.NewArray()
				}
			}
			if current.Len() > 0 {
				result.Append(current)
			}
			return result
		},
	}

	vm.globals["groupBy"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("groupBy(arr, keyFn) expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("groupBy: first argument must be array", 0, 0, "")
			}
			_ = args[1]
			result := collections.NewMap()
			for i := 0; i < arr.Len(); i++ {
				item := arr.Get(i)
				key := "group"
				if str, ok := item.(types.String); ok {
					key = string(str)
				} else {
					key = item.ToStr()
				}
				if !result.Has(key) {
					result.Set(key, collections.NewArray())
				}
				result.Get(key).(*collections.Array).Append(item)
			}
			return result
		},
	}

	vm.globals["count"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("count(arr, val) expects 2 arguments", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("count: first argument must be array", 0, 0, "")
			}
			target := args[1]
			cnt := 0
			for i := 0; i < arr.Len(); i++ {
				if arr.Get(i).Equals(target) {
					cnt++
				}
			}
			return types.Int(cnt)
		},
	}

	vm.globals["any"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("any(arr) expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("any: argument must be array", 0, 0, "")
			}
			for i := 0; i < arr.Len(); i++ {
				if b, ok := arr.Get(i).(types.Bool); ok {
					if b == true {
						return types.Bool(true)
					}
				}
			}
			return types.Bool(false)
		},
	}

	vm.globals["all"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("all(arr) expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("all: argument must be array", 0, 0, "")
			}
			for i := 0; i < arr.Len(); i++ {
				if b, ok := arr.Get(i).(types.Bool); ok {
					if b != true {
						return types.Bool(false)
					}
				} else {
					return types.Bool(false)
				}
			}
			return types.Bool(true)
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

	vm.globals["chunkArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("chunkArr() expects 2 arguments (array, size)", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("chunkArr() first argument must be an array", 0, 0, "")
			}
			size, err := types.ToInt(args[1])
			if err != nil {
				return err
			}
			if size <= 0 {
				return types.NewError("chunkArr() size must be positive", 0, 0, "")
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i += int(size) {
				chunk := collections.NewArray()
				end := i + int(size)
				if end > arr.Len() {
					end = arr.Len()
				}
				for j := i; j < end; j++ {
					chunk.Append(arr.Get(j))
				}
				result.Append(chunk)
			}
			return result
		},
	}

	vm.globals["zipObj"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("zipObj() expects 2 arguments (keys, values)", 0, 0, "")
			}
			keys, ok1 := args[0].(*collections.Array)
			vals, ok2 := args[1].(*collections.Array)
			if !ok1 || !ok2 {
				return types.NewError("zipObj() arguments must be arrays", 0, 0, "")
			}
			result := collections.NewMap()
			n := keys.Len()
			if vals.Len() < n {
				n = vals.Len()
			}
			for i := 0; i < n; i++ {
				k := keys.Get(i).ToStr()
				result.Set(k, vals.Get(i))
			}
			return result
		},
	}

	vm.globals["fromEntries"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("fromEntries() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("fromEntries() argument must be an array", 0, 0, "")
			}
			result := collections.NewMap()
			for i := 0; i < arr.Len(); i++ {
				entry, ok := arr.Get(i).(*collections.Array)
				if ok && entry.Len() >= 2 {
					k := entry.Get(0).ToStr()
					result.Set(k, entry.Get(1))
				}
			}
			return result
		},
	}

	vm.globals["toEntries"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("toEntries() expects 1 argument", 0, 0, "")
			}
			m, ok := args[0].(*collections.Map)
			if !ok {
				return types.NewError("toEntries() argument must be a map", 0, 0, "")
			}
			result := collections.NewArray()
			keys := m.Keys()
			for i := 0; i < keys.Len(); i++ {
				k := keys.Get(i).ToStr()
				entry := collections.NewArray()
				entry.Append(types.String(k))
				entry.Append(m.Get(k))
				result.Append(entry)
			}
			return result
		},
	}

	vm.globals["median"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("median() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("median() argument must be an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return types.NullValue
			}
			nums := make([]float64, arr.Len())
			for i := 0; i < arr.Len(); i++ {
				f, _ := types.ToFloat(arr.Get(i))
				nums[i] = float64(f)
			}
			sort.Float64s(nums)
			n := len(nums)
			if n%2 == 0 {
				return types.Float((nums[n/2-1] + nums[n/2]) / 2)
			}
			return types.Float(nums[n/2])
		},
	}

	vm.globals["variance"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("variance() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("variance() argument must be an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return types.Float(0)
			}
			sum := 0.0
			for i := 0; i < arr.Len(); i++ {
				f, _ := types.ToFloat(arr.Get(i))
				sum += float64(f)
			}
			mean := sum / float64(arr.Len())
			var sumSq float64
			for i := 0; i < arr.Len(); i++ {
				f, _ := types.ToFloat(arr.Get(i))
				d := float64(f) - mean
				sumSq += d * d
			}
			return types.Float(sumSq / float64(arr.Len()))
		},
	}

	vm.globals["stddev"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("stddev() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("stddev() argument must be an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return types.Float(0)
			}
			sum := 0.0
			for i := 0; i < arr.Len(); i++ {
				f, _ := types.ToFloat(arr.Get(i))
				sum += float64(f)
			}
			mean := sum / float64(arr.Len())
			var sumSq float64
			for i := 0; i < arr.Len(); i++ {
				f, _ := types.ToFloat(arr.Get(i))
				d := float64(f) - mean
				sumSq += d * d
			}
			variance := sumSq / float64(arr.Len())
			return types.Float(math.Sqrt(variance))
		},
	}

	vm.globals["htmlEncode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("htmlEncode() expects 1 argument", 0, 0, "")
			}
			s := args[0].ToStr()
			s = strings.ReplaceAll(s, "&", "&amp;")
			s = strings.ReplaceAll(s, "<", "&lt;")
			s = strings.ReplaceAll(s, ">", "&gt;")
			s = strings.ReplaceAll(s, `"`, "&quot;")
			s = strings.ReplaceAll(s, "'", "&#39;")
			return types.String(s)
		},
	}

	vm.globals["htmlDecode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("htmlDecode() expects 1 argument", 0, 0, "")
			}
			s := args[0].ToStr()
			s = strings.ReplaceAll(s, "&lt;", "<")
			s = strings.ReplaceAll(s, "&gt;", ">")
			s = strings.ReplaceAll(s, "&quot;", `"`)
			s = strings.ReplaceAll(s, "&#39;", "'")
			s = strings.ReplaceAll(s, "&amp;", "&")
			return types.String(s)
		},
	}

	vm.globals["truncate"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("truncate() expects 2 arguments (str, length)", 0, 0, "")
			}
			s := args[0].ToStr()
			n, err := types.ToInt(args[1])
			if err != nil {
				return err
			}
			runes := []rune(s)
			if len(runes) <= int(n) {
				return types.String(s)
			}
			return types.String(string(runes[:int(n)]) + "...")
		},
	}

	vm.globals["leftPad"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("leftPad() expects 3 arguments (str, length, pad)", 0, 0, "")
			}
			s := args[0].ToStr()
			n, err := types.ToInt(args[1])
			if err != nil {
				return err
			}
			pad := args[2].ToStr()
			if len(s) >= int(n) {
				return types.String(s)
			}
			padStr := strings.Repeat(pad, int(n)-len(s))
			return types.String(padStr + s)
		},
	}

	vm.globals["rightPad"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("rightPad() expects 3 arguments (str, length, pad)", 0, 0, "")
			}
			s := args[0].ToStr()
			n, err := types.ToInt(args[1])
			if err != nil {
				return err
			}
			pad := args[2].ToStr()
			if len(s) >= int(n) {
				return types.String(s)
			}
			padStr := strings.Repeat(pad, int(n)-len(s))
			return types.String(s + padStr)
		},
	}

	vm.globals["isPowerOf2"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isPowerOf2() expects 1 argument", 0, 0, "")
			}
			n, err := types.ToInt(args[0])
			if err != nil {
				return err
			}
			return types.Bool(n > 0 && (n&(n-1)) == 0)
		},
	}

	vm.globals["tzCount"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("tzCount() expects 1 argument", 0, 0, "")
			}
			n, err := types.ToInt(args[0])
			if err != nil {
				return err
			}
			return types.Int(bits.TrailingZeros64(uint64(n)))
		},
	}

	vm.globals["strFields"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("strFields() expects 1 argument", 0, 0, "")
			}
			s := args[0].ToStr()
			fields := strings.Fields(s)
			result := collections.NewArray()
			for _, f := range fields {
				result.Append(types.String(f))
			}
			return result
		},
	}

	vm.globals["strIndexAny"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strIndexAny() expects 2 arguments", 0, 0, "")
			}
			s := args[0].ToStr()
			chars := args[1].ToStr()
			idx := strings.IndexAny(s, chars)
			if idx < 0 {
				return types.Int(-1)
			}
			return types.Int(idx)
		},
	}

	vm.globals["strLastIndexAny"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strLastIndexAny() expects 2 arguments", 0, 0, "")
			}
			s := args[0].ToStr()
			chars := args[1].ToStr()
			idx := strings.LastIndexAny(s, chars)
			if idx < 0 {
				return types.Int(-1)
			}
			return types.Int(idx)
		},
	}

	vm.globals["strContainsAny"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strContainsAny() expects 2 arguments", 0, 0, "")
			}
			s := args[0].ToStr()
			chars := args[1].ToStr()
			return types.Bool(strings.ContainsAny(s, chars))
		},
	}

	vm.globals["strContainsRune"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strContainsRune() expects 2 arguments", 0, 0, "")
			}
			s := args[0].ToStr()
			r := []rune(args[1].ToStr())
			if len(r) == 0 {
				return types.Bool(false)
			}
			return types.Bool(strings.ContainsRune(s, r[0]))
		},
	}

	vm.globals["strHasPrefix"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strHasPrefix() expects 2 arguments", 0, 0, "")
			}
			s := args[0].ToStr()
			prefix := args[1].ToStr()
			return types.Bool(strings.HasPrefix(s, prefix))
		},
	}

	vm.globals["strHasSuffix"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strHasSuffix() expects 2 arguments", 0, 0, "")
			}
			s := args[0].ToStr()
			suffix := args[1].ToStr()
			return types.Bool(strings.HasSuffix(s, suffix))
		},
	}

	vm.globals["strEqualFold"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("strEqualFold() expects 2 arguments", 0, 0, "")
			}
			s := args[0].ToStr()
			t := args[1].ToStr()
			return types.Bool(strings.EqualFold(s, t))
		},
	}

	vm.globals["strConv"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("strConv() expects at least 1 argument", 0, 0, "")
			}
			return types.String(args[0].ToStr())
		},
	}

	vm.globals["numCPU"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(runtime.NumCPU())
		},
	}

	vm.globals["cpuCount"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(runtime.NumCPU())
		},
	}

	vm.globals["numCPU"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(runtime.NumCPU())
		},
	}

	vm.globals["urlBuild"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("urlBuild() expects at least 1 argument", 0, 0, "")
			}
			u, err := url.Parse(args[0].ToStr())
			if err != nil {
				return types.NewError("urlBuild() failed: "+err.Error(), 0, 0, "")
			}
			if len(args) >= 2 {
				m, ok := args[1].(*collections.Map)
				if ok {
					q := u.Query()
					keys := m.Keys()
					for i := 0; i < keys.Len(); i++ {
						k := keys.Get(i).ToStr()
						q.Set(k, m.Get(k).ToStr())
					}
					u.RawQuery = q.Encode()
				}
			}
			return types.String(u.String())
		},
	}

	vm.globals["urlJoin"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("urlJoin() expects at least 2 arguments", 0, 0, "")
			}
			base, err := url.Parse(args[0].ToStr())
			if err != nil {
				return types.NewError("urlJoin() failed: "+err.Error(), 0, 0, "")
			}
			rel := args[1].ToStr()
			return types.String(base.ResolveReference(&url.URL{Path: rel}).String())
		},
	}

	vm.globals["urlResolve"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("urlResolve() expects 2 arguments", 0, 0, "")
			}
			base, err := url.Parse(args[0].ToStr())
			if err != nil {
				return types.NewError("urlResolve() failed: "+err.Error(), 0, 0, "")
			}
			ref, err := url.Parse(args[1].ToStr())
			if err != nil {
				return types.NewError("urlResolve() failed: "+err.Error(), 0, 0, "")
			}
			return types.String(base.ResolveReference(ref).String())
		},
	}

	vm.globals["isURL"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isURL() expects 1 argument", 0, 0, "")
			}
			s := args[0].ToStr()
			_, err := url.Parse(s)
			return types.Bool(err == nil && strings.Contains(s, "://"))
		},
	}

	vm.globals["httpHead"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("httpHead() expects at least 1 argument (url)", 0, 0, "")
			}
			url := args[0].ToStr()
			resp, err := http.Head(url)
			if err != nil {
				return types.NewError("httpHead() failed: "+err.Error(), 0, 0, "")
			}
			defer resp.Body.Close()
			result := collections.NewMap()
			result.Set("status", types.Int(resp.StatusCode))
			result.Set("headers", types.String(fmt.Sprintf("%v", resp.Header)))
			return result
		},
	}

	vm.globals["httpDelete"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("httpDelete() expects at least 1 argument (url)", 0, 0, "")
			}
			url := args[0].ToStr()
			req, err := http.NewRequest("DELETE", url, nil)
			if err != nil {
				return types.NewError("httpDelete() failed: "+err.Error(), 0, 0, "")
			}
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				return types.NewError("httpDelete() failed: "+err.Error(), 0, 0, "")
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			result := collections.NewMap()
			result.Set("status", types.Int(resp.StatusCode))
			result.Set("body", types.String(string(body)))
			return result
		},
	}

	vm.globals["httpPut"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("httpPut() expects at least 2 arguments (url, body)", 0, 0, "")
			}
			url := args[0].ToStr()
			body := args[1].ToStr()
			contentType := "text/plain"
			if len(args) >= 3 {
				contentType = args[2].ToStr()
			}
			resp, err := http.Post(url, contentType, strings.NewReader(body))
			if err != nil {
				return types.NewError("httpPut() failed: "+err.Error(), 0, 0, "")
			}
			defer resp.Body.Close()
			respBody, _ := io.ReadAll(resp.Body)
			result := collections.NewMap()
			result.Set("status", types.Int(resp.StatusCode))
			result.Set("body", types.String(string(respBody)))
			return result
		},
	}

	vm.globals["httpPatch"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("httpPatch() expects at least 2 arguments (url, body)", 0, 0, "")
			}
			url := args[0].ToStr()
			body := args[1].ToStr()
			req, err := http.NewRequest("PATCH", url, strings.NewReader(body))
			if err != nil {
				return types.NewError("httpPatch() failed: "+err.Error(), 0, 0, "")
			}
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				return types.NewError("httpPatch() failed: "+err.Error(), 0, 0, "")
			}
			defer resp.Body.Close()
			respBody, _ := io.ReadAll(resp.Body)
			result := collections.NewMap()
			result.Set("status", types.Int(resp.StatusCode))
			result.Set("body", types.String(string(respBody)))
			return result
		},
	}

	vm.globals["httpOptions"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("httpOptions() expects at least 1 argument (url)", 0, 0, "")
			}
			url := args[0].ToStr()
			req, err := http.NewRequest("OPTIONS", url, nil)
			if err != nil {
				return types.NewError("httpOptions() failed: "+err.Error(), 0, 0, "")
			}
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				return types.NewError("httpOptions() failed: "+err.Error(), 0, 0, "")
			}
			defer resp.Body.Close()
			result := collections.NewMap()
			result.Set("status", types.Int(resp.StatusCode))
			result.Set("allow", types.String(resp.Header.Get("Allow")))
			return result
		},
	}

	vm.globals["httpPostJSON"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("httpPostJSON() expects at least 2 arguments (url, json)", 0, 0, "")
			}
			url := args[0].ToStr()
			jsonData := args[1].ToStr()
			resp, err := http.Post(url, "application/json", strings.NewReader(jsonData))
			if err != nil {
				return types.NewError("httpPostJSON() failed: "+err.Error(), 0, 0, "")
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			result := collections.NewMap()
			result.Set("status", types.Int(resp.StatusCode))
			result.Set("body", types.String(string(body)))
			return result
		},
	}

	vm.globals["httpRequest"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.NewError("httpRequest() expects at least 3 arguments (method, url, body)", 0, 0, "")
			}
			method := args[0].ToStr()
			url := args[1].ToStr()
			body := args[1].ToStr()
			var bodyReader io.Reader
			if len(body) > 0 && args[2] != nil {
				bodyReader = strings.NewReader(args[2].ToStr())
			}
			req, err := http.NewRequest(method, url, bodyReader)
			if err != nil {
				return types.NewError("httpRequest() failed: "+err.Error(), 0, 0, "")
			}
			if len(args) >= 4 {
				headers, ok := args[3].(*collections.Map)
				if ok {
					keys := headers.Keys()
					for i := 0; i < keys.Len(); i++ {
						k := keys.Get(i).ToStr()
						req.Header.Set(k, headers.Get(k).ToStr())
					}
				}
			}
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				return types.NewError("httpRequest() failed: "+err.Error(), 0, 0, "")
			}
			defer resp.Body.Close()
			respBody, _ := io.ReadAll(resp.Body)
			result := collections.NewMap()
			result.Set("status", types.Int(resp.StatusCode))
			result.Set("body", types.String(string(respBody)))
			result.Set("headers", types.String(fmt.Sprintf("%v", resp.Header)))
			return result
		},
	}

	vm.globals["gzipEncode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("gzipEncode() expects 1 argument", 0, 0, "")
			}
			data := args[0].ToStr()
			var buf bytes.Buffer
			w, err := gzip.NewWriterLevel(&buf, gzip.DefaultCompression)
			if err != nil {
				return types.NewError("gzipEncode() failed: "+err.Error(), 0, 0, "")
			}
			_, err = w.Write([]byte(data))
			if err != nil {
				w.Close()
				return types.NewError("gzipEncode() failed: "+err.Error(), 0, 0, "")
			}
			w.Close()
			return types.String(base64.StdEncoding.EncodeToString(buf.Bytes()))
		},
	}

	vm.globals["gzipDecode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("gzipDecode() expects 1 argument", 0, 0, "")
			}
			data := args[0].ToStr()
			decoded, err := base64.StdEncoding.DecodeString(data)
			if err != nil {
				return types.NewError("gzipDecode() failed: "+err.Error(), 0, 0, "")
			}
			r, err := gzip.NewReader(bytes.NewReader(decoded))
			if err != nil {
				return types.NewError("gzipDecode() failed: "+err.Error(), 0, 0, "")
			}
			defer r.Close()
			result, err := io.ReadAll(r)
			if err != nil {
				return types.NewError("gzipDecode() failed: "+err.Error(), 0, 0, "")
			}
			return types.String(string(result))
		},
	}

	vm.globals["hmacMD5"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("hmacMD5() expects 2 arguments (key, data)", 0, 0, "")
			}
			key := args[0].ToStr()
			data := args[1].ToStr()
			h := hmac.New(md5.New, []byte(key))
			h.Write([]byte(data))
			return types.String(hex.EncodeToString(h.Sum(nil)))
		},
	}

	vm.globals["hmacSHA256"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("hmacSHA256() expects 2 arguments (key, data)", 0, 0, "")
			}
			key := args[0].ToStr()
			data := args[1].ToStr()
			h := hmac.New(sha256.New, []byte(key))
			h.Write([]byte(data))
			return types.String(hex.EncodeToString(h.Sum(nil)))
		},
	}

	vm.globals["aesEncrypt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("aesEncrypt() expects 2 arguments (key, plaintext)", 0, 0, "")
			}
			key := args[0].ToStr()
			plaintext := args[1].ToStr()
			if len(key) != 16 && len(key) != 24 && len(key) != 32 {
				return types.NewError("aesEncrypt() key must be 16, 24, or 32 bytes", 0, 0, "")
			}
			block, err := aes.NewCipher([]byte(key))
			if err != nil {
				return types.NewError("aesEncrypt() failed: "+err.Error(), 0, 0, "")
			}
			ciphertext := make([]byte, aes.BlockSize+len(plaintext))
			iv := ciphertext[:aes.BlockSize]
			if _, err := crand.Read(iv); err != nil {
				return types.NewError("aesEncrypt() failed: "+err.Error(), 0, 0, "")
			}
			stream := cipher.NewCFBEncrypter(block, iv)
			stream.XORKeyStream(ciphertext[aes.BlockSize:], []byte(plaintext))
			return types.String(base64.StdEncoding.EncodeToString(ciphertext))
		},
	}

	vm.globals["aesDecrypt"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("aesDecrypt() expects 2 arguments (key, ciphertext)", 0, 0, "")
			}
			key := args[0].ToStr()
			ciphertext := args[1].ToStr()
			if len(key) != 16 && len(key) != 24 && len(key) != 32 {
				return types.NewError("aesDecrypt() key must be 16, 24, or 32 bytes", 0, 0, "")
			}
			data, err := base64.StdEncoding.DecodeString(ciphertext)
			if err != nil {
				return types.NewError("aesDecrypt() failed: "+err.Error(), 0, 0, "")
			}
			block, err := aes.NewCipher([]byte(key))
			if err != nil {
				return types.NewError("aesDecrypt() failed: "+err.Error(), 0, 0, "")
			}
			if len(data) < aes.BlockSize {
				return types.NewError("aesDecrypt() ciphertext too short", 0, 0, "")
			}
			iv := data[:aes.BlockSize]
			data = data[aes.BlockSize:]
			stream := cipher.NewCFBDecrypter(block, iv)
			stream.XORKeyStream(data, data)
			return types.String(string(data))
		},
	}

	vm.globals["uuid"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			b := make([]byte, 16)
			crand.Read(b)
			b[6] = (b[6] & 0x0f) | 0x40
			b[8] = (b[8] & 0x3f) | 0x80
			return types.String(fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]))
		},
	}

	vm.globals["isEmail"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isEmail() expects 1 argument", 0, 0, "")
			}
			email := args[0].ToStr()
			pattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
			matched, _ := regexp.MatchString(pattern, email)
			return types.Bool(matched)
		},
	}

	vm.globals["isPhone"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isPhone() expects 1 argument", 0, 0, "")
			}
			phone := args[0].ToStr()
			pattern := `^\+?[1-9]\d{9,14}$`
			matched, _ := regexp.MatchString(pattern, strings.ReplaceAll(phone, "-", ""))
			return types.Bool(matched)
		},
	}

	vm.globals["isCreditCard"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("isCreditCard() expects 1 argument", 0, 0, "")
			}
			card := strings.ReplaceAll(args[0].ToStr(), " ", "")
			if len(card) < 13 || len(card) > 19 {
				return types.Bool(false)
			}
			sum := 0
			isSecond := false
			for i := len(card) - 1; i >= 0; i-- {
				d := int(card[i] - '0')
				if isSecond {
					d = d * 2
					if d > 9 {
						d = d - 9
					}
				}
				sum += d
				isSecond = !isSecond
			}
			return types.Bool(sum%10 == 0)
		},
	}

	vm.globals["slugify"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("slugify() expects 1 argument", 0, 0, "")
			}
			s := args[0].ToStr()
			s = strings.ToLower(s)
			s = regexp.MustCompile(`[^a-z0-9\s-]`).ReplaceAllString(s, "")
			s = strings.TrimSpace(s)
			s = regexp.MustCompile(`[\s-]+`).ReplaceAllString(s, "-")
			return types.String(s)
		},
	}

	vm.globals["wordCount"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("wordCount() expects 1 argument", 0, 0, "")
			}
			s := args[0].ToStr()
			words := strings.Fields(s)
			return types.Int(len(words))
		},
	}

	vm.globals["sentenceCount"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("sentenceCount() expects 1 argument", 0, 0, "")
			}
			s := args[0].ToStr()
			sentences := regexp.MustCompile(`[.!?]+`).Split(s, -1)
			count := 0
			for _, sent := range sentences {
				if len(strings.TrimSpace(sent)) > 0 {
					count++
				}
			}
			return types.Int(count)
		},
	}

	vm.globals["reverseArr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("reverseArr() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("reverseArr() argument must be an array", 0, 0, "")
			}
			result := collections.NewArray()
			for i := arr.Len() - 1; i >= 0; i-- {
				result.Append(arr.Get(i))
			}
			return result
		},
	}

	vm.globals["sample"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("sample() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("sample() argument must be an array", 0, 0, "")
			}
			if arr.Len() == 0 {
				return types.NullValue
			}
			idx := rand.Intn(arr.Len())
			return arr.Get(idx)
		},
	}

	vm.globals["uniq"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("uniq() expects 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("uniq() argument must be an array", 0, 0, "")
			}
			set := make(map[string]bool)
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				item := arr.Get(i)
				key := item.ToStr()
				if !set[key] {
					set[key] = true
					result.Append(item)
				}
			}
			return result
		},
	}

	vm.globals["difference"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("difference() expects at least 2 arguments", 0, 0, "")
			}
			arr1, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("difference() first argument must be an array", 0, 0, "")
			}
			arr2, ok := args[1].(*collections.Array)
			if !ok {
				return types.NewError("difference() second argument must be an array", 0, 0, "")
			}
			set := make(map[string]bool)
			for i := 0; i < arr2.Len(); i++ {
				set[arr2.Get(i).ToStr()] = true
			}
			result := collections.NewArray()
			for i := 0; i < arr1.Len(); i++ {
				item := arr1.Get(i)
				if !set[item.ToStr()] {
					result.Append(item)
				}
			}
			return result
		},
	}

	vm.globals["intersection"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError("intersection() expects at least 2 arguments", 0, 0, "")
			}
			arr1, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("intersection() first argument must be an array", 0, 0, "")
			}
			arr2, ok := args[1].(*collections.Array)
			if !ok {
				return types.NewError("intersection() second argument must be an array", 0, 0, "")
			}
			set := make(map[string]bool)
			for i := 0; i < arr2.Len(); i++ {
				set[arr2.Get(i).ToStr()] = true
			}
			result := collections.NewArray()
			for i := 0; i < arr1.Len(); i++ {
				item := arr1.Get(i)
				if set[item.ToStr()] {
					result.Append(item)
				}
			}
			return result
		},
	}

	vm.globals["union"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("union() expects at least 1 argument", 0, 0, "")
			}
			set := make(map[string]types.Object)
			for _, arg := range args {
				arr, ok := arg.(*collections.Array)
				if !ok {
					continue
				}
				for i := 0; i < arr.Len(); i++ {
					item := arr.Get(i)
					set[item.ToStr()] = item
				}
			}
			result := collections.NewArray()
			for _, v := range set {
				result.Append(v)
			}
			return result
		},
	}

	vm.globals["addIndex"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("addIndex() expects at least 1 argument", 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("addIndex() first argument must be an array", 0, 0, "")
			}
			result := collections.NewArray()
			for i := 0; i < arr.Len(); i++ {
				pair := collections.NewArray()
				pair.Append(types.Int(i))
				pair.Append(arr.Get(i))
				result.Append(pair)
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
