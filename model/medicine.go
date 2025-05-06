package model

import (
	"mime/multipart"
	"time"

	"github.com/google/uuid"
)

type Medicine struct {
	MedicationID         string                           `json:"medicationID"`
	MedicalName          string                           `json:"medicalName,omitempty"`
	Brands               []MedicineBrand                  `json:"brands,omitempty"`
	Houses               []MedicineHouseView              `json:"houses,omitempty"`
	BlisterDateHistories []MedicineBlisterDateHistoryView `json:"blisterDateHistories,omitempty"`
}

type MedicineBrand struct {
	ID              uuid.UUID `json:"id"`
	MedicationID    string    `json:"medicationID,omitempty"`
	TradeID         string    `json:"tradeID"`
	TradeName       *string   `json:"tradeName,omitempty"`
	BlisterImageURL *string   `json:"blisterImageURL,omitempty"`
	TabletImageURL  *string   `json:"tabletImageURL,omitempty"`
	BoxImageURL     *string   `json:"boxImageURL,omitempty"`

	// JOIN ONLY
	MedicalName *string `json:"medicalName,omitempty"`
}

type MedicineHouse struct {
	ID           uuid.UUID `json:"id"`
	MedicationID string    `json:"medicationID,omitempty"`
	WarehouseID  string    `json:"warehouseID,omitempty"`
	Locker       string    `json:"locker"`
	Floor        int32     `json:"floor"`
	No           int32     `json:"no"`
	Label        *string   `json:"label,omitempty"`

	// JOIN ONLY
	WarehouseName *string `json:"warehouseName,omitempty"`
	MedicalName   *string `json:"medicalName,omitempty"`
}

type MedicineBlisterDateHistory struct {
	ID                uuid.UUID  `json:"id"`
	MedicationID      *string    `json:"medicationID,omitempty"`
	WarehouseID       string     `json:"warehouseID"`
	BrandID           *uuid.UUID `json:"brandID,omitempty"`
	TradeID           *string    `json:"tradeID,omitempty"`
	BlisterChangeDate time.Time  `json:"blisterChangeDate"`

	// JOIN ONLY
	WarehouseName *string `json:"warehouseName,omitempty"`
	TradeName     *string `json:"tradeName,omitempty"`
}

type MedicineView struct {
	MedicationID string  `json:"medicationID"`
	MedicalName  string  `json:"medicalName,omitempty"`
	WarehouseID  *string `json:"warehouseID,omitempty"`
	Locker       *string `json:"locker,omitempty"`
	Floor        *int32  `json:"floor,omitempty"`
	No           *int32  `json:"no,omitempty"`
	Label        *string `json:"label,omitempty"`
	TradeID      *string `json:"tradeID,omitempty"`
	TradeName    *string `json:"tradeName,omitempty"`
}

type MedicineHouseView struct {
	WarehouseID   string                    `json:"warehouseID"`
	WarehouseName *string                   `json:"warehouseName,omitempty"`
	Addresses     []MedicineHouseDetailView `json:"addresses"`
}

type MedicineHouseDetailView struct {
	ID     uuid.UUID `json:"id"`
	Locker string    `json:"locker"`
	Floor  int32     `json:"floor"`
	No     int32     `json:"no"`
	Label  *string   `json:"label,omitempty"`
}

type MedicineBlisterDateHistoryView struct {
	WarehouseID   string                                `json:"warehouseID"`
	WarehouseName *string                               `json:"warehouseName,omitempty"`
	Brands        []MedicineBrandBlisterDateHistoryView `json:"brands"`
}

type MedicineBrandBlisterDateHistoryView struct {
	TradeID        *string                                     `json:"tradeID,omitempty"`
	TradeName      *string                                     `json:"tradeName,omitempty"`
	BlisterChanges []MedicineBrandBlisterDateDetailHistoryView `json:"blisterChanges"`
}

type MedicineBrandBlisterDateDetailHistoryView struct {
	ID   uuid.UUID `json:"id"`
	Date string    `json:"date"`
}

type FilterMedicine struct {
	Pagination
	WarehouseID string `json:"-" query:"warehouseID"`
}

type FilterMedicineBlisterDateHistory struct {
	Pagination
	WarehouseID string `json:"-" query:"warehouseID"`
}

type ListMedicine struct {
	WarehouseID string
}

type ListMedicineHouse struct {
	Pagination
	WarehouseID string `json:"-" query:"warehouseID" validate:"required"`
}

type FilterMedicineHouse struct {
	ID           uuid.UUID
	MedicationID string
	WarehouseID  string
}

type FilterMedicineBrand struct {
	MedicationID string
	WarehouseID  string
	BrandID      uuid.UUID
}

type FilterMedicineWithBrand struct {
	Pagination
}

type CreateMedicineRequest struct {
	MedicationID string  `param:"medicationID" validate:"required"`
	MedicalName  *string `json:"medicalName,omitempty"`
}

type UpdateMedicineRequest struct {
	MedicationID string  `param:"medicationID" validate:"required"`
	MedicalName  *string `json:"medicalName,omitempty"`
}

type DeleteMedicineFilter struct {
	MedicationID string
	WarehouseID  string
}

type CreateMedicineHouseRequest struct {
	MedicationID string  `json:"medicationID" validate:"required"`
	WarehouseID  string  `json:"warehouseID" validate:"required"`
	Locker       string  `json:"locker" validate:"required"`
	Floor        int32   `json:"floor" validate:"omitempty,min=1"`
	No           int32   `json:"no" validate:"omitempty,min=1"`
	Label        *string `json:"label"`
}

type UpdateMedicineHouseRequest struct {
	ID           uuid.UUID `param:"id" validate:"required,uuid"`
	MedicationID string    `json:"medicationID" validate:"required"`
	Locker       string    `json:"locker" validate:"required"`
	Floor        int32     `json:"floor" validate:"omitempty,min=1"`
	No           int32     `json:"no" validate:"omitempty,min=1"`
	Label        *string   `json:"label"`
}

type DeleteMedicineHouseRequest struct {
	ID uuid.UUID `param:"id" validate:"required,uuid"`
}

type DeleteMedicineHouseFilter struct {
	MedicationID string
	WarehouseID  string
	ID           uuid.UUID
}

type CreateMedicineBrandRequest struct {
	MedicationID     string                `form:"medicationID" validate:"required"`
	TradeID          string                `form:"tradeID" validate:"required"`
	TradeName        *string               `form:"tradeName"`
	BlisterImageFile *multipart.FileHeader `form:"blisterImageFile"`
	TabletImageFile  *multipart.FileHeader `form:"tabletImageFile"`
	BoxImageFile     *multipart.FileHeader `form:"boxImageFile"`
	BlisterImageURL  *string               `form:"-"`
	TabletImageURL   *string               `form:"-"`
	BoxImageURL      *string               `form:"-"`
}

type UpdateMedicineBrandRequest struct {
	BrandID            uuid.UUID             `param:"id" validate:"required,uuid"`
	TradeName          *string               `form:"tradeName"`
	DeleteBlisterImage bool                  `form:"deleteBlisterImage"`
	DeleteTabletImage  bool                  `form:"deleteTabletImage"`
	DeleteBoxImage     bool                  `form:"deleteBoxImage"`
	BlisterImageFile   *multipart.FileHeader `form:"blisterImageFile"`
	TabletImageFile    *multipart.FileHeader `form:"tabletImageFile"`
	BoxImageFile       *multipart.FileHeader `form:"boxImageFile"`
	BlisterImageURL    *string               `form:"-"`
	TabletImageURL     *string               `form:"-"`
	BoxImageURL        *string               `form:"-"`
}

type DeleteMedicineBrandRequest struct {
	ID uuid.UUID `param:"id" validate:"required,uuid"`
}

type DeleteMedicineBrandFilter struct {
	MedicationID string
	TradeID      string
	BrandID      uuid.UUID
}

type FilterMedicineBrandBlisterDateHistory struct {
	BrandID      *uuid.UUID
	MedicationID *string
}

type MedicineBlisterDateHistoryGroup struct {
	MedicationID  string                                      `json:"medicationID"`
	MedicalName   string                                      `json:"medicalName"`
	WarehouseID   string                                      `json:"warehouseID"`
	WarehouseName string                                      `json:"warehouseName"`
	BrandID       *uuid.UUID                                  `json:"brandID,omitempty"`
	TradeID       *string                                     `json:"tradeID,omitempty"`
	TradeName     *string                                     `json:"tradeName,omitempty"`
	Histories     []MedicineBrandBlisterDateDetailHistoryView `json:"histories"`
}

type CreateMedicineBlisterChangeDateHistoryRequest struct {
	MedicationID      string     `json:"medicationID" validate:"required"`
	WarehouseID       string     `json:"warehouseID" validate:"required"`
	BrandID           *uuid.UUID `json:"brandID" validate:"omitempty,uuid"`
	Date              string     `json:"date" validate:"required"`
	BlisterChangeDate time.Time  `json:"-"`
}

type DeleteMedicineBlisterChangeDateHistoryRequest struct {
	HistoryID uuid.UUID `param:"id" validate:"required,uuid"`
}

// TODO: temporary struct for medicine sheet
type MedicineSheet struct {
	MedicationID string `csv:"รหัส" json:"medicationID"`
	LockerName   string `csv:"ตู้" json:"lockerName"`
	Floor        int32  `csv:"ชั้น" json:"floor"`
	No           int32  `csv:"ลำดับที่" json:"no"`
	Address      string `csv:"บ้านเลขที่ยา" json:"address"`
	Description  string `csv:"ชื่อสามัญทางยา" json:"description"`
	MedicalName  string `csv:"ชื่อการค้า" json:"medicalName,omitempty"`
	Label        string `csv:"Label ตะกร้า" json:"label,omitempty"`
}

func (m *MedicineSheet) IsDifferent(medicineReq Medicine, isSyncUniqueByID bool) bool {
	panic("TODO: implement me")
	// medicine := MedicineSheet{
	// 	LockerName:  m.LockerName,
	// 	Floor:       medicineReq.Floor,
	// 	No:          medicineReq.No,
	// 	Address:     medicineReq.Address,
	// 	Description: medicineReq.Description,
	// 	MedicalName: medicineReq.MedicalName,
	// 	Label:       medicineReq.Label,
	// }
	// if isSyncUniqueByID || m.MedicationID != "" {
	// 	medicine.MedicationID = medicineReq.MedicationID
	// }
	// d1, _ := json.Marshal(medicine)
	// d2, _ := json.Marshal(m)
	// return string(d1) != string(d2)
}
