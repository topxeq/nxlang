package compiler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/topxeq/nxlang/bytecode"
	"github.com/topxeq/nxlang/parser"
	"github.com/topxeq/nxlang/types"
)

// Module represents a compiled Nxlang module
type Module struct {
	Name     string
	Path     string
	Exports  map[string]string // Maps export names to symbol names in the module
	Bytecode *bytecode.Bytecode
}

// Compiler compiles AST nodes into bytecode
type Compiler struct {
	constants   []bytecode.Constant
	symbolTable *SymbolTable

	scopes     []CompilationScope
	scopeIndex int

	errors []string

	// Line number tracking
	currentLine     int
	currentColumn   int
	lineNumberTable []bytecode.LineInfo

	// Loop stack for break/continue support
	loopStack []LoopContext

	// Module system support
	ModulePath string             // Path of the current module being compiled
	modules    map[string]*Module // Cache of already compiled modules
	Exports    map[string]string  // Exports from the current module: export name -> symbol name
	isModule   bool               // Whether we're compiling a module

	// Interface cache for implementation checking
	interfaces map[string]*bytecode.InterfaceConstant // Map of interface name to interface constant

	// Class stack for super reference resolution in inheritance
	classStack []ClassContext
}

// ClassContext holds the current class being compiled
type ClassContext struct {
	Name       string // Current class name
	SuperName  string // Superclass name (empty if no superclass)
	SuperIndex int    // Index of superclass constant in constants array
}

// LoopContext holds the target positions for break and continue in a loop
type LoopContext struct {
	continueTarget int   // Position to jump to for continue
	breakTarget    int   // Position to jump to for break
	continueJumps  []int // Positions of continue jump instructions to patch
}

// CompilationScope represents a function scope being compiled
type CompilationScope struct {
	instructions  []byte
	numLocals     int
	numParameters int
	isVariadic    bool
	defaultValues []int
}

// NewCompiler creates a new compiler instance
func NewCompiler() *Compiler {
	mainScope := CompilationScope{
		instructions: []byte{},
		numLocals:    0,
	}

	symbolTable := NewSymbolTable()
	// Register built-in functions
	registerBuiltins(symbolTable)

	return &Compiler{
		constants:       []bytecode.Constant{},
		symbolTable:     symbolTable,
		scopes:          []CompilationScope{mainScope},
		scopeIndex:      0,
		errors:          []string{},
		lineNumberTable: []bytecode.LineInfo{},
		modules:         make(map[string]*Module),
		Exports:         make(map[string]string),
		interfaces:      make(map[string]*bytecode.InterfaceConstant),
	}
}

// NewModuleCompiler creates a new compiler instance for compiling a module
func NewModuleCompiler(modulePath string, modules map[string]*Module) *Compiler {
	c := NewCompiler()
	c.ModulePath = modulePath
	c.modules = modules
	c.isModule = true
	return c
}

// resolveModule resolves and compiles an imported module
func (c *Compiler) resolveModule(importPath string) (*Module, error) {
	// Check if module is already cached
	if mod, ok := c.modules[importPath]; ok {
		return mod, nil
	}

	// Standard library modules are built-in and don't have source files
	// They are registered at runtime by the VM, so we skip file lookup for them
	standardModules := map[string]bool{
		"math":       true,
		"string":     true,
		"collection": true,
		"time":       true,
		"json":       true,
		"thread":     true,
		"http":       true,
	}
	if standardModules[importPath] {
		// Create a placeholder module - exports will be populated at runtime by VM
		// The VM will handle providing the actual built-in functions
		placeholder := &Module{
			Name:    importPath,
			Path:    "builtin:" + importPath,
			Exports: make(map[string]string),
		}
		c.modules[importPath] = placeholder
		return placeholder, nil
	}

	// Resolve the module file path
	var moduleFilePath string
	if strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../") {
		// Relative import - resolve relative to current module
		currentDir := filepath.Dir(c.ModulePath)
		moduleFilePath = filepath.Join(currentDir, importPath)
	} else {
		// Package import - search in nx_modules directories up the tree
		currentDir := filepath.Dir(c.ModulePath)
		found := false

		// Search up the directory tree for nx_modules
		for {
			nxModulesPath := filepath.Join(currentDir, "nx_modules", importPath)

			// Check if it's a file
			if stat, err := os.Stat(nxModulesPath + ".nx"); err == nil && !stat.IsDir() {
				moduleFilePath = nxModulesPath + ".nx"
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
								moduleFilePath = filepath.Join(nxModulesPath, main)
								found = true
								break
							}
						}
					}
				}

				// Fallback to index.nx
				indexPath := filepath.Join(nxModulesPath, "index.nx")
				if _, err := os.Stat(indexPath); err == nil {
					moduleFilePath = indexPath
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
				pkgPath := filepath.Join(sysPath, importPath)
				if stat, err := os.Stat(pkgPath + ".nx"); err == nil && !stat.IsDir() {
					moduleFilePath = pkgPath + ".nx"
					found = true
					break
				}
				if stat, err := os.Stat(pkgPath); err == nil && stat.IsDir() {
					indexPath := filepath.Join(pkgPath, "index.nx")
					if _, err := os.Stat(indexPath); err == nil {
						moduleFilePath = indexPath
						found = true
						break
					}
				}
			}
		}

		if !found {
			return nil, fmt.Errorf("package not found: %s (searched in nx_modules directories and system paths)", importPath)
		}
	}

	// Add .nx extension if missing
	if filepath.Ext(moduleFilePath) == "" {
		moduleFilePath += ".nx"
	}

	// Check if file exists
	if _, err := os.Stat(moduleFilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("module not found: %s", importPath)
	}

	// Read the module source
	source, err := os.ReadFile(moduleFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read module %s: %v", importPath, err)
	}

	// Parse the module
	lexer := parser.NewLexer(string(source))
	parser := parser.NewParser(lexer)
	program := parser.ParseProgram()

	if len(parser.Errors()) > 0 {
		return nil, fmt.Errorf("parse errors in module %s: %v", importPath, parser.Errors())
	}

	// Compile the module
	moduleCompiler := NewModuleCompiler(moduleFilePath, c.modules)
	if err := moduleCompiler.Compile(program); err != nil {
		return nil, fmt.Errorf("compilation errors in module %s: %v", importPath, err)
	}

	// Create module object
	module := &Module{
		Name:     filepath.Base(moduleFilePath),
		Path:     moduleFilePath,
		Exports:  moduleCompiler.Exports,
		Bytecode: moduleCompiler.Bytecode(),
	}

	// Cache the module
	c.modules[importPath] = module
	c.modules[moduleFilePath] = module

	return module, nil
}

// registerBuiltins registers all built-in functions in the symbol table
func registerBuiltins(st *SymbolTable) {
	builtins := []string{
		"pr", "pln", "pl", "printf", "sprintf",
		"typeCode", "typeName", "typeOf", "isErr", "toStr",
		"len", "append", "array", "map", "orderedMap", "stack", "queue", "seq", "keys", "values", "delete", "sortMap", "reverseMap", "moveKey", "moveKeyToFirst", "moveKeyToLast",
		"abs", "sqrt", "sin", "cos", "tan", "floor", "ceil", "round", "pow", "random",
		"toUpper", "toLower", "trim", "split", "join", "contains", "replace", "substr", "startsWith", "endsWith",
		"now", "unix", "unixMilli", "unixNano", "timestamp", "formatTime", "parseTime", "addDate", "addDuration", "year", "month", "day", "hour", "minute", "second", "weekday", "isAfter", "isBefore", "dateDiff", "sleep", "thread", "mutex", "rwMutex", "waitForThreads",
		"toJson", "fromJson",
		"compile", "runByteCode", "runCode", "addMethod", "addMember",
		"ref", "deref", "setref",
		// File operations
		"readFile", "writeFile",
		// Debug functions
		"debug", "trace", "vars", "breakpoint", "typeInfo",
		// Functional programming
		"range", "xrange", "fastSum", "fastRangeSum", "fastEach", "fastMap", "fastFilter", "fastReduce", "each", "map", "filter", "reduce", "call",
		// Utility functions
		"repeat", "clamp", "min", "max", "abs", "sum", "avg",
		// Array functions
		"includes", "find", "slice", "concat", "reverse",
		// String functions
		"trimLeft", "trimRight", "replaceAll", "indexOf", "lastIndexOf",
		// Type check functions
		"isArray", "isMap", "isNumber", "isString", "isBool", "isFunction",
		// Performance and system functions
		"profilerStart", "profilerStop", "instructionCount", "gc", "memoryUsage", "version", "exit",
		// Encoding and hash functions
		"md5", "sha1", "sha256", "base64Encode", "base64Decode", "hexEncode", "hexDecode",
		// Regex functions
		"match", "replaceRegex", "splitRegex", "findRegex",
		// String conversion
		"strconv",
		// Type conversion functions
		"int", "float", "bool", "string", "byte", "uint", "char", "bytes", "chars",
		// Graphics/Canvas functions
		"canvas", "clear", "drawPoint", "drawLine", "drawRectangle", "fillRectangle", "drawCircle", "fillCircle", "savePNG", "loadPNG", "getPixel", "canvasWidth", "canvasHeight",
		// Data/CSV functions
		"openCSV", "readCSV", "writeCSV", "parseCSV", "toCSV", "createCSVReader", "createCSVWriter", "closeCSV", "readCSVRow", "readCSVAll", "writeCSVRow", "getCSVHeaders",
		// Plugin functions
		"loadPlugin", "unloadPlugin", "callPlugin", "listPlugins",
		// HTTP functions
		"httpGet", "httpPost", "httpPostJSON", "httpPut", "httpDelete", "httpRequest",
	}

	for i, name := range builtins {
		st.DefineBuiltin(i, name)
	}

	// Register predefined constants as built-in symbols
	st.DefineConstant("undefined", types.UndefinedValue)
	st.DefineConstant("null", types.NullValue)
	st.DefineConstant("nil", types.NullValue)

	// Predefined variable 'args' for dynamic argument passing (e.g., in runCode)
	// The actual value is set at runtime
	st.DefineBuiltin(len(builtins), "args")

	// Mathematical constants
	st.DefineConstant("piC", types.Float(3.141592653589793))
	st.DefineConstant("eC", types.Float(2.718281828459045))
}

// Compile compiles an AST program into bytecode
func (c *Compiler) Compile(node parser.Node) error {
	// Set current line from AST node
	if lineNode, ok := node.(interface{ Line() int }); ok {
		c.currentLine = lineNode.Line()
	}

	switch n := node.(type) {
	case *parser.Program:
		for _, stmt := range n.Statements {
			if err := c.Compile(stmt); err != nil {
				return err
			}
		}
		// Add return at end of main function
		c.emit(OpReturnVoid)
		return nil

	// Statements
	case *parser.ExpressionStatement:
		if err := c.Compile(n.Expression); err != nil {
			return err
		}
		// Only emit OpPop if not in the top-level main scope
		// This allows the result to remain on the stack for REPL and testing
		if c.scopeIndex > 0 {
			c.emit(OpPop)
		}
		return nil

	case *parser.WhileStatement:
		// Label for condition check (continue target)
		conditionPos := len(c.currentInstructions())

		// Compile condition
		if err := c.Compile(n.Condition); err != nil {
			return err
		}

		// Jump if false to after loop (break target will be patched later)
		jumpAfterPos := c.emit(OpJmpIfFalse, 0xFFFF)

		// Push loop context to stack
		c.loopStack = append(c.loopStack, LoopContext{
			continueTarget: conditionPos,
			breakTarget:    0xFFFF, // Will be patched later
		})

		// Compile body
		if err := c.Compile(n.Body); err != nil {
			return err
		}

		// Patch all continue jumps in this loop
		loopCtx := c.loopStack[len(c.loopStack)-1]
		for _, jmpPos := range loopCtx.continueJumps {
			c.changeOperand(jmpPos, loopCtx.continueTarget)
		}

		// Pop loop context
		c.loopStack = c.loopStack[:len(c.loopStack)-1]

		// Jump back to condition
		c.emit(OpJmp, conditionPos)

		// Patch jump after (break target)
		afterLoopPos := len(c.currentInstructions())
		c.changeOperand(jumpAfterPos, afterLoopPos)

		// Patch all break jumps in this loop
		for i := 0; i < len(c.currentInstructions())-2; i++ {
			if c.currentInstructions()[i] == byte(OpJmp) &&
				(int(c.currentInstructions()[i+1])<<8|int(c.currentInstructions()[i+2])) == 0xFFFF {
				c.changeOperand(i, afterLoopPos)
			}
		}

		return nil

	case *parser.ForStatement:
		// Handle for...in loop
		if n.IsForIn {
			// Check for small range unrolling optimization
			if count := getSmallRangeCount(n.Iterate); count > 0 {
				// Unroll small loops
				keySym := c.symbolTable.Define(n.Key.Value)
				var valSym *Symbol
				if n.Value != nil {
					vs := c.symbolTable.Define(n.Value.Value)
					valSym = &vs
				}

				// Generate unrolled loop body
				for i := 0; i < count; i++ {
					// Set iteration variable
					c.emit(OpPush, c.addConstant(&bytecode.IntConstant{Value: int64(i)}))
					c.storeSymbol(keySym)
					if valSym != nil {
						c.emit(OpPush, c.addConstant(&bytecode.IntConstant{Value: int64(i)}))
						c.storeSymbol(*valSym)
					}

					// Compile loop body
					if err := c.Compile(n.Body); err != nil {
						return err
					}
				}
				return nil
			}

			// Check for optimized range loop
			if n.Iterate != nil {
				if call, ok := n.Iterate.(*parser.CallExpression); ok {
					if ident, ok := call.Function.(*parser.Identifier); ok && ident.Value == "range" {
						// Check if it's range(n) with known integer
						var count int = -1
						if len(call.Arguments) == 1 {
							if lit, ok := call.Arguments[0].(*parser.IntLiteral); ok {
								count = int(lit.Value)
							} else if identArg, ok := call.Arguments[0].(*parser.Identifier); ok {
								// Check if we have type inference for this variable
								if sym, ok := c.symbolTable.Resolve(identArg.Value); ok {
									if sym.Type != nil {
										if intVal, ok := sym.Type.(types.Int); ok {
											count = int(intVal)
										}
									}
								}
							}
						}
						// If count is small, unroll it
						if count > 0 && count <= 16 {
							keySym := c.symbolTable.Define(n.Key.Value)
							var valSym *Symbol
							if n.Value != nil {
								vs := c.symbolTable.Define(n.Value.Value)
								valSym = &vs
							}
							for i := 0; i < count; i++ {
								c.emit(OpPush, c.addConstant(&bytecode.IntConstant{Value: int64(i)}))
								c.storeSymbol(keySym)
								if valSym != nil {
									c.emit(OpPush, c.addConstant(&bytecode.IntConstant{Value: int64(i)}))
									c.storeSymbol(*valSym)
								}
								if err := c.Compile(n.Body); err != nil {
									return err
								}
							}
							return nil
						}
						// For larger counts, generate optimized loop without array creation
						if count > 16 {
							keySym := c.symbolTable.Define(n.Key.Value)
							var valSym *Symbol
							if n.Value != nil {
								vs := c.symbolTable.Define(n.Value.Value)
								valSym = &vs
							}
							// Create counter variable
							counterSym := c.symbolTable.Define("__counter__")
							// Initialize counter to 0
							c.emit(OpPush, c.addConstant(&bytecode.IntConstant{Value: 0}))
							c.storeSymbol(counterSym)
							// Create end variable
							var endVal int64 = int64(count)
							if len(call.Arguments) >= 1 {
								if lit, ok := call.Arguments[0].(*parser.IntLiteral); ok {
									endVal = int64(lit.Value)
								}
							}
							endSym := c.symbolTable.Define("__end__")
							c.emit(OpPush, c.addConstant(&bytecode.IntConstant{Value: endVal}))
							c.storeSymbol(endSym)
							// Loop body position
							loopBodyPos := len(c.currentInstructions())
							// Load counter
							c.loadSymbol(counterSym)
							// Store to key
							c.storeSymbol(keySym)
							if valSym != nil {
								c.loadSymbol(counterSym)
								c.storeSymbol(*valSym)
							}
							// Compile loop body
							if err := c.Compile(n.Body); err != nil {
								return err
							}
							// Increment counter
							c.loadSymbol(counterSym)
							c.emit(OpPush, c.addConstant(&bytecode.IntConstant{Value: 1}))
							c.emit(OpAddInt)
							c.storeSymbol(counterSym)
							// Check condition: counter < end
							c.loadSymbol(counterSym)
							c.loadSymbol(endSym)
							c.emit(OpLtInt)
							// Jump back if true
							c.emit(OpJmpIfTrue, loopBodyPos)
							return nil
						}
					}
				}
			}

			// Compile the iterate expression (original array-based approach)
			if err := c.Compile(n.Iterate); err != nil {
				return err
			}

			// Define local variables for iteration state
			iterSym := c.symbolTable.Define("__iter__")
			c.storeSymbol(iterSym)

			keysSym := c.symbolTable.Define("__keys__")
			lenSym := c.symbolTable.Define("__len__")
			idxSym := c.symbolTable.Define("__idx__")

			// Define key and value variables
			keySym := c.symbolTable.Define(n.Key.Value)
			var valSym *Symbol
			if n.Value != nil {
				vs := c.symbolTable.Define(n.Value.Value)
				valSym = &vs
			}

			// Get type code of iteratee
			c.loadSymbol(iterSym)
			c.emit(OpTypeCode)

			// Check if it's array (type code 0x10 = 16)
			c.emit(OpPush, c.addConstant(&bytecode.IntConstant{Value: 0x10}))
			c.emit(OpEq)
			jumpToArray := c.emit(OpJmpIfTrue, 0xFFFF)

			// Check if it's string (type code 0x08 = 8)
			c.loadSymbol(iterSym)
			c.emit(OpTypeCode)
			c.emit(OpPush, c.addConstant(&bytecode.IntConstant{Value: 0x08}))
			c.emit(OpEq)
			jumpToString := c.emit(OpJmpIfTrue, 0xFFFF)

			// Check if it's int (type code 0x05 = 5)
			c.loadSymbol(iterSym)
			c.emit(OpTypeCode)
			c.emit(OpPush, c.addConstant(&bytecode.IntConstant{Value: 0x05}))
			c.emit(OpEq)
			jumpToInt := c.emit(OpJmpIfTrue, 0xFFFF)

			// Check if it's float (type code 0x07 = 7)
			c.loadSymbol(iterSym)
			c.emit(OpTypeCode)
			c.emit(OpPush, c.addConstant(&bytecode.IntConstant{Value: 0x07}))
			c.emit(OpEq)
			jumpToFloat := c.emit(OpJmpIfTrue, 0xFFFF)

			// Map/Object case: get keys array
			c.loadSymbol(iterSym)
			c.emit(OpLoadGlobal, c.addConstant(&bytecode.StringConstant{Value: "keys"}))
			c.emit(OpCall, 1)
			c.storeSymbol(keysSym)

			// Get length of keys array
			c.loadSymbol(keysSym)
			c.emit(OpLen)
			c.storeSymbol(lenSym)

			jumpToInit := c.emit(OpJmp, 0xFFFF)

			// Array case handler
			arrayCasePos := len(c.currentInstructions())
			c.changeOperand(jumpToArray, arrayCasePos)
			// Get array length
			c.loadSymbol(iterSym)
			c.emit(OpLen)
			c.storeSymbol(lenSym)
			// Set keys to nil for sequential iteration
			c.emit(OpPush, c.addConstant(&bytecode.NilConstant{}))
			c.storeSymbol(keysSym)
			// Jump to common init
			jumpToInitArr := c.emit(OpJmp, 0xFFFF)

			// String case handler
			stringCasePos := len(c.currentInstructions())
			c.changeOperand(jumpToString, stringCasePos)
			// Get string length (rune count)
			c.loadSymbol(iterSym)
			c.emit(OpLen)
			c.storeSymbol(lenSym)
			// Set keys to nil for sequential iteration
			c.emit(OpPush, c.addConstant(&bytecode.NilConstant{}))
			c.storeSymbol(keysSym)
			// Jump to common init
			jumpToInitStr := c.emit(OpJmp, 0xFFFF)

			// Int case handler
			intCasePos := len(c.currentInstructions())
			c.changeOperand(jumpToInt, intCasePos)
			// Use the int value directly as length
			c.loadSymbol(iterSym)
			c.storeSymbol(lenSym)
			// Set keys to nil for sequential iteration
			c.emit(OpPush, c.addConstant(&bytecode.NilConstant{}))
			c.storeSymbol(keysSym)
			// Jump to common init
			jumpToInitInt := c.emit(OpJmp, 0xFFFF)

			// Float case handler
			floatCasePos := len(c.currentInstructions())
			c.changeOperand(jumpToFloat, floatCasePos)
			// Convert float to int and use as length
			c.loadSymbol(iterSym)
			c.storeSymbol(lenSym)
			// Set keys to nil for sequential iteration
			c.emit(OpPush, c.addConstant(&bytecode.NilConstant{}))
			c.storeSymbol(keysSym)
			// Jump to common init
			jumpToInitFloat := c.emit(OpJmp, 0xFFFF)

			// Map/Object case continues here (no jump needed)

			// Patch all case handler jumps to common init
			jumpToInitPos := len(c.currentInstructions())
			c.changeOperand(jumpToInit, jumpToInitPos)
			c.changeOperand(jumpToInitArr, jumpToInitPos)
			c.changeOperand(jumpToInitStr, jumpToInitPos)
			c.changeOperand(jumpToInitInt, jumpToInitPos)
			c.changeOperand(jumpToInitFloat, jumpToInitPos)

			// Initialize index to 0
			c.emit(OpPush, c.addConstant(&bytecode.IntConstant{Value: 0}))
			c.storeSymbol(idxSym)

			// Compare index < length
			conditionPos := len(c.currentInstructions())
			c.loadSymbol(idxSym)
			c.loadSymbol(lenSym)
			c.emit(OpLt)

			// Jump if false to end of loop
			jumpAfterPos := c.emit(OpJmpIfFalse, 0xFFFF)

			// Push loop context
			c.loopStack = append(c.loopStack, LoopContext{
				continueTarget: 0,
				breakTarget:    0xFFFF,
			})

			// Check if we have keys (map/object iteration)
			c.loadSymbol(keysSym)
			c.emit(OpIsNil)
			jumpToSeqIter := c.emit(OpJmpIfTrue, 0xFFFF)

			// Map/Object iteration: get key from keys array
			c.loadSymbol(keysSym)
			c.loadSymbol(idxSym)
			c.emit(OpIndexGet)
			// Store key variable
			c.storeSymbol(keySym)

			// Get value from iteratee using key
			c.loadSymbol(iterSym)
			c.loadSymbol(keySym)
			c.emit(OpIndexGet)

			// Store value if requested
			if n.Value != nil {
				c.storeSymbol(*valSym)
			} else {
				c.emit(OpPop)
			}

			jumpToBody := c.emit(OpJmp, 0xFFFF)

			// Sequential iteration for array/string/numeric
			seqIterPos := len(c.currentInstructions())
			c.changeOperand(jumpToSeqIter, seqIterPos)

			// Check if iteratee is a number (int or float) for range iteration
			c.loadSymbol(iterSym)
			c.emit(OpTypeCode)
			// Check for int (0x05)
			c.emit(OpPush, c.addConstant(&bytecode.IntConstant{Value: 0x05}))
			c.emit(OpEq)
			jumpToIntRange := c.emit(OpJmpIfTrue, 0xFFFF)

			// Check for float (0x07)
			c.loadSymbol(iterSym)
			c.emit(OpTypeCode)
			c.emit(OpPush, c.addConstant(&bytecode.IntConstant{Value: 0x07}))
			c.emit(OpEq)
			jumpToFloatRange := c.emit(OpJmpIfTrue, 0xFFFF)

			// Array/String case: Get element by index
			arrayStringPos := len(c.currentInstructions())
			c.changeOperand(jumpToIntRange, arrayStringPos)
			c.loadSymbol(iterSym)
			c.loadSymbol(idxSym)
			c.emit(OpIndexGet)

			// Store value if requested
			if n.Value != nil {
				c.storeSymbol(*valSym)
			} else {
				// If no Value variable (e.g., "for i in arr"), store value to key
				c.storeSymbol(keySym)
			}

			// Store key (index for array/string)
			c.loadSymbol(idxSym)
			// Only store index to key if we also have a value variable
			if n.Value != nil {
				c.storeSymbol(keySym)
			} else {
				// Pop the index since we don't need it when there's no value var
				c.emit(OpPop)
			}

			jumpToBody2 := c.emit(OpJmp, 0xFFFF)

			// Int range case: value is the index itself
			intRangePos := len(c.currentInstructions())
			c.changeOperand(jumpToIntRange, intRangePos)
			// Store key (index)
			c.loadSymbol(idxSym)
			c.storeSymbol(keySym)
			// Store value (same as key for range)
			if n.Value != nil {
				c.loadSymbol(idxSym)
				c.storeSymbol(*valSym)
			}

			jumpToBody3 := c.emit(OpJmp, 0xFFFF)

			// Float range case: value is the index (as float)
			floatRangePos := len(c.currentInstructions())
			c.changeOperand(jumpToFloatRange, floatRangePos)
			// Store key (index as int)
			c.loadSymbol(idxSym)
			c.storeSymbol(keySym)
			// Store value (index converted to float for float range)
			if n.Value != nil {
				c.loadSymbol(idxSym)
				// Convert int to float if needed
				c.emit(OpPush, c.addConstant(&bytecode.IntConstant{Value: 0}))
				c.emit(OpAdd) // This will convert to float if lenSym is float
			}

			bodyPos := len(c.currentInstructions())
			c.changeOperand(jumpToBody, bodyPos)
			c.changeOperand(jumpToBody2, bodyPos)
			c.changeOperand(jumpToBody3, bodyPos)

			// Compile loop body
			if err := c.Compile(n.Body); err != nil {
				return err
			}

			// Continue target: increment index
			continueTarget := len(c.currentInstructions())
			loopCtx := &c.loopStack[len(c.loopStack)-1]
			loopCtx.continueTarget = continueTarget

			// Patch continue jumps
			for _, jmpPos := range loopCtx.continueJumps {
				c.changeOperand(jmpPos, continueTarget)
			}

			// Increment index
			c.loadSymbol(idxSym)
			c.emit(OpPush, c.addConstant(&bytecode.IntConstant{Value: 1}))
			c.emit(OpAdd)
			c.storeSymbol(idxSym)

			// Jump back to condition
			c.emit(OpJmp, conditionPos)

			// Patch break jumps
			endPos := len(c.currentInstructions())
			c.changeOperand(jumpAfterPos, endPos)

			// Patch all break jumps
			for i := 0; i < len(c.currentInstructions())-2; i++ {
				if c.currentInstructions()[i] == byte(OpJmp) &&
					(int(c.currentInstructions()[i+1])<<8|int(c.currentInstructions()[i+2])) == 0xFFFF {
					c.changeOperand(i, endPos)
				}
			}

			// Pop loop context
			c.loopStack = c.loopStack[:len(c.loopStack)-1]

			return nil
		}

		// Regular for loop
		// Compile init statement
		if n.Init != nil {
			if err := c.Compile(n.Init); err != nil {
				return err
			}
		}

		// Label for condition check
		conditionPos := len(c.currentInstructions())

		// Compile condition
		if n.Condition != nil {
			if err := c.Compile(n.Condition); err != nil {
				return err
			}
		} else {
			// No condition = infinite loop
			c.emit(OpPush, c.addConstant(&bytecode.BoolConstant{Value: true}))
		}

		// Jump if false to after loop
		jumpAfterPos := c.emit(OpJmpIfFalse, 0xFFFF)

		// Push loop context to stack
		c.loopStack = append(c.loopStack, LoopContext{
			continueTarget: 0,      // Will be set after body compilation
			breakTarget:    0xFFFF, // Will be patched later
		})

		// Compile body
		if err := c.Compile(n.Body); err != nil {
			return err
		}

		// Continue target is the start of update statement
		continueTarget := len(c.currentInstructions())
		loopCtx := &c.loopStack[len(c.loopStack)-1]
		loopCtx.continueTarget = continueTarget
		// Patch all continue jumps
		for _, jmpPos := range loopCtx.continueJumps {
			c.changeOperand(jmpPos, continueTarget)
		}

		// Pop loop context
		c.loopStack = c.loopStack[:len(c.loopStack)-1]

		// Compile update statement
		if n.Update != nil {
			if err := c.Compile(n.Update); err != nil {
				return err
			}
		}

		// Jump back to condition
		c.emit(OpJmp, conditionPos)

		// Patch jump after (break target)
		afterLoopPos := len(c.currentInstructions())
		c.changeOperand(jumpAfterPos, afterLoopPos)

		// Patch all break jumps in this loop
		for i := 0; i < len(c.currentInstructions())-2; i++ {
			if c.currentInstructions()[i] == byte(OpJmp) &&
				(int(c.currentInstructions()[i+1])<<8|int(c.currentInstructions()[i+2])) == 0xFFFF {
				c.changeOperand(i, afterLoopPos)
			}
		}

		return nil

	case *parser.VarStatement, *parser.LetStatement, *parser.ConstStatement, *parser.DefineStatement:
		var name *parser.Identifier
		var value parser.Expression

		switch stmt := n.(type) {
		case *parser.VarStatement:
			name = stmt.Name
			value = stmt.Value
		case *parser.LetStatement:
			name = stmt.Name
			value = stmt.Value
		case *parser.ConstStatement:
			name = stmt.Name
			value = stmt.Value
		case *parser.DefineStatement:
			name = stmt.Name
			value = stmt.Value
		}

		// Compile the value expression
		if err := c.Compile(value); err != nil {
			return err
		}

		// Define the symbol
		symbol := c.symbolTable.Define(name.Value)

		// Update symbol type based on value expression
		inferredType := c.inferExpressionType(value)
		if inferredType != nil {
			c.symbolTable.UpdateType(symbol.Name, inferredType)
		}

		// Store the value
		if symbol.Scope == ScopeGlobal {
			c.emit(OpStoreGlobal, c.addConstant(&bytecode.StringConstant{Value: name.Value}))
		} else {
			c.emit(OpStoreLocal, symbol.Index)
		}
		return nil

	case *parser.BlockStatement:
		for _, stmt := range n.Statements {
			if err := c.Compile(stmt); err != nil {
				return err
			}
		}
		return nil

	case *parser.ReturnStatement:
		if n.ReturnValue != nil {
			if err := c.Compile(n.ReturnValue); err != nil {
				return err
			}
			c.emit(OpReturn)
		} else {
			c.emit(OpReturnVoid)
		}
		return nil

	case *parser.BreakStatement:
		if len(c.loopStack) == 0 {
			return fmt.Errorf("break statement not in loop")
		}
		// Jump to break target (temporarily use 0xFFFF, will be patched later)
		c.emit(OpJmp, 0xFFFF)
		return nil

	case *parser.ContinueStatement:
		if len(c.loopStack) == 0 {
			return fmt.Errorf("continue statement not in loop")
		}
		// Jump to continue target (will be patched later)
		loopCtx := &c.loopStack[len(c.loopStack)-1]
		jmpPos := c.emit(OpJmp, 0xFFFF)
		loopCtx.continueJumps = append(loopCtx.continueJumps, jmpPos)
		return nil

	case *parser.FallthroughStatement:
		// Fallthrough: jump to next case (will be patched during switch compilation)
		// We use 0xFFFE as a marker to identify fallthrough jumps
		c.emit(OpJmp, 0xFFFE)
		return nil

	case *parser.SwitchStatement:
		// Compile switch expression
		if err := c.Compile(n.Expression); err != nil {
			return err
		}

		// Keep track of jump positions for end of case blocks
		var endJumps []int
		var caseEndPositions []int
		var caseStartPositions []int // Store start positions of each case body for fallthrough

		// Compile each case
		for _, caseStmt := range n.Cases {
			// Compile all case expressions and compare with switch value
			for _, expr := range caseStmt.Expressions {
				// Duplicate the switch value for comparison
				c.emit(OpDup)
				// Compile case expression
				if err := c.Compile(expr); err != nil {
					return err
				}
				// Compare for equality
				c.emit(OpEq)
				// If equal, jump to case body
				caseStartPos := c.emit(OpJmpIfTrue, 0xFFFF)
				caseEndPositions = append(caseEndPositions, caseStartPos)
			}
		}

		// If none of the cases matched, jump to default or end
		defaultJump := c.emit(OpJmp, 0xFFFF)

		// Pop the duplicate switch value from stack
		c.emit(OpPop)

		// Compile case bodies
		for i, caseStmt := range n.Cases {
			// Patch the jump to this case body
			caseStart := len(c.currentInstructions())
			caseStartPositions = append(caseStartPositions, caseStart)
			for _, pos := range caseEndPositions[i*len(caseStmt.Expressions) : (i+1)*len(caseStmt.Expressions)] {
				c.changeOperand(pos, caseStart)
			}

			// Compile case body
			if err := c.Compile(caseStmt.Body); err != nil {
				return err
			}

			// Check if last statement is fallthrough
			hasFallthrough := false
			if len(caseStmt.Body.Statements) > 0 {
				_, hasFallthrough = caseStmt.Body.Statements[len(caseStmt.Body.Statements)-1].(*parser.FallthroughStatement)
			}

			// Only add jump to end if no fallthrough
			if !hasFallthrough {
				endJump := c.emit(OpJmp, 0xFFFF)
				endJumps = append(endJumps, endJump)
			}
		}

		// Compile default case
		defaultStart := len(c.currentInstructions())
		if n.DefaultCase != nil {
			// Patch the default jump
			c.changeOperand(defaultJump, defaultStart)
			// Compile default body
			if err := c.Compile(n.DefaultCase.Body); err != nil {
				return err
			}
		} else {
			// No default case: jump to end
			c.changeOperand(defaultJump, defaultStart)
		}

		// Patch all end jumps
		endPos := len(c.currentInstructions())
		for _, pos := range endJumps {
			c.changeOperand(pos, endPos)
		}

		// Patch all fallthrough jumps (marked with 0xFFFE)
		// Look for jumps with 0xFFFE operand and point them to next case
		for i := 0; i < len(c.currentInstructions())-2; i++ {
			if c.currentInstructions()[i] == byte(OpJmp) {
				operand := int(c.currentInstructions()[i+1])<<8 | int(c.currentInstructions()[i+2])
				if operand == 0xFFFE {
					// Find which case this fallthrough belongs to
					caseIdx := -1
					for j := len(caseStartPositions) - 1; j >= 0; j-- {
						if i > caseStartPositions[j] {
							caseIdx = j
							break
						}
					}
					if caseIdx != -1 && caseIdx < len(caseStartPositions)-1 {
						// Jump to next case
						c.changeOperand(i, caseStartPositions[caseIdx+1])
					} else {
						// Last case, fallthrough to end or default
						c.changeOperand(i, defaultStart)
					}
				}
			}
		}

		return nil

	case *parser.IfStatement:
		// Compile condition
		if err := c.Compile(n.Condition); err != nil {
			return err
		}

		// Emit jump if false with placeholder offset
		jumpFalsePos := c.emit(OpJmpIfFalse, 0xFFFF)

		// Compile consequence block
		if err := c.Compile(n.Consequence); err != nil {
			return err
		}

		// Emit jump to end with placeholder offset
		jumpPos := c.emit(OpJmp, 0xFFFF)

		// Patch jump false offset
		afterConsequencePos := len(c.currentInstructions())
		c.changeOperand(jumpFalsePos, afterConsequencePos)

		// Compile alternative if exists
		if n.Alternative != nil {
			if err := c.Compile(n.Alternative); err != nil {
				return err
			}
		}

		// Patch jump offset
		afterAlternativePos := len(c.currentInstructions())
		c.changeOperand(jumpPos, afterAlternativePos)
		return nil

	case *parser.FunctionDeclaration:
		// Add function name to outer scope first, so it can be referenced recursively
		c.symbolTable.Define(n.Name.Value)

		// Enter new scope for function
		c.enterScope()

		// Define function name in current scope for recursion
		c.symbolTable.DefineFunctionName(n.Name.Value)

		// Define parameters
		scope := c.currentScope()
		scope.defaultValues = make([]int, len(n.Parameters))
		for i, param := range n.Parameters {
			c.symbolTable.Define(param.Name.Value)
			scope.defaultValues[i] = -1 // -1 means no default value
			if param.Variadic {
				scope.isVariadic = true
			}
			if param.DefaultValue != nil {
				// Compile default value and add to constants
				if err := c.Compile(param.DefaultValue); err != nil {
					return err
				}
				// The compiled default value is on top of stack, pop it and get the last added constant
				c.emit(OpPop)
				// Default value was just added to constants when we compiled it
				// The constant index is the last one added
				scope.defaultValues[i] = len(c.constants) - 1
			}
		}

		c.currentScope().numParameters = len(n.Parameters)

		// Compile function body
		if err := c.Compile(n.Body); err != nil {
			return err
		}

		// Ensure we have a return at the end
		if lastOpcode(c.currentInstructions()) != OpReturn && lastOpcode(c.currentInstructions()) != OpReturnVoid {
			c.emit(OpReturnVoid)
		}

		// Update number of locals from symbol table
		c.currentScope().numLocals = c.symbolTable.Count()

		// Create function constant
		funcConst := &bytecode.FunctionConstant{
			Name:          n.Name.Value,
			Instructions:  c.currentInstructions(),
			NumLocals:     c.currentScope().numLocals,
			NumParameters: c.currentScope().numParameters,
			IsVariadic:    c.currentScope().isVariadic,
			DefaultValues: c.currentScope().defaultValues,
		}
		funcIdx := c.addConstant(funcConst)

		// Exit scope
		c.leaveScope()

		// Store function as global
		c.emit(OpLoadConst, funcIdx)
		c.emit(OpStoreGlobal, c.addConstant(&bytecode.StringConstant{Value: n.Name.Value}))
		return nil

	case *parser.FunctionLiteral:
		// Anonymous function literal - compile and push to stack
		// Enter new scope for function
		c.enterScope()

		// Define parameters (no function name for anonymous functions)
		scope := c.currentScope()
		scope.defaultValues = make([]int, len(n.Parameters))
		for i, param := range n.Parameters {
			c.symbolTable.Define(param.Name.Value)
			scope.defaultValues[i] = -1
			if param.Variadic {
				scope.isVariadic = true
			}
			if param.DefaultValue != nil {
				if err := c.Compile(param.DefaultValue); err != nil {
					return err
				}
				c.emit(OpPop)
				scope.defaultValues[i] = len(c.constants) - 1
			}
		}
		c.currentScope().numParameters = len(n.Parameters)

		// Compile function body
		if err := c.Compile(n.Body); err != nil {
			return err
		}

		// Ensure we have a return
		if lastOpcode(c.currentInstructions()) != OpReturn && lastOpcode(c.currentInstructions()) != OpReturnVoid {
			c.emit(OpReturnVoid)
		}

		// Update number of locals
		c.currentScope().numLocals = c.symbolTable.Count()

		// Create function constant (use empty name for anonymous)
		funcName := n.Name
		if funcName == "" {
			funcName = "__anonymous__"
		}
		funcConst := &bytecode.FunctionConstant{
			Name:          funcName,
			Instructions:  c.currentInstructions(),
			NumLocals:     c.currentScope().numLocals,
			NumParameters: c.currentScope().numParameters,
			IsVariadic:    c.currentScope().isVariadic,
			DefaultValues: c.currentScope().defaultValues,
		}
		funcIdx := c.addConstant(funcConst)

		// Exit scope
		c.leaveScope()

		// Push function constant to stack
		c.emit(OpLoadConst, funcIdx)
		return nil

	case *parser.ClassDeclaration:
		// Add class name to outer symbol table first, so it can be referenced
		c.symbolTable.Define(n.Name.Value)

		// Compile superclass first if it exists
		superIndex := -1
		if n.SuperClass != nil {
			// Load superclass symbol
			superSym, ok := c.symbolTable.Resolve(n.SuperClass.Value)
			if !ok {
				// Forward reference - will be resolved at runtime from globals
				// Define as a placeholder in symbol table
				superSym = c.symbolTable.Define(n.SuperClass.Value)
			}
			c.loadSymbol(superSym)
			superIndex = c.addConstant(&bytecode.StringConstant{Value: n.SuperClass.Value})
		}

		// Compile all methods
		methods := make(map[string]int)       // method name -> function constant index
		staticMethods := make(map[string]int) // static method name -> function constant index

		// Push class context for method compilation
		classCtx := ClassContext{
			Name:       n.Name.Value,
			SuperName:  "",
			SuperIndex: -1,
		}
		if n.SuperClass != nil {
			classCtx.SuperName = n.SuperClass.Value
			classCtx.SuperIndex = superIndex
		}
		c.classStack = append(c.classStack, classCtx)

		for _, method := range n.Methods {
			// Enter new scope for method
			c.enterScope()

			// Define 'this' as first local variable only for non-static methods
			if !method.IsStatic {
				thisSym := c.symbolTable.Define("this")
				_ = thisSym
			}

			// Define parameters
			scope := c.currentScope()
			scope.defaultValues = make([]int, len(method.Parameters))
			for i, param := range method.Parameters {
				c.symbolTable.Define(param.Name.Value)
				scope.defaultValues[i] = -1
				if param.Variadic {
					scope.isVariadic = true
				}
				if param.DefaultValue != nil {
					if err := c.Compile(param.DefaultValue); err != nil {
						return err
					}
					c.emit(OpPop)
					scope.defaultValues[i] = len(c.constants) - 1
				}
			}

			scope.numParameters = len(method.Parameters)

			// Compile method body
			if err := c.Compile(method.Body); err != nil {
				return err
			}

			// Ensure we have a return at the end
			if lastOpcode(c.currentInstructions()) != OpReturn && lastOpcode(c.currentInstructions()) != OpReturnVoid {
				c.emit(OpReturnVoid)
			}

			// Update number of locals
			scope.numLocals = c.symbolTable.Count()

			// Create function constant
			var flags uint8
			if method.IsGetter {
				flags |= 0x01 // Bit 0: isGetter
			}
			if method.IsSetter {
				flags |= 0x02 // Bit 1: isSetter
			}
			funcConst := &bytecode.FunctionConstant{
				Name:           method.Name,
				Instructions:   c.currentInstructions(),
				NumLocals:      scope.numLocals,
				NumParameters:  scope.numParameters,
				IsVariadic:     scope.isVariadic,
				IsStatic:       method.IsStatic,
				AccessModifier: uint8(method.AccessModifier),
				Flags:          flags,
				DefaultValues:  scope.defaultValues,
			}
			funcIdx := c.addConstant(funcConst)
			// Store in appropriate map based on whether it's static
			if method.IsStatic {
				staticMethods[method.Name] = funcIdx
			} else {
				methods[method.Name] = funcIdx
			}

			// Exit scope
			c.leaveScope()
		}

		// Check interface implementations
		for _, ifaceName := range n.Implements {
			iface, ok := c.interfaces[ifaceName.Value]
			if !ok {
				c.addError(fmt.Sprintf("interface '%s' not found", ifaceName.Value))
				continue
			}

			// Check that all interface methods are implemented
			for methodName, expectedParams := range iface.Methods {
				funcIdx, methodExists := methods[methodName]
				if !methodExists {
					c.addError(fmt.Sprintf("class '%s' does not implement interface method '%s.%s'",
						n.Name.Value, iface.Name, methodName))
					continue
				}

				// Get the function constant to check parameter count
				funcConst, ok := c.constants[funcIdx].(*bytecode.FunctionConstant)
				if !ok {
					continue
				}

				// Check parameter count matches
				if len(expectedParams) != funcConst.NumParameters {
					c.addError(fmt.Sprintf("method '%s' in class '%s' has %d parameters, but interface '%s' requires %d parameters",
						methodName, n.Name.Value, funcConst.NumParameters, iface.Name, len(expectedParams)))
				}
			}
		}

		// Create class constant
		classConst := &bytecode.ClassConstant{
			Name:          n.Name.Value,
			SuperClass:    "",
			Interfaces:    make([]string, len(n.Implements)),
			Methods:       methods,
			StaticMethods: staticMethods,
		}

		if n.SuperClass != nil {
			classConst.SuperClass = n.SuperClass.Value
		}

		// Add implemented interfaces
		for i, iface := range n.Implements {
			classConst.Interfaces[i] = iface.Value
		}

		classIdx := c.addConstant(classConst)

		// Store class as global variable
		classNameIdx := c.addConstant(&bytecode.StringConstant{Value: n.Name.Value})
		c.emit(OpLoadConst, classIdx)
		c.emit(OpStoreGlobal, classNameIdx)

		// Compile static field initializations
		for _, fieldStmt := range n.Fields {
			var fieldName *parser.Identifier
			var fieldValue parser.Expression

			// Extract field name and value from the statement
			switch stmt := fieldStmt.(type) {
			case *parser.VarStatement:
				fieldName = stmt.Name
				fieldValue = stmt.Value
			case *parser.LetStatement:
				fieldName = stmt.Name
				fieldValue = stmt.Value
			case *parser.ConstStatement:
				fieldName = stmt.Name
				fieldValue = stmt.Value
			default:
				continue
			}

			if fieldValue != nil {
				// Load the class from global
				c.emit(OpLoadGlobal, classNameIdx)
				// Compile the field value
				if err := c.Compile(fieldValue); err != nil {
					return err
				}
				// Store as member on the class object
				fieldNameIdx := c.addConstant(&bytecode.StringConstant{Value: fieldName.Value})
				c.emit(OpMemberSet, fieldNameIdx)
			}
		}

		// Pop class context
		if len(c.classStack) > 0 {
			c.classStack = c.classStack[:len(c.classStack)-1]
		}

		return nil

	case *parser.InterfaceDeclaration:
		// Add interface name to symbol table
		c.symbolTable.Define(n.Name.Value)

		// Compile interface methods
		methods := make(map[string][]string)
		for _, method := range n.Methods {
			paramNames := make([]string, len(method.Parameters))
			for i, param := range method.Parameters {
				paramNames[i] = param.Name.Value
			}
			methods[method.Name.Value] = paramNames
		}

		// Create interface constant
		ifaceConst := &bytecode.InterfaceConstant{
			Name:    n.Name.Value,
			Methods: methods,
		}
		// Add interface to cache for implementation checking
		c.interfaces[n.Name.Value] = ifaceConst
		ifaceIdx := c.addConstant(ifaceConst)

		// Store interface as global variable
		c.emit(OpLoadConst, ifaceIdx)
		c.emit(OpStoreGlobal, c.addConstant(&bytecode.StringConstant{Value: n.Name.Value}))
		return nil

	// Expressions
	case *parser.NewExpression:
		// Compile class expression
		if err := c.Compile(n.Class); err != nil {
			return err
		}

		// Compile constructor arguments
		for _, arg := range n.Args {
			if err := c.Compile(arg); err != nil {
				return err
			}
		}

		// Emit new object opcode: OpNewObject <arg_count>
		c.emit(OpNewObject, len(n.Args))
		return nil

	case *parser.IntLiteral:
		c.emit(OpPush, c.addConstant(&bytecode.IntConstant{Value: int64(n.Value)}))
		return nil

	case *parser.FloatLiteral:
		c.emit(OpPush, c.addConstant(&bytecode.FloatConstant{Value: float64(n.Value)}))
		return nil

	case *parser.StringLiteral:
		c.emit(OpPush, c.addConstant(&bytecode.StringConstant{Value: string(n.Value)}))
		return nil

	case *parser.CharLiteral:
		c.emit(OpPush, c.addConstant(&bytecode.CharConstant{Value: rune(n.Value)}))
		return nil

	case *parser.BoolLiteral:
		c.emit(OpPush, c.addConstant(&bytecode.BoolConstant{Value: bool(n.Value)}))
		return nil

	case *parser.NullLiteral:
		c.emit(OpPush, c.addConstant(&bytecode.NilConstant{}))
		return nil

	case *parser.Identifier:
		symbol, ok := c.symbolTable.Resolve(n.Value)
		if !ok {
			return fmt.Errorf("undefined variable: %s at line %d", n.Value, n.Line())
		}

		c.loadSymbol(symbol)
		return nil

	case *parser.ThisExpression:
		// 'this' is always the first local variable in methods
		symbol, ok := c.symbolTable.Resolve("this")
		if !ok {
			return fmt.Errorf("'this' keyword not allowed outside of class methods")
		}
		c.loadSymbol(symbol)
		return nil

	case *parser.SuperExpression:
		// 'super' is only allowed in class methods
		symbol, ok := c.symbolTable.Resolve("this")
		if !ok {
			return fmt.Errorf("'super' keyword not allowed outside of class methods")
		}
		// Check if we have a class context
		if len(c.classStack) == 0 {
			return fmt.Errorf("'super' keyword not allowed outside of class methods")
		}
		// Get current class context
		currentClass := c.classStack[len(c.classStack)-1]
		if currentClass.SuperName == "" {
			return fmt.Errorf("class '%s' has no superclass", currentClass.Name)
		}
		// Load 'this' instance, then get its superclass using the stored super index
		c.loadSymbol(symbol)
		c.emit(OpGetSuper2, currentClass.SuperIndex)
		return nil

	case *parser.MemberExpression:
		// Compile the object first
		if err := c.Compile(n.Object); err != nil {
			return err
		}
		// Emit member get opcode with property name constant
		nameIdx := c.addConstant(&bytecode.StringConstant{Value: n.Member.Value})
		c.emit(OpMemberGet, nameIdx)
		return nil

	case *parser.PrefixExpression:
		switch n.Operator {
		case "!", "-", "~":
			// Regular prefix operators
			if err := c.Compile(n.Right); err != nil {
				return err
			}

			switch n.Operator {
			case "!":
				c.emit(OpNot)
			case "-":
				c.emit(OpNeg)
			case "~":
				c.emit(OpBitNot)
			}
		case "++", "--":
			// Prefix increment/decrement: ++x or --x
			// First, check that the operand is assignable
			ident, ok := n.Right.(*parser.Identifier)
			if !ok {
				return fmt.Errorf("prefix %s operator requires an assignable identifier", n.Operator)
			}

			symbol, ok := c.symbolTable.Resolve(ident.Value)
			if !ok {
				return fmt.Errorf("undefined variable: %s", ident.Value)
			}

			// Load current value
			c.loadSymbol(symbol)
			// Push constant 1
			constIdx := c.addConstant(&bytecode.IntConstant{Value: int64(1)})
			c.emit(OpPush, constIdx)
			// Add or subtract
			if n.Operator == "++" {
				c.emit(OpAdd)
			} else {
				c.emit(OpSub)
			}
			// Duplicate the result before storing (so we can return it)
			c.emit(OpDup)
			// Store back
			c.storeSymbol(symbol)
		default:
			return fmt.Errorf("unknown prefix operator: %s", n.Operator)
		}
		return nil

	case *parser.InfixExpression:
		// Short-circuit operators
		if n.Operator == "&&" || n.Operator == "||" {
			return c.compileShortCircuit(n)
		}

		if err := c.Compile(n.Left); err != nil {
			return err
		}
		if err := c.Compile(n.Right); err != nil {
			return err
		}

		// Use fast integer opcodes when both operands are integer literals or inferred integers
		leftIsInt := isIntegerLiteral(n.Left) || isIntegerType(c.inferExpressionType(n.Left))
		rightIsInt := isIntegerLiteral(n.Right) || isIntegerType(c.inferExpressionType(n.Right))
		bothInts := leftIsInt && rightIsInt

		switch n.Operator {
		case "+":
			if bothInts {
				c.emit(OpAddInt)
			} else {
				c.emit(OpAdd)
			}
		case "-":
			if bothInts {
				c.emit(OpSubInt)
			} else {
				c.emit(OpSub)
			}
		case "*":
			if bothInts {
				c.emit(OpMulInt)
			} else {
				c.emit(OpMul)
			}
		case "/":
			c.emit(OpDiv)
		case "%":
			c.emit(OpMod)
		case "==":
			if bothInts {
				c.emit(OpEqInt)
			} else {
				c.emit(OpEq)
			}
		case "!=":
			c.emit(OpNotEq)
		case "<":
			if bothInts {
				c.emit(OpLtInt)
			} else {
				c.emit(OpLt)
			}
		case "<=":
			c.emit(OpLte)
		case ">":
			c.emit(OpGt)
		case ">=":
			c.emit(OpGte)
		case "&":
			c.emit(OpBitAnd)
		case "|":
			c.emit(OpBitOr)
		case "^":
			c.emit(OpBitXor)
		case "<<":
			c.emit(OpShiftL)
		case ">>":
			c.emit(OpShiftR)
		default:
			return fmt.Errorf("unknown infix operator: %s", n.Operator)
		}
		return nil

	case *parser.CallExpression:
		// Compile arguments first (stack order: args first, then function)
		for _, arg := range n.Arguments {
			if err := c.Compile(arg); err != nil {
				return err
			}
		}

		// Then compile the function reference
		if err := c.Compile(n.Function); err != nil {
			return err
		}

		c.emit(OpCall, len(n.Arguments))
		return nil

	case *parser.AssignmentExpression:
		// Handle different assignment types
		switch left := n.Left.(type) {
		case *parser.Identifier:
			var symbol Symbol
			var ok bool
			if n.Operator == ":=" {
				// Short variable declaration: define new symbol
				symbol = c.symbolTable.Define(left.Value)
			} else {
				// Regular assignment: resolve existing symbol
				symbol, ok = c.symbolTable.Resolve(left.Value)
				if !ok {
					return fmt.Errorf("undefined variable: %s", left.Value)
				}
			}

			// Handle compound assignments (+=, -=, etc.)
			if n.Operator != "=" && n.Operator != ":=" {
				// Load current value first
				c.loadSymbol(symbol)
				// Then compile right hand side
				if err := c.Compile(n.Right); err != nil {
					return err
				}
				// Compile operation
				switch n.Operator {
				case "+=":
					c.emit(OpAdd)
				case "-=":
					c.emit(OpSub)
				case "*=":
					c.emit(OpMul)
				case "/=":
					c.emit(OpDiv)
				case "%=":
					c.emit(OpMod)
				case "&=":
					c.emit(OpBitAnd)
				case "|=":
					c.emit(OpBitOr)
				case "^=":
					c.emit(OpBitXor)
				case "<<=":
					c.emit(OpShiftL)
				case ">>=":
					c.emit(OpShiftR)
				}
			} else {
				// Simple assignment, just compile right hand side
				if err := c.Compile(n.Right); err != nil {
					return err
				}
			}

			// Store the result
			c.storeSymbol(symbol)

			// Update symbol type based on right-hand side expression
			inferredType := c.inferExpressionType(n.Right)
			if inferredType != nil {
				c.symbolTable.UpdateType(symbol.Name, inferredType)
			}

			// Assignment expressions return the assigned value, so push it back to the stack
			c.loadSymbol(symbol)

		case *parser.IndexExpression:
			// Index assignment: arr[index] = value
			// Compile array/collection
			if err := c.Compile(left.Left); err != nil {
				return err
			}
			// Compile index
			if err := c.Compile(left.Index); err != nil {
				return err
			}
			// Compile value
			if err := c.Compile(n.Right); err != nil {
				return err
			}
			// Store to index
			c.emit(OpIndexSet)
			// Assignment returns the value
			if err := c.Compile(n.Right); err != nil {
				return err
			}

		case *parser.MemberExpression:
			// Member assignment: obj.property = value
			// Compile object
			if err := c.Compile(left.Object); err != nil {
				return err
			}
			// Compile value
			if err := c.Compile(n.Right); err != nil {
				return err
			}
			// Emit member set opcode with property name constant
			nameIdx := c.addConstant(&bytecode.StringConstant{Value: left.Member.Value})
			c.emit(OpMemberSet, nameIdx)
			// Assignment returns the value
			if err := c.Compile(n.Right); err != nil {
				return err
			}

		default:
			return fmt.Errorf("cannot assign to %T", n.Left)
		}
		return nil

	case *parser.PostfixExpression:
		// Postfix increment/decrement: i++ or i--
		// Check that the operand is assignable
		ident, ok := n.Left.(*parser.Identifier)
		if !ok {
			return fmt.Errorf("postfix %s operator requires an assignable identifier", n.Operator)
		}

		symbol, ok := c.symbolTable.Resolve(ident.Value)
		if !ok {
			return fmt.Errorf("undefined variable: %s", ident.Value)
		}

		// Load current value (this is the value we return)
		c.loadSymbol(symbol)
		// Duplicate it for the operation
		c.emit(OpDup)
		// Push constant 1
		constIdx := c.addConstant(&bytecode.IntConstant{Value: int64(1)})
		c.emit(OpPush, constIdx)
		// Add or subtract
		if n.Operator == "++" {
			c.emit(OpAdd)
		} else {
			c.emit(OpSub)
		}
		// Store the new value back
		c.storeSymbol(symbol)
		// Postfix operator returns the original value (already on stack)
		return nil

	case *parser.ArrayLiteral:
		// Compile all elements (pushed to stack in order)
		for _, elem := range n.Elements {
			if err := c.Compile(elem); err != nil {
				return err
			}
		}
		// Create array with element count
		c.emit(OpNewArray, len(n.Elements))
		return nil

	case *parser.IndexExpression:
		// Compile array/collection
		if err := c.Compile(n.Left); err != nil {
			return err
		}
		// Compile index
		if err := c.Compile(n.Index); err != nil {
			return err
		}
		// Get value from index
		c.emit(OpIndexGet)
		return nil

	case *parser.MapLiteral:
		// Create empty map
		c.emit(OpNewMap)
		// Add all key-value pairs
		for keyExpr, valueExpr := range n.Pairs {
			// Duplicate map reference (since we'll consume one for each set)
			c.emit(OpDup)
			// Compile key (will be converted to string)
			if err := c.Compile(keyExpr); err != nil {
				return err
			}
			// Compile value
			if err := c.Compile(valueExpr); err != nil {
				return err
			}
			// Set to map
			c.emit(OpIndexSet)
			// Pop the returned value (since IndexSet returns the value)
			c.emit(OpPop)
		}
		return nil

	case *parser.ImportStatement:
		// Module path
		modulePath := string(n.ModulePath.Value)
		pathConstIdx := c.addConstant(&bytecode.StringConstant{Value: modulePath})

		// Resolve the module to check if it exists and has the exports
		_, err := c.resolveModule(modulePath)
		if err != nil {
			c.addError(err.Error())
			return err
		}

		// Emit import opcode to load the module at runtime
		c.emit(OpImport, pathConstIdx)

		// Handle import types
		if n.Namespace != nil && n.Specs == nil {
			// Namespace import: import * as ns from "mod"
			moduleSymbol := c.symbolTable.Define(n.Namespace.Value)
			if moduleSymbol.Scope == ScopeGlobal {
				c.emit(OpStoreGlobal, c.addConstant(&bytecode.StringConstant{Value: n.Namespace.Value}))
			} else {
				c.emit(OpStoreLocal, moduleSymbol.Index)
			}
			return nil
		}

		if n.DefaultImport != nil {
			// Handle default import
			// Duplicate module reference if there are also named imports
			if len(n.Specs) > 0 {
				c.emit(OpDup)
			}

			// Import the "default" export
			nameConstIdx := c.addConstant(&bytecode.StringConstant{Value: "default"})
			c.emit(OpImportMember, nameConstIdx)

			// Store in local variable
			localSymbol := c.symbolTable.Define(n.DefaultImport.Value)
			if localSymbol.Scope == ScopeGlobal {
				c.emit(OpStoreGlobal, c.addConstant(&bytecode.StringConstant{Value: n.DefaultImport.Value}))
			} else {
				c.emit(OpStoreLocal, localSymbol.Index)
			}

			// If there are no named imports, return immediately to avoid falling through to bare import
			if len(n.Specs) == 0 {
				return nil
			}
		}

		if len(n.Specs) > 0 {
			// Named imports: import { a, b } from "mod"
			// Duplicate the module reference for each import except the last
			for i, spec := range n.Specs {
				if i < len(n.Specs)-1 {
					// Duplicate the module reference for next import
					c.emit(OpDup)
				}

				exportName := spec.Name.Value
				localName := exportName
				if spec.Alias != nil {
					localName = spec.Alias.Value
				}

				// Emit import member opcode
				nameConstIdx := c.addConstant(&bytecode.StringConstant{Value: exportName})
				c.emit(OpImportMember, nameConstIdx)

				// Store in local variable
				localSymbol := c.symbolTable.Define(localName)
				if localSymbol.Scope == ScopeGlobal {
					c.emit(OpStoreGlobal, c.addConstant(&bytecode.StringConstant{Value: localName}))
				} else {
					c.emit(OpStoreLocal, localSymbol.Index)
				}
			}
			return nil
		}

		// Bare import: import "mod"
		// Just pop the module reference
		c.emit(OpPop)
		return nil

	case *parser.ExportStatement:
		if n.IsDefault {
			// Default export
			var symbolName string

			if n.Declaration != nil {
				switch decl := n.Declaration.(type) {
				case *parser.FunctionDeclaration:
					// Function declaration: already has a name
					symbolName = decl.Name.Value
					if err := c.Compile(n.Declaration); err != nil {
						return err
					}

				case *parser.VarStatement:
					// Var declaration
					symbolName = decl.Name.Value
					if err := c.Compile(n.Declaration); err != nil {
						return err
					}

				case *parser.LetStatement:
					// Let declaration
					symbolName = decl.Name.Value
					if err := c.Compile(n.Declaration); err != nil {
						return err
					}

				case *parser.ConstStatement:
					// Const declaration
					symbolName = decl.Name.Value
					if err := c.Compile(n.Declaration); err != nil {
						return err
					}

				case *parser.ClassDeclaration:
					// Class declaration
					symbolName = decl.Name.Value
					if err := c.Compile(n.Declaration); err != nil {
						return err
					}

				case *parser.ExpressionStatement:
					// Expression statement: need to store in a temp variable
					defaultVarName := "__default_export__"
					symbolName = defaultVarName
					// Compile the expression (value stays on stack)
					if err := c.Compile(decl.Expression); err != nil {
						return err
					}
					// Create a hidden variable to hold the value
					defaultSymbol := c.symbolTable.Define(defaultVarName)
					// Store the value from the stack
					if defaultSymbol.Scope == ScopeGlobal {
						c.emit(OpStoreGlobal, c.addConstant(&bytecode.StringConstant{Value: defaultVarName}))
					} else {
						c.emit(OpStoreLocal, defaultSymbol.Index)
					}

				default:
					return fmt.Errorf("unsupported default export type: %T", decl)
				}
			} else {
				return fmt.Errorf("default export requires a declaration or expression")
			}

			// Export as "default"
			c.Exports["default"] = symbolName
			return nil
		}

		if n.Declaration != nil {
			// Export a declaration (func, const, let, class)
			if err := c.Compile(n.Declaration); err != nil {
				return err
			}

			// Track the export
			var name string
			switch decl := n.Declaration.(type) {
			case *parser.FunctionDeclaration:
				name = decl.Name.Value
			case *parser.VarStatement:
				name = decl.Name.Value
			case *parser.LetStatement:
				name = decl.Name.Value
			case *parser.ConstStatement:
				name = decl.Name.Value
			case *parser.ClassDeclaration:
				name = decl.Name.Value
			default:
				c.addError(fmt.Sprintf("cannot export declaration of type %T", decl))
				return nil
			}

			// Store the export: export name -> symbol name
			c.Exports[name] = name
			return nil
		}

		if len(n.Specs) > 0 {
			// Export list: export { a, b as c }
			for _, spec := range n.Specs {
				name := spec.Name.Value
				alias := name
				if spec.Alias != nil {
					alias = spec.Alias.Value
				}

				// Store the export with alias: export alias -> original symbol name
				c.Exports[alias] = name
			}
			return nil
		}

		c.addError("invalid export statement")
		return nil

	case *parser.TryStatement:
		// Emit TRY opcode with placeholder offsets
		tryPos := c.emit(OpTry, 0xFFFF, 0xFFFF)

		// Compile try block
		if err := c.Compile(n.TryBlock); err != nil {
			return err
		}

		// If there's a finally block, jump to it first
		var jumpAfterTry int
		if n.Finally != nil {
			jumpAfterTry = c.emit(OpJmp, 0xFFFF) // Will jump to finally block
		} else {
			jumpAfterTry = c.emit(OpJmp, 0xFFFF) // Jump to end
		}

		// Patch catch offset
		catchPos := len(c.currentInstructions())
		c.changeOperand(tryPos, catchPos, 0) // First operand is catch offset

		// Compile catch block if exists
		if n.Catch != nil {
			if err := c.Compile(n.Catch); err != nil {
				return err
			}
		}

		// If there's a finally block, jump to it after catch
		var jumpAfterCatch int
		if n.Finally != nil {
			jumpAfterCatch = c.emit(OpJmp, 0xFFFF) // Will jump to finally block
		} else {
			jumpAfterCatch = c.emit(OpJmp, 0xFFFF) // Jump to end
		}

		// Patch finally offset
		finallyPos := len(c.currentInstructions())
		c.changeOperand(tryPos, finallyPos, 1) // Second operand is finally offset

		// Compile finally block if exists
		if n.Finally != nil {
			if err := c.Compile(n.Finally); err != nil {
				return err
			}
		}

		// Patch jumps
		endPos := len(c.currentInstructions())
		if n.Finally != nil {
			// Jump from end of try to finally
			c.changeOperand(jumpAfterTry, finallyPos)
			// Jump from end of catch to finally
			c.changeOperand(jumpAfterCatch, finallyPos)
		} else {
			// Jump directly to end
			c.changeOperand(jumpAfterTry, endPos)
			c.changeOperand(jumpAfterCatch, endPos)
		}

		return nil

	case *parser.CatchStatement:
		// Emit CATCH opcode to get the exception
		c.emit(OpCatch)

		// Store the exception in the catch parameter if provided
		if n.Param != nil {
			symbol := c.symbolTable.Define(n.Param.Value)
			if symbol.Scope == ScopeGlobal {
				c.emit(OpStoreGlobal, c.addConstant(&bytecode.StringConstant{Value: n.Param.Value}))
			} else {
				c.emit(OpStoreLocal, symbol.Index)
			}
		} else {
			// No parameter, pop the exception
			c.emit(OpPop)
		}

		// Compile catch block
		if err := c.Compile(n.CatchBlock); err != nil {
			return err
		}

		return nil

	case *parser.FinallyStatement:
		// Emit FINALLY opcode
		c.emit(OpFinally)

		// Compile finally block
		if err := c.Compile(n.FinallyBlock); err != nil {
			return err
		}

		return nil

	case *parser.DeferStatement:
		// Handle block syntax: defer { ... }
		if n.Block != nil {
			// For defer block, we need to compile the block into a function
			// and then defer that function call
			c.enterScope()

			// Compile the block body
			for _, stmt := range n.Block.Statements {
				if err := c.Compile(stmt); err != nil {
					return err
				}
			}

			// Add implicit return if not present
			c.emit(OpReturnVoid)

			// Get the compiled instructions
			instructions := c.currentInstructions()
			numLocals := c.symbolTable.Count()

			c.leaveScope()

			// Create a function constant for the deferred block
			fnConstant := &bytecode.FunctionConstant{
				Name:          "defer",
				NumLocals:     numLocals,
				NumParameters: 0,
				IsVariadic:    false,
				Instructions:  instructions,
			}

			// Add to constant pool
			fnIdx := c.addConstant(fnConstant)

			// Emit OpLoadConst to load the function
			c.emit(OpLoadConst, fnIdx)

			// Emit DEFER opcode with 0 arguments
			c.emit(OpDefer, 0)

			return nil
		}

		// Original function call syntax: defer functionCall()
		// Compile all arguments first (evaluated immediately)
		for _, arg := range n.Call.Arguments {
			if err := c.Compile(arg); err != nil {
				return err
			}
		}

		// Compile the function expression (evaluated immediately)
		if err := c.Compile(n.Call.Function); err != nil {
			return err
		}

		// Emit DEFER opcode with argument count
		c.emit(OpDefer, len(n.Call.Arguments))

		// Defer statement doesn't leave anything on the stack
		return nil

	case *parser.ThrowStatement:
		// Compile the value to throw
		if err := c.Compile(n.Value); err != nil {
			return err
		}

		// Emit THROW opcode
		c.emit(OpThrow)

		return nil

	default:
		return fmt.Errorf("unsupported node type: %T", n)
	}
}

// compileShortCircuit compiles && and || operators with short-circuit evaluation
func (c *Compiler) compileShortCircuit(exp *parser.InfixExpression) error {
	if err := c.Compile(exp.Left); err != nil {
		return err
	}

	var jumpOp Opcode
	var jumpPos int

	if exp.Operator == "&&" {
		// AND: if left is false, jump to end (left is already on stack)
		jumpOp = OpJmpIfFalse
	} else {
		// OR: if left is true, jump to end (left is already on stack)
		jumpOp = OpJmpIfTrue
	}

	// Duplicate the left value for the jump check
	c.emit(OpDup)

	jumpPos = c.emit(jumpOp, 0xFFFF)

	// Pop the left value, evaluate right side
	c.emit(OpPop)

	// Compile right expression
	if err := c.Compile(exp.Right); err != nil {
		return err
	}

	// Patch jump offset
	endPos := len(c.currentInstructions())
	c.changeOperand(jumpPos, endPos)
	return nil
}

// loadSymbol emits the appropriate load instruction for a symbol
func (c *Compiler) loadSymbol(symbol Symbol) {
	switch symbol.Scope {
	case ScopeGlobal:
		nameIdx := c.addConstant(&bytecode.StringConstant{Value: symbol.Name})
		c.emit(OpLoadGlobal, nameIdx)
	case ScopeLocal:
		c.emit(OpLoadLocal, symbol.Index)
	case ScopeBuiltin:
		// Check if it's a constant (has a Type value)
		if symbol.Type != nil {
			// It's a constant, convert types.Object to bytecode.Constant and push the value directly
			var constVal bytecode.Constant
			switch v := symbol.Type.(type) {
			case types.Int:
				constVal = &bytecode.IntConstant{Value: int64(v)}
			case types.Float:
				constVal = &bytecode.FloatConstant{Value: float64(v)}
			case types.Bool:
				constVal = &bytecode.BoolConstant{Value: bool(v)}
			case types.String:
				constVal = &bytecode.StringConstant{Value: string(v)}
			case types.Char:
				constVal = &bytecode.CharConstant{Value: rune(v)}
			case *types.Null:
				constVal = &bytecode.NilConstant{}
			case *types.Undefined:
				constVal = &bytecode.NilConstant{}
			default:
				// For other types (like functions), load as global
				nameIdx := c.addConstant(&bytecode.StringConstant{Value: symbol.Name})
				c.emit(OpLoadGlobal, nameIdx)
				return
			}
			constIdx := c.addConstant(constVal)
			c.emit(OpPush, constIdx)
		} else {
			// For built-in functions, load as a global - the VM will resolve it
			nameIdx := c.addConstant(&bytecode.StringConstant{Value: symbol.Name})
			c.emit(OpLoadGlobal, nameIdx)
		}
	case ScopeFree:
		c.emit(OpLoadUpvalue, symbol.Index)
	case ScopeFunction:
		// Function name refers to current function
		nameIdx := c.addConstant(&bytecode.StringConstant{Value: symbol.Name})
		c.emit(OpLoadGlobal, nameIdx)
	}
}

// storeSymbol emits the appropriate store instruction for a symbol
func (c *Compiler) storeSymbol(symbol Symbol) {
	switch symbol.Scope {
	case ScopeGlobal:
		nameIdx := c.addConstant(&bytecode.StringConstant{Value: symbol.Name})
		c.emit(OpStoreGlobal, nameIdx)
	case ScopeLocal:
		c.emit(OpStoreLocal, symbol.Index)
	case ScopeFree:
		c.emit(OpStoreUpvalue, symbol.Index)
	default:
		// Cannot assign to built-in or function scope symbols
		c.addError(fmt.Sprintf("cannot assign to %s variable %s", symbol.Scope, symbol.Name))
	}
}

// isIntegerLiteral checks if an expression is an integer literal
func isIntegerLiteral(expr parser.Expression) bool {
	_, ok := expr.(*parser.IntLiteral)
	return ok
}

// inferExpressionType infers the type of an expression at compile time
func (c *Compiler) inferExpressionType(expr parser.Expression) types.Object {
	switch e := expr.(type) {
	case *parser.IntLiteral:
		return e.Value
	case *parser.FloatLiteral:
		return e.Value
	case *parser.StringLiteral:
		return e.Value
	case *parser.BoolLiteral:
		return e.Value
	case *parser.Identifier:
		if symbol, ok := c.symbolTable.Resolve(e.Value); ok {
			return symbol.Type
		}
	case *parser.InfixExpression:
		leftType := c.inferExpressionType(e.Left)
		rightType := c.inferExpressionType(e.Right)
		if leftType == nil || rightType == nil {
			return nil
		}
		// Type inference for operations
		switch e.Operator {
		case "+", "-", "*", "/", "%":
			// If both are same numeric type, result is that type
			if _, ok := leftType.(types.Int); ok {
				if _, ok := rightType.(types.Int); ok {
					return leftType
				}
			}
			if _, ok := leftType.(types.Float); ok {
				if _, ok := rightType.(types.Float); ok {
					return leftType
				}
			}
		case "==", "!=", "<", "<=", ">", ">=":
			return types.Bool(true)
		}
	}
	return nil
}

// isIntegerType checks if an object is an integer type
func isIntegerType(obj types.Object) bool {
	_, ok := obj.(types.Int)
	return ok
}

// getSmallRangeCount checks if expr is range(n) with small n, returns count or -1
func getSmallRangeCount(expr parser.Expression) int {
	call, ok := expr.(*parser.CallExpression)
	if !ok {
		return -1
	}
	// Check if function is "range"
	if ident, ok := call.Function.(*parser.Identifier); !ok || ident.Value != "range" {
		return -1
	}
	// Check arguments - range(n) where n is an integer literal
	if len(call.Arguments) != 1 {
		return -1
	}
	if lit, ok := call.Arguments[0].(*parser.IntLiteral); ok {
		count := int(lit.Value)
		if count > 0 && count <= 16 { // Unroll small loops up to 16 iterations
			return count
		}
	}
	return -1
}

// emit appends an opcode and its operands to the current instructions
func (c *Compiler) emit(op Opcode, operands ...int) int {
	pos := c.addInstruction(op, operands...)

	// Add line number info
	if c.currentLine > 0 {
		c.lineNumberTable = append(c.lineNumberTable, bytecode.LineInfo{
			Offset: pos,
			Line:   c.currentLine,
			Column: c.currentColumn,
		})
	}

	return pos
}

// addInstruction adds an instruction to the current scope
func (c *Compiler) addInstruction(op Opcode, operands ...int) int {
	inst := makeInstruction(op, operands...)
	pos := len(c.currentInstructions())
	c.scopes[c.scopeIndex].instructions = append(c.currentInstructions(), inst...)
	return pos
}

// makeInstruction creates a bytecode instruction from opcode and operands
func makeInstruction(op Opcode, operands ...int) []byte {
	info, ok := OpcodeTable[op]
	if !ok {
		return []byte{}
	}

	instruction := make([]byte, 1+info.Operands)
	instruction[0] = byte(op)

	offset := 1
	operandIndex := 0
	remainingOperands := info.Operands

	for remainingOperands > 0 && operandIndex < len(operands) {
		o := operands[operandIndex]

		if remainingOperands >= 2 {
			// 2-byte operand
			instruction[offset] = byte(o >> 8)
			instruction[offset+1] = byte(o & 0xFF)
			offset += 2
			remainingOperands -= 2
		} else {
			// 1-byte operand
			instruction[offset] = byte(o)
			offset += 1
			remainingOperands -= 1
		}

		operandIndex++
	}

	return instruction
}

// changeOperand changes the operand of an existing instruction
// operandIndex specifies which operand to change (0-based)
func (c *Compiler) changeOperand(pos int, newOperand int, operandIndex ...int) {
	op := Opcode(c.currentInstructions()[pos])

	idx := 0
	if len(operandIndex) > 0 {
		idx = operandIndex[0]
	}

	offset := 1 // offset from start of instruction (pos)
	// Calculate offset based on operand sizes
	for i := 0; i < idx; i++ {
		// Assume operands are either 1 or 2 bytes
		// For simplicity, treat all jump operands as 2 bytes, others as per their definition
		// This is a simplification that works for our current opcode set
		if op == OpJmp || op == OpJmpIfTrue || op == OpJmpIfFalse || op == OpTry {
			offset += 2
		} else {
			offset += 1
		}
	}

	absoluteOffset := pos + offset

	if op == OpJmp || op == OpJmpIfTrue || op == OpJmpIfFalse || op == OpTry {
		// 2-byte operand
		if absoluteOffset+1 >= len(c.currentInstructions()) {
			return
		}
		c.currentInstructions()[absoluteOffset] = byte(newOperand >> 8)
		c.currentInstructions()[absoluteOffset+1] = byte(newOperand & 0xFF)
	} else {
		// 1-byte operand
		if absoluteOffset >= len(c.currentInstructions()) {
			return
		}
		c.currentInstructions()[absoluteOffset] = byte(newOperand)
	}
}

// addConstant adds a constant to the constant pool and returns its index
// Deduplicates existing constants to avoid redundant entries
func (c *Compiler) addConstant(constant bytecode.Constant) int {
	// Check if constant already exists
	for i, existing := range c.constants {
		if constantsEqual(existing, constant) {
			return i
		}
	}
	// If not found, add new constant
	c.constants = append(c.constants, constant)
	return len(c.constants) - 1
}

// constantsEqual checks if two constants are identical
func constantsEqual(a, b bytecode.Constant) bool {
	if a.Type() != b.Type() {
		return false
	}

	switch ac := a.(type) {
	case *bytecode.NilConstant:
		_, ok := b.(*bytecode.NilConstant)
		return ok
	case *bytecode.BoolConstant:
		bc, ok := b.(*bytecode.BoolConstant)
		return ok && ac.Value == bc.Value
	case *bytecode.IntConstant:
		bc, ok := b.(*bytecode.IntConstant)
		return ok && ac.Value == bc.Value
	case *bytecode.CharConstant:
		bc, ok := b.(*bytecode.CharConstant)
		return ok && ac.Value == bc.Value
	case *bytecode.FloatConstant:
		bc, ok := b.(*bytecode.FloatConstant)
		return ok && ac.Value == bc.Value
	case *bytecode.StringConstant:
		bc, ok := b.(*bytecode.StringConstant)
		return ok && ac.Value == bc.Value
	case *bytecode.FunctionConstant:
		bc, ok := b.(*bytecode.FunctionConstant)
		if !ok {
			return false
		}
		// Compare all function fields
		if ac.Name != bc.Name || ac.NumLocals != bc.NumLocals || ac.NumParameters != bc.NumParameters || ac.IsVariadic != bc.IsVariadic {
			return false
		}
		// Compare instructions
		if len(ac.Instructions) != len(bc.Instructions) {
			return false
		}
		for i := range ac.Instructions {
			if ac.Instructions[i] != bc.Instructions[i] {
				return false
			}
		}
		// Compare default values
		if len(ac.DefaultValues) != len(bc.DefaultValues) {
			return false
		}
		for i := range ac.DefaultValues {
			if ac.DefaultValues[i] != bc.DefaultValues[i] {
				return false
			}
		}
		return true
	case *bytecode.ClassConstant:
		bc, ok := b.(*bytecode.ClassConstant)
		if !ok {
			return false
		}
		// Compare all class fields
		if ac.Name != bc.Name || ac.SuperClass != bc.SuperClass {
			return false
		}
		// Compare methods
		if len(ac.Methods) != len(bc.Methods) {
			return false
		}
		for i := range ac.Methods {
			if ac.Methods[i] != bc.Methods[i] {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// enterScope enters a new compilation scope (for functions)
func (c *Compiler) enterScope() {
	scope := CompilationScope{
		instructions: []byte{},
		numLocals:    0,
	}
	c.scopes = append(c.scopes, scope)
	c.scopeIndex++
	c.symbolTable = NewEnclosedSymbolTable(c.symbolTable)
}

// leaveScope exits the current compilation scope
func (c *Compiler) leaveScope() {
	c.scopes = c.scopes[:len(c.scopes)-1]
	c.scopeIndex--
	c.symbolTable = c.symbolTable.Outer
}

// currentInstructions returns the instructions of the current scope
func (c *Compiler) currentInstructions() []byte {
	return c.scopes[c.scopeIndex].instructions
}

// currentScope returns the current compilation scope
func (c *Compiler) currentScope() *CompilationScope {
	return &c.scopes[c.scopeIndex]
}

// Bytecode returns the compiled bytecode
func (c *Compiler) Bytecode() *bytecode.Bytecode {
	// Find main function (create if not exists)
	// Use symbolTable.Count() to get the actual number of local definitions
	mainFunc := &bytecode.FunctionConstant{
		Name:         "main",
		Instructions: c.currentInstructions(),
		NumLocals:    c.symbolTable.Count(),
	}
	mainIdx := c.addConstant(mainFunc)

	return &bytecode.Bytecode{
		Constants:       c.constants,
		MainFunc:        mainIdx,
		LineNumberTable: c.lineNumberTable,
	}
}

// Errors returns the list of compilation errors
func (c *Compiler) Errors() []string {
	return c.errors
}

// addError adds a compilation error
func (c *Compiler) addError(msg string) {
	c.errors = append(c.errors, msg)
}

// lastOpcode returns the last opcode in the instruction list
func lastOpcode(instructions []byte) Opcode {
	if len(instructions) == 0 {
		return OpNOP
	}
	return Opcode(instructions[len(instructions)-1])
}
