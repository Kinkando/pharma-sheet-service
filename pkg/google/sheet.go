package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	"github.com/gocarina/gocsv"
	httpinterceptor "github.com/kinkando/pharma-sheet-service/pkg/http/interceptor"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	options "github.com/kinkando/pharma-sheet-service/pkg/option"
	"github.com/xuri/excelize/v2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const (
	delimiter           = ","
	newLine             = "\n"
	doubleQuote         = "\""
	delimiterReplacer   = "${DELIMITER}"
	newLineReplacer     = "${NEW_LINE}"
	doubleQuoteReplacer = "${DOUBLE_QUOTE}"
)

//go:generate mockgen -source=google_sheet.go -destination=google_sheet_mock.go -package=googlesheet
type Sheet interface {
	Create(ctx context.Context, title string, opts ...options.GoogleSheetCreateOption) (*sheets.Spreadsheet, error)
	List(ctx context.Context, folderID string, opts ...options.GoogleSheetListOption) ([]*drive.File, error)
	Get(ctx context.Context, spreadsheetID string) (*sheets.Spreadsheet, error)
	Update(ctx context.Context, spreadsheetID string, opts ...options.GoogleSheetUpdateOption) error
	RenameSheet(ctx context.Context, spreadsheetID string, sheetId int64, title string) error
	ReadColumns(ctx context.Context, sheet *sheets.Sheet, opts ...options.GoogleSheetReadColumnOption) ([]options.GoogleSheetUpdateColumn, error)
	ReadData(ctx context.Context, sheet *sheets.Sheet, opts ...options.GoogleSheetReadDataOption) ([][]options.GoogleSheetUpdateData, error)
	Read(ctx context.Context, sheet *sheets.Sheet, data any, opts ...options.GoogleSheetReadOption) ([]byte, error)
	Write(ctx context.Context, data any, opts ...options.GoogleSheetWriteOption) ([][]options.GoogleSheetUpdateData, error)
}

type googleSheet struct {
	sheet *sheets.Service
	drive *drive.Service
	opt   *options.GoogleSheetClient
}

func NewSheet(opts ...options.GoogleSheetClientOption) Sheet {
	opt := &options.GoogleSheetClient{}
	for _, o := range opts {
		o.Apply(opt)
	}

	if opt.CredentialJSON == nil {
		logger.Fatal("google: sheet: credential json is required")
	}

	var cred = struct {
		Email      string `json:"client_email"`
		PrivateKey string `json:"private_key"`
	}{}
	err := json.Unmarshal(opt.CredentialJSON, &cred)
	if err != nil {
		logger.Fatal(err)
	}
	config := &jwt.Config{
		Email:      cred.Email,
		PrivateKey: []byte(cred.PrivateKey),
		Scopes:     []string{sheets.SpreadsheetsScope, drive.DriveScope},
		TokenURL:   google.JWTTokenURL,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := config.Client(ctx)

	if opt.RateLimiter != nil {
		client.Transport = httpinterceptor.NewRateLimiterTransport(
			options.WithHTTPInterceptorRateLimiterTransport(client.Transport),
			options.WithHTTPInterceptorRateLimiterRateLimiter(opt.RateLimiter),
		)
	}

	sheetSrv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		logger.Fatalf("google: sheet: unable to create sheet service: %v", err)
	}

	driveSrv, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		logger.Fatalf("google: sheet: unable to create drive service: %v", err)
	}

	return &googleSheet{
		drive: driveSrv,
		sheet: sheetSrv,
		opt:   opt,
	}
}

func (g *googleSheet) Create(ctx context.Context, title string, opts ...options.GoogleSheetCreateOption) (*sheets.Spreadsheet, error) {
	opt := &options.GoogleSheetCreate{}
	for _, o := range opts {
		o.Apply(opt)
	}

	sheet := &sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{
			Title: title,
		},
	}
	spreadsheet, err := g.sheet.Spreadsheets.Create(sheet).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("google: sheet: Create: unable to create sheet: %v", err)
	}

	if opt.FolderID != "" {
		_, err = g.drive.Files.Update(spreadsheet.SpreadsheetId, nil).AddParents(opt.FolderID).RemoveParents("root").Do()
		if err != nil {
			return nil, fmt.Errorf("google: sheet: Create: unable to move sheet to folder: %v", err)
		}
	}

	if opt.SheetTitle != "" {
		err = g.RenameSheet(ctx, spreadsheet.SpreadsheetId, spreadsheet.Sheets[0].Properties.SheetId, opt.SheetTitle)
		if err != nil {
			return nil, err
		}
	}

	return spreadsheet, nil
}

func (g *googleSheet) List(ctx context.Context, folderID string, opts ...options.GoogleSheetListOption) ([]*drive.File, error) {
	opt := &options.GoogleSheetList{
		PageSize: 100,
		Field:    googleapi.Field("nextPageToken, files(id, name, mimeType, kind, fileExtension, size, createdTime, modifiedTime, parents)"),
		OrderBy:  "name asc",
	}
	for _, o := range opts {
		o.Apply(opt)
	}

	query := fmt.Sprintf("'%s' in parents and mimeType = 'application/vnd.google-apps.spreadsheet'", folderID)
	if opt.PrefixFileName != "" {
		query = fmt.Sprintf("%s and name contains '%s'", query, opt.PrefixFileName)
	}
	if opt.Query != "" {
		query = fmt.Sprintf("%s and %s", query, opt.Query)
	}

	var files []*drive.File
	var pageToken string
	for isRemaining := true; isRemaining; isRemaining = pageToken != "" {
		fileList, err := g.drive.Files.
			List().
			PageToken(pageToken).
			PageSize(opt.PageSize).
			Fields(opt.Field).
			OrderBy(opt.OrderBy).
			Context(ctx).
			Q(query).
			Do()
		if err != nil {
			return nil, fmt.Errorf("google: sheet: List: unable to list files: %v", err)
		}

		pageToken = fileList.NextPageToken
		files = append(files, fileList.Files...)
	}
	return files, nil
}

func (g *googleSheet) Get(ctx context.Context, spreadsheetID string) (*sheets.Spreadsheet, error) {
	spreadsheet, err := g.sheet.Spreadsheets.Get(spreadsheetID).IncludeGridData(true).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("google: sheet: Get: unable to get sheet: %v", err)
	}
	return spreadsheet, nil
}

func (g *googleSheet) Update(ctx context.Context, spreadsheetID string, opts ...options.GoogleSheetUpdateOption) (err error) {
	opt := &options.GoogleSheetUpdate{
		SheetID:          0,
		SheetTitle:       "Sheet1",
		ValueInputOption: "RAW",
		StartCellRange:   "A1",
		EndCellRange:     "A1",
		FontSize:         10,
		ColumnStartIndex: 1,
	}
	for _, o := range opts {
		o.Apply(opt)
	}

	if len(opt.Columns) > 0 {
		err = g.setHeader(ctx, spreadsheetID, opt)
		if err != nil {
			return fmt.Errorf("google: sheet: Update: %v", err)
		}
	}

	if len(opt.Data) > 0 {
		err = g.setData(ctx, spreadsheetID, opt)
		if err != nil {
			return fmt.Errorf("google: sheet: Update: %v", err)
		}
	}

	return nil
}

func (g *googleSheet) RenameSheet(ctx context.Context, spreadsheetID string, sheetId int64, title string) error {
	requests := []*sheets.Request{
		{
			UpdateSheetProperties: &sheets.UpdateSheetPropertiesRequest{
				Properties: &sheets.SheetProperties{
					SheetId: sheetId,
					Title:   title,
				},
				Fields: "title",
			},
		},
	}
	_, err := g.sheet.Spreadsheets.BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{Requests: requests}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("google: sheet: RenameSheet: unable to rename sheet: %v", err)
	}
	return nil
}

func (g *googleSheet) ReadColumns(ctx context.Context, sheet *sheets.Sheet, opts ...options.GoogleSheetReadColumnOption) (columns []options.GoogleSheetUpdateColumn, err error) {
	opt := &options.GoogleSheetReadColumn{
		IncludeValidData:        true,
		IgnoreUserEnteredFormat: false,
	}
	for _, o := range opts {
		o.Apply(opt)
	}

	if len(sheet.Data) == 0 || len(sheet.Data[0].RowData) == 0 {
		return nil, nil
	}

	for index, cell := range sheet.Data[0].RowData[0].Values {
		if cell.FormattedValue == "" && opt.ExcludeEmptyColumn {
			continue
		}
		columns = append(columns, options.GoogleSheetUpdateColumn{
			Value:      cell.FormattedValue,
			CellFormat: cell.UserEnteredFormat,
			Width:      sheet.Data[0].ColumnMetadata[index].PixelSize,
		})
	}

	if opt.ExcludeEmptyColumn {
		return columns, nil
	}

	if opt.IncludeValidData && len(sheet.Data[0].RowData) > 1 {
		max := math.MinInt64
		for _, rowData := range sheet.Data[0].RowData[1:] {
			lastValidColumn := 0
			for index, cell := range rowData.Values {
				if cell.FormattedValue != "" || (cell.UserEnteredFormat != nil && !opt.IgnoreUserEnteredFormat) {
					lastValidColumn = index
				}
			}
			if lastValidColumn+1 > max {
				max = lastValidColumn + 1
			}
		}
		for i := len(columns); i < max; i++ {
			columns = append(columns, options.GoogleSheetUpdateColumn{
				Value:      "",
				Width:      0,
				CellFormat: nil,
			})
		}
		if len(columns) > max {
			columns = columns[:max]
		}
	}

	return columns, nil
}

func (g *googleSheet) ReadData(ctx context.Context, sheet *sheets.Sheet, opts ...options.GoogleSheetReadDataOption) ([][]options.GoogleSheetUpdateData, error) {
	opt := &options.GoogleSheetReadData{
		IncludeValidData:        true,
		ExcludeEmptyRow:         true,
		IgnoreUserEnteredFormat: false,
	}
	for _, o := range opts {
		o.Apply(opt)
	}

	if len(sheet.Data) == 0 || len(sheet.Data[0].RowData) <= 1 {
		return nil, nil
	}

	var data [][]options.GoogleSheetUpdateData
	for _, rowData := range sheet.Data[0].RowData[1:] {
		var row []options.GoogleSheetUpdateData
		for _, cell := range rowData.Values {
			if cell.FormattedValue == "" && opt.ExcludeEmptyCell {
				continue
			}
			row = append(row, options.GoogleSheetUpdateData{
				Value:      cell.FormattedValue,
				CellFormat: cell.UserEnteredFormat,
			})
		}
		if len(row) > 0 || !opt.ExcludeEmptyRow {
			data = append(data, row)
		}
	}

	if opt.ExcludeEmptyCell {
		return data, nil
	}

	if opt.IncludeValidData {
		max := math.MinInt64
		for _, rowData := range sheet.Data[0].RowData[1:] {
			lastValidColumn := 0
			for index, cell := range rowData.Values {
				if cell.FormattedValue != "" || (cell.UserEnteredFormat != nil && !opt.IgnoreUserEnteredFormat) {
					lastValidColumn = index
				}
			}
			if lastValidColumn+1 > max {
				max = lastValidColumn + 1
			}
		}
		for i := range data {
			if len(data[i]) < max {
				for j := len(data[i]); j < max; j++ {
					data[i] = append(data[i], options.GoogleSheetUpdateData{
						Value:      "",
						CellFormat: nil,
					})
				}
			}
		}
	}

	return data, nil
}

// unmarshal google sheet to struct
func (g *googleSheet) Read(ctx context.Context, sheet *sheets.Sheet, data any, opts ...options.GoogleSheetReadOption) ([]byte, error) {
	opt := &options.GoogleSheetRead{
		ExcludeEmptyRow: true,
	}
	for _, o := range opts {
		o.Apply(opt)
	}

	if len(sheet.Data) == 0 || len(sheet.Data[0].RowData) <= 1 {
		return nil, nil
	}

	if opt.ColumnCount == 0 {
		opt.ColumnCount = len(sheet.Data[0].RowData[0].Values)
	}

	var columnNames []string
	for _, cell := range sheet.Data[0].RowData[0].Values[0:opt.ColumnCount] {
		columnNames = append(columnNames, cell.FormattedValue)
	}

	// read data from google sheet to csv format
	texts := [][]string{columnNames}
	for _, rowData := range sheet.Data[0].RowData[1:] {
		var values []string
		lastColumn := int(math.Min(float64(opt.ColumnCount), float64(len(rowData.Values))))
		for _, value := range rowData.Values[0:lastColumn] {
			values = append(values, value.FormattedValue)
		}
		for i := len(values); i < opt.ColumnCount; i++ {
			values = append(values, "")
		}
		texts = append(texts, values)
	}

	// unmarshal csv format to struct
	rawData := ""
	for i := range texts {
		// replace new line (\n) to another character to prevent an one row data with multiple lines
		// and replace delimiter to another character to prevent csv custom unmarshal with dynamic delimiter that impact to lotus
		rawTexts := make([]string, len(texts[i]))
		isEmpty := true
		for idx, text := range texts[i] {
			if text != "" {
				isEmpty = false
			}
			rawTexts[idx] = encodeDelimiterAndNewLine(text)
		}
		if isEmpty && opt.ExcludeEmptyRow {
			continue
		}
		rawData += strings.Join(rawTexts, delimiter) + newLine
	}
	rawData = strings.TrimSuffix(rawData, newLine)

	err := gocsv.Unmarshal(bytes.NewReader([]byte(rawData)), data)
	if err != nil {
		return nil, err
	}

	v := reflect.ValueOf(data)
	if v.Kind() != reflect.Ptr && v.Elem().Kind() != reflect.Slice {
		return nil, fmt.Errorf("google: sheet: Read: data must be a slice of struct")
	}

	// replace another character to new line (\n) and delimiter
	slice := v.Elem()
	for i := 0; i < slice.Len(); i++ {
		row := slice.Index(i)
		if row.Kind() != reflect.Struct {
			return nil, fmt.Errorf("google: sheet: Read: data must be a struct")
		}
		for j := 0; j < row.NumField(); j++ {
			field := row.Field(j)
			if field.Kind() == reflect.Ptr {
				field = field.Elem()
			}
			if field.Kind() == reflect.String {
				field.SetString(decodeDelimiterAndNewLine(field.String()))
			}
		}
	}

	return json.Marshal(data)
}

// convert slice of struct to google sheet update data
func (g *googleSheet) Write(ctx context.Context, data any, opts ...options.GoogleSheetWriteOption) (output [][]options.GoogleSheetUpdateData, err error) {
	opt := &options.GoogleSheetWrite{}
	for _, o := range opts {
		o.Apply(opt)
	}

	v := reflect.ValueOf(data)
	t := reflect.TypeOf(data)
	if v.Kind() != reflect.Slice || (v.Kind() == reflect.Ptr && v.Elem().Kind() != reflect.Slice) {
		return nil, fmt.Errorf("google: sheet: Write: data must be a slice of struct")
	}

	slice := v
	if v.Kind() == reflect.Ptr {
		slice = v.Elem()
		t = t.Elem()
	}

	if t.Kind() == reflect.Slice {
		t = t.Elem()
	}

	isOrderedByColumn := len(opt.ColumnNames) > 0
	for i := 0; i < slice.Len(); i++ {
		row := slice.Index(i)
		reflectType := t
		if row.Kind() == reflect.Ptr {
			row = row.Elem()
			reflectType = t.Elem()
		}
		if row.Kind() != reflect.Struct {
			return nil, fmt.Errorf("google: sheet: Write: data must be a struct")
		}

		var cells []options.GoogleSheetUpdateData
		value := make(map[string]any)
		for j := 0; j < reflectType.NumField(); j++ {
			field := row.Field(j)
			rawValue := field.Interface()
			if !field.IsValid() {
				rawValue = ""
			}

			if !isOrderedByColumn {
				cells = append(cells, options.GoogleSheetUpdateData{Value: rawValue})
				continue
			}

			value[reflectType.Field(j).Tag.Get("csv")] = rawValue
		}

		if isOrderedByColumn {
			for _, col := range opt.ColumnNames {
				cells = append(cells, options.GoogleSheetUpdateData{Value: value[col]})
			}
		}

		output = append(output, cells)
	}

	return output, nil
}

// colIndex starts from 0
func (g *googleSheet) resizeColumnWidth(ctx context.Context, spreadsheetID string, sheetID, colIndex, width int64) error {
	request := &sheets.Request{
		UpdateDimensionProperties: &sheets.UpdateDimensionPropertiesRequest{
			Range: &sheets.DimensionRange{
				Dimension:  "COLUMNS",
				SheetId:    sheetID,
				StartIndex: colIndex,
				EndIndex:   colIndex + 1,
			},
			Properties: &sheets.DimensionProperties{
				PixelSize: width,
			},
			Fields: "pixelSize",
		},
	}

	updateRequest := &sheets.BatchUpdateSpreadsheetRequest{
		Requests:                     []*sheets.Request{request},
		IncludeSpreadsheetInResponse: false,
	}
	_, err := g.sheet.Spreadsheets.BatchUpdate(spreadsheetID, updateRequest).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("unable to resize column width: %v", err)
	}
	return nil
}

func (g *googleSheet) getSheet(ctx context.Context, spreadsheetID, sheetTitle string, sheetID int64) (*sheets.Sheet, error) {
	spreadSheet, err := g.Get(ctx, spreadsheetID)
	if err != nil {
		return nil, err
	}

	var sheet *sheets.Sheet
	for _, s := range spreadSheet.Sheets {
		if s.Properties.Title == sheetTitle || (sheetTitle == "" && s.Properties.SheetId == sheetID) {
			sheet = s
			break
		}
	}
	if sheet == nil {
		return nil, fmt.Errorf("unable to find sheet: %s", sheetTitle)
	}
	return sheet, nil
}

func (g *googleSheet) setHeader(ctx context.Context, spreadsheetID string, opt *options.GoogleSheetUpdate) error {
	cellRange := fmt.Sprintf("%s1:%s1", ColumnNumberToLetter(int(opt.ColumnStartIndex)), ColumnNumberToLetter(len(opt.Columns)+int(opt.ColumnStartIndex)-1))
	sheetRange := fmt.Sprintf("%s!%s", opt.SheetTitle, cellRange)
	columns := []any{}
	for _, column := range opt.Columns {
		columns = append(columns, column.Value)
	}
	vr := &sheets.ValueRange{Values: [][]any{columns}}
	_, err := g.sheet.Spreadsheets.Values.Update(spreadsheetID, sheetRange, vr).ValueInputOption(string(opt.ValueInputOption)).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("unable to set sheet header: %v", err)
	}

	range_ := sheets.GridRange{
		SheetId:          opt.SheetID,
		StartRowIndex:    0,
		EndRowIndex:      1,
		StartColumnIndex: opt.ColumnStartIndex - 1,
		EndColumnIndex:   int64(len(opt.Columns)) + opt.ColumnStartIndex - 1,
	}

	err = g.setCellOption(ctx, spreadsheetID, range_, true, opt)
	if err != nil {
		return err
	}

	averageCharWidth, padding := 7, 10
	for index, column := range opt.Columns {
		width := column.Width
		if opt.IsAutoResizeColumns {
			width = int64(len(column.Value)*averageCharWidth + padding)
		}

		if width > 0 {
			err = g.resizeColumnWidth(ctx, spreadsheetID, opt.SheetID, int64(index)+opt.ColumnStartIndex-1, width)
			if err != nil {
				return fmt.Errorf("google: sheet: Update: %v", err)
			}
		}
	}

	if opt.ApplyFilter {
		err = g.applyFilter(ctx, spreadsheetID, opt.SheetID, opt.ColumnStartIndex-1, int64(len(opt.Columns)))
		if err != nil {
			return fmt.Errorf("google: sheet: Update: %v", err)
		}
	}

	if opt.IsLockedCell || opt.IsLockedCellColumn {
		err = g.lockedCell(ctx, spreadsheetID, opt.SheetTitle, range_)
		if err != nil {
			return err
		}

	} else if opt.IsUnlockedCell || opt.IsUnlockedCellColumn {
		err = g.unlockedCell(ctx, spreadsheetID, opt.SheetTitle, range_)
		if err != nil {
			return err
		}
	}

	return nil
}

func (g *googleSheet) setData(ctx context.Context, spreadsheetID string, opt *options.GoogleSheetUpdate) error {
	if opt.StartCellRange == "A1" {
		opt.StartCellRange = "A2"
	}
	if opt.EndCellRange == "A1" {
		col, row, err := excelize.CellNameToCoordinates(opt.StartCellRange)
		if err != nil {
			return err
		}
		opt.EndCellRange = getLastCell(opt.Data, row-1, col-1)
	}

	cellRange := opt.StartCellRange + ":" + opt.EndCellRange
	if len(opt.Columns) > 0 {
		if len(opt.Data) > 0 {
			cellRange = "A2:" + getLastCell(opt.Data, 1, 0)
		} else {
			cellRange = "A2:" + fmt.Sprintf("%s%d", ColumnNumberToLetter(len(opt.Columns)), len(opt.Data)+1)
		}
	}

	if opt.IsAppendData {
		sheet, err := g.getSheet(ctx, spreadsheetID, opt.SheetTitle, opt.SheetID)
		if err != nil {
			return err
		}

		row := 1
		if len(sheet.Data) > 0 {
			lastValidRow := 1
			for index, rowData := range sheet.Data[0].RowData {
				isEmpty := true
				for _, value := range rowData.Values {
					if value.FormattedValue != "" {
						isEmpty = false
						break
					}
				}
				if !isEmpty {
					lastValidRow = index + 1
				}
			}
			row = lastValidRow + 1
		}
		cellRange = fmt.Sprintf("A%d:%s", row, getLastCell(opt.Data, row-1, 0))
	}

	var data [][]any
	for i := range opt.Data {
		var row []any
		for j := range opt.Data[i] {
			row = append(row, opt.Data[i][j].Value)
		}
		data = append(data, row)
	}

	sheetRange := fmt.Sprintf("%s!%s", opt.SheetTitle, cellRange)
	vr := &sheets.ValueRange{Values: data}
	_, err := g.sheet.Spreadsheets.Values.Update(spreadsheetID, sheetRange, vr).ValueInputOption(string(opt.ValueInputOption)).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("unable to update sheet: %v", err)
	}

	cells := strings.Split(cellRange, ":")
	startColumn, startRow, _ := excelize.CellNameToCoordinates(cells[0])
	endColumn, endRow, _ := excelize.CellNameToCoordinates(cells[1])

	range_ := sheets.GridRange{
		SheetId:          opt.SheetID,
		StartRowIndex:    int64(startRow) - 1,
		EndRowIndex:      int64(endRow),
		StartColumnIndex: int64(startColumn) - 1,
		EndColumnIndex:   int64(endColumn),
	}

	err = g.setCellOption(ctx, spreadsheetID, range_, false, opt)
	if err != nil {
		return err
	}

	if opt.IsLockedCell || opt.IsLockedCellData {
		err = g.lockedCell(ctx, spreadsheetID, opt.SheetTitle, range_)
		if err != nil {
			return err
		}

	} else if opt.IsUnlockedCell || opt.IsUnlockedCellData {
		err = g.unlockedCell(ctx, spreadsheetID, opt.SheetTitle, range_)
		if err != nil {
			return err
		}
	}

	return nil
}

func (g *googleSheet) setCellOption(ctx context.Context, spreadsheetID string, range_ sheets.GridRange, isHeader bool, opt *options.GoogleSheetUpdate) error {
	fields := "userEnteredFormat.textFormat.fontSize"
	if isHeader {
		fields += ",userEnteredFormat.textFormat.bold"
	}
	if opt.IsTextWraping {
		fields += ",userEnteredFormat.wrapStrategy"
	}

	var rows []*sheets.RowData
	for i := range_.StartRowIndex; i < range_.EndRowIndex; i++ {
		var values []*sheets.CellData
		for j := range_.StartColumnIndex; j < range_.EndColumnIndex; j++ {
			format := &sheets.CellFormat{
				TextFormat: &sheets.TextFormat{
					FontSize: opt.FontSize,
				},
			}
			if opt.IsTextWraping {
				format.WrapStrategy = "WRAP"
			}
			if isHeader {
				format.TextFormat.Bold = true
				if len(opt.Columns) == 0 || len(opt.Columns) != int(range_.EndColumnIndex-range_.StartColumnIndex) {
					return fmt.Errorf("unable to set cell option: column count is not match with end column index")
				}
				if cellFormat := opt.Columns[j-range_.StartColumnIndex].CellFormat; cellFormat != nil {
					format = cellFormat
					if fields != "userEnteredFormat" {
						fields = "userEnteredFormat"
					}
				}

			} else if len(opt.Data) > 0 {
				r, c := i-range_.StartRowIndex, j-range_.StartColumnIndex
				if r < int64(len(opt.Data)) && c < int64(len(opt.Data[r])) {
					if cellFormat := opt.Data[r][c].CellFormat; cellFormat != nil {
						format = cellFormat
						if fields != "userEnteredFormat" {
							fields = "userEnteredFormat"
						}
					}
				}
			}

			values = append(values, &sheets.CellData{
				UserEnteredFormat: format,
			})
		}
		rows = append(rows, &sheets.RowData{Values: values})
	}

	request := &sheets.Request{
		UpdateCells: &sheets.UpdateCellsRequest{
			Range:  &range_,
			Rows:   rows,
			Fields: fields,
		},
	}

	updateRequest := &sheets.BatchUpdateSpreadsheetRequest{
		Requests:                     []*sheets.Request{request},
		IncludeSpreadsheetInResponse: false,
	}
	_, err := g.sheet.Spreadsheets.BatchUpdate(spreadsheetID, updateRequest).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("unable to enable text wrapping: %v", err)
	}
	return nil
}

func (g *googleSheet) applyFilter(ctx context.Context, spreadsheetID string, sheetID int64, startColumnIndex, columnLength int64) error {
	request := &sheets.Request{
		SetBasicFilter: &sheets.SetBasicFilterRequest{
			Filter: &sheets.BasicFilter{
				Range: &sheets.GridRange{
					SheetId:          sheetID,                         // The sheet ID to which you want to apply the filter
					StartRowIndex:    0,                               // The row index to start the filter (0 for the first row)
					EndRowIndex:      1,                               // The row index to end the filter (1 for the first row)
					StartColumnIndex: startColumnIndex,                // The column index to start the filter
					EndColumnIndex:   startColumnIndex + columnLength, // The column index to end the filter
				},
			},
		},
	}

	updateRequest := &sheets.BatchUpdateSpreadsheetRequest{
		Requests:                     []*sheets.Request{request},
		IncludeSpreadsheetInResponse: false,
	}
	_, err := g.sheet.Spreadsheets.BatchUpdate(spreadsheetID, updateRequest).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("unable to apply filter: %v", err)
	}
	return nil
}

func (g *googleSheet) lockedCell(ctx context.Context, spreadsheetID, sheetTitle string, range_ sheets.GridRange) error {
	sheet, err := g.getSheet(ctx, spreadsheetID, sheetTitle, range_.SheetId)
	if err != nil {
		return err
	}

	for _, protectedRange := range sheet.ProtectedRanges {
		isAllIndexMatch := protectedRange.Range.StartRowIndex == range_.StartRowIndex && protectedRange.Range.EndRowIndex == range_.EndRowIndex &&
			protectedRange.Range.StartColumnIndex == range_.StartColumnIndex && protectedRange.Range.EndColumnIndex == range_.EndColumnIndex

		if isAllIndexMatch {
			return nil
		}
	}

	request := &sheets.Request{
		AddProtectedRange: &sheets.AddProtectedRangeRequest{
			ProtectedRange: &sheets.ProtectedRange{
				Range:       &range_,
				WarningOnly: false,
				Editors: &sheets.Editors{
					// Add 'allAuthenticatedUsers' to allow all users to modify or remove the protection
					Users:              []string{},
					Groups:             []string{},
					DomainUsersCanEdit: true, // This makes sure only explicitly added users can edit, so we don’t inadvertently grant access
					// 'allAuthenticatedUsers' allows everyone to remove the protection
				},
			},
		},
	}

	updateRequest := &sheets.BatchUpdateSpreadsheetRequest{
		Requests:                     []*sheets.Request{request},
		IncludeSpreadsheetInResponse: false,
	}
	_, err = g.sheet.Spreadsheets.BatchUpdate(spreadsheetID, updateRequest).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("unable to locked cell: %v", err)
	}
	return nil
}

func (g *googleSheet) unlockedCell(ctx context.Context, spreadsheetID, sheetTitle string, range_ sheets.GridRange) error {
	sheet, err := g.getSheet(ctx, spreadsheetID, sheetTitle, range_.SheetId)
	if err != nil {
		return err
	}

	for _, protectedRange := range sheet.ProtectedRanges {
		isAllIndexMatch := protectedRange.Range.StartRowIndex == range_.StartRowIndex && protectedRange.Range.EndRowIndex == range_.EndRowIndex &&
			protectedRange.Range.StartColumnIndex == range_.StartColumnIndex && protectedRange.Range.EndColumnIndex == range_.EndColumnIndex

		isOverlap := protectedRange.Range.StartRowIndex < range_.EndRowIndex && protectedRange.Range.EndRowIndex > range_.StartRowIndex &&
			protectedRange.Range.StartColumnIndex < range_.EndColumnIndex && protectedRange.Range.EndColumnIndex > range_.StartColumnIndex

		if isAllIndexMatch || isOverlap {
			updateRequest := &sheets.BatchUpdateSpreadsheetRequest{
				Requests: []*sheets.Request{
					{
						DeleteProtectedRange: &sheets.DeleteProtectedRangeRequest{
							ProtectedRangeId: protectedRange.ProtectedRangeId,
						},
					},
				},
				IncludeSpreadsheetInResponse: false,
			}
			_, err := g.sheet.Spreadsheets.BatchUpdate(spreadsheetID, updateRequest).Context(ctx).Do()
			if err != nil {
				return fmt.Errorf("unable to enable unlocked cell: %v", err)
			}
		}
	}
	return nil
}

// Row and column are zero-based indexes, so adjust accordingly
func CellAddress(rowIndex, colIndex int) string {
	return fmt.Sprintf("%s%d", ColumnNumberToLetter(colIndex+1), rowIndex+1)
}

// ColumnNumberToLetter converts a column number (1-based) to a column letter.
func ColumnNumberToLetter(columnNumber int) string {
	var columnName string
	for columnNumber > 0 {
		columnNumber-- // Adjust for 1-based indexing (A=1, B=2,...)
		columnName = string(rune(columnNumber%26+'A')) + columnName
		columnNumber /= 26
	}
	return columnName
}

// ColumnLetterToNumber converts a column letter (e.g., "A", "Z", "AA") to its column number (1, 26, 27, etc.).
func ColumnLetterToNumber(columnLetter string) int {
	columnLetter = strings.ToUpper(columnLetter) // Ensure the column letter is uppercase
	columnNumber := 0
	for i := 0; i < len(columnLetter); i++ {
		char := columnLetter[i]
		// Subtract 'A' to get a 0-based index, then multiply by 26 for the position
		columnNumber = columnNumber*26 + int(char-'A'+1)
	}
	return columnNumber
}

// currentColumn is zero-based index
func getLastCell(data [][]options.GoogleSheetUpdateData, currentRow, currentColumn int) string {
	// Adjust for header row
	if currentRow < 1 {
		currentRow = 1
	}

	max := math.MinInt64
	for _, row := range data {
		if len(row) > max {
			max = len(row)
		}
	}
	return fmt.Sprintf("%s%d", ColumnNumberToLetter(max+currentColumn), len(data)+currentRow)
}

func encodeDelimiterAndNewLine(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(text, delimiter, delimiterReplacer), newLine, newLineReplacer), doubleQuote, doubleQuoteReplacer)
}

func decodeDelimiterAndNewLine(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(text, delimiterReplacer, delimiter), newLineReplacer, newLine), doubleQuoteReplacer, doubleQuote)
}
