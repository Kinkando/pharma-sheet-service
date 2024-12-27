package option

type GoogleSheetReadOption interface {
	Apply(*GoogleSheetRead)
}

type googleSheetReadOptionFunc func(*GoogleSheetRead)

func (f googleSheetReadOptionFunc) Apply(o *GoogleSheetRead) {
	f(o)
}

func WithGoogleSheetReadColumnCount(columnCount int) GoogleSheetReadOption {
	return googleSheetReadOptionFunc(func(o *GoogleSheetRead) {
		o.ColumnCount = columnCount
	})
}

type GoogleSheetRead struct {
	ColumnCount int
}
