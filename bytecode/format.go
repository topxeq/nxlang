package bytecode

// Magic number for Nxlang bytecode files
const Magic = 0x4E584230 // "NXB0" in hex

// Version represents the bytecode format version
const (
	VersionMajor = 1
	VersionMinor = 0
	VersionPatch = 0
)

// FullVersion combines version numbers into a single uint32
var FullVersion = uint32(VersionMajor<<16 | VersionMinor<<8 | VersionPatch)

// ConstantType identifies the type of constant in the constant pool
type ConstantType byte

const (
	ConstNil        ConstantType = 0x00
	ConstBool       ConstantType = 0x01
	ConstInt        ConstantType = 0x02
	ConstFloat      ConstantType = 0x03
	ConstString     ConstantType = 0x04
	ConstFunction   ConstantType = 0x05
	ConstClass      ConstantType = 0x06
	ConstChar       ConstantType = 0x07
)

// Constant represents an entry in the constant pool
type Constant interface {
	Type() ConstantType
}

// NilConstant represents a nil value
type NilConstant struct{}

func (c *NilConstant) Type() ConstantType { return ConstNil }

// BoolConstant represents a boolean value
type BoolConstant struct {
	Value bool
}

func (c *BoolConstant) Type() ConstantType { return ConstBool }

// IntConstant represents an integer value
type IntConstant struct {
	Value int64
}

func (c *IntConstant) Type() ConstantType { return ConstInt }

// CharConstant represents a Unicode character value
type CharConstant struct {
	Value rune
}

func (c *CharConstant) Type() ConstantType { return ConstChar }

// FloatConstant represents a floating point value
type FloatConstant struct {
	Value float64
}

func (c *FloatConstant) Type() ConstantType { return ConstFloat }

// StringConstant represents a string value
type StringConstant struct {
	Value string
}

func (c *StringConstant) Type() ConstantType { return ConstString }

// FunctionConstant represents a compiled function
type FunctionConstant struct {
	Name          string
	Instructions  []byte
	NumLocals     int
	NumParameters int
	IsVariadic    bool
	DefaultValues []int // Indices of default values in constant pool
}

func (c *FunctionConstant) Type() ConstantType { return ConstFunction }

// ClassConstant represents a compiled class
type ClassConstant struct {
	Name       string
	SuperClass string // Name of superclass
	Methods    map[string]int // Map of method name to function constant index
}

func (c *ClassConstant) Type() ConstantType { return ConstClass }

// Bytecode represents a complete compiled program
type Bytecode struct {
	Constants  []Constant
	MainFunc   int // Index of main function in constant pool
	SourceFile string
	LineNumberTable []LineInfo // Maps instruction positions to line numbers
}

// LineInfo maps an instruction offset to a source line number
type LineInfo struct {
	Offset   int
	Line     int
	Column   int
}

// Header represents the header of a .nxb bytecode file
type Header struct {
	Magic      uint32
	Version    uint32
	Flags      uint32
	ConstCount uint32
	CodeSize   uint32
	LineCount  uint32
	Reserved   [16]byte
}
