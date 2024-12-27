package option

import "google.golang.org/api/sheets/v4"

type ValueInputOption string

const (
	ValueInputOptionRaw         ValueInputOption = "RAW"
	ValueInputOptionUserEntered ValueInputOption = "USER_ENTERED"
)

type GoogleSheetUpdateOption interface {
	Apply(*GoogleSheetUpdate)
}

type googleSheetUpdateOptionFunc func(*GoogleSheetUpdate)

func (f googleSheetUpdateOptionFunc) Apply(o *GoogleSheetUpdate) {
	f(o)
}

func WithGoogleSheetUpdateSheetID(sheetID int64) GoogleSheetUpdateOption {
	return googleSheetUpdateOptionFunc(func(o *GoogleSheetUpdate) {
		o.SheetID = sheetID
	})
}

func WithGoogleSheetUpdateSheetTitle(title string) GoogleSheetUpdateOption {
	return googleSheetUpdateOptionFunc(func(o *GoogleSheetUpdate) {
		o.SheetTitle = title
	})
}

func WithGoogleSheetUpdateStartCellRange(startCellRange string) GoogleSheetUpdateOption {
	return googleSheetUpdateOptionFunc(func(o *GoogleSheetUpdate) {
		o.StartCellRange = startCellRange
	})
}

func WithGoogleSheetUpdateEndCellRange(endCellRange string) GoogleSheetUpdateOption {
	return googleSheetUpdateOptionFunc(func(o *GoogleSheetUpdate) {
		o.EndCellRange = endCellRange
	})
}

func WithGoogleSheetUpdateFontSize(fontSize int64) GoogleSheetUpdateOption {
	return googleSheetUpdateOptionFunc(func(o *GoogleSheetUpdate) {
		o.FontSize = fontSize
	})
}

func WithGoogleSheetUpdateValueInputOption(valueInputOption ValueInputOption) GoogleSheetUpdateOption {
	return googleSheetUpdateOptionFunc(func(o *GoogleSheetUpdate) {
		o.ValueInputOption = valueInputOption
	})
}

func WithGoogleSheetUpdateData(data [][]GoogleSheetUpdateData) GoogleSheetUpdateOption {
	return googleSheetUpdateOptionFunc(func(o *GoogleSheetUpdate) {
		o.Data = data
	})
}

func WithGoogleSheetUpdateColumns(columns []GoogleSheetUpdateColumn) GoogleSheetUpdateOption {
	return googleSheetUpdateOptionFunc(func(o *GoogleSheetUpdate) {
		o.Columns = columns
	})
}

func WithGoogleSheetUpdateApplyFilter(applyFilter bool) GoogleSheetUpdateOption {
	return googleSheetUpdateOptionFunc(func(o *GoogleSheetUpdate) {
		o.ApplyFilter = applyFilter
	})
}

func WithGoogleSheetUpdateIsAppendData(isAppendData bool) GoogleSheetUpdateOption {
	return googleSheetUpdateOptionFunc(func(o *GoogleSheetUpdate) {
		o.IsAppendData = isAppendData
	})
}

func WithGoogleSheetUpdateIsAutoResizeColumns(autoResizeColumn bool) GoogleSheetUpdateOption {
	return googleSheetUpdateOptionFunc(func(o *GoogleSheetUpdate) {
		o.IsAutoResizeColumns = autoResizeColumn
	})
}

func WithGoogleSheetUpdateIsTextWraping(isTextWraping bool) GoogleSheetUpdateOption {
	return googleSheetUpdateOptionFunc(func(o *GoogleSheetUpdate) {
		o.IsTextWraping = isTextWraping
	})
}

// ColumnStartIndex is 1-based index: A=1, B=2, C=3, ...
func WithGoogleSheetUpdateColumnStartIndex(columnStartIndex int64) GoogleSheetUpdateOption {
	return googleSheetUpdateOptionFunc(func(o *GoogleSheetUpdate) {
		o.ColumnStartIndex = columnStartIndex
	})
}

type GoogleSheetUpdate struct {
	SheetID          int64
	SheetTitle       string
	StartCellRange   string
	EndCellRange     string
	FontSize         int64
	ValueInputOption ValueInputOption

	Data         [][]GoogleSheetUpdateData
	IsAppendData bool

	// ColumnStartIndex is 1-based index: A=1, B=2, C=3, ...
	ColumnStartIndex    int64
	Columns             []GoogleSheetUpdateColumn
	ApplyFilter         bool
	IsAutoResizeColumns bool
	IsTextWraping       bool
}

type GoogleSheetUpdateData struct {
	Value      any
	CellFormat *sheets.CellFormat
}

type GoogleSheetUpdateColumn struct {
	Value      string
	Width      int64
	CellFormat *sheets.CellFormat
}
