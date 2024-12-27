package option

import "google.golang.org/api/googleapi"

type GoogleSheetListOption interface {
	Apply(*GoogleSheetList)
}

type googleSheetListOptionFunc func(*GoogleSheetList)

func (f googleSheetListOptionFunc) Apply(o *GoogleSheetList) {
	f(o)
}

func WithGoogleSheetListPrefixFileName(prefixFileName string) GoogleSheetListOption {
	return googleSheetListOptionFunc(func(o *GoogleSheetList) {
		o.PrefixFileName = prefixFileName
	})
}

func WithGoogleSheetListPageSize(pageSize int64) GoogleSheetListOption {
	return googleSheetListOptionFunc(func(o *GoogleSheetList) {
		o.PageSize = pageSize
	})
}

func WithGoogleSheetListField(field googleapi.Field) GoogleSheetListOption {
	return googleSheetListOptionFunc(func(o *GoogleSheetList) {
		o.Field = field
	})
}

func WithGoogleSheetListOrderBy(orderBy string) GoogleSheetListOption {
	return googleSheetListOptionFunc(func(o *GoogleSheetList) {
		o.OrderBy = orderBy
	})
}

func WithGoogleSheetListQuery(query string) GoogleSheetListOption {
	return googleSheetListOptionFunc(func(o *GoogleSheetList) {
		o.Query = query
	})
}

type GoogleSheetList struct {
	PrefixFileName string
	PageSize       int64
	Field          googleapi.Field
	OrderBy        string
	Query          string
}
