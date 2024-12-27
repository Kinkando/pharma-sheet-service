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
	ReadColumns(ctx context.Context, sheet *sheets.Sheet, opts ...options.GoogleSheetReadColumnOption) ([]string, error)
	Read(ctx context.Context, sheet *sheets.Sheet, data any, opts ...options.GoogleSheetReadOption) ([]byte, error)
	Write(ctx context.Context, data any, opts ...options.GoogleSheetWriteOption) ([][]options.GoogleSheetUpdateData, error)
}

type googleSheet struct {
	sheet *sheets.Service
	drive *drive.Service
}

func NewSheet(credential []byte) Sheet {
	var cred = struct {
		Email      string `json:"client_email"`
		PrivateKey string `json:"private_key"`
	}{}
	err := json.Unmarshal(credential, &cred)
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

	if opt.ApplyFilter {
		err = g.applyFilter(ctx, spreadsheetID, opt.SheetID, int64(len(opt.Columns)))
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

func (g *googleSheet) ReadColumns(ctx context.Context, sheet *sheets.Sheet, opts ...options.GoogleSheetReadColumnOption) ([]string, error) {
	opt := &options.GoogleSheetReadColumn{
		IncludeValidData: true,
	}
	for _, o := range opts {
		o.Apply(opt)
	}

	if len(sheet.Data) == 0 || len(sheet.Data[0].RowData) == 0 {
		return nil, nil
	}

	var columnNames []string
	for _, cell := range sheet.Data[0].RowData[0].Values {
		if cell.FormattedValue == "" && opt.ExcludeEmptyColumn {
			continue
		}
		columnNames = append(columnNames, cell.FormattedValue)
	}

	if opt.ExcludeEmptyColumn {
		return columnNames, nil
	}

	if opt.IncludeValidData && len(sheet.Data[0].RowData) > 1 {
		max := math.MinInt64
		for _, rowData := range sheet.Data[0].RowData[1:] {
			lastValidColumn := 0
			for index, cell := range rowData.Values {
				if cell.FormattedValue != "" {
					lastValidColumn = index
				}
			}
			if lastValidColumn+1 > max {
				max = lastValidColumn + 1
			}
		}
		for i := len(columnNames); i < max; i++ {
			columnNames = append(columnNames, "")
		}
		if len(columnNames) > max {
			columnNames = columnNames[:max]
		}
	}

	return columnNames, nil
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

func (g *googleSheet) getSheet(ctx context.Context, spreadsheetID, sheetTitle string) (*sheets.Sheet, error) {
	spreadSheet, err := g.Get(ctx, spreadsheetID)
	if err != nil {
		return nil, err
	}

	var sheet *sheets.Sheet
	for _, s := range spreadSheet.Sheets {
		if s.Properties.Title == sheetTitle {
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

	err = g.setCellOption(ctx, spreadsheetID, opt.SheetID, 0, 1, 0, int64(len(opt.Columns)), true, opt)
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
			err = g.resizeColumnWidth(ctx, spreadsheetID, opt.SheetID, int64(index), width)
			if err != nil {
				return fmt.Errorf("google: sheet: Update: %v", err)
			}
		}
	}

	return nil
}

func (g *googleSheet) setData(ctx context.Context, spreadsheetID string, opt *options.GoogleSheetUpdate) error {
	if opt.StartCellRange == "A1" {
		opt.StartCellRange = "A2"
	}
	if opt.EndCellRange == "A1" {
		opt.EndCellRange = getLastCell(opt.Data, 0)
	}

	cellRange := opt.StartCellRange + ":" + opt.EndCellRange
	if len(opt.Columns) > 0 {
		if len(opt.Data) > 0 {
			cellRange = "A2:" + getLastCell(opt.Data, 1)
		} else {
			cellRange = "A2:" + fmt.Sprintf("%s%d", ColumnNumberToLetter(len(opt.Columns)), len(opt.Data)+1)
		}
	}

	if opt.IsAppendData {
		sheet, err := g.getSheet(ctx, spreadsheetID, opt.SheetTitle)
		if err != nil {
			return err
		}

		row := 1
		if len(sheet.Data) > 0 {
			row = len(sheet.Data[0].RowData) + 1
		}
		cellRange = fmt.Sprintf("A%d:%s", row, getLastCell(opt.Data, row-1))
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
	err = g.setCellOption(ctx, spreadsheetID, opt.SheetID, int64(startRow)-1, int64(endRow), int64(startColumn)-1, int64(endColumn), false, opt)
	if err != nil {
		return err
	}

	return nil
}

func (g *googleSheet) setCellOption(ctx context.Context, spreadsheetID string, sheetID, startRowIndex, endRowIndex, startColumnIndex, endColumnIndex int64, isHeader bool, opt *options.GoogleSheetUpdate) error {
	fields := "userEnteredFormat.textFormat.fontSize"
	if isHeader {
		fields += ",userEnteredFormat.textFormat.bold"
	}
	if opt.IsTextWraping {
		fields += ",userEnteredFormat.wrapStrategy"
	}

	var rows []*sheets.RowData
	for i := startRowIndex; i < endRowIndex; i++ {
		var values []*sheets.CellData
		for j := startColumnIndex; j < endColumnIndex; j++ {
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
				if len(opt.Columns) == 0 || len(opt.Columns) != int(endColumnIndex) {
					return fmt.Errorf("unable to set cell option: column count is not match with end column index")
				}
				if cellFormat := opt.Columns[j].CellFormat; cellFormat != nil {
					format = cellFormat
					if fields != "userEnteredFormat" {
						fields = "userEnteredFormat"
					}
				}

			} else if len(opt.Data) > 0 {
				r, c := i-startRowIndex, j-startColumnIndex
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
			Range: &sheets.GridRange{
				SheetId:          sheetID,
				StartRowIndex:    startRowIndex,
				EndRowIndex:      endRowIndex,
				StartColumnIndex: startColumnIndex,
				EndColumnIndex:   endColumnIndex,
			},
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

func (g *googleSheet) applyFilter(ctx context.Context, spreadsheetID string, sheetID int64, columnLength int64) error {
	request := &sheets.Request{
		SetBasicFilter: &sheets.SetBasicFilterRequest{
			Filter: &sheets.BasicFilter{
				Range: &sheets.GridRange{
					SheetId:          sheetID,      // The sheet ID to which you want to apply the filter
					StartRowIndex:    0,            // The row index to start the filter (0 for the first row)
					EndRowIndex:      1,            // The row index to end the filter (1 for the first row)
					StartColumnIndex: 0,            // The column index to start the filter
					EndColumnIndex:   columnLength, // The column index to end the filter
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

func getLastCell(data [][]options.GoogleSheetUpdateData, currentRow int) string {
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
	return fmt.Sprintf("%s%d", ColumnNumberToLetter(max), len(data)+currentRow)
}

func encodeDelimiterAndNewLine(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(text, delimiter, delimiterReplacer), newLine, newLineReplacer), doubleQuote, doubleQuoteReplacer)
}

func decodeDelimiterAndNewLine(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(text, delimiterReplacer, delimiter), newLineReplacer, newLine), doubleQuoteReplacer, doubleQuote)
}
