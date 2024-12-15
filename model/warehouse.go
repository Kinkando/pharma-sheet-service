package model

import "github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/model"

type Warehouse struct {
	WarehouseID string   `json:"warehouseID"`
	Name        string   `json:"warehouseName"`
	Role        string   `json:"role"`
	Lockers     []Locker `json:"lockers,omitempty"`
}

type Locker struct {
	LockerID   string `json:"lockerID"`
	LockerName string `json:"lockerName"`
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

type GetWarehouseUserRequest struct {
	WarehouseID string `param:"warehouseID" validate:"required,uuid"`
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

type WarehouseUser struct {
	User
	Role model.Role `json:"role"`
}
