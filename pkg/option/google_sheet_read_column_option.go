package option

type GoogleSheetReadColumnOption interface {
	Apply(*GoogleSheetReadColumn)
}

type googleSheetReadColumnOptionFunc func(*GoogleSheetReadColumn)

func (f googleSheetReadColumnOptionFunc) Apply(o *GoogleSheetReadColumn) {
	f(o)
}

func WithGoogleSheetReadColumnExcludeEmptyColumn(excludeEmptyColumn bool) GoogleSheetReadColumnOption {
	return googleSheetReadColumnOptionFunc(func(o *GoogleSheetReadColumn) {
		o.ExcludeEmptyColumn = excludeEmptyColumn
	})
}

func WithGoogleSheetReadColumnIncludeValidData(includeValidData bool) GoogleSheetReadColumnOption {
	return googleSheetReadColumnOptionFunc(func(o *GoogleSheetReadColumn) {
		o.IncludeValidData = includeValidData
	})
}

type GoogleSheetReadColumn struct {
	// ExcludeEmptyColumn is used to exclude empty column from the result
	// Default is false, only use either ExcludeEmptyColumn or IncludeValidData
	ExcludeEmptyColumn bool

	// IncludeValidData is used to check all maximum possible valid data on each column from the entire row
	// Default is false, only use either IncludeValidData or ExcludeEmptyColumn
	IncludeValidData bool
}
