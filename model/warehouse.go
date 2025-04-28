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
	WarehouseID                         string     `json:"warehouseID"`
	Name                                string     `json:"warehouseName"`
	Role                                string     `json:"role"`
	SheetURL                            *string    `json:"sheetURL,omitempty"`
	MedicineSheetName                   *string    `json:"medicineSheetName,omitempty"`
	MedicineHouseSheetName              *string    `json:"medicineHouseSheetName,omitempty"`
	MedicineBrandSheetName              *string    `json:"medicineBrandSheetName,omitempty"`
	MedicineBlisterDateHistorySheetName *string    `json:"medicineBlisterDateHistorySheetName,omitempty"`
	LatestSyncedAt                      *time.Time `json:"latestSyncedAt,omitempty"`
}

type FilterWarehouseDetail struct {
	Pagination
	Status model.PharmaSheetApprovalStatus `query:"status" validate:"omitempty,oneof=APPROVED PENDING"`
	Group  WarehouseGroup                  `query:"group" validate:"omitempty,oneof=MY_WAREHOUSE OTHER_WAREHOUSE OTHER_WAREHOUSE_PENDING"`
}

type WarehouseDetail struct {
	WarehouseID   string                           `json:"warehouseID"`
	Name          string                           `json:"warehouseName"`
	Role          *string                          `json:"role,omitempty"`
	Status        *model.PharmaSheetApprovalStatus `json:"status,omitempty"`
	TotalMedicine uint64                           `json:"totalMedicine"`
	Users         []WarehouseUser                  `json:"users,omitempty"`
}

type CreateWarehouseRequest struct {
	WarehouseID   string `param:"warehouseID" validate:"required"`
	WarehouseName string `json:"warehouseName" validate:"required"`
}

type UpdateWarehouseRequest struct {
	WarehouseID   string `param:"warehouseID" validate:"required"`
	WarehouseName string `json:"warehouseName" validate:"required"`
}

type FilterWarehouseUser struct {
	Pagination
	WarehouseID string                          `param:"warehouseID" validate:"required"`
	Status      model.PharmaSheetApprovalStatus `query:"status" validate:"omitempty,oneof=APPROVED PENDING"`
	Role        model.PharmaSheetRole           `query:"role" validate:"omitempty,oneof=ADMIN EDITOR VIEWER"`
}

type CreateWarehouseUserRequest struct {
	WarehouseID string                `param:"warehouseID" validate:"required"`
	Email       string                `json:"email" validate:"required,email"`
	Role        model.PharmaSheetRole `json:"role" validate:"required,oneof=ADMIN EDITOR VIEWER"`
}

type UpdateWarehouseUserRequest struct {
	WarehouseID string                `param:"warehouseID" validate:"required"`
	UserID      string                `param:"userID" validate:"required,uuid"`
	Role        model.PharmaSheetRole `param:"role" validate:"required,oneof=ADMIN EDITOR VIEWER"`
}

type DeleteWarehouseUserRequest struct {
	WarehouseID string `param:"warehouseID" validate:"required"`
	UserID      string `param:"userID" validate:"required,uuid"`
}

type ApprovalWarehouseUserRequest struct {
	WarehouseID string `param:"warehouseID" validate:"required"`
	UserID      string `param:"userID" validate:"required,uuid"`
}

type WarehouseRequest struct {
	WarehouseID string `param:"warehouseID" validate:"required"`
}

type CountWarehouseUserStatus struct {
	TotalApproved uint64 `json:"totalApproved"`
	TotalPending  uint64 `json:"totalPending"`
}

type WarehouseUser struct {
	User
	Role   model.PharmaSheetRole           `json:"role"`
	Status model.PharmaSheetApprovalStatus `json:"status"`
}

type GetSyncMedicineMetadataRequest struct {
	WarehouseID string `param:"warehouseID" validate:"required"`
	URL         string `query:"url" validate:"required,url"`
}

type SyncMedicineRequest struct {
	WarehouseID string `param:"warehouseID" validate:"required"`
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
