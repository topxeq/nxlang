package vm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/topxeq/nxlang/bytecode"
	"github.com/topxeq/nxlang/compiler"
	"github.com/topxeq/nxlang/parser"
	"github.com/topxeq/nxlang/types"
	"github.com/topxeq/nxlang/types/collections"
)

// TryFrame represents an active try/catch/finally block
type TryFrame struct {
	frameIndex    int       // The frame index where this try block is located
	catchOffset   int       // Offset of the catch block
	finallyOffset int       // Offset of the finally block
	stackPointer  int       // Stack pointer at the start of the try block
	basePointer   int       // Base pointer at the start of the try block
}

// DeferredCall represents a function call to be executed later
type DeferredCall struct {
	fn   types.Object // The function to call
	args []types.Object // Arguments to pass to the function
}

// Module represents a loaded Nxlang module
type Module struct {
	Name     string
	Path     string
	Exports  map[string]types.Object // Exported symbols from the module
}

// VM represents the Nxlang virtual machine
type VM struct {
	constants []bytecode.Constant
	stack     *Stack
	frames    []*Frame
	framePointer int // Current frame index
	globals   map[string]types.Object
	lastError *types.Error

	// Constant value cache to avoid duplicate instances
	functionCache map[int]*types.Function // Maps constant index to function instance
	classCache    map[int]*types.Class    // Maps constant index to class instance

	// Exception handling
	tryStack   []*TryFrame   // Stack of active try blocks
	deferStack [][]*DeferredCall // Stack of deferred function calls (per frame)

	// Module system support
	modules      map[string]*Module // Cache of loaded modules
	modulePaths  []string // Search paths for modules
}


// NewVM creates a new virtual machine instance
func NewVM(bc *bytecode.Bytecode) *VM {
	// Get main function
	mainFunc := bc.Constants[bc.MainFunc].(*bytecode.FunctionConstant)

	// Initialize call frames
	frames := make([]*Frame, MaxCallStackDepth)
	frames[0] = NewFrame(mainFunc, 0)

	vm := &VM{
		constants: bc.Constants,
		stack:     NewStack(),
		frames:    frames,
		framePointer: 1, // 0 is reserved for main, starts at 1 so we can push frames
		globals:   make(map[string]types.Object),
		functionCache: make(map[int]*types.Function),
		tryStack:  []*TryFrame{},
		deferStack: make([][]*DeferredCall, MaxCallStackDepth),
		modules:   make(map[string]*Module),
		modulePaths: []string{".", "./nx_modules", "/usr/local/nx/modules"}, // Default module search paths
	}

	// Register built-in functions
	vm.registerBuiltins()

	// Register standard library modules
	vm.registerStandardModules()

	return vm
}

// Globals returns the global variables map
func (vm *VM) Globals() map[string]types.Object {
	return vm.globals
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
	// Base type constants
	vm.globals["undefined"] = types.UndefinedValue
	vm.globals["null"] = types.NullValue

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

	// Type system builtins
	vm.globals["toStr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("toStr() expects 1 argument, got 0", 0, 0, "")
			}
			return types.ToStr(args[0])
		},
	}

	vm.globals["typeCode"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("typeCode() expects 1 argument, got 0", 0, 0, "")
			}
			return types.TypeCode(args[0])
		},
	}

	vm.globals["typeName"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("typeName() expects 1 argument, got 0", 0, 0, "")
			}
			return types.TypeName(args[0])
		},
	}

	vm.globals["isErr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("isErr() expects 1 argument, got 0", 0, 0, "")
			}
			return types.Bool(types.IsErr(args[0]))
		},
	}

	// Type conversion builtins
	vm.globals["int"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("int() expects 1 argument, got 0", 0, 0, "")
			}
			val, err := types.ToInt(args[0])
			if err != nil {
				return err
			}
			return val
		},
	}

	vm.globals["uint"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("uint() expects 1 argument, got 0", 0, 0, "")
			}
			val, err := types.ToInt(args[0])
			if err != nil {
				return err
			}
			return types.UInt(val)
		},
	}

	vm.globals["byte"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("byte() expects 1 argument, got 0", 0, 0, "")
			}
			val, err := types.ToByte(args[0])
			if err != nil {
				return err
			}
			return val
		},
	}

	vm.globals["char"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("char() expects 1 argument, got 0", 0, 0, "")
			}
			val, err := types.ToChar(args[0])
			if err != nil {
				return err
			}
			return val
		},
	}

	vm.globals["float"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("float() expects 1 argument, got 0", 0, 0, "")
			}
			val, err := types.ToFloat(args[0])
			if err != nil {
				return err
			}
			return val
		},
	}

	vm.globals["bool"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("bool() expects 1 argument, got 0", 0, 0, "")
			}
			return types.Bool(types.ToBool(args[0]))
		},
	}

	vm.globals["string"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.NewError("string() expects 1 argument, got 0", 0, 0, "")
			}
			return types.ToString(args[0])
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

	vm.globals["append"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.NewError(fmt.Sprintf("append() expects at least 2 arguments, got %d", len(args)), 0, 0, "")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.NewError("first argument to append() must be an array", 0, 0, "")
			}
			// Append all elements
			for _, elem := range args[1:] {
				arr.Append(elem)
			}
			return arr
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

	// 睡眠指定秒数
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
			_, ok := args[0].(*types.Error)
			return types.Bool(ok)
		},
	}
}

// registerStandardModules registers all standard library modules
func (vm *VM) registerStandardModules() {
	// Math module
	mathModule := &Module{
		Name: "math",
		Exports: map[string]types.Object{
			"abs": vm.globals["abs"],
			"sqrt": vm.globals["sqrt"],
			"sin": vm.globals["sin"],
			"cos": vm.globals["cos"],
			"tan": vm.globals["tan"],
			"floor": vm.globals["floor"],
			"ceil": vm.globals["ceil"],
			"round": vm.globals["round"],
			"pow": vm.globals["pow"],
			"random": vm.globals["random"],
		},
	}
	vm.modules["math"] = mathModule

	// String module
	stringModule := &Module{
		Name: "string",
		Exports: map[string]types.Object{
			"toUpper": vm.globals["toUpper"],
			"toLower": vm.globals["toLower"],
			"trim": vm.globals["trim"],
			"split": vm.globals["split"],
			"join": vm.globals["join"],
			"contains": vm.globals["contains"],
			"replace": vm.globals["replace"],
			"substr": vm.globals["substr"],
			"startsWith": vm.globals["startsWith"],
			"endsWith": vm.globals["endsWith"],
		},
	}
	vm.modules["string"] = stringModule

	// Collection module
	collectionModule := &Module{
		Name: "collection",
		Exports: map[string]types.Object{
			"array": vm.globals["array"],
			"append": vm.globals["append"],
			"map": vm.globals["map"],
			"orderedMap": vm.globals["orderedMap"],
			"stack": vm.globals["stack"],
			"queue": vm.globals["queue"],
			"keys": vm.globals["keys"],
			"values": vm.globals["values"],
			"delete": vm.globals["delete"],
			"sortMap": vm.globals["sortMap"],
			"reverseMap": vm.globals["reverseMap"],
			"moveKey": vm.globals["moveKey"],
			"moveKeyToFirst": vm.globals["moveKeyToFirst"],
			"moveKeyToLast": vm.globals["moveKeyToLast"],
		},
	}
	vm.modules["collection"] = collectionModule

	// Time module
	timeModule := &Module{
		Name: "time",
		Exports: map[string]types.Object{
			"now": vm.globals["now"],
			"unix": vm.globals["unix"],
			"unixMilli": vm.globals["unixMilli"],
			"formatTime": vm.globals["formatTime"],
			"parseTime": vm.globals["parseTime"],
			"sleep": vm.globals["sleep"],
		},
	}
	vm.modules["time"] = timeModule

	// JSON module
	jsonModule := &Module{
		Name: "json",
		Exports: map[string]types.Object{
			"toJson": vm.globals["toJson"],
			"fromJson": nil, // TODO: Implement fromJson
		},
	}
	vm.modules["json"] = jsonModule

	// Thread/concurrency module
	threadModule := &Module{
		Name: "thread",
		Exports: map[string]types.Object{
			"thread": vm.globals["thread"],
			"mutex": vm.globals["mutex"],
			"rwMutex": vm.globals["rwMutex"],
		},
	}
	vm.modules["thread"] = threadModule
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

		if err := vm.executeOpcode(op, currentFrame); err != nil {
			return err
		}

		if vm.lastError != nil {
			return vm.lastError
		}
	}

	return nil
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
		val, ok := vm.globals[nameConst.Value]
		if !ok {
			return vm.newError(fmt.Sprintf("undefined variable: %s", nameConst.Value), frame.ip)
		}
		return vm.stack.Push(val)

	case compiler.OpStoreGlobal:
		nameIdx := int(frame.ReadUint16())
		nameConst := vm.constants[nameIdx].(*bytecode.StringConstant)
		val := vm.stack.Pop()
		vm.globals[nameConst.Value] = val

	case compiler.OpAdd:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		// Auto convert to match types (left-side priority)
		convA, convB, convErr := types.AutoConvert(a, b)
		if convErr != nil {
			return vm.newError(convErr.Message, frame.ip)
		}
		res, err := vm.addObjects(convA, convB)
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

	case compiler.OpDiv:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		res, err := vm.divObjects(a, b)
		if err != nil {
			return err
		}
		return vm.stack.Push(res)

	case compiler.OpMod:
		b := vm.stack.Pop()
		a := vm.stack.Pop()
		res, err := vm.modObjects(a, b)
		if err != nil {
			return err
		}
		return vm.stack.Push(res)

	case compiler.OpNeg:
		a := vm.stack.Pop()
		res, err := vm.negObject(a)
		if err != nil {
			return err
		}
		return vm.stack.Push(res)

	case compiler.OpNot:
		a := vm.stack.Pop()
		res := types.Bool(!types.ToBool(a))
		return vm.stack.Push(res)

	case compiler.OpIsNil:
		a := vm.stack.Pop()
		// Check if value is nil or undefined
		isNil := a == nil || a.TypeCode() == types.TypeUndefined
		res := types.Bool(isNil)
		return vm.stack.Push(res)

	case compiler.OpBitNot:
		a := vm.stack.Pop()
		res, err := vm.bitNotObject(a)
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
		if stackSize < argCount + 1 {
			return vm.newError(fmt.Sprintf("not enough arguments for function call: expected %d, got %d", argCount, stackSize - 1), frame.ip)
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
		case *types.BoundMethod:
			// Bound method call: prepend 'this' as first argument only if it's not a static method
			var boundArgs []types.Object
			if fn.Method.IsStatic {
				boundArgs = args // Static method doesn't need this
			} else {
				boundArgs = make([]types.Object, 0, len(args)+1)
				boundArgs = append(boundArgs, fn.Instance)
				boundArgs = append(boundArgs, args...)
			}
			// Recursively call with the bound function and new args
			vm.stack.Push(fn.Method)
			for _, arg := range boundArgs {
				vm.stack.Push(arg)
			}
			// Emit OpCall with new arg count (will be processed in next iteration)
			frame.ip -= 2 // Rewind to re-execute OpCall
			return nil

		case *types.NativeFunction:
			// Native function call
			result := fn.Fn(args...)
			return vm.stack.Push(result)

		case *types.Function:
			// Nxlang function call
			processedArgs := make([]types.Object, fn.NumParameters)

			// Copy provided arguments
			for i := 0; i < argCount && i < fn.NumParameters; i++ {
				processedArgs[i] = args[i]
			}

			// Fill default values for missing arguments
			for i := argCount; i < fn.NumParameters; i++ {
				if fn.DefaultValues == nil || fn.DefaultValues[i] == -1 {
					if !fn.IsVariadic {
						return vm.newError(fmt.Sprintf("expected at least %d arguments, got %d", i+1, argCount), frame.ip)
					}
					// For variadic functions, the last parameter is the variadic array
					break
				}
				// Get default value from constant pool
				defaultConst := fn.ConstantPool[fn.DefaultValues[i]]
				defaultVal, err := vm.constantToObject(fn.DefaultValues[i], defaultConst)
				if err != nil {
					return err
				}
				processedArgs[i] = defaultVal
			}

			// Handle variadic parameters
			if fn.IsVariadic {
				// The last parameter is the variadic array
				variadicIdx := fn.NumParameters - 1
				var variadicArgs []types.Object

				if argCount > fn.NumParameters {
					// Collect all extra arguments
					variadicArgs = args[fn.NumParameters-1:]
				} else if argCount == fn.NumParameters {
					// User provided an array for the variadic parameter
					// Check if it's already an array
					if args[variadicIdx] != nil {
						if arr, ok := args[variadicIdx].(*collections.Array); ok {
							// Already an array, use it directly
							processedArgs[variadicIdx] = arr
						} else {
							// Wrap single value in array
							variadicArgs = []types.Object{args[variadicIdx]}
						}
					}
				}

				// If we have variadic args, create array
				if variadicArgs != nil {
					processedArgs[variadicIdx] = collections.NewArrayWithElements(variadicArgs)
				}

				// Ensure we always have an array for variadic parameter
				if processedArgs[variadicIdx] == nil {
					// No variadic args provided, create empty array
					processedArgs[variadicIdx] = collections.NewArrayWithElements([]types.Object{})
				}
			}

			// Create bytecode function constant for frame
			bcFunc := &bytecode.FunctionConstant{
				Name:          fn.Name,
				NumLocals:     fn.NumLocals,
				NumParameters: fn.NumParameters,
				IsVariadic:    fn.IsVariadic,
				Instructions:  fn.Instructions,
				DefaultValues: fn.DefaultValues,
			}

			// Create new frame
			basePointer := vm.stack.Size()
			newFrame := NewFrame(bcFunc, basePointer)

			// Copy processed arguments to locals
			for i := 0; i < fn.NumParameters; i++ {
				newFrame.locals[i] = processedArgs[i]
			}

			if vm.framePointer >= MaxCallStackDepth {
				return &StackOverflowError{}
			}

			vm.frames[vm.framePointer] = newFrame
			vm.framePointer++

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
		// If we're returning from main, exit gracefully
		if vm.framePointer == 0 {
			// Execution complete
			return nil
		}
		// Clear stack up to callee's base pointer
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
		// Clear stack up to callee's base pointer
		for vm.stack.Size() > calleeFrame.basePointer {
			vm.stack.Pop()
		}
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

		// Pop arguments in reverse order
		args := make([]types.Object, argCount)
		for i := argCount - 1; i >= 0; i-- {
			args[i] = vm.stack.Pop()
		}

		// Pop class from stack
		classVal := vm.stack.Pop()
		class, ok := classVal.(*types.Class)
		if !ok {
			return vm.newError("cannot instantiate non-class type", frame.ip)
		}

		// Create new instance
		instance := &types.Instance{
			Class:      class,
			Properties: make(map[string]types.Object),
		}

		// Call constructor if exists
		if initMethod, ok := class.Methods["init"]; ok {
			// Push instance as 'this' argument
			allArgs := append([]types.Object{instance}, args...)

			// Create frame for constructor call
			bcFunc := &bytecode.FunctionConstant{
				Name:          initMethod.Name,
				NumLocals:     initMethod.NumLocals,
				NumParameters: initMethod.NumParameters,
				IsVariadic:    initMethod.IsVariadic,
				Instructions:  initMethod.Instructions,
				DefaultValues: initMethod.DefaultValues,
			}

			basePointer := vm.stack.Size()
			newFrame := NewFrame(bcFunc, basePointer)

			// Copy arguments to locals (first local is 'this')
			for i := 0; i < len(allArgs) && i < initMethod.NumParameters; i++ {
				newFrame.locals[i] = allArgs[i]
			}

			// Fill default values for missing parameters
			for i := len(allArgs); i < initMethod.NumParameters; i++ {
				if initMethod.DefaultValues != nil && initMethod.DefaultValues[i] != -1 {
					defaultConst := initMethod.ConstantPool[initMethod.DefaultValues[i]]
					defaultVal, err := vm.constantToObject(initMethod.DefaultValues[i], defaultConst)
					if err != nil {
						return err
					}
					newFrame.locals[i] = defaultVal
				} else {
					return vm.newError(fmt.Sprintf("constructor expected %d arguments, got %d", initMethod.NumParameters, argCount), frame.ip)
				}
			}

			// Push constructor frame
			if vm.framePointer >= MaxCallStackDepth {
				return &StackOverflowError{}
			}
			vm.frames[vm.framePointer] = newFrame
			vm.framePointer++
		}

		// Push the new instance to stack
		return vm.stack.Push(instance)

	case compiler.OpPrintLine:
		val := vm.stack.Pop()
		fmt.Fprintln(os.Stdout, val.ToStr())
		return vm.stack.Push(types.UndefinedValue)

	case compiler.OpPrint:
		val := vm.stack.Pop()
		fmt.Fprint(os.Stdout, val.ToStr())
		return vm.stack.Push(types.UndefinedValue)

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

	case compiler.OpTypeCode:
		obj := vm.stack.Pop()
		return vm.stack.Push(types.Int(obj.TypeCode()))

	case compiler.OpTypeName:
		obj := vm.stack.Pop()
		return vm.stack.Push(types.String(obj.TypeName()))

	case compiler.OpIsError:
		obj := vm.stack.Pop()
		_, isErr := obj.(*types.Error)
		return vm.stack.Push(types.Bool(isErr))

	case compiler.OpMemberGet:
		nameIdx := int(frame.ReadUint16())
		nameConst := vm.constants[nameIdx].(*bytecode.StringConstant)
		memberName := nameConst.Value

		// Pop object from stack
		objVal := vm.stack.Pop()

		// Check if it's a class instance
		if instance, ok := objVal.(*types.Instance); ok {
			// Check for getter method first
			getterName := "get" + strings.ToUpper(memberName[:1]) + memberName[1:]
			if getter, ok := instance.Class.Methods[getterName]; ok && getter.IsGetter {
				// Call getter method automatically
				boundGetter := &types.BoundMethod{
					Instance: instance,
					Method:   getter,
				}
				vm.stack.Push(boundGetter)
				// Rewind IP to call the getter
				frame.ip -= 2
				// Dummy arg count 0 for getter
				vm.stack.Push(types.Int(0))
				return nil
			}

			// Check instance properties
			if val, ok := instance.Properties[memberName]; ok {
				return vm.stack.Push(val)
			}
			// Check class methods
			if method, ok := instance.Class.Methods[memberName]; ok {
				// Return bound method with instance as 'this'
				boundMethod := &types.BoundMethod{
					Instance: instance,
					Method:   method,
				}
				return vm.stack.Push(boundMethod)
			}
			// If not found, return undefined
			return vm.stack.Push(types.UndefinedValue)
		}

		// Check if it's a super reference
		if superRef, ok := objVal.(*types.SuperReference); ok {
			// Look up method on the superclass
			if method, ok := superRef.Super.Methods[memberName]; ok {
				// Return bound method with the current instance as 'this'
				boundMethod := &types.BoundMethod{
					Instance: superRef.Instance,
					Method:   method,
				}
				return vm.stack.Push(boundMethod)
			}
			// Check instance properties (same as normal member access)
			if val, ok := superRef.Instance.Properties[memberName]; ok {
				return vm.stack.Push(val)
			}
			// If not found, return undefined
			return vm.stack.Push(types.UndefinedValue)
		}

		// Check if it's a class
		if class, ok := objVal.(*types.Class); ok {
			// First check static fields
			if val, ok := class.StaticFields[memberName]; ok {
				return vm.stack.Push(val)
			}
			// Then look up static methods
			if method, ok := class.Methods[memberName]; ok {
				if method.IsStatic {
					// Static method: return plain function, no binding needed
					return vm.stack.Push(method)
				}
				// Instance method called on class: return undefined or error?
				// For now return undefined, could add warning later
			}
			// If not found, return undefined
			return vm.stack.Push(types.UndefinedValue)
		}

		return vm.newError(fmt.Sprintf("cannot get member '%s' of non-object type %s", memberName, objVal.TypeName()), frame.ip)

	case compiler.OpMemberSet:
		nameIdx := int(frame.ReadUint16())
		nameConst := vm.constants[nameIdx].(*bytecode.StringConstant)
		memberName := nameConst.Value

		// Pop value and object from stack
		value := vm.stack.Pop()
		objVal := vm.stack.Pop()

		// Check if it's a class instance
		if instance, ok := objVal.(*types.Instance); ok {
			// Check for setter method first
			setterName := "set" + strings.ToUpper(memberName[:1]) + memberName[1:]
			if setter, ok := instance.Class.Methods[setterName]; ok && setter.IsSetter {
				// Call setter method automatically with the value
				boundSetter := &types.BoundMethod{
					Instance: instance,
					Method:   setter,
				}
				vm.stack.Push(boundSetter)
				vm.stack.Push(value)
				// Rewind IP to call the setter with 1 argument
				frame.ip -= 2
				// Arg count 1 for setter
				vm.stack.Push(types.Int(1))
				return nil
			}

			// Set property on instance
			instance.Properties[memberName] = value
			return vm.stack.Push(value)
		}

		// Check if it's a class (for static properties)
		if class, ok := objVal.(*types.Class); ok {
			// Initialize static fields map if needed
			if class.StaticFields == nil {
				class.StaticFields = make(map[string]types.Object)
			}
			// Set static property on class
			class.StaticFields[memberName] = value
			return vm.stack.Push(value)
		}

		return vm.newError(fmt.Sprintf("cannot set member '%s' of non-object type %s", memberName, objVal.TypeName()), frame.ip)

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

	case compiler.OpLen:
		obj := vm.stack.Pop()
		switch val := obj.(type) {
		case *collections.Array:
			return vm.stack.Push(types.Int(val.Len()))
		case types.String:
			// Return rune count for UTF-8 strings
			runes := []rune(string(val))
			return vm.stack.Push(types.Int(len(runes)))
		case *collections.Map:
			return vm.stack.Push(types.Int(val.Len()))
		default:
			return vm.newError(fmt.Sprintf("cannot get length of type %s", obj.TypeName()), frame.ip)
		}

	case compiler.OpGetSuper:
		obj := vm.stack.Pop()
		instance, ok := obj.(*types.Instance)
		if !ok {
			return vm.newError(fmt.Sprintf("super keyword only valid on class instances, got %s", obj.TypeName()), frame.ip)
		}
		if instance.Class.SuperClass == nil {
			return vm.stack.Push(types.NullValue)
		}
		// Return super reference containing both instance and superclass
		superRef := &types.SuperReference{
			Instance: instance,
			Super:    instance.Class.SuperClass,
		}
		return vm.stack.Push(superRef)

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
			IsStatic:      constType.IsStatic,
			AccessModifier: constType.AccessModifier,
			IsGetter:      (constType.Flags & 0x01) != 0,
			IsSetter:      (constType.Flags & 0x02) != 0,
			DefaultValues: constType.DefaultValues,
			Instructions:  constType.Instructions,
			ConstantPool:  vm.constants,
		}
		// Cache the function instance
		vm.functionCache[index] = fn
		return fn, nil

	case *bytecode.ClassConstant:
		// Check class cache first
		if cls, ok := vm.classCache[index]; ok {
			return cls, nil
		}
		// Convert bytecode class to types.Class
		cls := &types.Class{
			Name:         constType.Name,
			Methods:      make(map[string]*types.Function),
			StaticFields: make(map[string]types.Object),
		}

		// Resolve superclass if present
		if constType.SuperClass != "" {
			// Look up superclass in global scope
			superVal, ok := vm.globals[constType.SuperClass]
			if !ok {
				return nil, vm.newError(fmt.Sprintf("superclass %s not found", constType.SuperClass), 0)
			}
			superCls, ok := superVal.(*types.Class)
			if !ok {
				return nil, vm.newError(fmt.Sprintf("%s is not a class", constType.SuperClass), 0)
			}
			cls.SuperClass = superCls

			// Inherit methods from superclass
			for name, method := range superCls.Methods {
				cls.Methods[name] = method
			}
		}

		// Add own methods (override inherited ones)
		for methodName, funcIdx := range constType.Methods {
			// Resolve method function from constant pool
			funcConst := vm.constants[funcIdx]
			methodObj, err := vm.constantToObject(funcIdx, funcConst)
			if err != nil {
				return nil, err
			}
			method, ok := methodObj.(*types.Function)
			if !ok {
				return nil, vm.newError(fmt.Sprintf("method %s is not a function", methodName), 0)
			}
			method.OwnerClass = cls // Set the owner class of this method
			cls.Methods[methodName] = method
		}

		// Cache the class instance
		if vm.classCache == nil {
			vm.classCache = make(map[int]*types.Class)
		}
		vm.classCache[index] = cls
		return cls, nil

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
	// Check if module is already loaded
	if mod, ok := vm.modules[modulePath]; ok {
		return mod, nil
	}

	// Check if it's a standard library module (already registered in vm.modules)
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
	var line int
	// TODO: Use line number table to find line from IP
	stack := vm.collectCallStack()
	err := types.NewErrorWithStack(message, line, 0, "", stack)
	vm.lastError = err
	return err
}


// runDeferred runs all deferred functions for the given frame index
func (vm *VM) runDeferred(frameIndex int) {
	deferred := vm.deferStack[frameIndex]
	// Run deferred functions in reverse order (LIFO)
	for i := len(deferred) - 1; i >= 0; i-- {
		call := deferred[i]
		// Execute the call
		// Push arguments
		for _, arg := range call.args {
			vm.stack.Push(arg)
		}
		// Push function
		vm.stack.Push(call.fn)
		// Emit call opcode
		// TODO: Implement proper function call execution
		// For now, just pop them
		vm.stack.Pop() // fn
		for range call.args {
			vm.stack.Pop() // args
		}
	}
	// Clear deferred functions for this frame
	vm.deferStack[frameIndex] = nil
}

// Comparison operators
const (
	lt = iota
	lte
	gt
	gte
)
