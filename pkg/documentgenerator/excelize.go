package documentgenerator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/gocarina/gocsv"
	"github.com/xuri/excelize/v2"
)

const (
	delimiter           = ","
	newLine             = "\n"
	doubleQuote         = "\""
	delimiterReplacer   = "${DELIMITER}"
	newLineReplacer     = "${NEW_LINE}"
	doubleQuoteReplacer = "${DOUBLE_QUOTE}"
)

type InitializeExcelizeOption struct {
	SheetName string
}

type Excelize struct {
	*excelize.File
}

func NewExcelizeFile(opts ...excelize.Options) *Excelize {
	return &Excelize{excelize.NewFile(opts...)}
}

func (e *Excelize) SetSheetData(sheetName string, data [][]any, options ...ExcelizeSetSheetDataOption) error {
	opt := &ExcelizeSetSheetData{
		FirstColumn: "A",
		CellMode:    CellModeValue,
	}
	for _, o := range options {
		o.Apply(opt)
	}

	// Create new sheet by given sheet name from parameters
	if opt.IsNewSheet {
		_, err := e.NewSheet(sheetName)
		if err != nil {
			return err
		}
	}

	// Set the default active sheet of the workbook
	if opt.IsActiveSheet {
		sheetIndex, err := e.GetSheetIndex(sheetName)
		if err != nil {
			return err
		}

		e.SetActiveSheet(sheetIndex)
	}

	rows, err := e.GetRows(sheetName)
	if err != nil {
		return err
	}
	firstCol, firstRow := 1, len(rows)+1

	if opt.FirstRow != 0 {
		firstRow = opt.FirstRow
	}

	if opt.FirstColumn != "" {
		firstCol, err = excelize.ColumnNameToNumber(opt.FirstColumn)
		if err != nil {
			return err
		}
	}

	// Add empty row (new line) before set sheet data
	if opt.NewLine > 0 {
		firstRow += int(opt.NewLine)
	}

	if len(opt.CustomHeaders) > 0 {
		for _, custom := range opt.CustomHeaders {
			e.SetCellValue(sheetName, custom.Cell, custom.Value)
		}
	}

	if opt.Dropdown != nil {
		dropdown := excelize.NewDataValidation(true)
		dropdown.SetSqref(opt.Dropdown.Range)
		dropdown.SetSqrefDropList(opt.Dropdown.Ref)
		if err = e.AddDataValidation(sheetName, dropdown); err != nil {
			return err
		}
	}

	// iterate on data (like table) and set sheet data
	for r := range data {
		for c, element := range data[r] {
			columnName, err := excelize.ColumnNumberToName(c + firstCol)
			if err != nil {
				return err
			}

			cell := fmt.Sprintf("%s%d", columnName, r+firstRow)

			cellMode := opt.CellMode
			if cellMode == CellModeFormula {
				if formula, ok := element.(string); !ok || !strings.HasPrefix(formula, "=") {
					cellMode = CellModeValue
				}
			}

			switch cellMode {
			case CellModeFormula:
				formula, ok := element.(string)
				if !ok || !strings.HasPrefix(formula, "=") {
					return fmt.Errorf("document generator: excelize: SetSheetData: expected formula to be a string, got %T", element)
				}
				if err = e.SetCellFormula(sheetName, cell, formula[1:]); err != nil {
					return err
				}

			default:
				if err = e.SetCellValue(sheetName, cell, element); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (e *Excelize) SetAutoWidth(cellMargin int, sheetNames ...string) error {
	if len(sheetNames) == 0 {
		sheetNames = e.GetSheetList()
	}

	for _, sheetName := range sheetNames {
		cols, err := e.GetCols(sheetName)
		if err != nil {
			return err
		}

		for index, col := range cols {
			maxWidth := math.MinInt64
			for _, rowCell := range col {
				if cellWidth := utf8.RuneCountInString(rowCell); cellWidth > maxWidth {
					maxWidth = cellWidth
				}
			}

			name, err := excelize.ColumnNumberToName(index + 1)
			if err != nil {
				return err
			}
			e.SetColWidth(sheetName, name, name, float64(maxWidth+cellMargin))
		}
	}
	return nil
}

func (e *Excelize) DeleteDefaultSheet() error {
	return e.DeleteSheet("Sheet1")
}

// Bind data to struct with csv tag
func UnmarshalExcelize(rawData []byte, data any) ([]byte, error) {
	excelFile, err := excelize.OpenReader(bytes.NewReader(rawData))
	if err != nil {
		return nil, err
	}
	defer excelFile.Close()

	sheetName := excelFile.GetSheetList()[0]
	rows, err := excelFile.GetRows(sheetName)
	if err != nil {
		return nil, err
	}

	if len(rows) <= 1 {
		return nil, nil
	}

	text := ""
	for _, column := range rows[0] {
		text += encodeDelimiterAndNewLine(column) + delimiter
	}
	text = strings.TrimSuffix(text, delimiter) + newLine

	for _, row := range rows[1:] {
		for _, value := range row {
			text += encodeDelimiterAndNewLine(value) + delimiter
		}
		// Add empty column if the row length is less than the header length
		if len(row) < len(rows[0]) {
			for i := 0; i < len(rows[0])-len(row); i++ {
				text += delimiter
			}
		}
		text = strings.TrimSuffix(text, delimiter) + newLine
	}

	err = gocsv.Unmarshal(bytes.NewReader([]byte(text)), data)
	if err != nil {
		return nil, err
	}

	v := reflect.ValueOf(data)
	if v.Kind() != reflect.Ptr && v.Elem().Kind() != reflect.Slice {
		return nil, fmt.Errorf("document generator: excelize: UnmarshalExcelize: data must be a slice of struct")
	}

	// replace another character to new line (\n) and delimiter
	slice := v.Elem()
	for i := 0; i < slice.Len(); i++ {
		row := slice.Index(i)
		if row.Kind() != reflect.Struct {
			return nil, fmt.Errorf("document generator: excelize: UnmarshalExcelize: data must be a struct")
		}
		for j := 0; j < row.NumField(); j++ {
			field := row.Field(j)
			rawField := row.Field(j)
			for field.Kind() == reflect.Ptr {
				field = field.Elem()
			}
			if field.Kind() == reflect.String {
				value := decodeDelimiterAndNewLine(field.String())
				if value == "" && rawField.Kind() == reflect.Ptr {
					rawField.Set(reflect.Zero(rawField.Type()))
				} else {
					field.SetString(value)
				}
			}
		}
	}

	return json.Marshal(data)
}

type MarshalXlsx interface {
	MarshalCSV() (string, error)
}

// Marshal data to excel file with csv tag column
func MarshalExcelize(data any) ([]byte, error) {
	v := reflect.ValueOf(data)
	t := reflect.TypeOf(data)
	if v.Kind() != reflect.Slice || (v.Kind() == reflect.Ptr && v.Elem().Kind() != reflect.Slice) {
		return nil, fmt.Errorf("google: sheet: Write: data must be a slice of struct")
	}

	slice := v
	if v.Kind() == reflect.Ptr {
		slice = v.Elem()
		t = t.Elem()
	}

	if t.Kind() == reflect.Slice {
		t = t.Elem()
	}

	var columns []any
	for i := 0; i < t.NumField(); i++ {
		columnName := t.Field(i).Tag.Get("csv")
		if columnName == "" || columnName == "-" {
			continue
		}
		columns = append(columns, columnName)
	}

	sheetData := [][]any{columns}
	for i := 0; i < slice.Len(); i++ {
		row := slice.Index(i)
		reflectType := t
		if row.Kind() == reflect.Ptr {
			row = row.Elem()
			reflectType = t.Elem()
		}
		if row.Kind() != reflect.Struct {
			return nil, fmt.Errorf("document generator: excelize: MarshalExcelize: data must be a struct")
		}

		value := make(map[string]any)
		for j := 0; j < reflectType.NumField(); j++ {
			field := row.Field(j)
			for field.Kind() == reflect.Ptr {
				field = field.Elem()
			}

			var rawValue any = ""
			if field.IsValid() && field.CanInterface() {
				rawValue = field.Interface()
			}

			// Marshal CSV if the field implements MarshalXlsx interface
			if field.CanAddr() {
				if marshaler, ok := field.Addr().Interface().(MarshalXlsx); ok {
					rawValue, _ = marshaler.MarshalCSV()
				}
			}

			value[reflectType.Field(j).Tag.Get("csv")] = rawValue
		}

		var cells []any
		for _, col := range columns {
			column := col.(string)
			cells = append(cells, value[column])
		}
		sheetData = append(sheetData, cells)
	}

	e := NewExcelizeFile()
	sheetName := e.GetSheetName(0)
	err := e.SetSheetData(sheetName, sheetData)
	if err != nil {
		return nil, err
	}

	excel, err := e.WriteToBuffer()
	if err != nil {
		return nil, err
	}

	return excel.Bytes(), nil
}

func encodeDelimiterAndNewLine(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(text, delimiter, delimiterReplacer), newLine, newLineReplacer), doubleQuote, doubleQuoteReplacer)
}

func decodeDelimiterAndNewLine(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(text, delimiterReplacer, delimiter), newLineReplacer, newLine), doubleQuoteReplacer, doubleQuote)
}
