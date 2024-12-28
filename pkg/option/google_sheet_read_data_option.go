package option

const (
	// GoogleSheetReadDataSettingNone is none setting
	GoogleSheetReadDataSettingNone int = iota

	// GoogleSheetReadDataSettingExcludeEmptyCell is used to exclude empty cell from the result
	GoogleSheetReadDataSettingExcludeEmptyCell

	// GoogleSheetReadDataSettingExcludeInvalidData is used to check all valid data on entired column from the each row and exclude invalid data
	// Default setting
	GoogleSheetReadDataSettingExcludeInvalidData
)

type GoogleSheetReadDataOption interface {
	Apply(*GoogleSheetReadData)
}

type googleSheetReadDataOptionFunc func(*GoogleSheetReadData)

func (f googleSheetReadDataOptionFunc) Apply(o *GoogleSheetReadData) {
	f(o)
}

func WithGoogleSheetReadDataSetting(setting int) GoogleSheetReadDataOption {
	return googleSheetReadDataOptionFunc(func(o *GoogleSheetReadData) {
		o.Setting = setting
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
	// Setting of read column
	// Default is GoogleSheetReadDataSettingExcludeInvalidData
	Setting int

	// ExcludeEmptyRow is used to exclude empty row from the result
	// Default is false
	ExcludeEmptyRow bool

	// IgnoreUserEnteredFormat is used to ignore user entered format
	// Default is false
	IgnoreUserEnteredFormat bool
}
