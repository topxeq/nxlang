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
			Name:          name,
			Instructions:  instructions,
			NumLocals:     int(numLocals),
			NumParameters: int(numParams),
			IsVariadic:    isVariadic,
			DefaultValues: defaultValues,
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

		// Read superclass index
		var superClass uint32
		if err := binary.Read(r.r, binary.LittleEndian, &superClass); err != nil {
			return nil, err
		}

		// Read methods
		var methodCount uint32
		if err := binary.Read(r.r, binary.LittleEndian, &methodCount); err != nil {
			return nil, err
		}
		methods := make([]int, methodCount)
		for i := 0; i < int(methodCount); i++ {
			var idx uint32
			if err := binary.Read(r.r, binary.LittleEndian, &idx); err != nil {
				return nil, err
			}
			methods[i] = int(idx)
		}

		return &ClassConstant{
			Name:       name,
			SuperClass: int(superClass),
			Methods:    methods,
		}, nil

	default:
		return nil, &ErrInvalidConstantType{Type: constType}
	}
}

// readByte reads a single byte from the input
func (r *Reader) readByte() (byte, error) {
	var b [1]byte
	_, err := io.ReadFull(r.r, b[:])
	return b[0], err
}
