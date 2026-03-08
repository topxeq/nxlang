package types

import "fmt"

// StackFrame represents a single frame in the call stack
type StackFrame struct {
	FunctionName string
	Line         int
	Column       int
	Filename     string
}

// Error represents a runtime error in Nxlang
type Error struct {
	Message    string
	Line       int
	Column     int
	Filename   string
	Code       string // Code snippet where the error occurred
	Stack      []StackFrame // Call stack trace
	ErrorType  string // Custom error type name
	Properties map[string]Object // Custom properties for user-defined errors
	Methods    map[string]*Function // Custom methods for user-defined errors
}

// NewError creates a new Error instance with basic message
func NewError(message string, line int, column int, filename string) *Error {
	return &Error{
		Message:    message,
		Line:       line,
		Column:     column,
		Filename:   filename,
		ErrorType:  "Error",
		Properties: make(map[string]Object),
		Methods:    make(map[string]*Function),
	}
}

// NewErrorWithCode creates a new Error instance with code snippet
func NewErrorWithCode(message string, line int, column int, filename string, code string) *Error {
	err := NewError(message, line, column, filename)
	err.Code = code
	return err
}

// NewErrorWithStack creates a new Error instance with call stack trace
func NewErrorWithStack(message string, line int, column int, filename string, stack []StackFrame) *Error {
	err := NewError(message, line, column, filename)
	err.Stack = stack
	return err
}

// NewCustomError creates a new custom Error instance with the given type name
func NewCustomError(typeName string, message string, line int, column int, filename string) *Error {
	err := NewError(message, line, column, filename)
	err.ErrorType = typeName
	return err
}

// TypeCode implements Object interface
func (e *Error) TypeCode() uint8 {
	return TypeError
}

// TypeName implements Object interface
func (e *Error) TypeName() string {
	if e.ErrorType != "" {
		return e.ErrorType
	}
	return typeNames[TypeError]
}

// ToStr implements Object interface
func (e *Error) ToStr() string {
	var result string
	errorType := "TXERROR"
	if e.ErrorType != "" && e.ErrorType != "Error" {
		errorType = e.ErrorType
	}
	if e.Line == 0 {
		result = fmt.Sprintf("%s: %s", errorType, e.Message)
	} else if e.Code != "" {
		result = fmt.Sprintf("%s: %s at %s:%d:%d\n%s", errorType, e.Message, e.Filename, e.Line, e.Column, e.Code)
	} else {
		result = fmt.Sprintf("%s: %s at %s:%d:%d", errorType, e.Message, e.Filename, e.Line, e.Column)
	}

	// Add stack trace if available
	if len(e.Stack) > 0 {
		result += "\nStack trace:"
		for i, frame := range e.Stack {
			if frame.FunctionName == "" {
				result += fmt.Sprintf("\n  %d: at %s:%d:%d", i+1, frame.Filename, frame.Line, frame.Column)
			} else {
				result += fmt.Sprintf("\n  %d: in %s() at %s:%d:%d", i+1, frame.FunctionName, frame.Filename, frame.Line, frame.Column)
			}
		}
	}

	return result
}

// Equals implements Object interface
func (e *Error) Equals(other Object) bool {
	otherErr, ok := other.(*Error)
	if !ok {
		return false
	}
	if e.ErrorType != otherErr.ErrorType || e.Message != otherErr.Message || e.Line != otherErr.Line || e.Column != otherErr.Column || e.Filename != otherErr.Filename {
		return false
	}
	// Compare custom properties
	if len(e.Properties) != len(otherErr.Properties) {
		return false
	}
	for k, v := range e.Properties {
		otherV, ok := otherErr.Properties[k]
		if !ok || !v.Equals(otherV) {
			return false
		}
	}
	return true
}

// Error implements the standard Go error interface
func (e *Error) Error() string {
	return e.ToStr()
}
