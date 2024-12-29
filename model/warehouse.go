package model

import (
	"time"

	"github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/model"
	"google.golang.org/api/sheets/v4"
)

type WarehouseGroup string

const (
	MyWarehouse           WarehouseGroup = "MY_WAREHOUSE"
	OtherWarehouse        WarehouseGroup = "OTHER_WAREHOUSE"
	OtherWarehousePending WarehouseGroup = "OTHER_WAREHOUSE_PENDING"
)

type Warehouse struct {
	WarehouseID    string     `json:"warehouseID"`
	Name           string     `json:"warehouseName"`
	Role           string     `json:"role"`
	Lockers        []Locker   `json:"lockers"`
	SheetURL       *string    `json:"sheetURL,omitempty"`
	LatestSyncedAt *time.Time `json:"latestSyncedAt,omitempty"`
}

type Locker struct {
	LockerID   string `json:"lockerID"`
	LockerName string `json:"lockerName"`
}

type FilterWarehouseDetail struct {
	Pagination
	Search string               `query:"search"`
	Status model.ApprovalStatus `query:"status" validate:"omitempty,oneof=APPROVED PENDING"`
	Group  WarehouseGroup       `query:"group" validate:"omitempty,oneof=MY_WAREHOUSE OTHER_WAREHOUSE OTHER_WAREHOUSE_PENDING"`
}

type WarehouseDetail struct {
	WarehouseID   string                `json:"warehouseID"`
	Name          string                `json:"warehouseName"`
	Role          *string               `json:"role,omitempty"`
	Status        *model.ApprovalStatus `json:"status,omitempty"`
	LockerDetails []LockerDetail        `json:"lockerDetails"`
	TotalLocker   uint64                `json:"totalLocker"`
	TotalMedicine uint64                `json:"totalMedicine"`
	Users         []WarehouseUser       `json:"users,omitempty"`
}

type LockerDetail struct {
	LockerID      string `json:"lockerID"`
	LockerName    string `json:"lockerName"`
	TotalMedicine uint64 `json:"totalMedicine"`
}

type CreateWarehouseRequest struct {
	WarehouseName string `json:"warehouseName" validate:"required"`
}

type UpdateWarehouseRequest struct {
	WarehouseID   string `param:"warehouseID" validate:"required,uuid"`
	WarehouseName string `json:"warehouseName" validate:"required"`
}

type CreateWarehouseLockerRequest struct {
	WarehouseID string `param:"warehouseID" validate:"required,uuid"`
	LockerName  string `json:"lockerName" validate:"required"`
}

type UpdateWarehouseLockerRequest struct {
	WarehouseID string `param:"warehouseID" validate:"required,uuid"`
	LockerID    string `param:"lockerID" validate:"required,uuid"`
	LockerName  string `json:"lockerName" validate:"required"`
}

type DeleteWarehouseLockerRequest struct {
	WarehouseID string `param:"warehouseID" validate:"required,uuid"`
	LockerID    string `param:"lockerID" validate:"required,uuid"`
}

type FilterWarehouseUser struct {
	Pagination
	WarehouseID string               `param:"warehouseID" validate:"required,uuid"`
	Search      string               `query:"search"`
	Status      model.ApprovalStatus `query:"status" validate:"omitempty,oneof=APPROVED PENDING"`
	Role        model.Role           `query:"role" validate:"omitempty,oneof=ADMIN EDITOR VIEWER"`
}

type CreateWarehouseUserRequest struct {
	WarehouseID string     `param:"warehouseID" validate:"required,uuid"`
	Email       string     `json:"email" validate:"required,email"`
	Role        model.Role `json:"role" validate:"required,oneof=ADMIN EDITOR VIEWER"`
}

type UpdateWarehouseUserRequest struct {
	WarehouseID string     `param:"warehouseID" validate:"required,uuid"`
	UserID      string     `param:"userID" validate:"required,uuid"`
	Role        model.Role `param:"role" validate:"required,oneof=ADMIN EDITOR VIEWER"`
}

type DeleteWarehouseUserRequest struct {
	WarehouseID string `param:"warehouseID" validate:"required,uuid"`
	UserID      string `param:"userID" validate:"required,uuid"`
}

type ApprovalWarehouseUserRequest struct {
	WarehouseID string `param:"warehouseID" validate:"required,uuid"`
	UserID      string `param:"userID" validate:"required,uuid"`
}

type WarehouseRequest struct {
	WarehouseID string `param:"warehouseID" validate:"required,uuid"`
}

type CountWarehouseUserStatus struct {
	TotalApproved uint64 `json:"totalApproved"`
	TotalPending  uint64 `json:"totalPending"`
}

type WarehouseUser struct {
	User
	Role   model.Role           `json:"role"`
	Status model.ApprovalStatus `json:"status"`
}

type DeleteLockerFilter struct {
	LockerID    string
	WarehouseID string
}

type GetSyncMedicineMetadataRequest struct {
	WarehouseID string `param:"warehouseID" validate:"required,uuid"`
	URL         string `query:"url" validate:"required,url"`
}

type SyncMedicineRequest struct {
	WarehouseID string `param:"warehouseID" validate:"required,uuid"`
	URL         string `json:"url" validate:"required,url"`
}

type SyncMedicineMetadata struct {
	Title                string `json:"title"`
	SheetName            string `json:"sheetName"`
	TotalMedicine        uint64 `json:"totalMedicine"`
	TotalNewMedicine     uint64 `json:"totalNewMedicine"`
	TotalUpdatedMedicine uint64 `json:"totalUpdatedMedicine"`
	TotalSkippedMedicine uint64 `json:"totalSkippedMedicine"`
}

type GoogleSheetData struct {
	Sheet            *sheets.Sheet
	SpreadsheetTitle string
	SpreadsheetID    string
	LockerID         map[string]string
	MedicineSheets   []MedicineSheet
	MedicineData     map[string]Medicine
}

type GetWarehouseUsersResponse struct {
	PagingWithMetadata[WarehouseUser]
	CountWarehouseUserStatus
}
