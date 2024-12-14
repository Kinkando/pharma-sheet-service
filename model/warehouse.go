package model

type Warehouse struct {
	WarehouseID string `json:"warehouseID"`
	Name        string `json:"name"`
	Role        string `json:"role"`
}

type CreateWarehouseRequest struct {
	WarehouseName string `json:"name" validate:"required"`
}
