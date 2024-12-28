package option

type GoogleSheetReadDataOption interface {
	Apply(*GoogleSheetReadData)
}

type googleSheetReadDataOptionFunc func(*GoogleSheetReadData)

func (f googleSheetReadDataOptionFunc) Apply(o *GoogleSheetReadData) {
	f(o)
}

func WithGoogleSheetReadDataExcludeEmptyCell(excludeEmptyCell bool) GoogleSheetReadDataOption {
	return googleSheetReadDataOptionFunc(func(o *GoogleSheetReadData) {
		o.ExcludeEmptyCell = excludeEmptyCell
	})
}

func WithGoogleSheetReadDataIncludeValidData(includeValidData bool) GoogleSheetReadDataOption {
	return googleSheetReadDataOptionFunc(func(o *GoogleSheetReadData) {
		o.IncludeValidData = includeValidData
	})
}

func WithGoogleSheetReadDataExcludeEmptyRow(excludeEmptyRow bool) GoogleSheetReadDataOption {
	return googleSheetReadDataOptionFunc(func(o *GoogleSheetReadData) {
		o.ExcludeEmptyRow = excludeEmptyRow
	})
}

func WithGoogleSheetReadDataIgnoreUserEnteredFormat(ignoreUserEnteredFormat bool) GoogleSheetReadDataOption {
	return googleSheetReadDataOptionFunc(func(o *GoogleSheetReadData) {
		o.IgnoreUserEnteredFormat = ignoreUserEnteredFormat
	})
}

type GoogleSheetReadData struct {
	// ExcludeEmptyRow is used to exclude empty row from the result
	// Default is false
	ExcludeEmptyRow bool

	// ExcludeEmptyCell is used to exclude empty column from the result
	// Default is false, only use either ExcludeEmptyCell or IncludeValidData
	ExcludeEmptyCell bool

	// IncludeValidData is used to check all maximum possible valid data on each column from the entire row
	// Default is false, only use either IncludeValidData or ExcludeEmptyCell
	IncludeValidData bool

	// IgnoreUserEnteredFormat is used to ignore user entered format
	// Default is false
	IgnoreUserEnteredFormat bool
}
