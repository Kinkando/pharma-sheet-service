package service

import (
	"context"

	"github.com/kinkando/pharma-sheet-service/model"
	"github.com/kinkando/pharma-sheet-service/repository"
)

type Warehouse interface {
	GetWarehouses(ctx context.Context) ([]model.Warehouse, error)
	CreateWarehouse(ctx context.Context, req model.Warehouse) (string, error)
}

type warehouse struct {
	warehouseRepository repository.Warehouse
}

func NewWarehouseService(
	warehouseRepository repository.Warehouse,
) Warehouse {
	return &warehouse{
		warehouseRepository: warehouseRepository,
	}
}

func (s *warehouse) GetWarehouses(ctx context.Context) ([]model.Warehouse, error) {
	return s.warehouseRepository.GetWarehouses(ctx)
}

func (s *warehouse) CreateWarehouse(ctx context.Context, req model.Warehouse) (string, error) {
	return s.warehouseRepository.CreateWarehouse(ctx, req)
}
