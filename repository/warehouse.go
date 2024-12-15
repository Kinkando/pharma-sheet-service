package repository

import (
	"context"
	"database/sql"
	"strings"
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
	GetWarehouseDetails(ctx context.Context, filter model.FilterWarehouseDetail) (data []model.WarehouseDetail, total uint64, err error)
	GetWarehouseRole(ctx context.Context, warehouseID, userID string) (genmodel.Role, error)
	CreateWarehouse(ctx context.Context, req model.Warehouse) (string, error)
	UpdateWarehouse(ctx context.Context, req model.Warehouse) error
	DeleteWarehouse(ctx context.Context, warehouseID string) error

	GetWarehouseUsers(ctx context.Context, warehouseID string) ([]model.WarehouseUser, error)
	CreateWarehouseUser(ctx context.Context, warehouseID, userID string, role genmodel.Role) error
	UpdateWarehouseUser(ctx context.Context, warehouseID, userID string, role genmodel.Role) error
	DeleteWarehouseUser(ctx context.Context, warehouseID string, userID *string) error
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

func (r *warehouse) GetWarehouseDetails(ctx context.Context, filter model.FilterWarehouseDetail) (data []model.WarehouseDetail, total uint64, err error) {
	userProfile, err := profile.UseProfile(ctx)
	if err != nil {
		return
	}

	condition := table.WarehouseUsers.UserID.EQ(postgres.UUID(uuid.MustParse(userProfile.UserID)))
	if filter.Search != "" {
		search := postgres.String("%" + strings.ToLower(filter.Search) + "%")
		condition = condition.AND(postgres.LOWER(table.Warehouses.Name).LIKE(search))
	}

	query, args := table.Warehouses.
		INNER_JOIN(table.WarehouseUsers, table.Warehouses.WarehouseID.EQ(table.WarehouseUsers.WarehouseID)).
		SELECT(postgres.COUNT(table.Warehouses.WarehouseID)).
		WHERE(condition).
		Sql()
	err = r.pgPool.QueryRow(ctx, query, args...).Scan(&total)
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}

	if total == 0 {
		return
	}

	query, args = table.Warehouses.
		INNER_JOIN(table.WarehouseUsers, table.Warehouses.WarehouseID.EQ(table.WarehouseUsers.WarehouseID)).
		SELECT(table.Warehouses.WarehouseID, table.Warehouses.Name, table.WarehouseUsers.Role).
		WHERE(condition).
		LIMIT(int64(filter.Limit)).
		OFFSET(int64(filter.Offset)).
		ORDER_BY(table.Warehouses.Name.ASC()).
		Sql()

	rows, err := r.pgPool.Query(ctx, query, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var warehouse model.WarehouseDetail
		err = rows.Scan(&warehouse.WarehouseID, &warehouse.Name, &warehouse.Role)
		if err != nil {
			logger.Context(ctx).Error(err)
			return nil, 0, err
		}
		data = append(data, warehouse)
	}

	return data, total, nil
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

func (r *warehouse) UpdateWarehouse(ctx context.Context, req model.Warehouse) error {
	warehouses := table.Warehouses

	now := time.Now()
	warehouse := genmodel.Warehouses{
		WarehouseID: uuid.MustParse(req.WarehouseID),
		Name:        req.Name,
		UpdatedAt:   &now,
	}

	sql, args := warehouses.
		UPDATE(warehouses.Name, warehouses.UpdatedAt).
		WHERE(warehouses.WarehouseID.EQ(postgres.UUID(warehouse.WarehouseID))).
		MODEL(warehouse).
		Sql()
	_, err := r.pgPool.Exec(ctx, sql, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return nil
}

func (r *warehouse) DeleteWarehouse(ctx context.Context, warehouseID string) error {
	stmt, args := table.Warehouses.DELETE().WHERE(table.Warehouses.WarehouseID.EQ(postgres.UUID(uuid.MustParse(warehouseID)))).Sql()
	result, err := r.pgPool.Exec(ctx, stmt, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}
	if result.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *warehouse) GetWarehouseRole(ctx context.Context, warehouseID, userID string) (role genmodel.Role, err error) {
	query, args := table.WarehouseUsers.
		SELECT(table.WarehouseUsers.Role).
		WHERE(table.WarehouseUsers.UserID.EQ(postgres.UUID(uuid.MustParse(userID)))).
		Sql()

	err = r.pgPool.QueryRow(ctx, query, args...).Scan(&role)
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}

	return role, nil
}

func (r *warehouse) GetWarehouseUsers(ctx context.Context, warehouseID string) (warehouseUsers []model.WarehouseUser, err error) {
	query, args := table.WarehouseUsers.
		INNER_JOIN(table.Users, table.WarehouseUsers.UserID.EQ(table.Users.UserID)).
		SELECT(
			table.WarehouseUsers.UserID,
			table.WarehouseUsers.Role,
			table.Users.FirebaseUID,
			table.Users.Email,
			table.Users.DisplayName,
			table.Users.ImageURL,
		).
		WHERE(table.WarehouseUsers.WarehouseID.EQ(postgres.UUID(uuid.MustParse(warehouseID)))).
		ORDER_BY(table.Users.Email.ASC()).
		Sql()

	rows, err := r.pgPool.Query(ctx, query, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var warehouseUser model.WarehouseUser
		err = rows.Scan(
			&warehouseUser.UserID,
			&warehouseUser.Role,
			&warehouseUser.FirebaseUID,
			&warehouseUser.Email,
			&warehouseUser.DisplayName,
			&warehouseUser.ImageURL,
		)
		if err != nil {
			logger.Context(ctx).Error(err)
			return nil, err
		}
		warehouseUsers = append(warehouseUsers, warehouseUser)
	}

	return warehouseUsers, nil
}

func (r *warehouse) CreateWarehouseUser(ctx context.Context, warehouseID, userID string, role genmodel.Role) error {
	warehouseUsers := table.WarehouseUsers

	warehouse := genmodel.WarehouseUsers{
		WarehouseID: uuid.MustParse(warehouseID),
		UserID:      uuid.MustParse(userID),
		Role:        role,
		CreatedAt:   time.Now(),
	}

	sql, args := warehouseUsers.
		INSERT(warehouseUsers.WarehouseID, warehouseUsers.UserID, warehouseUsers.Role, warehouseUsers.CreatedAt).
		MODEL(warehouse).
		Sql()
	_, err := r.pgPool.Exec(ctx, sql, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return nil
}

func (r *warehouse) UpdateWarehouseUser(ctx context.Context, warehouseID, userID string, role genmodel.Role) error {
	warehouseUsers := table.WarehouseUsers

	now := time.Now()
	warehouse := genmodel.WarehouseUsers{
		WarehouseID: uuid.MustParse(warehouseID),
		UserID:      uuid.MustParse(userID),
		Role:        role,
		UpdatedAt:   &now,
	}

	stmt, args := warehouseUsers.
		UPDATE(warehouseUsers.Role, warehouseUsers.UpdatedAt).
		WHERE(warehouseUsers.WarehouseID.EQ(postgres.UUID(warehouse.WarehouseID)).AND(warehouseUsers.UserID.EQ(postgres.UUID(warehouse.UserID)))).
		MODEL(warehouse).
		Sql()
	result, err := r.pgPool.Exec(ctx, stmt, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	if result.RowsAffected() == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *warehouse) DeleteWarehouseUser(ctx context.Context, warehouseID string, userID *string) error {
	warehouseUsers := table.WarehouseUsers
	condition := warehouseUsers.WarehouseID.EQ(postgres.UUID(uuid.MustParse(warehouseID)))
	if userID != nil {
		condition = condition.AND(warehouseUsers.UserID.EQ(postgres.UUID(uuid.MustParse(*userID))))
	}
	stmt, args := table.WarehouseUsers.DELETE().WHERE(condition).Sql()
	result, err := r.pgPool.Exec(ctx, stmt, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	if result.RowsAffected() == 0 {
		return sql.ErrNoRows
	}

	return nil
}
