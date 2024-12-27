package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/enum"
	genmodel "github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/model"
	"github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/table"
	"github.com/kinkando/pharma-sheet-service/model"
	"github.com/kinkando/pharma-sheet-service/pkg/database/postgresql"
	"github.com/kinkando/pharma-sheet-service/pkg/generator"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"github.com/kinkando/pharma-sheet-service/pkg/profile"
)

type Warehouse interface {
	GetWarehouse(ctx context.Context, warehouseID string) (model.Warehouse, error)
	GetWarehouses(ctx context.Context) ([]model.Warehouse, error)
	GetWarehouseDetails(ctx context.Context, filter model.FilterWarehouseDetail) (data []model.WarehouseDetail, total uint64, err error)
	GetWarehouseRole(ctx context.Context, warehouseID, userID string) (genmodel.Role, error)
	CreateWarehouse(ctx context.Context, req model.Warehouse) (string, error)
	UpdateWarehouse(ctx context.Context, req model.Warehouse) error
	DeleteWarehouse(ctx context.Context, warehouseID string) error

	GetWarehouseUsers(ctx context.Context, warehouseID string, filter model.FilterWarehouseUser) (data []model.WarehouseUser, total uint64, err error)
	GetWarehouseUserStatus(ctx context.Context, warehouseID, userID string) (genmodel.ApprovalStatus, error)
	CreateWarehouseUser(ctx context.Context, warehouseID, userID string, role genmodel.Role, status genmodel.ApprovalStatus) error
	UpdateWarehouseUser(ctx context.Context, warehouseUser genmodel.WarehouseUsers) error
	DeleteWarehouseUser(ctx context.Context, warehouseID string, userID *string) error

	UpsertWarehouseSheet(ctx context.Context, warehouseSheet genmodel.WarehouseSheets) error
}

type warehouse struct {
	pgPool *pgxpool.Pool
}

func NewWarehouseRepository(pgPool *pgxpool.Pool) Warehouse {
	return &warehouse{pgPool: pgPool}
}

func (r *warehouse) GetWarehouse(ctx context.Context, warehouseID string) (model.Warehouse, error) {
	query, args := table.Warehouses.
		INNER_JOIN(table.WarehouseUsers, table.Warehouses.WarehouseID.EQ(table.WarehouseUsers.WarehouseID)).
		LEFT_JOIN(table.WarehouseSheets, table.Warehouses.WarehouseID.EQ(table.WarehouseSheets.WarehouseID)).
		SELECT(
			table.Warehouses.WarehouseID,
			table.Warehouses.Name,
			table.WarehouseUsers.Role,
			table.WarehouseSheets.SpreadsheetID,
			table.WarehouseSheets.SheetID,
			table.WarehouseSheets.LatestSyncedAt,
		).
		WHERE(table.Warehouses.WarehouseID.EQ(postgres.UUID(uuid.MustParse(warehouseID)))).
		Sql()

	var warehouse model.Warehouse
	var spreadsheetID *string
	var sheetID *int32

	err := r.pgPool.
		QueryRow(ctx, query, args...).
		Scan(
			&warehouse.WarehouseID,
			&warehouse.Name,
			&warehouse.Role,
			&spreadsheetID,
			&sheetID,
			&warehouse.LatestSyncedAt,
		)
	if err != nil {
		logger.Context(ctx).Error(err)
		return model.Warehouse{}, err
	}

	if sheetID != nil && spreadsheetID != nil {
		sheetURL := fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/edit#gid=%d", *spreadsheetID, *sheetID)
		warehouse.SheetURL = &sheetURL
	}

	return warehouse, nil
}

func (r *warehouse) GetWarehouses(ctx context.Context) (warehouses []model.Warehouse, err error) {
	userProfile, err := profile.UseProfile(ctx)
	if err != nil {
		return nil, err
	}

	query, args := table.Warehouses.
		INNER_JOIN(table.WarehouseUsers, table.Warehouses.WarehouseID.EQ(table.WarehouseUsers.WarehouseID)).
		LEFT_JOIN(table.WarehouseSheets, table.Warehouses.WarehouseID.EQ(table.WarehouseSheets.WarehouseID)).
		SELECT(
			table.Warehouses.WarehouseID,
			table.Warehouses.Name,
			table.WarehouseUsers.Role,
			table.WarehouseSheets.SpreadsheetID,
			table.WarehouseSheets.SheetID,
			table.WarehouseSheets.LatestSyncedAt,
		).
		WHERE(table.WarehouseUsers.UserID.EQ(postgres.UUID(uuid.MustParse(userProfile.UserID))).AND(table.WarehouseUsers.Status.EQ(enum.ApprovalStatus.Approved))).
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
		var spreadsheetID *string
		var sheetID *int32

		err = rows.Scan(
			&warehouse.WarehouseID,
			&warehouse.Name,
			&warehouse.Role,
			&spreadsheetID,
			&sheetID,
			&warehouse.LatestSyncedAt,
		)
		if err != nil {
			logger.Context(ctx).Error(err)
			return nil, err
		}

		if sheetID != nil && spreadsheetID != nil {
			sheetURL := fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/edit#gid=%d", *spreadsheetID, *sheetID)
			warehouse.SheetURL = &sheetURL
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

	condition := postgres.Bool(true)
	if filter.MyWarehouse {
		condition = condition.AND(table.WarehouseUsers.UserID.EQ(postgres.UUID(uuid.MustParse(userProfile.UserID))).AND(table.WarehouseUsers.Status.EQ(enum.ApprovalStatus.Approved)))
	}

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
		SELECT(table.Warehouses.WarehouseID, table.Warehouses.Name, table.WarehouseUsers.Role, table.WarehouseUsers.Status).
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
		err = rows.Scan(&warehouse.WarehouseID, &warehouse.Name, &warehouse.Role, &warehouse.Status)
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
			Status:      genmodel.ApprovalStatus_Approved,
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
			INSERT(table.WarehouseUsers.WarehouseID, table.WarehouseUsers.UserID, table.WarehouseUsers.Role, table.WarehouseUsers.Status, table.Warehouses.CreatedAt).
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
		WHERE(
			table.WarehouseUsers.UserID.EQ(postgres.UUID(uuid.MustParse(userID))).AND(
				table.WarehouseUsers.Status.EQ(enum.ApprovalStatus.Approved)).AND(
				table.WarehouseUsers.WarehouseID.EQ(postgres.UUID(uuid.MustParse(warehouseID)))),
		).
		Sql()

	err = r.pgPool.QueryRow(ctx, query, args...).Scan(&role)
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}

	return role, nil
}

func (r *warehouse) GetWarehouseUsers(ctx context.Context, warehouseID string, filter model.FilterWarehouseUser) (warehouseUsers []model.WarehouseUser, total uint64, err error) {
	condition := table.WarehouseUsers.WarehouseID.EQ(postgres.UUID(uuid.MustParse(warehouseID)))

	if filter.Role != "" {
		condition = condition.AND(table.WarehouseUsers.Role.EQ(postgres.NewEnumValue(string(filter.Role))))
	}
	if filter.Status != "" {
		condition = condition.AND(table.WarehouseUsers.Status.EQ(postgres.NewEnumValue(string(filter.Status))))
	}
	if strings.TrimSpace(filter.Search) != "" {
		search := postgres.String("%" + strings.ToLower(filter.Search) + "%")
		condition = condition.AND(
			postgres.OR(
				postgres.LOWER(table.Users.DisplayName).LIKE(search),
				postgres.LOWER(table.Users.Email).LIKE(search),
			),
		)
	}

	query, args := table.WarehouseUsers.
		INNER_JOIN(table.Users, table.WarehouseUsers.UserID.EQ(table.Users.UserID)).
		SELECT(postgres.COUNT(postgres.STAR)).
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

	query, args = table.WarehouseUsers.
		INNER_JOIN(table.Users, table.WarehouseUsers.UserID.EQ(table.Users.UserID)).
		SELECT(
			table.WarehouseUsers.UserID,
			table.WarehouseUsers.Role,
			table.Users.FirebaseUID,
			table.Users.Email,
			table.Users.DisplayName,
			table.Users.ImageURL,
			table.WarehouseUsers.Status,
		).
		WHERE(condition).
		LIMIT(int64(filter.Limit)).
		OFFSET(int64(filter.Offset)).
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
			&warehouseUser.Status,
		)
		if err != nil {
			logger.Context(ctx).Error(err)
			return
		}
		warehouseUsers = append(warehouseUsers, warehouseUser)
	}

	return warehouseUsers, total, nil
}

func (r *warehouse) GetWarehouseUserStatus(ctx context.Context, warehouseID, userID string) (status genmodel.ApprovalStatus, err error) {
	query, args := table.WarehouseUsers.
		SELECT(table.WarehouseUsers.Status).
		WHERE(table.WarehouseUsers.UserID.EQ(postgres.UUID(uuid.MustParse(userID))).AND(table.WarehouseUsers.WarehouseID.EQ(postgres.UUID(uuid.MustParse(warehouseID))))).
		Sql()

	err = r.pgPool.QueryRow(ctx, query, args...).Scan(&status)
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}

	return status, nil
}

func (r *warehouse) CreateWarehouseUser(ctx context.Context, warehouseID, userID string, role genmodel.Role, status genmodel.ApprovalStatus) error {
	warehouseUsers := table.WarehouseUsers

	warehouse := genmodel.WarehouseUsers{
		WarehouseID: uuid.MustParse(warehouseID),
		UserID:      uuid.MustParse(userID),
		Role:        role,
		Status:      status,
		CreatedAt:   time.Now(),
	}

	sql, args := warehouseUsers.
		INSERT(warehouseUsers.WarehouseID, warehouseUsers.UserID, warehouseUsers.Role, warehouseUsers.Status, warehouseUsers.CreatedAt).
		MODEL(warehouse).
		Sql()
	_, err := r.pgPool.Exec(ctx, sql, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return nil
}

func (r *warehouse) UpdateWarehouseUser(ctx context.Context, warehouseUser genmodel.WarehouseUsers) error {
	warehouseUsers := table.WarehouseUsers

	if warehouseUser.Role == "" && warehouseUser.Status == "" {
		return errors.New("no specific column is update")
	}

	columnNames := postgres.ColumnList{warehouseUsers.UpdatedAt}
	columnValues := []any{postgres.TimestampzT(time.Now())}

	if warehouseUser.Role != "" {
		columnNames = append(columnNames, warehouseUsers.Role)
		columnValues = append(columnValues, warehouseUser.Role)
	}
	if warehouseUser.Status != "" {
		columnNames = append(columnNames, warehouseUsers.Status)
		columnValues = append(columnValues, warehouseUser.Status)
	}

	stmt, args := warehouseUsers.
		UPDATE(columnNames).
		SET(columnValues[0], columnValues[1:]...).
		WHERE(warehouseUsers.WarehouseID.EQ(postgres.UUID(warehouseUser.WarehouseID)).AND(warehouseUsers.UserID.EQ(postgres.UUID(warehouseUser.UserID)))).
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

func (r *warehouse) UpsertWarehouseSheet(ctx context.Context, warehouseSheet genmodel.WarehouseSheets) error {
	warehouseSheet.LatestSyncedAt = time.Now()
	warehouseSheet.CreatedAt = time.Now()

	stmt, args := table.WarehouseSheets.
		INSERT(table.WarehouseSheets.WarehouseID, table.WarehouseSheets.SpreadsheetID, table.WarehouseSheets.SheetID, table.WarehouseSheets.LatestSyncedAt, table.WarehouseSheets.CreatedAt).
		MODEL(warehouseSheet).
		ON_CONFLICT(table.WarehouseSheets.WarehouseID).
		DO_UPDATE(postgres.SET(
			table.WarehouseSheets.SpreadsheetID.SET(postgres.String(warehouseSheet.SpreadsheetID)),
			table.WarehouseSheets.SheetID.SET(postgres.Int32(warehouseSheet.SheetID)),
			table.WarehouseSheets.LatestSyncedAt.SET(postgres.TimestampzT(time.Now())),
		)).
		Sql()
	_, err := r.pgPool.Exec(ctx, stmt, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return nil
}
