package option

type GoogleSheetCreateOption interface {
	Apply(*GoogleSheetCreate)
}

type googleSheetCreateOptionFunc func(*GoogleSheetCreate)

func (f googleSheetCreateOptionFunc) Apply(o *GoogleSheetCreate) {
	f(o)
}

func WithGoogleSheetCreateFolderID(folderID string) GoogleSheetCreateOption {
	return googleSheetCreateOptionFunc(func(o *GoogleSheetCreate) {
		o.FolderID = folderID
	})
}

func WithGoogleSheetCreateTitle(title string) GoogleSheetCreateOption {
	return googleSheetCreateOptionFunc(func(o *GoogleSheetCreate) {
		o.SheetTitle = title
	})
}

type GoogleSheetCreate struct {
	FolderID   string
	SheetTitle string
}
