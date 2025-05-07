package model

import (
	"strconv"
	"strings"
	"time"

	"github.com/kinkando/pharma-sheet-service/pkg/google"
	"github.com/kinkando/pharma-sheet-service/pkg/util"
	"google.golang.org/api/sheets/v4"
)

const (
	DateLayout = "2/1/2006"
)

type GetSyncMedicineMetadataRequest struct {
	WarehouseID string `param:"warehouseID" validate:"required"`
	URL         string `query:"url" validate:"required,url"`
}

type SyncMedicineRequest struct {
	WarehouseID string `param:"warehouseID" validate:"required"`
	URL         string `json:"url" validate:"required,url"`
}

type SyncMedicineMetadata struct {
	Title       string           `json:"title"`
	Medication  MedicineMetadata `json:"medication"`
	House       MedicineMetadata `json:"house"`
	Brand       MedicineMetadata `json:"brand"`
	BlisterDate MedicineMetadata `json:"blisterDate"`
}

type MedicineMetadata struct {
	SheetName            string `json:"sheetName"`
	TotalMedicine        uint64 `json:"totalMedicine"`
	TotalNewMedicine     uint64 `json:"totalNewMedicine"`
	TotalUpdatedMedicine uint64 `json:"totalUpdatedMedicine"`
	TotalSkippedMedicine uint64 `json:"totalSkippedMedicine"`
}

type GoogleSheetData struct {
	SpreadsheetTitle string
	SpreadsheetID    string
	Medication       MedicineSheetMetadata
	Brand            MedicineBrandSheetMetadata
	House            MedicineHouseSheetMetadata
	BlisterDate      MedicineBlisterDateSheetMetadata
}

type MedicineSheetMetadata struct {
	Sheet          *sheets.Sheet
	MedicineSheets []MedicineSheet
	MedicineData   map[string]Medicine
}

type MedicineBrandSheetMetadata struct {
	Sheet          *sheets.Sheet
	MedicineSheets []MedicineBrandSheet
	MedicineData   map[string]MedicineBrand
}

type MedicineSheet struct {
	MedicationID string `csv:"Medication_ID" json:"medicationID"`
	MedicalName  string `csv:"ชื่อสามัญทางยา" json:"medicalName,omitempty"`
}

func (m *MedicineSheet) IsDifferent(req Medicine) bool {
	return m.MedicationID != req.MedicationID ||
		m.MedicalName != req.MedicalName
}

func (m *MedicineSheet) IsInvalid() bool {
	return m.MedicationID == "" || m.MedicalName == ""
}

func (m *MedicineSheet) ExternalID() string {
	return m.MedicationID
}

type MedicineBrandSheet struct {
	MedicationID    string `csv:"Medication_ID" json:"medicationID"`
	MedicalName     string `csv:"ชื่อสามัญทางยา" json:"medicalName,omitempty"`
	TradeID         string `csv:"TRADENAME_ID" json:"tradeID,omitempty"`
	TradeName       string `csv:"ชื่อการค้า" json:"tradeName,omitempty"`
	BlisterImageURL string `csv:"Link_แผงยา" json:"blisterImageURL,omitempty"`
	TabletImageURL  string `csv:"Link_เม็ดยา" json:"tabletImageURL,omitempty"`
	BoxImageURL     string `csv:"Link_กล่องยา" json:"boxImageURL,omitempty"`
}

func (m *MedicineBrandSheet) FileIDs() (blisterFileID, tabletFileID, boxFileID *string) {
	if m.BlisterImageURL != "" {
		if fileID := google.FileID(m.BlisterImageURL); fileID != "" && fileID != "-" {
			blisterFileID = &fileID
		}
	}
	if m.TabletImageURL != "" {
		if fileID := google.FileID(m.TabletImageURL); fileID != "" && fileID != "-" {
			tabletFileID = &fileID
		}
	}
	if m.BoxImageURL != "" {
		if fileID := google.FileID(m.BoxImageURL); fileID != "" && fileID != "-" {
			boxFileID = &fileID
		}
	}
	return blisterFileID, tabletFileID, boxFileID
}

func (m *MedicineBrandSheet) IsDifferent(req MedicineBrand) bool {
	blisterFileID, tabletFileID, boxFileID := m.FileIDs()
	return m.MedicationID != req.MedicationID ||
		m.TradeID != req.TradeID ||
		util.Value(blisterFileID) != util.Value(req.BlisterImageURL) ||
		util.Value(tabletFileID) != util.Value(req.TabletImageURL) ||
		util.Value(boxFileID) != util.Value(req.BoxImageURL)
}

func (m *MedicineBrandSheet) IsInvalid() bool {
	if fileID := google.FileID(m.BlisterImageURL); fileID == "" || fileID == "-" {
		m.BlisterImageURL = ""
	}
	if fileID := google.FileID(m.TabletImageURL); fileID == "" || fileID == "-" {
		m.TabletImageURL = ""
	}
	if fileID := google.FileID(m.BoxImageURL); fileID == "" || fileID == "-" {
		m.BoxImageURL = ""
	}
	m.TradeName = strings.TrimSpace(strings.ReplaceAll(m.TradeName, "-", ""))
	return m.MedicationID == "" || m.TradeID == "" || (m.TradeName == "" && m.BlisterImageURL == "" && m.TabletImageURL == "" && m.BoxImageURL == "")
}

func (m *MedicineBrandSheet) ExternalID() string {
	return m.MedicationID + "-" + m.TradeID
}

type MedicineHouseSheetMetadata struct {
	Sheet          *sheets.Sheet
	MedicineSheets []MedicineHouseSheet
	MedicineData   map[string]MedicineHouse
}

type MedicineHouseSheet struct {
	WarehouseID  string `csv:"ศูนย์" json:"warehouseID"`
	HouseID      string `csv:"House_ID" json:"houseID"`
	Locker       string `csv:"ตู้" json:"locker"`
	FloorText    string `csv:"ชั้น" json:"floor"`
	NoText       string `csv:"ลำดับที่" json:"no"`
	Address      string `csv:"บ้านเลขที่ยา" json:"address"`
	MedicationID string `csv:"Medication_ID" json:"medicationID"`
	MedicalName  string `csv:"ชื่อสามัญทางยา" json:"medicalName,omitempty"`
	Label        string `csv:"Label ตะกร้า" json:"label,omitempty"`
}

func (m *MedicineHouseSheet) Floor() int32 {
	if floor, err := strconv.Atoi(m.FloorText); err == nil && floor > 0 {
		return int32(floor)
	}
	return 0
}

func (m *MedicineHouseSheet) No() int32 {
	if no, err := strconv.Atoi(m.NoText); err == nil && no > 0 {
		return int32(no)
	}
	return 0
}

func (m *MedicineHouseSheet) IsDifferent(req MedicineHouse) bool {
	return m.WarehouseID != req.WarehouseID ||
		m.MedicationID != req.MedicationID ||
		m.Locker != req.Locker ||
		m.Floor() != req.Floor ||
		m.No() != req.No ||
		m.Address != req.Address() ||
		m.Label != util.Value(req.Label)
}

func (m *MedicineHouseSheet) IsInvalid() bool {
	return m.WarehouseID == "" || m.HouseID == "" || m.MedicationID == "" || m.MedicalName == "" ||
		m.Locker == "" || m.Floor() <= 0 || m.No() <= 0 || m.Address == ""
}

func (m *MedicineHouseSheet) ExternalID() string {
	return m.WarehouseID + "-" + m.MedicationID + "-" + m.Address
}

type MedicineBlisterDateSheetMetadata struct {
	Sheet          *sheets.Sheet
	MedicineSheets []MedicineBlisterDateSheet
	MedicineData   map[string]MedicineBlisterDateHistory
}

type MedicineBlisterDateSheet struct {
	WarehouseID  string `csv:"ศูนย์" json:"warehouseID"`
	HouseID      string `csv:"House_ID" json:"houseID"`
	MedicationID string `csv:"Medication_ID" json:"medicationID"`
	MedicalName  string `csv:"ชื่อสามัญทางยา" json:"medicalName,omitempty"`
	TradeID      string `csv:"TRADENAME_ID" json:"tradeID,omitempty"`
	TradeName    string `csv:"ชื่อการค้า" json:"tradeName,omitempty"`
	BlisterDate  string `csv:"วันที่เปลี่ยนแผงยา" json:"date,omitempty"`
}

func (m *MedicineBlisterDateSheet) IsDifferent(req MedicineBlisterDateHistory) bool {
	date, _ := time.Parse(DateLayout, m.BlisterDate)
	reqDate := req.BlisterChangeDate.Format(DateLayout)
	return m.MedicationID != req.MedicationID ||
		m.WarehouseID != req.WarehouseID ||
		m.TradeID != req.TradeID ||
		date.Format(DateLayout) != reqDate
}

func (m *MedicineBlisterDateSheet) IsInvalid() bool {
	date, _ := time.Parse(DateLayout, m.BlisterDate)
	return m.MedicationID == "" || m.WarehouseID == "" ||
		m.HouseID == "" || m.TradeID == "" || date.IsZero()
}

func (m *MedicineBlisterDateSheet) ExternalID() string {
	date, _ := time.Parse(DateLayout, m.BlisterDate)
	return m.WarehouseID + "-" + m.MedicationID + "-" + m.TradeID + "-" + date.Format(time.DateOnly)
}
