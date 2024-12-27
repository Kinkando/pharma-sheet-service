package model

import (
	"time"

	"github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/model"
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
	Search      string `json:"-" query:"search"`
	MyWarehouse bool   `query:"myWarehouse"`
}

type WarehouseDetail struct {
	WarehouseID   string               `json:"warehouseID"`
	Name          string               `json:"warehouseName"`
	Role          string               `json:"role"`
	Status        model.ApprovalStatus `json:"status"`
	LockerDetails []LockerDetail       `json:"lockerDetails"`
	TotalLocker   uint64               `json:"totalLocker"`
	TotalMedicine uint64               `json:"totalMedicine"`
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

type GetWarehouseRequest struct {
	WarehouseID string `param:"warehouseID" validate:"required,uuid"`
}

type DeleteWarehouseRequest struct {
	WarehouseID string `param:"warehouseID" validate:"required,uuid"`
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

type JoinWarehouseRequest struct {
	WarehouseID string `param:"warehouseID" validate:"required,uuid"`
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

type SyncMedicineRequest struct {
	WarehouseID string `param:"warehouseID" validate:"required,uuid"`
	URL         string `json:"url" validate:"required,url"`
}
