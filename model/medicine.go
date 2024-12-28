package model

import (
	"encoding/json"
	"mime/multipart"
)

type Medicine struct {
	MedicineID  string  `json:"medicineID"`
	WarehouseID string  `json:"warehouseID"`
	LockerID    string  `json:"lockerID"`
	LockerName  string  `json:"lockerName"`
	Floor       int32   `json:"floor"`
	No          int32   `json:"no"`
	Address     string  `json:"address"`
	Description string  `json:"description"`
	MedicalName string  `json:"medicalName,omitempty"`
	Label       string  `json:"label,omitempty"`
	ImageURL    *string `json:"imageURL,omitempty"`
}

type FilterMedicine struct {
	Pagination
	WarehouseID string `query:"warehouseID" validate:"required,uuid"`
	Search      string `json:"-" query:"search"`
}

type ListMedicine struct {
	WarehouseID string
	LockerID    string
}

type CreateMedicineRequest struct {
	WarehouseID string `form:"warehouseID" validate:"required"`
	LockerID    string `form:"lockerID" validate:"required,uuid"`
	Floor       int32  `form:"floor" validate:"omitempty,min=1"`
	No          int32  `form:"no" validate:"omitempty,min=1"`
	Address     string `form:"address" validate:"required"`
	Description string `form:"description" validate:"required"`
	MedicalName string `form:"medicalName" validate:"required"`
	Label       string `form:"label" validate:"required"`
	File        *multipart.FileHeader
	ImageURL    *string
}

type UpdateMedicineRequest struct {
	MedicineID  string `param:"medicineID" validate:"required"`
	LockerID    string `form:"lockerID" validate:"required,uuid"`
	Floor       int32  `form:"floor" validate:"omitempty,min=1"`
	No          int32  `form:"no" validate:"omitempty,min=1"`
	Address     string `form:"address" validate:"required"`
	Description string `form:"description" validate:"required"`
	MedicalName string `form:"medicalName" validate:"required"`
	Label       string `form:"label" validate:"required"`
	DeleteImage bool   `form:"deleteImage"`
	File        *multipart.FileHeader
	ImageURL    *string
}

type DeleteMedicineFilter struct {
	MedicineID  string
	LockerID    string
	WarehouseID string
}

type MedicineSheet struct {
	MedicineID  string `csv:"รหัส" json:"medicineID"`
	LockerName  string `csv:"ตู้" json:"lockerName"`
	Floor       int32  `csv:"ชั้น" json:"floor"`
	No          int32  `csv:"ลำดับที่" json:"no"`
	Address     string `csv:"บ้านเลขที่ยา" json:"address"`
	Description string `csv:"ชื่อสามัญทางยา" json:"description"`
	MedicalName string `csv:"ชื่อการค้า" json:"medicalName,omitempty"`
	Label       string `csv:"Label ตะกร้า" json:"label,omitempty"`
}

func (m *MedicineSheet) IsDifferent(medicineReq Medicine) bool {
	medicine := MedicineSheet{
		LockerName:  m.LockerName,
		Floor:       medicineReq.Floor,
		No:          medicineReq.No,
		Address:     medicineReq.Address,
		Description: medicineReq.Description,
		MedicalName: medicineReq.MedicalName,
		Label:       medicineReq.Label,
	}
	d1, _ := json.Marshal(medicine)
	d2, _ := json.Marshal(m)
	return string(d1) != string(d2)
}
