package model

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
