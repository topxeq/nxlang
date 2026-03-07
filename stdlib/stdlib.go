package stdlib

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/topxeq/nxlang/types"
	"github.com/topxeq/nxlang/types/collections"
	"github.com/topxeq/nxlang/types/concurrency"
	"github.com/topxeq/nxlang/vm"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// RegisterAll registers all standard library functions in the VM's global scope
func RegisterAll(vm *vm.VM) {
	registerMathFunctions(vm)
	registerStringFunctions(vm)
	registerCollectionFunctions(vm)
	registerTimeFunctions(vm)
	registerJSONFunctions(vm)
	registerThreadFunctions(vm)
}

// Math functions
func registerMathFunctions(vm *vm.VM) {
	vm.Globals()["abs"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.Float(0)
			}
			f, err := types.ToFloat(args[0])
			if err != nil {
				return err
			}
			return types.Float(math.Abs(float64(f)))
		},
	}

	vm.Globals()["sqrt"] = &types.NativeFunction{
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

	vm.Globals()["sin"] = &types.NativeFunction{
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

	vm.Globals()["cos"] = &types.NativeFunction{
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

	vm.Globals()["tan"] = &types.NativeFunction{
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

	vm.Globals()["floor"] = &types.NativeFunction{
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

	vm.Globals()["ceil"] = &types.NativeFunction{
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

	vm.Globals()["round"] = &types.NativeFunction{
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

	vm.Globals()["pow"] = &types.NativeFunction{
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

	vm.Globals()["random"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Float(rand.Float64())
		},
	}
}

// String functions
func registerStringFunctions(vm *vm.VM) {
	vm.Globals()["len"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.Int(0)
			}
			switch v := args[0].(type) {
			case types.String:
				return types.Int(len(v))
			case *collections.Array:
				return types.Int(v.Len())
			case *collections.Map:
				return types.Int(v.Len())
			case *collections.OrderedMap:
				return types.Int(v.Len())
			case *collections.Stack:
				return types.Int(v.Len())
			case *collections.Queue:
				return types.Int(v.Len())
			default:
				return types.Int(0)
			}
		},
	}

	vm.Globals()["toUpper"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.String("")
			}
			s := types.ToString(args[0])
			return types.String(strings.ToUpper(string(s)))
		},
	}

	vm.Globals()["toLower"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.String("")
			}
			s := types.ToString(args[0])
			return types.String(strings.ToLower(string(s)))
		},
	}

	vm.Globals()["trim"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.String("")
			}
			s := types.ToString(args[0])
			return types.String(strings.TrimSpace(string(s)))
		},
	}

	vm.Globals()["split"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return collections.NewArray()
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

	vm.Globals()["join"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.String("")
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.String("")
			}
			sep := types.ToString(args[1])
			strParts := make([]string, arr.Len())
			for i := 0; i < arr.Len(); i++ {
				strParts[i] = arr.Get(i).ToStr()
			}
			return types.String(strings.Join(strParts, string(sep)))
		},
	}

	vm.Globals()["contains"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.Bool(false)
			}
			s := types.ToString(args[0])
			substr := types.ToString(args[1])
			return types.Bool(strings.Contains(string(s), string(substr)))
		},
	}

	vm.Globals()["replace"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 3 {
				return types.String("")
			}
			s := types.ToString(args[0])
			old := types.ToString(args[1])
			newStr := types.ToString(args[2])
			n := -1
			if len(args) > 3 {
				count, err := types.ToInt(args[3])
				if err == nil {
					n = int(count)
				}
			}
			return types.String(strings.Replace(string(s), string(old), string(newStr), n))
		},
	}

	vm.Globals()["substr"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.String("")
			}
			s := types.ToString(args[0])
			start, err := types.ToInt(args[1])
			if err != nil {
				return types.String("")
			}
			runes := []rune(string(s))
			if start < 0 {
				start = types.Int(len(runes)) + start
			}
			if start < 0 || int(start) >= len(runes) {
				return types.String("")
			}
			if len(args) > 2 {
				length, err := types.ToInt(args[2])
				if err == nil {
					end := start + length
					if end > types.Int(len(runes)) {
						end = types.Int(len(runes))
					}
					return types.String(string(runes[start:end]))
				}
			}
			return types.String(string(runes[start:]))
		},
	}
}

// Collection functions
func registerCollectionFunctions(vm *vm.VM) {
	vm.Globals()["array"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			arr := collections.NewArray()
			for _, arg := range args {
				arr.Append(arg)
			}
			return arr
		},
	}

	vm.Globals()["append"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 2 {
				return types.UndefinedValue
			}
			arr, ok := args[0].(*collections.Array)
			if !ok {
				return types.UndefinedValue
			}
			for i := 1; i < len(args); i++ {
				arr.Append(args[i])
			}
			return arr
		},
	}

	vm.Globals()["map"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			m := collections.NewMap()
			// Arguments should be key-value pairs
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

	vm.Globals()["orderedMap"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			m := collections.NewOrderedMap()
			// Arguments should be key-value pairs
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

	vm.Globals()["stack"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			s := collections.NewStack()
			for _, arg := range args {
				s.Push(arg)
			}
			return s
		},
	}

	vm.Globals()["queue"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			q := collections.NewQueue()
			for _, arg := range args {
				q.Enqueue(arg)
			}
			return q
		},
	}

	vm.Globals()["keys"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return collections.NewArray()
			}
			switch v := args[0].(type) {
			case *collections.Map:
				return v.Keys()
			case *collections.OrderedMap:
				return v.Keys()
			default:
				return collections.NewArray()
			}
		},
	}

	vm.Globals()["values"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return collections.NewArray()
			}
			switch v := args[0].(type) {
			case *collections.Map:
				return v.Values()
			case *collections.OrderedMap:
				return v.Values()
			default:
				return collections.NewArray()
			}
		},
	}
}

// Time functions
func registerTimeFunctions(vm *vm.VM) {
	vm.Globals()["now"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(time.Now().Unix())
		},
	}

	vm.Globals()["unix"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(time.Now().Unix())
		},
	}

	vm.Globals()["unixMilli"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return types.Int(time.Now().UnixMilli())
		},
	}

	// formatTime(timestamp, format) - timestamp is seconds since epoch, format is Go style (default: "2006-01-02 15:04:05")
	vm.Globals()["formatTime"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("formatTime() expects at least 1 argument (timestamp[, format])", 0, 0, "")
			}

			// Support both Int and Float timestamps
			var ts int64
			switch v := args[0].(type) {
			case types.Int:
				ts = int64(v)
			case types.Float:
				ts = int64(v)
			default:
				return types.NewError("timestamp must be a number", 0, 0, "")
			}

			format := "2006-01-02 15:04:05"
			if len(args) >= 2 {
				format = string(types.ToString(args[1]))
			}

			t := time.Unix(ts, 0)
			return types.String(t.Format(format))
		},
	}

	// parseTime(timeStr, format) - parse time string to timestamp (seconds), format defaults to "2006-01-02 15:04:05"
	vm.Globals()["parseTime"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) < 1 {
				return types.NewError("parseTime() expects at least 1 argument (timeStr[, format])", 0, 0, "")
			}

			timeStr := string(types.ToString(args[0]))
			format := "2006-01-02 15:04:05"
			if len(args) >= 2 {
				format = string(types.ToString(args[1]))
			}

			t, err := time.ParseInLocation(format, timeStr, time.Local)
			if err != nil {
				return types.NewError(fmt.Sprintf("failed to parse time: %v", err), 0, 0, "")
			}

			return types.Int(t.Unix())
		},
	}

	// sleep(seconds) - sleep for the specified number of seconds
	vm.Globals()["sleep"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.UndefinedValue
			}
			sec, err := types.ToInt(args[0])
			if err != nil {
				// Try float if int conversion fails
				if secFloat, err2 := types.ToFloat(args[0]); err2 == nil {
					time.Sleep(time.Duration(float64(secFloat) * float64(time.Second)))
					return types.UndefinedValue
				}
				return err
			}
			time.Sleep(time.Duration(sec) * time.Second)
			return types.UndefinedValue
		},
	}
}

// JSON functions
func registerJSONFunctions(vm *vm.VM) {
	vm.Globals()["toJson"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.String("{}")
			}
			// Convert Nxlang object to Go value for serialization
			var goValue interface{}
			switch v := args[0].(type) {
			case *collections.Map:
				m := make(map[string]interface{})
				for key, val := range v.Entries {
					m[key] = objectToGoValue(val)
				}
				goValue = m
			case *collections.OrderedMap:
				m := make(map[string]interface{})
				for key, val := range v.Entries {
					m[key] = objectToGoValue(val)
				}
				goValue = m
			case *collections.Array:
				arr := make([]interface{}, v.Len())
				for i := 0; i < v.Len(); i++ {
					arr[i] = objectToGoValue(v.Get(i))
				}
				goValue = arr
			case types.Int:
				goValue = int64(v)
			case types.Float:
				goValue = float64(v)
			case types.Bool:
				goValue = bool(v)
			case types.String:
				goValue = string(v)
			case *types.Null:
				goValue = nil
			default:
				goValue = v.ToStr()
			}

			var data []byte
			var err error

			// Check for indent option
			indent := false
			if len(args) > 1 {
				indent = types.ToBool(args[1])
			}

			if indent {
				data, err = json.MarshalIndent(goValue, "", "  ")
			} else {
				data, err = json.Marshal(goValue)
			}

			if err != nil {
				return types.NewError(err.Error(), 0, 0, "")
			}

			return types.String(data)
		},
	}
}

// Thread/concurrency functions
func registerThreadFunctions(vm *vm.VM) {
	vm.Globals()["thread"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			if len(args) == 0 {
				return types.UndefinedValue
			}
			fn, ok := args[0].(*types.Function)
			if !ok {
				return types.NewError("first argument to thread must be a function", 0, 0, "")
			}

			// Start goroutine
			go func() {
				// TODO: Create new VM instance for thread
				fmt.Printf("Thread started for function: %s\n", fn.Name)
				// For now just print that we would run the function
			}()

			return types.UndefinedValue
		},
	}

	vm.Globals()["mutex"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return concurrency.NewMutex()
		},
	}

	vm.Globals()["rwMutex"] = &types.NativeFunction{
		Fn: func(args ...types.Object) types.Object {
			return concurrency.NewRWMutex()
		},
	}
}

// Helper to convert Nxlang object to Go value for JSON serialization
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
