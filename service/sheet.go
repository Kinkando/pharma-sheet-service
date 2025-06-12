package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
	genmodel "github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/model"
	"github.com/kinkando/pharma-sheet-service/model"
	"github.com/kinkando/pharma-sheet-service/pkg/documentgenerator"
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
	ExportSheet(ctx context.Context, req model.ExportSheetRequest) (io.Reader, error)
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

func (s *sheet) ExportSheet(ctx context.Context, req model.ExportSheetRequest) (io.Reader, error) {
	file := documentgenerator.NewExcelizeFile()
	defer file.Close()

	medicines, err := s.medicineRepository.ListMedicinesMaster(ctx)
	if err != nil {
		logger.Context(ctx).Error(err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	medicineBrands, err := s.medicineRepository.ListMedicineBrands(ctx)
	if err != nil {
		logger.Context(ctx).Error(err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	medicineMapping := make(map[string]string)
	for _, medicine := range medicines {
		medicineMapping[medicine.MedicationID] = medicine.MedicalName
	}

	var histories []model.MedicineBlisterDateHistory
	var houses []model.MedicineHouse
	for _, warehouseID := range req.WarehouseIDs {
		if err = s.checkWarehouseManagementRole(ctx, warehouseID, genmodel.PharmaSheetRole_Admin, genmodel.PharmaSheetRole_Editor, genmodel.PharmaSheetRole_Viewer); err != nil {
			logger.Context(ctx).Error(err)
			return nil, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}

		medicineHouses, err := s.medicineRepository.GetMedicineHouses(ctx, model.FilterMedicineHouse{WarehouseID: warehouseID})
		if err != nil {
			logger.Context(ctx).Error(err)
			return nil, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}
		houses = append(houses, medicineHouses...)

		medicineBlisterDateHistories, err := s.medicineRepository.ListMedicineBlisterChangeDateHistory(ctx, model.FilterMedicineBrandBlisterDateHistory{WarehouseID: &warehouseID})
		if err != nil {
			logger.Context(ctx).Error(err)
			return nil, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}
		histories = append(histories, medicineBlisterDateHistories...)
	}

	err = s.exportSheetMedicineID(ctx, file, medicines)
	if err != nil {
		logger.Context(ctx).Error(err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	err = s.exportSheetMedicineHouse(ctx, file, medicineMapping, houses)
	if err != nil {
		logger.Context(ctx).Error(err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	err = s.exportSheetMedicineBlisterDate(ctx, file, medicineMapping, houses, medicineBrands, histories)
	if err != nil {
		logger.Context(ctx).Error(err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	err = s.exportSheetMedicineBrand(ctx, file, medicineMapping, medicineBrands)
	if err != nil {
		logger.Context(ctx).Error(err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	_ = file.DeleteDefaultSheet()
	if err := file.SetAutoWidth(4); err != nil {
		logger.Context(ctx).Error(err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	data, err := file.WriteToBuffer()
	if err != nil {
		logger.Context(ctx).Error(err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return data, nil
}

func (s *sheet) exportSheetMedicineID(_ context.Context, file *documentgenerator.Excelize, medicines []model.Medicine) error {
	if len(medicines) == 0 {
		return nil
	}

	data := [][]any{{"Medication ID", "ชื่อสามัญทางยา"}}
	for _, medicine := range medicines {
		data = append(data, []any{medicine.MedicationID, medicine.MedicalName})
	}
	return file.SetSheetData(medicationSheetName, data, documentgenerator.WithExcelizeSetSheetDataIsNewSheet(true))
}

func (s *sheet) exportSheetMedicineHouse(_ context.Context, file *documentgenerator.Excelize, medicineMapping map[string]string, medicineHouses []model.MedicineHouse) error {
	if len(medicineHouses) == 0 || len(medicineMapping) == 0 {
		return nil
	}

	data := [][]any{{
		"ศูนย์",
		"House_ID",
		"ตู้",
		"ชั้น",
		"ลำดับที่",
		"บ้านเลขที่ยา",
		"ชื่อสามัญทางยา",
		"Medication_ID",
		"Label ตะกร้า",
	}}

	currentRow := len(data)
	for _, medicine := range medicineHouses {
		currentRow++
		label := "-"
		if medicine.Label != nil {
			label = *medicine.Label
		}
		data = append(data, []any{
			medicine.WarehouseID,
			fmt.Sprintf(`=CONCATENATE(A%d, "-", H%d)`, currentRow, currentRow),
			medicine.Locker,
			medicine.Floor,
			medicine.No,
			fmt.Sprintf(`=CONCATENATE(C%d, "-", D%d, "-", E%d)`, currentRow, currentRow, currentRow),
			medicineMapping[medicine.MedicationID],
			fmt.Sprintf(`=VLOOKUP(G%d, {%s!B:B, %s!A:A}, 2, FALSE)`, currentRow, medicationSheetName, medicationSheetName),
			label,
		})
	}

	return file.SetSheetData(
		houseSheetName,
		data,
		documentgenerator.WithExcelizeSetSheetDataIsNewSheet(true),
		documentgenerator.WithExcelizeSetSheetDataCell(documentgenerator.CellModeFormula),
		documentgenerator.WithExcelizeSetSheetDataDropdown(documentgenerator.DropdownOption{Range: fmt.Sprintf("G2:G%d", currentRow), Ref: fmt.Sprintf("%s!$B$2:$B$%d", medicationSheetName, len(medicineMapping)+1)}),
	)
}

func (s *sheet) exportSheetMedicineBlisterDate(_ context.Context, file *documentgenerator.Excelize, medicineMapping map[string]string, medicineHouses []model.MedicineHouse, medicineBrands []model.MedicineBrand, histories []model.MedicineBlisterDateHistory) error {
	if len(medicineHouses) == 0 || len(medicineBrands) == 0 || len(medicineMapping) == 0 {
		return nil
	}

	medicineBrandMapping := make(map[string][]model.MedicineBrand)
	for _, medicineBrand := range medicineBrands {
		if _, ok := medicineBrandMapping[medicineBrand.MedicationID]; !ok {
			medicineBrandMapping[medicineBrand.MedicationID] = []model.MedicineBrand{}
		}
		medicineBrandMapping[medicineBrand.MedicationID] = append(medicineBrandMapping[medicineBrand.MedicationID], medicineBrand)
	}

	historyMapping := make(map[string][]model.MedicineBlisterDateHistory)
	for _, history := range histories {
		id := history.WarehouseID + "-" + history.MedicationID
		if history.TradeID != nil && *history.TradeID != "" {
			id += "-" + *history.TradeID
		}
		if _, ok := historyMapping[id]; !ok {
			historyMapping[id] = []model.MedicineBlisterDateHistory{}
		}
		historyMapping[id] = append(historyMapping[id], history)
	}

	data := [][]any{{
		"ศูนย์",
		"Medication_ID",
		"House_ID",
		"ชื่อสามัญทางยา",
		"ชื่อการค้า",
		"วันที่เปลี่ยนแผงยา",
		"TRADENAME_ID",
	}}

	currentRow := len(data) + 1
	for _, medicineHouse := range medicineHouses {
		medicineBrand, ok := medicineBrandMapping[medicineHouse.MedicationID]
		if !ok {
			continue
		}
		for _, brand := range medicineBrand {
			tradeName := "-"
			if brand.TradeName != nil {
				tradeName = *brand.TradeName
			}
			if histories, ok := historyMapping[medicineHouse.WarehouseID+"-"+medicineHouse.MedicationID+"-"+brand.TradeID]; ok && len(histories) > 0 {
				for _, history := range histories {
					data = append(data, []any{
						medicineHouse.WarehouseID,
						brand.MedicationID,
						fmt.Sprintf(`=CONCATENATE(A%d, "-", B%d)`, currentRow, currentRow),
						medicineMapping[brand.MedicationID],
						tradeName,
						history.BlisterChangeDate.Format(model.DateLayout),
						brand.TradeID,
					})
					currentRow++
				}
				continue
			}
			data = append(data, []any{
				medicineHouse.WarehouseID,
				brand.MedicationID,
				fmt.Sprintf(`=CONCATENATE(A%d, "-", B%d)`, currentRow, currentRow),
				medicineMapping[brand.MedicationID],
				tradeName,
				"ไม่มีข้อมูล",
				brand.TradeID,
			})
			currentRow++
		}
	}

	return file.SetSheetData(
		blisterDateSheetName,
		data,
		documentgenerator.WithExcelizeSetSheetDataIsNewSheet(true),
		documentgenerator.WithExcelizeSetSheetDataCell(documentgenerator.CellModeFormula),
		documentgenerator.WithExcelizeSetSheetDataDropdown(documentgenerator.DropdownOption{Range: fmt.Sprintf("D2:D%d", currentRow-1), Ref: fmt.Sprintf("%s!$B$2:$B$%d", medicationSheetName, len(medicineMapping)+1)}),
	)
}

func (s *sheet) exportSheetMedicineBrand(ctx context.Context, file *documentgenerator.Excelize, medicineMapping map[string]string, medicineBrands []model.MedicineBrand) error {
	if len(medicineBrands) == 0 || len(medicineMapping) == 0 {
		return nil
	}

	link := func(fileID *string) string {
		if fileID == nil || *fileID == "" {
			return "-"
		}
		return google.PublicURL(*fileID)
	}

	imageFormula := `=IF(%s%d="-", "-", IMAGE(CONCAT("https://drive.google.com/uc?export=view&id=", REGEXEXTRACT(%s%d, "/d/([a-zA-Z0-9_-]+)"))))`

	data := [][]any{{
		"Medication_ID",
		"ชื่อสามัญทางยา",
		"TRADENAME_ID",
		"ชื่อการค้า",
		"Link_แผงยา",
		"ภาพแผงยา",
		"Link_เม็ดยา",
		"ภาพเม็ดยา",
		"Link_กล่องยา",
		"ภาพกล่องยา",
	}}

	currentRow := len(data)
	for _, medicineBrand := range medicineBrands {
		currentRow++
		tradeName := "-"
		if medicineBrand.TradeName != nil {
			tradeName = *medicineBrand.TradeName
		}
		data = append(data, []any{
			medicineBrand.MedicationID,
			medicineMapping[medicineBrand.MedicationID],
			medicineBrand.TradeID,
			tradeName,
			link(medicineBrand.BlisterImageURL),
			fmt.Sprintf(imageFormula, "E", currentRow, "E", currentRow),
			link(medicineBrand.TabletImageURL),
			fmt.Sprintf(imageFormula, "G", currentRow, "G", currentRow),
			link(medicineBrand.BoxImageURL),
			fmt.Sprintf(imageFormula, "I", currentRow, "I", currentRow),
		})
	}

	return file.SetSheetData(
		brandSheetName,
		data,
		documentgenerator.WithExcelizeSetSheetDataIsNewSheet(true),
		documentgenerator.WithExcelizeSetSheetDataCell(documentgenerator.CellModeFormula),
		documentgenerator.WithExcelizeSetSheetDataDropdown(documentgenerator.DropdownOption{Range: fmt.Sprintf("B2:B%d", currentRow), Ref: fmt.Sprintf("%s!$B$2:$B$%d", medicationSheetName, len(medicineMapping)+1)}),
	)
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
