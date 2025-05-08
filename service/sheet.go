package service

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
	genmodel "github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/model"
	"github.com/kinkando/pharma-sheet-service/model"
	"github.com/kinkando/pharma-sheet-service/pkg/google"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"github.com/kinkando/pharma-sheet-service/pkg/profile"
	"github.com/kinkando/pharma-sheet-service/repository"
	"github.com/labstack/echo/v4"
	"github.com/sourcegraph/conc/pool"
	"google.golang.org/api/sheets/v4"
)

const (
	medicationSheetName  = "Medication_ID"
	brandSheetName       = "Pictures"
	blisterDateSheetName = "วันที่เปลี่ยนแผงยา"
	houseSheetName       = "บ้านเลขที่ยา"
)

type Sheet interface {
	SummarizeMedicineFromGoogleSheet(ctx context.Context, req model.GetSyncMedicineMetadataRequest) (model.SyncMedicineMetadata, error)
	SyncMedicineFromGoogleSheet(ctx context.Context, req model.SyncMedicineRequest) error
}

type sheet struct {
	warehouseRepository repository.Warehouse
	medicineRepository  repository.Medicine
	drive               google.Drive
	sheet               google.Sheet
}

func NewSheetService(
	warehouseRepository repository.Warehouse,
	medicineRepository repository.Medicine,
	drive google.Drive,
	googleSheet google.Sheet,
) Sheet {
	return &sheet{
		warehouseRepository: warehouseRepository,
		medicineRepository:  medicineRepository,
		drive:               drive,
		sheet:               googleSheet,
	}
}

func (s *sheet) SummarizeMedicineFromGoogleSheet(ctx context.Context, req model.GetSyncMedicineMetadataRequest) (metadata model.SyncMedicineMetadata, err error) {
	data, err := s.getGoogleSheetData(ctx, model.SyncMedicineRequest(req), false)
	if err != nil {
		return
	}

	metadata = model.SyncMedicineMetadata{
		Title: data.SpreadsheetTitle,
		Medication: model.MedicineMetadata{
			SheetName: data.Medication.Sheet.Properties.Title,
		},
		Brand: model.MedicineMetadata{
			SheetName: data.Brand.Sheet.Properties.Title,
		},
		House: model.MedicineMetadata{
			SheetName: data.House.Sheet.Properties.Title,
		},
		BlisterDate: model.MedicineMetadata{
			SheetName: data.BlisterDate.Sheet.Properties.Title,
		},
	}

	for _, medicineSheet := range data.Medication.MedicineSheets {
		metadata.Medication.TotalMedicine++

		medicine, ok := data.Medication.MedicineData[medicineSheet.MedicationID]
		if !ok {
			metadata.Medication.TotalNewMedicine++
			continue
		}

		if medicineSheet.IsDifferent(medicine) {
			metadata.Medication.TotalUpdatedMedicine++
		} else {
			metadata.Medication.TotalSkippedMedicine++
		}
	}

	for _, medicineSheet := range data.Brand.MedicineSheets {
		metadata.Brand.TotalMedicine++

		medicine, ok := data.Brand.MedicineData[medicineSheet.ExternalID()]
		if !ok {
			metadata.Brand.TotalNewMedicine++
			continue
		}

		if medicineSheet.IsDifferent(medicine) {
			metadata.Brand.TotalUpdatedMedicine++
		} else {
			metadata.Brand.TotalSkippedMedicine++
		}
	}

	for _, medicineSheet := range data.House.MedicineSheets {
		metadata.House.TotalMedicine++

		medicine, ok := data.House.MedicineData[medicineSheet.ExternalID()]
		if !ok {
			metadata.House.TotalNewMedicine++
			continue
		}

		if medicineSheet.IsDifferent(medicine) {
			metadata.House.TotalUpdatedMedicine++
		} else {
			metadata.House.TotalSkippedMedicine++
		}
	}

	for _, medicineSheet := range data.BlisterDate.MedicineSheets {
		metadata.BlisterDate.TotalMedicine++

		if _, ok := data.BlisterDate.MedicineData[medicineSheet.ExternalID()]; !ok {
			metadata.BlisterDate.TotalNewMedicine++
		} else {
			metadata.BlisterDate.TotalSkippedMedicine++
		}
	}

	return metadata, nil
}

func (s *sheet) SyncMedicineFromGoogleSheet(ctx context.Context, req model.SyncMedicineRequest) error {
	data, err := s.getGoogleSheetData(ctx, req, true)
	if err != nil {
		return err
	}

	for _, medicineSheet := range data.Medication.MedicineSheets {
		medicine, ok := data.Medication.MedicineData[medicineSheet.MedicationID]
		if !ok {
			_, err = s.medicineRepository.CreateMedicine(ctx, model.CreateMedicineRequest{MedicationID: medicineSheet.MedicationID, MedicalName: &medicineSheet.MedicalName})
			if err != nil {
				logger.Context(ctx).Error(err)
				if model.IsConflictError(err) {
					return echo.NewHTTPError(http.StatusConflict, echo.Map{"error": "medicine already exists"})
				}
				return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
			}
			continue
		}

		if medicineSheet.IsDifferent(medicine) {
			err = s.medicineRepository.UpdateMedicine(ctx, model.UpdateMedicineRequest{MedicationID: medicineSheet.MedicationID, MedicalName: &medicineSheet.MedicalName})
			if err != nil {
				logger.Context(ctx).Error(err)
				return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
			}
		}
	}

	for _, medicineSheet := range data.Brand.MedicineSheets {
		blisterFileID, tabletFileID, boxFileID := medicineSheet.FileIDs()
		medicine, ok := data.Brand.MedicineData[medicineSheet.ExternalID()]
		if !ok {
			_, err := s.medicineRepository.CreateMedicineBrand(ctx, model.CreateMedicineBrandRequest{
				MedicationID:    medicineSheet.MedicationID,
				TradeID:         medicineSheet.TradeID,
				TradeName:       &medicineSheet.TradeName,
				BlisterImageURL: blisterFileID,
				TabletImageURL:  tabletFileID,
				BoxImageURL:     boxFileID,
			})
			if err != nil {
				logger.Context(ctx).Error(err)
				if model.IsConflictError(err) {
					return echo.NewHTTPError(http.StatusConflict, echo.Map{"error": "medicine brand already exists"})
				}
				return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
			}
			continue
		}

		if medicineSheet.IsDifferent(medicine) {
			deleteFileID := "null"
			if blisterFileID == nil {
				blisterFileID = &deleteFileID
			}
			if tabletFileID == nil {
				tabletFileID = &deleteFileID
			}
			if boxFileID == nil {
				boxFileID = &deleteFileID
			}
			err = s.medicineRepository.UpdateMedicineBrand(ctx, model.UpdateMedicineBrandRequest{
				BrandID:         medicine.ID,
				TradeName:       &medicineSheet.TradeName,
				BlisterImageURL: blisterFileID,
				TabletImageURL:  tabletFileID,
				BoxImageURL:     boxFileID,
			})
			if err != nil {
				logger.Context(ctx).Error(err)
				return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
			}
		}
	}

	for _, medicineSheet := range data.House.MedicineSheets {
		medicine, ok := data.House.MedicineData[medicineSheet.ExternalID()]
		if !ok {
			data := model.CreateMedicineHouseRequest{
				MedicationID: medicineSheet.MedicationID,
				WarehouseID:  medicineSheet.WarehouseID,
				Locker:       medicineSheet.Locker,
				Floor:        medicineSheet.Floor(),
				No:           medicineSheet.No(),
				Label:        &medicineSheet.Label,
			}
			_, err := s.medicineRepository.CreateMedicineHouse(ctx, data)
			if err != nil {
				logger.Context(ctx).With("data", data).Error(err)
				if model.IsConflictError(err) {
					return echo.NewHTTPError(http.StatusConflict, echo.Map{"error": "medicine house already exists"})
				}
				return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
			}
			continue
		}

		if medicineSheet.IsDifferent(medicine) {
			data := model.UpdateMedicineHouseRequest{
				ID:           medicine.ID,
				MedicationID: medicineSheet.MedicationID,
				Locker:       medicineSheet.Locker,
				Floor:        medicineSheet.Floor(),
				No:           medicineSheet.No(),
				Label:        &medicineSheet.Label,
			}
			err = s.medicineRepository.UpdateMedicineHouse(ctx, data)
			if err != nil {
				logger.Context(ctx).With("data", data).Error(err)
				return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
			}
		}
	}

	brands, err := s.medicineRepository.ListMedicineBrands(ctx)
	if err != nil {
		logger.Context(ctx).Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	brandID := make(map[string]uuid.UUID)
	for _, brand := range brands {
		brandID[brand.MedicationID+"-"+brand.TradeID] = brand.ID
	}
	for _, medicineSheet := range data.BlisterDate.MedicineSheets {
		date, _ := time.Parse(model.DateLayout, medicineSheet.BlisterDate)
		var medicineBrandID *uuid.UUID
		if id, ok := brandID[medicineSheet.MedicationID+"-"+medicineSheet.TradeID]; ok && id != uuid.Nil {
			medicineBrandID = &id
		}

		_, ok := data.BlisterDate.MedicineData[medicineSheet.ExternalID()]
		if !ok {
			_, err := s.medicineRepository.CreateMedicineBlisterChangeDateHistory(ctx, model.CreateMedicineBlisterChangeDateHistoryRequest{
				MedicationID:      medicineSheet.MedicationID,
				WarehouseID:       medicineSheet.WarehouseID,
				BrandID:           medicineBrandID,
				BlisterChangeDate: date,
			})
			if err != nil {
				logger.Context(ctx).Error(err)
				if model.IsConflictError(err) {
					return echo.NewHTTPError(http.StatusConflict, echo.Map{"error": "medicine blister date history already exists"})
				}
				return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
			}
			continue
		}
	}

	return nil
}

func (s *sheet) checkWarehouseManagementRole(ctx context.Context, warehouseID string, roles ...genmodel.PharmaSheetRole) (err error) {
	userProfile, err := profile.UseProfile(ctx)
	if err != nil {
		return
	}

	role, err := s.warehouseRepository.GetWarehouseRole(ctx, warehouseID, userProfile.UserID)
	if err != nil {
		logger.Context(ctx).Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	if !slices.Contains(roles, role) {
		return echo.NewHTTPError(http.StatusForbidden, echo.Map{"error": "permission denied"})
	}

	return nil
}

func (s *sheet) getGoogleSheetData(ctx context.Context, req model.SyncMedicineRequest, isUpdateWarehouseSheet bool) (data model.GoogleSheetData, err error) {
	err = s.checkWarehouseManagementRole(ctx, req.WarehouseID, genmodel.PharmaSheetRole_Admin, genmodel.PharmaSheetRole_Editor)
	if err != nil {
		logger.Context(ctx).Error(err)
		return data, err
	}

	spreadsheetID, _, err := extractSpreadsheetInfo(req.URL)
	if err != nil {
		logger.Context(ctx).Error(err)
		return data, echo.NewHTTPError(http.StatusBadRequest, echo.Map{"error": "url is invalid"})
	}

	spreadsheet, err := s.sheet.Get(ctx, spreadsheetID)
	if err != nil {
		logger.Context(ctx).Error(err)
		return data, echo.NewHTTPError(http.StatusNotFound, echo.Map{"error": "spreadsheetID is not found"})
	}

	targetSheets := []string{medicationSheetName, brandSheetName, blisterDateSheetName, houseSheetName}
	sheets := make(map[string]*sheets.Sheet)
	for _, spreadSheet := range spreadsheet.Sheets {
		if slices.Contains(targetSheets, spreadSheet.Properties.Title) {
			sheets[spreadSheet.Properties.Title] = spreadSheet
		}
	}
	if len(sheets) != len(targetSheets) {
		logger.Context(ctx).Errorf("sheet is invalid")
		return data, echo.NewHTTPError(http.StatusBadRequest, echo.Map{"error": "sheet is invalid"})
	}

	if isUpdateWarehouseSheet {
		err = s.warehouseRepository.UpsertWarehouseSheet(ctx, genmodel.PharmaSheetWarehouseSheets{
			WarehouseID:                         req.WarehouseID,
			SpreadsheetID:                       spreadsheetID,
			MedicineSheetID:                     int32(sheets[medicationSheetName].Properties.SheetId),
			MedicineSheetName:                   sheets[medicationSheetName].Properties.Title,
			MedicineBrandSheetID:                int32(sheets[brandSheetName].Properties.SheetId),
			MedicineBrandSheetName:              sheets[brandSheetName].Properties.Title,
			MedicineHouseSheetID:                int32(sheets[houseSheetName].Properties.SheetId),
			MedicineHouseSheetName:              sheets[houseSheetName].Properties.Title,
			MedicineBlisterDateHistorySheetID:   int32(sheets[blisterDateSheetName].Properties.SheetId),
			MedicineBlisterDateHistorySheetName: sheets[blisterDateSheetName].Properties.Title,
		})
		if err != nil {
			logger.Context(ctx).Error(err)
			return data, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}
	}

	data = model.GoogleSheetData{
		SpreadsheetTitle: spreadsheet.Properties.Title,
		SpreadsheetID:    spreadsheetID,
	}

	conc := pool.New().WithContext(ctx)
	conc.Go(func(ctx context.Context) error {
		data.Medication, err = s.mappingMedicineSheet(ctx, sheets[medicationSheetName])
		return err
	})
	conc.Go(func(ctx context.Context) error {
		data.Brand, err = s.mappingMedicineBrandSheet(ctx, sheets[brandSheetName])
		return err
	})
	conc.Go(func(ctx context.Context) error {
		data.House, err = s.mappingMedicineHouseSheet(ctx, sheets[houseSheetName], req.WarehouseID)
		return err
	})
	conc.Go(func(ctx context.Context) error {
		data.BlisterDate, err = s.mappingMedicineBlisterDateSheet(ctx, sheets[blisterDateSheetName], req.WarehouseID)
		return err
	})
	if err = conc.Wait(); err != nil {
		return data, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return data, nil
}

func (s *sheet) mappingMedicineSheet(ctx context.Context, sheet *sheets.Sheet) (data model.MedicineSheetMetadata, err error) {
	data.Sheet = sheet

	medicineData, err := s.medicineRepository.ListMedicinesMaster(ctx)
	if err != nil {
		logger.Context(ctx).Error(err)
		return data, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	data.MedicineData = make(map[string]model.Medicine)
	for _, medicine := range medicineData {
		data.MedicineData[medicine.MedicationID] = medicine
	}

	var sheetData []model.MedicineSheet
	_, err = s.sheet.Read(ctx, sheet, &sheetData)
	if err != nil {
		logger.Context(ctx).Error(err)
		return data, echo.NewHTTPError(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	for _, sheetData := range sheetData {
		if !sheetData.IsInvalid() {
			data.MedicineSheets = append(data.MedicineSheets, sheetData)
		}
	}

	return data, nil
}

func (s *sheet) mappingMedicineBrandSheet(ctx context.Context, sheet *sheets.Sheet) (data model.MedicineBrandSheetMetadata, err error) {
	data.Sheet = sheet

	medicineData, err := s.medicineRepository.ListMedicineBrands(ctx)
	if err != nil {
		logger.Context(ctx).Error(err)
		return data, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	data.MedicineData = make(map[string]model.MedicineBrand)
	for _, medicine := range medicineData {
		data.MedicineData[medicine.ExternalID()] = medicine
	}

	var sheetData []model.MedicineBrandSheet
	_, err = s.sheet.Read(ctx, sheet, &sheetData)
	if err != nil {
		logger.Context(ctx).Error(err)
		return data, echo.NewHTTPError(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	for _, sheetData := range sheetData {
		if !sheetData.IsInvalid() {
			data.MedicineSheets = append(data.MedicineSheets, sheetData)
		}
	}

	return data, nil
}

func (s *sheet) mappingMedicineHouseSheet(ctx context.Context, sheet *sheets.Sheet, warehouseID string) (data model.MedicineHouseSheetMetadata, err error) {
	data.Sheet = sheet

	medicineData, err := s.medicineRepository.GetMedicineHouses(ctx, model.FilterMedicineHouse{WarehouseID: warehouseID})
	if err != nil {
		logger.Context(ctx).Error(err)
		return data, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	data.MedicineData = make(map[string]model.MedicineHouse)
	for _, medicine := range medicineData {
		data.MedicineData[medicine.ExternalID()] = medicine
	}

	var sheetData []model.MedicineHouseSheet
	_, err = s.sheet.Read(ctx, sheet, &sheetData)
	if err != nil {
		logger.Context(ctx).Error(err)
		return data, echo.NewHTTPError(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	for _, sheetData := range sheetData {
		if !sheetData.IsInvalid() && sheetData.WarehouseID == warehouseID {
			data.MedicineSheets = append(data.MedicineSheets, sheetData)
		}
	}

	return data, nil
}

func (s *sheet) mappingMedicineBlisterDateSheet(ctx context.Context, sheet *sheets.Sheet, warehouseID string) (data model.MedicineBlisterDateSheetMetadata, err error) {
	data.Sheet = sheet

	medicineData, err := s.medicineRepository.ListMedicineBlisterChangeDateHistory(ctx, model.FilterMedicineBrandBlisterDateHistory{WarehouseID: &warehouseID})
	if err != nil {
		logger.Context(ctx).Error(err)
		return data, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	data.MedicineData = make(map[string]model.MedicineBlisterDateHistory)
	for _, medicine := range medicineData {
		data.MedicineData[medicine.ExternalID()] = medicine
	}

	var sheetData []model.MedicineBlisterDateSheet
	_, err = s.sheet.Read(ctx, sheet, &sheetData)
	if err != nil {
		logger.Context(ctx).Error(err)
		return data, echo.NewHTTPError(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	for _, sheetData := range sheetData {
		if !sheetData.IsInvalid() && sheetData.WarehouseID == warehouseID {
			if sheetData.TradeID == "-" {
				sheetData.TradeID = ""
			}
			data.MedicineSheets = append(data.MedicineSheets, sheetData)
		}
	}

	return data, nil
}

func extractSpreadsheetInfo(url string) (string, int32, error) {
	// Regular expressions for extracting the spreadsheet ID and gid
	spreadsheetIDPattern := `\/d\/([a-zA-Z0-9-_]+)`
	gidPattern := `gid=(\d+)`

	// Compile the regex patterns
	spreadsheetIDRegex, err := regexp.Compile(spreadsheetIDPattern)
	if err != nil {
		return "", 0, fmt.Errorf("failed to compile spreadsheet ID regex: %v", err)
	}

	gidRegex, err := regexp.Compile(gidPattern)
	if err != nil {
		return "", 0, fmt.Errorf("failed to compile gid regex: %v", err)
	}

	// Find the spreadsheet ID and gid using the regex
	spreadsheetIDMatches := spreadsheetIDRegex.FindStringSubmatch(url)
	if len(spreadsheetIDMatches) < 2 {
		return "", 0, fmt.Errorf("failed to extract spreadsheet ID from the URL")
	}

	gidMatches := gidRegex.FindStringSubmatch(url)
	if len(gidMatches) < 2 {
		return "", 0, fmt.Errorf("failed to extract gid from the URL")
	}

	gid, err := strconv.Atoi(gidMatches[1])
	if err != nil {
		return "", 0, fmt.Errorf("failed to convert gid to integer: %v", err)
	}

	// Return the extracted values
	return spreadsheetIDMatches[1], int32(gid), nil
}
