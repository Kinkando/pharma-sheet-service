package option

type GoogleSheetWriteOption interface {
	Apply(*GoogleSheetWrite)
}

type googleSheetWriteOptionFunc func(*GoogleSheetWrite)

func (f googleSheetWriteOptionFunc) Apply(o *GoogleSheetWrite) {
	f(o)
}

func WithGoogleSheetWriteColumnNames(columnNames []string) GoogleSheetWriteOption {
	return googleSheetWriteOptionFunc(func(o *GoogleSheetWrite) {
		o.ColumnNames = columnNames
	})
}

type GoogleSheetWrite struct {
	ColumnNames []string
}
