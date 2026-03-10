package compiler

import "github.com/topxeq/nxlang/types"

// SymbolScope represents the scope of a symbol
type SymbolScope string

const (
	ScopeGlobal   SymbolScope = "GLOBAL"
	ScopeLocal    SymbolScope = "LOCAL"
	ScopeBuiltin  SymbolScope = "BUILTIN"
	ScopeFree     SymbolScope = "FREE"
	ScopeFunction SymbolScope = "FUNCTION"
)

// Symbol represents a variable/function symbol in the symbol table
type Symbol struct {
	Name  string
	Scope SymbolScope
	Index int
	Type  types.Object // Optional type information
}

// SymbolTable manages symbols across nested scopes
type SymbolTable struct {
	Outer *SymbolTable

	store          map[string]Symbol
	numDefinitions int

	// Free variables for closures
	FreeSymbols []Symbol
}

// NewSymbolTable creates a new root symbol table
func NewSymbolTable() *SymbolTable {
	s := make(map[string]Symbol)
	freeSymbols := []Symbol{}
	return &SymbolTable{store: s, FreeSymbols: freeSymbols}
}

// NewEnclosedSymbolTable creates a new symbol table enclosed in an outer scope
func NewEnclosedSymbolTable(outer *SymbolTable) *SymbolTable {
	st := NewSymbolTable()
	st.Outer = outer
	return st
}

// Define adds a new symbol to the current scope
func (st *SymbolTable) Define(name string) Symbol {
	symbol := Symbol{Name: name, Index: st.numDefinitions}
	if st.Outer == nil {
		symbol.Scope = ScopeGlobal
	} else {
		symbol.Scope = ScopeLocal
	}
	st.store[name] = symbol
	st.numDefinitions++
	return symbol
}

// DefineBuiltin adds a built-in function symbol
func (st *SymbolTable) DefineBuiltin(index int, name string) Symbol {
	symbol := Symbol{Name: name, Index: index, Scope: ScopeBuiltin}
	st.store[name] = symbol
	return symbol
}

// DefineConstant adds a constant symbol
func (st *SymbolTable) DefineConstant(name string, value types.Object) Symbol {
	symbol := Symbol{Name: name, Index: 0, Scope: ScopeBuiltin, Type: value}
	st.store[name] = symbol
	return symbol
}

// DefineFunctionName defines a function name symbol
func (st *SymbolTable) DefineFunctionName(name string) Symbol {
	symbol := Symbol{Name: name, Index: 0, Scope: ScopeFunction}
	st.store[name] = symbol
	return symbol
}

// DefineFree defines a free variable symbol (captured from outer scope)
func (st *SymbolTable) DefineFree(original Symbol) Symbol {
	st.FreeSymbols = append(st.FreeSymbols, original)
	symbol := Symbol{
		Name:  original.Name,
		Index: len(st.FreeSymbols) - 1,
		Scope: ScopeFree,
	}
	st.store[original.Name] = symbol
	return symbol
}

// Resolve looks up a symbol by name, searching outer scopes if needed
func (st *SymbolTable) Resolve(name string) (Symbol, bool) {
	symbol, ok := st.store[name]
	if !ok && st.Outer != nil {
		symbol, ok = st.Outer.Resolve(name)
		if !ok {
			return symbol, ok
		}

		// If symbol is from outer scope and not global/builtin, it's a free variable
		if symbol.Scope != ScopeGlobal && symbol.Scope != ScopeBuiltin {
			return st.DefineFree(symbol), true
		}

		return symbol, ok
	}
	return symbol, ok
}

// Count returns the number of defined symbols in current scope
func (st *SymbolTable) Count() int {
	return st.numDefinitions
}
