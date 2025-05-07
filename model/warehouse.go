package model

import (
	"time"

	"github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/model"
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
	MedicineSheetID                     *int32     `json:"-"`
	MedicineSheetName                   *string    `json:"medicineSheetName,omitempty"`
	MedicineBrandSheetID                *int32     `json:"-"`
	MedicineHouseSheetName              *string    `json:"medicineHouseSheetName,omitempty"`
	MedicineHouseSheetID                *int32     `json:"-"`
	MedicineBrandSheetName              *string    `json:"medicineBrandSheetName,omitempty"`
	MedicineBlisterDateHistorySheetID   *int32     `json:"-"`
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

type GetWarehouseUsersResponse struct {
	PagingWithMetadata[WarehouseUser]
	CountWarehouseUserStatus
}
