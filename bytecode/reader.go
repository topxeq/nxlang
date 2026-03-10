package bytecode

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

// Reader deserializes binary .nxb files into Bytecode structure
type Reader struct {
	r io.Reader
}

// NewReader creates a new bytecode reader from an io.Reader
func NewReader(r io.Reader) *Reader {
	return &Reader{r: r}
}

// NewReaderFromBytes creates a new bytecode reader from a byte slice
func NewReaderFromBytes(data []byte) *Reader {
	return &Reader{r: bytes.NewReader(data)}
}

// Read reads and parses the bytecode from the input
func (r *Reader) Read() (*Bytecode, error) {
	var header Header
	if err := binary.Read(r.r, binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	// Verify magic number
	if header.Magic != Magic {
		return nil, errors.New("invalid Nxlang bytecode file (wrong magic number)")
	}

	// Check version compatibility
	major := header.Version >> 16
	if major != VersionMajor {
		return nil, errors.New("incompatible bytecode version")
	}

	bc := &Bytecode{
		Constants:      make([]Constant, header.ConstCount),
		LineNumberTable: make([]LineInfo, header.LineCount),
	}

	// Read constant pool
	for i := 0; i < int(header.ConstCount); i++ {
		constType, err := r.readByte()
		if err != nil {
			return nil, err
		}

		c, err := r.readConstant(ConstantType(constType))
		if err != nil {
			return nil, err
		}
		bc.Constants[i] = c
	}

	// Read main function index
	var mainFunc uint32
	if err := binary.Read(r.r, binary.LittleEndian, &mainFunc); err != nil {
		return nil, err
	}
	bc.MainFunc = int(mainFunc)

	// Read source file name
	var sourceLen uint32
	if err := binary.Read(r.r, binary.LittleEndian, &sourceLen); err != nil {
		return nil, err
	}
	if sourceLen > 0 {
		sourceBytes := make([]byte, sourceLen)
		if _, err := io.ReadFull(r.r, sourceBytes); err != nil {
			return nil, err
		}
		bc.SourceFile = string(sourceBytes)
	}

	// Read line number table
	for i := 0; i < int(header.LineCount); i++ {
		var offset, line, column uint32
		if err := binary.Read(r.r, binary.LittleEndian, &offset); err != nil {
			return nil, err
		}
		if err := binary.Read(r.r, binary.LittleEndian, &line); err != nil {
			return nil, err
		}
		if err := binary.Read(r.r, binary.LittleEndian, &column); err != nil {
			return nil, err
		}
		bc.LineNumberTable[i] = LineInfo{
			Offset: int(offset),
			Line:   int(line),
			Column: int(column),
		}
	}

	return bc, nil
}

// readConstant reads a single constant from the input
func (r *Reader) readConstant(constType ConstantType) (Constant, error) {
	switch constType {
	case ConstNil:
		return &NilConstant{}, nil

	case ConstBool:
		val, err := r.readByte()
		if err != nil {
			return nil, err
		}
		return &BoolConstant{Value: val != 0}, nil

	case ConstInt:
		var val int64
		if err := binary.Read(r.r, binary.LittleEndian, &val); err != nil {
			return nil, err
		}
		return &IntConstant{Value: val}, nil

	case ConstFloat:
		var val float64
		if err := binary.Read(r.r, binary.LittleEndian, &val); err != nil {
			return nil, err
		}
		return &FloatConstant{Value: val}, nil

	case ConstString:
		var strLen uint32
		if err := binary.Read(r.r, binary.LittleEndian, &strLen); err != nil {
			return nil, err
		}
		strBytes := make([]byte, strLen)
		if _, err := io.ReadFull(r.r, strBytes); err != nil {
			return nil, err
		}
		return &StringConstant{Value: string(strBytes)}, nil

	case ConstFunction:
		// Read name
		var nameLen uint32
		if err := binary.Read(r.r, binary.LittleEndian, &nameLen); err != nil {
			return nil, err
		}
		var name string
		if nameLen > 0 {
			nameBytes := make([]byte, nameLen)
			if _, err := io.ReadFull(r.r, nameBytes); err != nil {
				return nil, err
			}
			name = string(nameBytes)
		}

		// Read instructions
		var instrLen uint32
		if err := binary.Read(r.r, binary.LittleEndian, &instrLen); err != nil {
			return nil, err
		}
		instructions := make([]byte, instrLen)
		if _, err := io.ReadFull(r.r, instructions); err != nil {
			return nil, err
		}

		// Read metadata
		var numLocals uint16
		if err := binary.Read(r.r, binary.LittleEndian, &numLocals); err != nil {
			return nil, err
		}
		var numParams uint8
		if err := binary.Read(r.r, binary.LittleEndian, &numParams); err != nil {
			return nil, err
		}
		isVariadicByte, err := r.readByte()
		if err != nil {
			return nil, err
		}
		isVariadic := isVariadicByte != 0

		// Read static flag
		isStaticByte, err := r.readByte()
		if err != nil {
			return nil, err
		}
		isStatic := isStaticByte != 0

		// Read access modifier
		accessModifier, err := r.readByte()
		if err != nil {
			return nil, err
		}

		// Read flags
		flags, err := r.readByte()
		if err != nil {
			return nil, err
		}

		// Read default values
		var defaultCount uint32
		if err := binary.Read(r.r, binary.LittleEndian, &defaultCount); err != nil {
			return nil, err
		}
		defaultValues := make([]int, defaultCount)
		for i := 0; i < int(defaultCount); i++ {
			var idx uint32
			if err := binary.Read(r.r, binary.LittleEndian, &idx); err != nil {
				return nil, err
			}
			defaultValues[i] = int(idx)
		}

		return &FunctionConstant{
			Name:           name,
			Instructions:   instructions,
			NumLocals:      int(numLocals),
			NumParameters:  int(numParams),
			IsVariadic:     isVariadic,
			IsStatic:       isStatic,
			AccessModifier: accessModifier,
			Flags:          flags,
			DefaultValues:  defaultValues,
		}, nil

	case ConstClass:
		// Read name
		var nameLen uint32
		if err := binary.Read(r.r, binary.LittleEndian, &nameLen); err != nil {
			return nil, err
		}
		var name string
		if nameLen > 0 {
			nameBytes := make([]byte, nameLen)
			if _, err := io.ReadFull(r.r, nameBytes); err != nil {
				return nil, err
			}
			name = string(nameBytes)
		}

		// Read superclass name (string)
		var superClassLen uint32
		if err := binary.Read(r.r, binary.LittleEndian, &superClassLen); err != nil {
			return nil, err
		}
		var superClass string
		if superClassLen > 0 {
			superClassBytes := make([]byte, superClassLen)
			if _, err := io.ReadFull(r.r, superClassBytes); err != nil {
				return nil, err
			}
			superClass = string(superClassBytes)
		}

		// Read interfaces
		var interfaceCount uint32
		if err := binary.Read(r.r, binary.LittleEndian, &interfaceCount); err != nil {
			return nil, err
		}
		interfaces := make([]string, interfaceCount)
		for i := 0; i < int(interfaceCount); i++ {
			var ifaceLen uint32
			if err := binary.Read(r.r, binary.LittleEndian, &ifaceLen); err != nil {
				return nil, err
			}
			if ifaceLen > 0 {
				ifaceBytes := make([]byte, ifaceLen)
				if _, err := io.ReadFull(r.r, ifaceBytes); err != nil {
					return nil, err
				}
				interfaces[i] = string(ifaceBytes)
			}
		}

		// Read methods (map[string]int)
		var methodCount uint32
		if err := binary.Read(r.r, binary.LittleEndian, &methodCount); err != nil {
			return nil, err
		}
		methods := make(map[string]int, methodCount)
		for i := 0; i < int(methodCount); i++ {
			// Read method name
			var nameLen uint32
			if err := binary.Read(r.r, binary.LittleEndian, &nameLen); err != nil {
				return nil, err
			}
			var methodName string
			if nameLen > 0 {
				nameBytes := make([]byte, nameLen)
				if _, err := io.ReadFull(r.r, nameBytes); err != nil {
					return nil, err
				}
				methodName = string(nameBytes)
			}
			// Read method index
			var methodIdx uint32
			if err := binary.Read(r.r, binary.LittleEndian, &methodIdx); err != nil {
				return nil, err
			}
			methods[methodName] = int(methodIdx)
		}

		// Read static methods (map[string]int)
		var staticMethodCount uint32
		if err := binary.Read(r.r, binary.LittleEndian, &staticMethodCount); err != nil {
			return nil, err
		}
		staticMethods := make(map[string]int, staticMethodCount)
		for i := 0; i < int(staticMethodCount); i++ {
			// Read method name
			var nameLen uint32
			if err := binary.Read(r.r, binary.LittleEndian, &nameLen); err != nil {
				return nil, err
			}
			var methodName string
			if nameLen > 0 {
				nameBytes := make([]byte, nameLen)
				if _, err := io.ReadFull(r.r, nameBytes); err != nil {
					return nil, err
				}
				methodName = string(nameBytes)
			}
			// Read method index
			var methodIdx uint32
			if err := binary.Read(r.r, binary.LittleEndian, &methodIdx); err != nil {
				return nil, err
			}
			staticMethods[methodName] = int(methodIdx)
		}

		return &ClassConstant{
			Name:          name,
			SuperClass:    superClass,
			Interfaces:    interfaces,
			Methods:       methods,
			StaticMethods: staticMethods,
		}, nil

	case ConstInterface:
		return r.readInterfaceConstant()

	default:
		return nil, &ErrInvalidConstantType{Type: constType}
	}
}

// readConstant reads a single constant from the input
func (r *Reader) readInterfaceConstant() (*InterfaceConstant, error) {
	// Read name
	var nameLen uint32
	if err := binary.Read(r.r, binary.LittleEndian, &nameLen); err != nil {
		return nil, err
	}
	var name string
	if nameLen > 0 {
		nameBytes := make([]byte, nameLen)
		if _, err := io.ReadFull(r.r, nameBytes); err != nil {
			return nil, err
		}
		name = string(nameBytes)
	}

	// Read method count
	var methodCount uint32
	if err := binary.Read(r.r, binary.LittleEndian, &methodCount); err != nil {
		return nil, err
	}

	// Read methods (map[string][]string)
	methods := make(map[string][]string, methodCount)
	for i := 0; i < int(methodCount); i++ {
		// Read method name
		var methodNameLen uint32
		if err := binary.Read(r.r, binary.LittleEndian, &methodNameLen); err != nil {
			return nil, err
		}
		var methodName string
		if methodNameLen > 0 {
			methodNameBytes := make([]byte, methodNameLen)
			if _, err := io.ReadFull(r.r, methodNameBytes); err != nil {
				return nil, err
			}
			methodName = string(methodNameBytes)
		}

		// Read parameter count
		var paramCount uint32
		if err := binary.Read(r.r, binary.LittleEndian, &paramCount); err != nil {
			return nil, err
		}

		// Read parameter names
		paramNames := make([]string, paramCount)
		for j := 0; j < int(paramCount); j++ {
			var paramLen uint32
			if err := binary.Read(r.r, binary.LittleEndian, &paramLen); err != nil {
				return nil, err
			}
			if paramLen > 0 {
				paramBytes := make([]byte, paramLen)
				if _, err := io.ReadFull(r.r, paramBytes); err != nil {
					return nil, err
				}
				paramNames[j] = string(paramBytes)
			}
		}

		methods[methodName] = paramNames
	}

	return &InterfaceConstant{
		Name:    name,
		Methods: methods,
	}, nil
}

// readByte reads a single byte from the input
func (r *Reader) readByte() (byte, error) {
	var b [1]byte
	_, err := io.ReadFull(r.r, b[:])
	return b[0], err
}
