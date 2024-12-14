package repository

import (
	"context"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	genmodel "github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/model"
	"github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/table"
	"github.com/kinkando/pharma-sheet-service/model"
	"github.com/kinkando/pharma-sheet-service/pkg/database/postgresql"
	"github.com/kinkando/pharma-sheet-service/pkg/generator"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"github.com/kinkando/pharma-sheet-service/pkg/profile"
)

type Warehouse interface {
	GetWarehouses(ctx context.Context) ([]model.Warehouse, error)
	CreateWarehouse(ctx context.Context, req model.Warehouse) (string, error)
}

type warehouse struct {
	pgPool *pgxpool.Pool
}

func NewWarehouseRepository(pgPool *pgxpool.Pool) Warehouse {
	return &warehouse{pgPool: pgPool}
}

func (r *warehouse) GetWarehouses(ctx context.Context) (warehouses []model.Warehouse, err error) {
	userProfile, err := profile.UseProfile(ctx)
	if err != nil {
		return nil, err
	}

	query, args := table.Warehouses.
		INNER_JOIN(table.WarehouseUsers, table.Warehouses.WarehouseID.EQ(table.WarehouseUsers.WarehouseID)).
		SELECT(table.Warehouses.WarehouseID, table.Warehouses.Name, table.WarehouseUsers.Role).
		WHERE(table.WarehouseUsers.UserID.EQ(postgres.UUID(uuid.MustParse(userProfile.UserID)))).
		ORDER_BY(table.Warehouses.Name.ASC()).
		Sql()

	rows, err := r.pgPool.Query(ctx, query, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var warehouse model.Warehouse
		err = rows.Scan(&warehouse.WarehouseID, &warehouse.Name, &warehouse.Role)
		if err != nil {
			logger.Context(ctx).Error(err)
			return nil, err
		}
		warehouses = append(warehouses, warehouse)
	}

	return warehouses, nil
}

func (r *warehouse) CreateWarehouse(ctx context.Context, req model.Warehouse) (warehouseID string, err error) {
	userProfile, err := profile.UseProfile(ctx)
	if err != nil {
		return
	}

	warehouseID = generator.UUID()
	err = postgresql.Commit(ctx, r.pgPool, func(ctx context.Context, tx pgx.Tx) error {
		warehouseData := genmodel.Warehouses{
			WarehouseID: uuid.MustParse(warehouseID),
			Name:        req.Name,
			CreatedAt:   time.Now(),
		}

		warehouseUserData := genmodel.WarehouseUsers{
			WarehouseID: uuid.MustParse(warehouseID),
			Role:        genmodel.Role_Admin,
			UserID:      uuid.MustParse(userProfile.UserID),
			CreatedAt:   time.Now(),
		}

		sql, args := table.Warehouses.
			INSERT(table.Warehouses.WarehouseID, table.Warehouses.Name, table.Warehouses.CreatedAt).
			MODEL(warehouseData).
			Sql()
		_, err = tx.Exec(ctx, sql, args...)
		if err != nil {
			logger.Context(ctx).Error(err)
			return err
		}

		sql, args = table.WarehouseUsers.
			INSERT(table.WarehouseUsers.WarehouseID, table.WarehouseUsers.UserID, table.WarehouseUsers.Role, table.Warehouses.CreatedAt).
			MODEL(warehouseUserData).
			Sql()
		_, err = tx.Exec(ctx, sql, args...)
		if err != nil {
			logger.Context(ctx).Error(err)
			return err
		}

		return nil
	})
	if err != nil {
		return
	}

	return warehouseID, nil
}
