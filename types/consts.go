// Package types defines the core type system and Object interface for Nxlang
package types

// Fixed type code constants (no iota for cross-version compatibility)
const (
	TypeUndefined = 0x00
	TypeNull      = 0x01
	TypeBool      = 0x02
	TypeByte      = 0x03
	TypeChar      = 0x04
	TypeInt       = 0x05
	TypeUInt      = 0x06
	TypeFloat     = 0x07
	TypeString    = 0x08
	TypeArray     = 0x10
	TypeMap       = 0x11
	TypeOrderedMap= 0x12
	TypeStack     = 0x13
	TypeQueue     = 0x14
	TypeSeq       = 0x15
	TypeFunction  = 0x20
	TypeClosure   = 0x21
	TypeNativeFunc= 0x22
	TypeClass     = 0x30
	TypeObject    = 0x31
	TypeInterface = 0x32
	TypeBoundMethod = 0x33
	TypeSuperReference = 0x34
	TypeRef         = 0x35 // Object reference type
	TypeObjectType  = 0x36 // Type object (for int, float, string etc. with static methods)
	TypeError     = 0x40
	TypeMutex     = 0x50
	TypeRWMutex   = 0x51
	TypeThread    = 0x52
	TypeFile      = 0x60
	TypeReader    = 0x61
	TypeWriter    = 0x62
	TypeStringBuilder = 0x63
	TypeBytesBuffer   = 0x64
)

// Type names corresponding to type codes
var typeNames = map[uint8]string{
	TypeUndefined: "undefined",
	TypeNull:      "null",
	TypeBool:      "bool",
	TypeByte:      "byte",
	TypeChar:      "char",
	TypeInt:       "int",
	TypeUInt:      "uint",
	TypeFloat:     "float",
	TypeString:    "string",
	TypeArray:     "array",
	TypeMap:       "map",
	TypeOrderedMap: "orderedMap",
	TypeStack:     "stack",
	TypeQueue:     "queue",
	TypeSeq:       "seq",
	TypeFunction:  "function",
	TypeClosure:   "closure",
	TypeNativeFunc: "nativeFunc",
	TypeClass:     "class",
	TypeObject:    "object",
	TypeInterface: "interface",
	TypeBoundMethod: "boundMethod",
	TypeSuperReference: "superReference",
	TypeRef:         "ref",
	TypeObjectType:  "typeObject",
	TypeError:     "error",
	TypeMutex:     "mutex",
	TypeRWMutex:   "rwMutex",
	TypeThread:    "thread",
	TypeFile:      "file",
	TypeReader:    "reader",
	TypeWriter:    "writer",
	TypeStringBuilder: "stringBuilder",
	TypeBytesBuffer: "bytesBuffer",
}
