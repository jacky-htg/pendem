package server

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const MaxRESPDepth = 32
const MaxBulkStringSize = 512 * 1024 * 1024 // 512MB

var (
	ErrInvalidRESP  = errors.New("invalid RESP format")
	ErrRESPTooDeep  = errors.New("RESP nesting too deep")
	ErrRESPTooLarge = errors.New("RESP value too large")
)

type RESPType int

const (
	SimpleString RESPType = iota
	Error
	Integer
	BulkString
	Array
)

type RESPValue struct {
	Type   RESPType
	Str    string
	Int    int64
	Array  []RESPValue
	IsNull bool
}

type RESPParser struct {
	reader *bufio.Reader
	depth  int // ← Track nesting depth
}

func NewRESPParser(r io.Reader) *RESPParser {
	return &RESPParser{
		reader: bufio.NewReader(r),
		depth:  0,
	}
}

func (p *RESPParser) Read() (*RESPValue, error) {
	b, err := p.reader.ReadByte()
	if err != nil {
		return nil, err
	}

	switch b {
	case '+':
		return p.readSimpleString()
	case '-':
		return p.readError()
	case ':':
		return p.readInteger()
	case '$':
		return p.readBulkString()
	case '*':
		return p.readArray()
	default:
		return nil, fmt.Errorf("unknown RESP type: %c", b)
	}
}

func (p *RESPParser) readSimpleString() (*RESPValue, error) {
	line, err := p.reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRESP, err)
	}

	// Validate CRLF
	if len(line) < 2 || line[len(line)-2] != '\r' {
		return nil, fmt.Errorf("%w: missing CRLF", ErrInvalidRESP)
	}

	return &RESPValue{
		Type: SimpleString,
		Str:  line[:len(line)-2],
	}, nil
}

func (p *RESPParser) readError() (*RESPValue, error) {
	line, err := p.reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRESP, err)
	}

	// Validate CRLF
	if len(line) < 2 || line[len(line)-2] != '\r' {
		return nil, fmt.Errorf("%w: missing CRLF", ErrInvalidRESP)
	}

	return &RESPValue{
		Type: Error,
		Str:  line[:len(line)-2],
	}, nil
}

func (p *RESPParser) readInteger() (*RESPValue, error) {
	line, err := p.reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRESP, err)
	}

	if len(line) < 2 || line[len(line)-2] != '\r' {
		return nil, fmt.Errorf("%w: missing CRLF", ErrInvalidRESP)
	}

	val, err := strconv.ParseInt(line[:len(line)-2], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRESP, err)
	}
	return &RESPValue{
		Type: Integer,
		Int:  val,
	}, nil
}

func (p *RESPParser) readBulkString() (*RESPValue, error) {
	// Baca panjang
	lengthStr, err := p.reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRESP, err)
	}
	length, err := strconv.Atoi(lengthStr[:len(lengthStr)-2])
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRESP, err)
	}

	if length == -1 {
		return &RESPValue{Type: BulkString, Str: "", IsNull: true}, nil
	}

	if length == 0 {
		return &RESPValue{Type: BulkString, Str: "", IsNull: false}, nil
	}

	if length > MaxBulkStringSize {
		return nil, fmt.Errorf("%w: bulk string too large (%d bytes)",
			ErrRESPTooLarge, length)
	}

	// Baca data
	data := make([]byte, length+2) // +2 untuk \r\n
	_, err = io.ReadFull(p.reader, data)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRESP, err)
	}

	return &RESPValue{
		Type: BulkString,
		Str:  string(data[:length]),
	}, nil
}

func (p *RESPParser) readArray() (*RESPValue, error) {
	p.depth++
	if p.depth > MaxRESPDepth {
		return nil, fmt.Errorf("%w: max depth %d exceeded",
			ErrRESPTooDeep, MaxRESPDepth)
	}
	defer func() { p.depth-- }()

	lengthStr, err := p.reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRESP, err)
	}
	length, err := strconv.Atoi(lengthStr[:len(lengthStr)-2])
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRESP, err)
	}

	if length == -1 {
		return &RESPValue{Type: Array, Array: nil, IsNull: true}, nil
	}

	array := make([]RESPValue, length)
	for i := 0; i < length; i++ {
		val, err := p.Read()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidRESP, err)
		}
		array[i] = *val
	}

	return &RESPValue{
		Type:  Array,
		Array: array,
	}, nil
}

// RESP Encoder
func EncodeRESP(val RESPValue) string {
	var builder strings.Builder
	// Pre-allocate untuk performa
	builder.Grow(64)
	encodeRESPToBuilder(&builder, val)
	return builder.String()
}
func encodeRESPToBuilder(b *strings.Builder, val RESPValue) {
	switch val.Type {
	case SimpleString:
		b.WriteString("+")
		b.WriteString(val.Str)
		b.WriteString("\r\n")

	case Error:
		b.WriteString("-")
		b.WriteString(val.Str)
		b.WriteString("\r\n")

	case Integer:
		b.WriteString(":")
		b.WriteString(strconv.FormatInt(val.Int, 10))
		b.WriteString("\r\n")

	case BulkString:
		if val.IsNull {
			b.WriteString("$-1\r\n")
			return
		}
		b.WriteString("$")
		b.WriteString(strconv.Itoa(len(val.Str)))
		b.WriteString("\r\n")
		b.WriteString(val.Str)
		b.WriteString("\r\n")

	case Array:
		if val.IsNull {
			b.WriteString("*-1\r\n")
			return
		}

		b.WriteString("*")
		b.WriteString(strconv.Itoa(len(val.Array)))
		b.WriteString("\r\n")
		for _, v := range val.Array {
			encodeRESPToBuilder(b, v)
		}

	default:
	}
}

// String() method untuk debugging
func (v *RESPValue) String() string {
	switch v.Type {
	case SimpleString:
		return fmt.Sprintf("SimpleString(%q)", v.Str)
	case Error:
		return fmt.Sprintf("Error(%q)", v.Str)
	case Integer:
		return fmt.Sprintf("Integer(%d)", v.Int)
	case BulkString:
		if v.IsNull {
			return "BulkString(null)"
		}
		return fmt.Sprintf("BulkString(%q)", v.Str)
	case Array:
		if v.Array == nil {
			return "Array(null)"
		}
		return fmt.Sprintf("Array(%d elements)", len(v.Array))
	default:
		return "Unknown"
	}
}
