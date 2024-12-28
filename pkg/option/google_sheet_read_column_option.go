package option

const (
	// GoogleSheetReadColumnSettingNone is none setting
	GoogleSheetReadColumnSettingNone int = iota

	// GoogleSheetReadColumnSettingExcludeEmptyCell is used to exclude empty cell from the result
	GoogleSheetReadColumnSettingExcludeEmptyCell

	// GoogleSheetReadColumnSettingExcludeInvalidData is used to check all valid data on each column from the entire row and exclude invalid data
	// Default setting
	GoogleSheetReadColumnSettingExcludeInvalidData
)

type GoogleSheetReadColumnOption interface {
	Apply(*GoogleSheetReadColumn)
}

type googleSheetReadColumnOptionFunc func(*GoogleSheetReadColumn)

func (f googleSheetReadColumnOptionFunc) Apply(o *GoogleSheetReadColumn) {
	f(o)
}

func WithGoogleSheetReadColumnSetting(setting int) GoogleSheetReadColumnOption {
	return googleSheetReadColumnOptionFunc(func(o *GoogleSheetReadColumn) {
		o.Setting = setting
	})
}

func WithGoogleSheetReadColumnIgnoreUserEnteredFormat(ignoreUserEnteredFormat bool) GoogleSheetReadColumnOption {
	return googleSheetReadColumnOptionFunc(func(o *GoogleSheetReadColumn) {
		o.IgnoreUserEnteredFormat = ignoreUserEnteredFormat
	})
}

type GoogleSheetReadColumn struct {
	// Setting of read column
	// Default is GoogleSheetReadColumnSettingExcludeInvalidData
	Setting int

	// IgnoreUserEnteredFormat is used to ignore user entered format
	// Default is false
	IgnoreUserEnteredFormat bool
}
