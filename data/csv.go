// Package data provides data processing capabilities for Nxlang
// including CSV and Excel file handling
package data

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/topxeq/nxlang/types"
	"github.com/topxeq/nxlang/types/collections"
)

// CSVReader represents a CSV file reader
type CSVReader struct {
	file    *os.File
	reader  *csv.Reader
	headers []string
	closed  bool
}

// TypeCode implements types.Object interface
func (r *CSVReader) TypeCode() uint8 {
	return 0x60 // CSVReader type code
}

// TypeName implements types.Object interface
func (r *CSVReader) TypeName() string {
	return "csvReader"
}

// ToStr implements types.Object interface
func (r *CSVReader) ToStr() string {
	if r.closed {
		return "CSVReader[closed]"
	}
	return "CSVReader[open]"
}

// Equals implements types.Object interface
func (r *CSVReader) Equals(other types.Object) bool {
	otherReader, ok := other.(*CSVReader)
	if !ok {
		return false
	}
	return r == otherReader
}

// Close closes the CSV reader
func (r *CSVReader) Close() {
	if r.file != nil {
		r.file.Close()
		r.closed = true
	}
}

// ReadRow reads the next row from the CSV file
func (r *CSVReader) ReadRow() types.Object {
	record, err := r.reader.Read()
	if err == io.EOF {
		return types.NullValue
	}
	if err != nil {
		return types.NewError("failed to read CSV row: "+err.Error(), 0, 0, "")
	}

	row := collections.NewArray()
	for _, field := range record {
		row.Append(types.String(field))
	}
	return row
}

// ReadAll reads all rows from the CSV file
func (r *CSVReader) ReadAll() types.Object {
	rows := collections.NewArray()
	for {
		row := r.ReadRow()
		if row == types.NullValue {
			break
		}
		if err, ok := row.(*types.Error); ok {
			return err
		}
		rows.Append(row)
	}
	return rows
}

// GetHeaders returns the CSV headers
func (r *CSVReader) GetHeaders() types.Object {
	if r.headers == nil {
		return types.NullValue
	}
	headers := collections.NewArray()
	for _, h := range r.headers {
		headers.Append(types.String(h))
	}
	return headers
}

// CSVWriter represents a CSV file writer
type CSVWriter struct {
	file   *os.File
	writer *csv.Writer
	closed bool
}

// TypeCode implements types.Object interface
func (w *CSVWriter) TypeCode() uint8 {
	return 0x61 // CSVWriter type code
}

// TypeName implements types.Object interface
func (w *CSVWriter) TypeName() string {
	return "csvWriter"
}

// ToStr implements types.Object interface
func (w *CSVWriter) ToStr() string {
	if w.closed {
		return "CSVWriter[closed]"
	}
	return "CSVWriter[open]"
}

// Equals implements types.Object interface
func (w *CSVWriter) Equals(other types.Object) bool {
	otherWriter, ok := other.(*CSVWriter)
	if !ok {
		return false
	}
	return w == otherWriter
}

// Close closes the CSV writer
func (w *CSVWriter) Close() {
	if w.writer != nil {
		w.writer.Flush()
	}
	if w.file != nil {
		w.file.Close()
		w.closed = true
	}
}

// WriteRow writes a row to the CSV file
func (w *CSVWriter) WriteRow(row types.Object) types.Object {
	arr, ok := row.(*collections.Array)
	if !ok {
		return types.NewError("argument must be an array", 0, 0, "")
	}

	record := make([]string, arr.Len())
	for i := 0; i < arr.Len(); i++ {
		record[i] = arr.Get(i).ToStr()
	}

	if err := w.writer.Write(record); err != nil {
		return types.NewError("failed to write CSV row: "+err.Error(), 0, 0, "")
	}
	return types.Bool(true)
}

// WriteRows writes multiple rows to the CSV file
func (w *CSVWriter) WriteRows(rows types.Object) types.Object {
	arr, ok := rows.(*collections.Array)
	if !ok {
		return types.NewError("argument must be an array", 0, 0, "")
	}

	for i := 0; i < arr.Len(); i++ {
		result := w.WriteRow(arr.Get(i))
		if err, ok := result.(*types.Error); ok {
			return err
		}
	}
	return types.Bool(true)
}

// WriteHeaders writes headers to the CSV file
func (w *CSVWriter) WriteHeaders(headers types.Object) types.Object {
	arr, ok := headers.(*collections.Array)
	if !ok {
		return types.NewError("argument must be an array", 0, 0, "")
	}

	record := make([]string, arr.Len())
	for i := 0; i < arr.Len(); i++ {
		record[i] = arr.Get(i).ToStr()
	}

	if err := w.writer.Write(record); err != nil {
		return types.NewError("failed to write CSV headers: "+err.Error(), 0, 0, "")
	}
	return types.Bool(true)
}

// Data processing functions for Nxlang

// OpenCSVFunc opens a CSV file for reading
func OpenCSVFunc(args ...types.Object) types.Object {
	if len(args) < 1 {
		return types.NewError("openCSV() expects at least 1 argument (filename)", 0, 0, "")
	}
	filename := string(types.ToString(args[0]))

	file, err := os.Open(filename)
	if err != nil {
		return types.NewError("failed to open file: "+err.Error(), 0, 0, "")
	}

	reader := &CSVReader{
		file:   file,
		reader: csv.NewReader(file),
	}

	// Check if first row is header (optional second argument)
	hasHeader := false
	if len(args) >= 2 {
		if b, ok := args[1].(types.Bool); ok {
			hasHeader = bool(b)
		}
	}

	if hasHeader {
		record, err := reader.reader.Read()
		if err == nil {
			reader.headers = record
		}
	}

	return reader
}

// ReadCSVFunc reads all data from a CSV file
func ReadCSVFunc(args ...types.Object) types.Object {
	if len(args) < 1 {
		return types.NewError("readCSV() expects at least 1 argument (filename)", 0, 0, "")
	}
	filename := string(types.ToString(args[0]))

	file, err := os.Open(filename)
	if err != nil {
		return types.NewError("failed to open file: "+err.Error(), 0, 0, "")
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return types.NewError("failed to read CSV: "+err.Error(), 0, 0, "")
	}

	rows := collections.NewArray()
	for _, record := range records {
		row := collections.NewArray()
		for _, field := range record {
			row.Append(types.String(field))
		}
		rows.Append(row)
	}

	return rows
}

// WriteCSVFunc writes data to a CSV file
func WriteCSVFunc(args ...types.Object) types.Object {
	if len(args) < 2 {
		return types.NewError("writeCSV() expects at least 2 arguments (filename, data)", 0, 0, "")
	}
	filename := string(types.ToString(args[0]))
	data, ok := args[1].(*collections.Array)
	if !ok {
		return types.NewError("second argument must be an array", 0, 0, "")
	}

	file, err := os.Create(filename)
	if err != nil {
		return types.NewError("failed to create file: "+err.Error(), 0, 0, "")
	}
	defer file.Close()

	writer := csv.NewWriter(file)

	// Check for optional headers
	if len(args) >= 3 {
		headers, ok := args[2].(*collections.Array)
		if ok {
			record := make([]string, headers.Len())
			for i := 0; i < headers.Len(); i++ {
				record[i] = headers.Get(i).ToStr()
			}
			if err := writer.Write(record); err != nil {
				return types.NewError("failed to write headers: "+err.Error(), 0, 0, "")
			}
		}
	}

	// Write data rows
	for i := 0; i < data.Len(); i++ {
		row, ok := data.Get(i).(*collections.Array)
		if !ok {
			return types.NewError("row "+strconv.Itoa(i)+" is not an array", 0, 0, "")
		}
		record := make([]string, row.Len())
		for j := 0; j < row.Len(); j++ {
			record[j] = row.Get(j).ToStr()
		}
		if err := writer.Write(record); err != nil {
			return types.NewError("failed to write row: "+err.Error(), 0, 0, "")
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return types.NewError("failed to flush CSV: "+err.Error(), 0, 0, "")
	}

	return types.Bool(true)
}

// ParseCSVFunc parses a CSV string
func ParseCSVFunc(args ...types.Object) types.Object {
	if len(args) < 1 {
		return types.NewError("parseCSV() expects at least 1 argument (csvString)", 0, 0, "")
	}
	csvString := string(types.ToString(args[0]))

	reader := csv.NewReader(strings.NewReader(csvString))
	records, err := reader.ReadAll()
	if err != nil {
		return types.NewError("failed to parse CSV: "+err.Error(), 0, 0, "")
	}

	rows := collections.NewArray()
	for _, record := range records {
		row := collections.NewArray()
		for _, field := range record {
			row.Append(types.String(field))
		}
		rows.Append(row)
	}

	return rows
}

// ToCSVFunc converts data to CSV string
func ToCSVFunc(args ...types.Object) types.Object {
	if len(args) < 1 {
		return types.NewError("toCSV() expects at least 1 argument (data)", 0, 0, "")
	}
	data, ok := args[0].(*collections.Array)
	if !ok {
		return types.NewError("argument must be an array", 0, 0, "")
	}

	var sb strings.Builder
	writer := csv.NewWriter(&sb)

	for i := 0; i < data.Len(); i++ {
		row, ok := data.Get(i).(*collections.Array)
		if !ok {
			return types.NewError("row "+strconv.Itoa(i)+" is not an array", 0, 0, "")
		}
		record := make([]string, row.Len())
		for j := 0; j < row.Len(); j++ {
			record[j] = row.Get(j).ToStr()
		}
		if err := writer.Write(record); err != nil {
			return types.NewError("failed to write row: "+err.Error(), 0, 0, "")
		}
	}

	writer.Flush()
	return types.String(sb.String())
}

// CreateCSVReaderFunc creates a new CSV reader
func CreateCSVReaderFunc(args ...types.Object) types.Object {
	return OpenCSVFunc(args...)
}

// CreateCSVWriterFunc creates a new CSV writer
func CreateCSVWriterFunc(args ...types.Object) types.Object {
	if len(args) < 1 {
		return types.NewError("createCSVWriter() expects at least 1 argument (filename)", 0, 0, "")
	}
	filename := string(types.ToString(args[0]))

	file, err := os.Create(filename)
	if err != nil {
		return types.NewError("failed to create file: "+err.Error(), 0, 0, "")
	}

	writer := &CSVWriter{
		file:   file,
		writer: csv.NewWriter(file),
	}

	return writer
}

// CloseCSVFunc closes a CSV reader or writer
func CloseCSVFunc(args ...types.Object) types.Object {
	if len(args) < 1 {
		return types.NewError("closeCSV() expects 1 argument", 0, 0, "")
	}

	switch v := args[0].(type) {
	case *CSVReader:
		v.Close()
		return types.Bool(true)
	case *CSVWriter:
		v.Close()
		return types.Bool(true)
	default:
		return types.NewError("argument must be a csvReader or csvWriter", 0, 0, "")
	}
}

// ReadCSVRowFunc reads next row from CSV reader
func ReadCSVRowFunc(args ...types.Object) types.Object {
	if len(args) < 1 {
		return types.NewError("readCSVRow() expects 1 argument (reader)", 0, 0, "")
	}
	reader, ok := args[0].(*CSVReader)
	if !ok {
		return types.NewError("argument must be a csvReader", 0, 0, "")
	}
	return reader.ReadRow()
}

// ReadCSVAllFunc reads all rows from CSV reader
func ReadCSVAllFunc(args ...types.Object) types.Object {
	if len(args) < 1 {
		return types.NewError("readCSVAll() expects 1 argument (reader)", 0, 0, "")
	}
	reader, ok := args[0].(*CSVReader)
	if !ok {
		return types.NewError("argument must be a csvReader", 0, 0, "")
	}
	return reader.ReadAll()
}

// WriteCSVRowFunc writes a row to CSV writer
func WriteCSVRowFunc(args ...types.Object) types.Object {
	if len(args) < 2 {
		return types.NewError("writeCSVRow() expects 2 arguments (writer, row)", 0, 0, "")
	}
	writer, ok := args[0].(*CSVWriter)
	if !ok {
		return types.NewError("first argument must be a csvWriter", 0, 0, "")
	}
	return writer.WriteRow(args[1])
}

// GetCSVHeadersFunc gets headers from CSV reader
func GetCSVHeadersFunc(args ...types.Object) types.Object {
	if len(args) < 1 {
		return types.NewError("getCSVHeaders() expects 1 argument (reader)", 0, 0, "")
	}
	reader, ok := args[0].(*CSVReader)
	if !ok {
		return types.NewError("argument must be a csvReader", 0, 0, "")
	}
	return reader.GetHeaders()
}

// Helper function
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [32]byte
	j := len(buf)
	for i > 0 {
		j--
		buf[j] = byte(i%10 + '0')
		i /= 10
	}
	if neg {
		j--
		buf[j] = '-'
	}
	return string(buf[j:])
}

// DataTable represents a table of data (for Excel-like operations)
type DataTable struct {
	headers []string
	rows    []*collections.Array
}

// NewDataTable creates a new data table
func NewDataTable(headers ...string) *DataTable {
	return &DataTable{
		headers: headers,
		rows:    make([]*collections.Array, 0),
	}
}

// TypeCode implements types.Object interface
func (t *DataTable) TypeCode() uint8 {
	return 0x62 // DataTable type code
}

// TypeName implements types.Object interface
func (t *DataTable) TypeName() string {
	return "dataTable"
}

// ToStr implements types.Object interface
func (t *DataTable) ToStr() string {
	return fmt.Sprintf("DataTable[%d rows x %d cols]", len(t.rows), len(t.headers))
}

// Equals implements types.Object interface
func (t *DataTable) Equals(other types.Object) bool {
	otherTable, ok := other.(*DataTable)
	if !ok {
		return false
	}
	return t == otherTable
}

// AddRow adds a row to the data table
func (t *DataTable) AddRow(row *collections.Array) {
	t.rows = append(t.rows, row)
}

// GetRowCount returns the number of rows
func (t *DataTable) GetRowCount() int {
	return len(t.rows)
}

// GetHeaders returns the headers
func (t *DataTable) GetHeaders() *collections.Array {
	headers := collections.NewArray()
	for _, h := range t.headers {
		headers.Append(types.String(h))
	}
	return headers
}

// GetRow returns a row by index
func (t *DataTable) GetRow(index int) types.Object {
	if index < 0 || index >= len(t.rows) {
		return types.NewError("row index out of range", 0, 0, "")
	}
	return t.rows[index]
}
