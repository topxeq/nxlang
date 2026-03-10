package bytecode

import (
	"bytes"
	"encoding/binary"
	"io"
)

// Writer serializes Bytecode into binary format
type Writer struct {
	buf *bytes.Buffer
}

// NewWriter creates a new bytecode writer
func NewWriter() *Writer {
	return &Writer{
		buf: bytes.NewBuffer(nil),
	}
}

// Write writes the bytecode to the internal buffer
func (w *Writer) Write(bc *Bytecode) error {
	// Write header
	header := Header{
		Magic:      Magic,
		Version:    FullVersion,
		Flags:      0,
		ConstCount: uint32(len(bc.Constants)),
		LineCount:  uint32(len(bc.LineNumberTable)),
	}

	if err := binary.Write(w.buf, binary.LittleEndian, header); err != nil {
		return err
	}

	// Write constant pool
	for _, c := range bc.Constants {
		if err := w.writeConstant(c); err != nil {
			return err
		}
	}

	// Write main function index
	if err := binary.Write(w.buf, binary.LittleEndian, uint32(bc.MainFunc)); err != nil {
		return err
	}

	// Write source file name
	sourceLen := uint32(len(bc.SourceFile))
	if err := binary.Write(w.buf, binary.LittleEndian, sourceLen); err != nil {
		return err
	}
	if sourceLen > 0 {
		if _, err := w.buf.WriteString(bc.SourceFile); err != nil {
			return err
		}
	}

	// Write line number table
	for _, lineInfo := range bc.LineNumberTable {
		if err := binary.Write(w.buf, binary.LittleEndian, uint32(lineInfo.Offset)); err != nil {
			return err
		}
		if err := binary.Write(w.buf, binary.LittleEndian, uint32(lineInfo.Line)); err != nil {
			return err
		}
		if err := binary.Write(w.buf, binary.LittleEndian, uint32(lineInfo.Column)); err != nil {
			return err
		}
	}

	return nil
}

// writeConstant writes a single constant to the buffer
func (w *Writer) writeConstant(c Constant) error {
	// Write constant type
	if err := w.buf.WriteByte(byte(c.Type())); err != nil {
		return err
	}

	switch constType := c.(type) {
	case *NilConstant:
		// No additional data
		return nil

	case *BoolConstant:
		val := byte(0)
		if constType.Value {
			val = 1
		}
		return w.buf.WriteByte(val)

	case *IntConstant:
		return binary.Write(w.buf, binary.LittleEndian, constType.Value)

	case *FloatConstant:
		return binary.Write(w.buf, binary.LittleEndian, constType.Value)

	case *StringConstant:
		strLen := uint32(len(constType.Value))
		if err := binary.Write(w.buf, binary.LittleEndian, strLen); err != nil {
			return err
		}
		_, err := w.buf.WriteString(constType.Value)
		return err

	case *FunctionConstant:
		// Write name
		nameLen := uint32(len(constType.Name))
		if err := binary.Write(w.buf, binary.LittleEndian, nameLen); err != nil {
			return err
		}
		if nameLen > 0 {
			if _, err := w.buf.WriteString(constType.Name); err != nil {
				return err
			}
		}

		// Write instruction count and instructions
		instrLen := uint32(len(constType.Instructions))
		if err := binary.Write(w.buf, binary.LittleEndian, instrLen); err != nil {
			return err
		}
		if _, err := w.buf.Write(constType.Instructions); err != nil {
			return err
		}

		// Write metadata
		if err := binary.Write(w.buf, binary.LittleEndian, uint16(constType.NumLocals)); err != nil {
			return err
		}
		if err := binary.Write(w.buf, binary.LittleEndian, uint8(constType.NumParameters)); err != nil {
			return err
		}
		isVariadic := byte(0)
		if constType.IsVariadic {
			isVariadic = 1
		}
		if err := w.buf.WriteByte(isVariadic); err != nil {
			return err
		}

		// Write static flag
		isStatic := byte(0)
		if constType.IsStatic {
			isStatic = 1
		}
		if err := w.buf.WriteByte(isStatic); err != nil {
			return err
		}

		// Write access modifier
		if err := w.buf.WriteByte(constType.AccessModifier); err != nil {
			return err
		}

		// Write flags (getter/setter)
		if err := w.buf.WriteByte(constType.Flags); err != nil {
			return err
		}

		// Write default values
		defaultCount := uint32(len(constType.DefaultValues))
		if err := binary.Write(w.buf, binary.LittleEndian, defaultCount); err != nil {
			return err
		}
		for _, idx := range constType.DefaultValues {
			if err := binary.Write(w.buf, binary.LittleEndian, uint32(idx)); err != nil {
				return err
			}
		}

		return nil

	case *ClassConstant:
		// Write name
		nameLen := uint32(len(constType.Name))
		if err := binary.Write(w.buf, binary.LittleEndian, nameLen); err != nil {
			return err
		}
		if nameLen > 0 {
			if _, err := w.buf.WriteString(constType.Name); err != nil {
				return err
			}
		}

		// Write superclass name (string)
		superClassLen := uint32(len(constType.SuperClass))
		if err := binary.Write(w.buf, binary.LittleEndian, superClassLen); err != nil {
			return err
		}
		if superClassLen > 0 {
			if _, err := w.buf.WriteString(constType.SuperClass); err != nil {
				return err
			}
		}

		// Write interfaces
		interfaceCount := uint32(len(constType.Interfaces))
		if err := binary.Write(w.buf, binary.LittleEndian, interfaceCount); err != nil {
			return err
		}
		for _, iface := range constType.Interfaces {
			ifaceLen := uint32(len(iface))
			if err := binary.Write(w.buf, binary.LittleEndian, ifaceLen); err != nil {
				return err
			}
			if ifaceLen > 0 {
				if _, err := w.buf.WriteString(iface); err != nil {
					return err
				}
			}
		}

		// Write methods (map[string]int)
		methodCount := uint32(len(constType.Methods))
		if err := binary.Write(w.buf, binary.LittleEndian, methodCount); err != nil {
			return err
		}
		for methodName, methodIdx := range constType.Methods {
			// Write method name
			methodNameLen := uint32(len(methodName))
			if err := binary.Write(w.buf, binary.LittleEndian, methodNameLen); err != nil {
				return err
			}
			if methodNameLen > 0 {
				if _, err := w.buf.WriteString(methodName); err != nil {
					return err
				}
			}
			// Write method index
			if err := binary.Write(w.buf, binary.LittleEndian, uint32(methodIdx)); err != nil {
				return err
			}
		}

		// Write static methods (map[string]int)
		staticMethodCount := uint32(len(constType.StaticMethods))
		if err := binary.Write(w.buf, binary.LittleEndian, staticMethodCount); err != nil {
			return err
		}
		for methodName, methodIdx := range constType.StaticMethods {
			// Write method name
			methodNameLen := uint32(len(methodName))
			if err := binary.Write(w.buf, binary.LittleEndian, methodNameLen); err != nil {
				return err
			}
			if methodNameLen > 0 {
				if _, err := w.buf.WriteString(methodName); err != nil {
					return err
				}
			}
			// Write method index
			if err := binary.Write(w.buf, binary.LittleEndian, uint32(methodIdx)); err != nil {
				return err
			}
		}

		return nil

	case *InterfaceConstant:
		// Write interface name
		nameLen := uint32(len(constType.Name))
		if err := binary.Write(w.buf, binary.LittleEndian, nameLen); err != nil {
			return err
		}
		if nameLen > 0 {
			if _, err := w.buf.WriteString(constType.Name); err != nil {
				return err
			}
		}
		// Write method count
		methodCount := uint32(len(constType.Methods))
		if err := binary.Write(w.buf, binary.LittleEndian, methodCount); err != nil {
			return err
		}
		// Write methods (map[string][]string)
		for methodName, paramNames := range constType.Methods {
			// Write method name
			methodNameLen := uint32(len(methodName))
			if err := binary.Write(w.buf, binary.LittleEndian, methodNameLen); err != nil {
				return err
			}
			if methodNameLen > 0 {
				if _, err := w.buf.WriteString(methodName); err != nil {
					return err
				}
			}
			// Write parameter count
			paramCount := uint32(len(paramNames))
			if err := binary.Write(w.buf, binary.LittleEndian, paramCount); err != nil {
				return err
			}
			// Write parameter names
			for _, param := range paramNames {
				paramLen := uint32(len(param))
				if err := binary.Write(w.buf, binary.LittleEndian, paramLen); err != nil {
					return err
				}
				if paramLen > 0 {
					if _, err := w.buf.WriteString(param); err != nil {
						return err
					}
				}
			}
		}
		return nil

	default:
		return &ErrInvalidConstantType{Type: c.Type()}
	}
}

// Bytes returns the serialized bytecode
func (w *Writer) Bytes() []byte {
	return w.buf.Bytes()
}

// WriteTo writes the serialized bytecode to an io.Writer
func (w *Writer) WriteTo(writer io.Writer) (int64, error) {
	return w.buf.WriteTo(writer)
}

// ErrInvalidConstantType is returned when an unknown constant type is encountered
type ErrInvalidConstantType struct {
	Type ConstantType
}

func (e *ErrInvalidConstantType) Error() string {
	return "invalid constant type: " + string(e.Type)
}
