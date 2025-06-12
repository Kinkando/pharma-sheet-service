package documentgenerator

type CellMode string

const (
	CellModeValue   CellMode = "value"
	CellModeFormula CellMode = "formula"
)

type ExcelizeSetSheetDataOption interface {
	Apply(*ExcelizeSetSheetData)
}

type excelizeSetSheetDataOptionFunc func(*ExcelizeSetSheetData)

func (essdo excelizeSetSheetDataOptionFunc) Apply(essd *ExcelizeSetSheetData) {
	essdo(essd)
}

func WithExcelizeSetSheetDataColumnWidths(columnWidths []float64) ExcelizeSetSheetDataOption {
	return excelizeSetSheetDataOptionFunc(func(essd *ExcelizeSetSheetData) {
		essd.ColumnWidths = columnWidths
	})
}

func WithExcelizeSetSheetIncludeHeaders(isIncludeHeader bool) ExcelizeSetSheetDataOption {
	return excelizeSetSheetDataOptionFunc(func(essd *ExcelizeSetSheetData) {
		essd.IsIncludeHeader = isIncludeHeader
	})
}

func WithExcelizeSetSheetDataDropdown(dropdown DropdownOption) ExcelizeSetSheetDataOption {
	return excelizeSetSheetDataOptionFunc(func(essd *ExcelizeSetSheetData) {
		essd.Dropdown = &dropdown
	})
}

func WithExcelizeSetSheetDataCell(mode CellMode) ExcelizeSetSheetDataOption {
	return excelizeSetSheetDataOptionFunc(func(essd *ExcelizeSetSheetData) {
		essd.CellMode = mode
	})
}

func WithExcelizeSetSheetDataNewLine(newLine uint64) ExcelizeSetSheetDataOption {
	return excelizeSetSheetDataOptionFunc(func(essd *ExcelizeSetSheetData) {
		essd.NewLine = newLine
	})
}

func WithExcelizeSetSheetDataIsNewSheet(isNewSheet bool) ExcelizeSetSheetDataOption {
	return excelizeSetSheetDataOptionFunc(func(essd *ExcelizeSetSheetData) {
		essd.IsNewSheet = isNewSheet
	})
}

func WithExcelizeSetSheetDataIsActiveSheet(isActiveSheet bool) ExcelizeSetSheetDataOption {
	return excelizeSetSheetDataOptionFunc(func(essd *ExcelizeSetSheetData) {
		essd.IsActiveSheet = isActiveSheet
	})
}

func WithCustomHeader(customHeader []CustomHeader) ExcelizeSetSheetDataOption {
	return excelizeSetSheetDataOptionFunc(func(essd *ExcelizeSetSheetData) {
		essd.CustomHeaders = customHeader
	})
}

func WithExcelizeSetSheetDataFirstColumn(firstColumn string) ExcelizeSetSheetDataOption {
	return excelizeSetSheetDataOptionFunc(func(essd *ExcelizeSetSheetData) {
		essd.FirstColumn = firstColumn
	})
}

func WithExcelizeSetSheetDataFirstRow(firstRow int) ExcelizeSetSheetDataOption {
	return excelizeSetSheetDataOptionFunc(func(essd *ExcelizeSetSheetData) {
		essd.FirstRow = firstRow
	})
}

type ExcelizeSetSheetData struct {
	// The count of new line before set sheet data
	NewLine uint64

	// Create a new sheet by given a worksheet name from arguments
	IsNewSheet bool

	// Set current sheet as active sheet when open file on the first time
	IsActiveSheet bool

	// The column that every first element of each row will be set data
	// if not provided, the default value is 'A'
	FirstColumn string

	// Start set sheet data from row
	FirstRow int

	CustomHeaders []CustomHeader

	// CellMode defines the mode of the cell, either value or formula
	CellMode CellMode

	// Dropdown data validation
	Dropdown *DropdownOption

	// IsIncludeHeader indicates whether to include headers in the sheet data
	IsIncludeHeader bool

	// ColumnWidths defines the widths of columns in the sheet
	ColumnWidths []float64
}

type DropdownOption struct {
	Ref   string // {{SHEET_NAME}}!${{COLUMN}}${{START_ROW}}:${{COLUMN}}${{END_ROW}} | The reference to the dropdown source, e.g., "Sheet1!$A$1:$A$10"
	Range string // Range defines the range of cells for the dropdown | The cell or range where the dropdown will appear such as "B2:B10", "B2"
}

type CustomHeader struct {
	Cell  string
	Value any
}
