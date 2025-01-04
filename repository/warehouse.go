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
	"github.com/kinkando/pharma-sheet-service/.gen/postgresql_kinkando/public/enum"
	genmodel "github.com/kinkando/pharma-sheet-service/.gen/postgresql_kinkando/public/model"
	"github.com/kinkando/pharma-sheet-service/.gen/postgresql_kinkando/public/table"
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
	GetWarehouseRole(ctx context.Context, warehouseID, userID string) (genmodel.PharmaSheetRole, error)
	CreateWarehouse(ctx context.Context, req model.Warehouse) (string, error)
	UpdateWarehouse(ctx context.Context, req model.Warehouse) error
	DeleteWarehouse(ctx context.Context, warehouseID string) error

	CountWarehouseUserStatus(ctx context.Context, warehouseID string) (model.CountWarehouseUserStatus, error)
	GetWarehouseUsers(ctx context.Context, warehouseID string, filter model.FilterWarehouseUser) (data []model.WarehouseUser, total uint64, err error)
	GetWarehouseUserStatus(ctx context.Context, warehouseID, userID string) (genmodel.PharmaSheetApprovalStatus, error)
	CreateWarehouseUser(ctx context.Context, warehouseID, userID string, role genmodel.PharmaSheetRole, status genmodel.PharmaSheetApprovalStatus) error
	UpdateWarehouseUser(ctx context.Context, warehouseUser genmodel.PharmaSheetWarehouseUsers) error
	DeleteWarehouseUser(ctx context.Context, warehouseID string, userID *string) error

	CheckConflictWarehouseSheet(ctx context.Context, warehouseID string, spreadsheetID string, sheetID int32) (bool, error)
	UpsertWarehouseSheet(ctx context.Context, warehouseSheet genmodel.PharmaSheetWarehouseSheets) error
	DeleteWarehouseSheet(ctx context.Context, warehouseID string) error
}

type warehouse struct {
	pgPool *pgxpool.Pool
}

func NewWarehouseRepository(pgPool *pgxpool.Pool) Warehouse {
	return &warehouse{pgPool: pgPool}
}

func (r *warehouse) GetWarehouse(ctx context.Context, warehouseID string) (model.Warehouse, error) {
	query, args := table.PharmaSheetWarehouses.
		INNER_JOIN(table.PharmaSheetWarehouseUsers, table.PharmaSheetWarehouses.WarehouseID.EQ(table.PharmaSheetWarehouseUsers.WarehouseID)).
		LEFT_JOIN(table.PharmaSheetWarehouseSheets, table.PharmaSheetWarehouses.WarehouseID.EQ(table.PharmaSheetWarehouseSheets.WarehouseID)).
		SELECT(
			table.PharmaSheetWarehouses.WarehouseID,
			table.PharmaSheetWarehouses.Name,
			table.PharmaSheetWarehouseUsers.Role,
			table.PharmaSheetWarehouseSheets.SpreadsheetID,
			table.PharmaSheetWarehouseSheets.SheetID,
			table.PharmaSheetWarehouseSheets.LatestSyncedAt,
		).
		WHERE(table.PharmaSheetWarehouses.WarehouseID.EQ(postgres.UUID(uuid.MustParse(warehouseID)))).
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

	query, args := table.PharmaSheetWarehouses.
		INNER_JOIN(table.PharmaSheetWarehouseUsers, table.PharmaSheetWarehouses.WarehouseID.EQ(table.PharmaSheetWarehouseUsers.WarehouseID)).
		LEFT_JOIN(table.PharmaSheetWarehouseSheets, table.PharmaSheetWarehouses.WarehouseID.EQ(table.PharmaSheetWarehouseSheets.WarehouseID)).
		SELECT(
			table.PharmaSheetWarehouses.WarehouseID,
			table.PharmaSheetWarehouses.Name,
			table.PharmaSheetWarehouseUsers.Role,
			table.PharmaSheetWarehouseSheets.SpreadsheetID,
			table.PharmaSheetWarehouseSheets.SheetID,
			table.PharmaSheetWarehouseSheets.LatestSyncedAt,
		).
		WHERE(table.PharmaSheetWarehouseUsers.UserID.EQ(postgres.UUID(uuid.MustParse(userProfile.UserID))).AND(table.PharmaSheetWarehouseUsers.Status.EQ(enum.PharmaSheetApprovalStatus.Approved))).
		ORDER_BY(table.PharmaSheetWarehouses.Name.ASC()).
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
	switch filter.Group {
	case "MY_WAREHOUSE":
		condition = condition.AND(table.PharmaSheetWarehouseUsers.UserID.EQ(postgres.UUID(uuid.MustParse(userProfile.UserID))).AND(table.PharmaSheetWarehouseUsers.Status.EQ(enum.PharmaSheetApprovalStatus.Approved)))
	case "OTHER_WAREHOUSE":
		condition = condition.AND(table.PharmaSheetWarehouseUsers.Status.IS_NULL())
	case "OTHER_WAREHOUSE_PENDING":
		condition = condition.AND(table.PharmaSheetWarehouseUsers.UserID.EQ(postgres.UUID(uuid.MustParse(userProfile.UserID))).AND(table.PharmaSheetWarehouseUsers.Status.EQ(enum.PharmaSheetApprovalStatus.Pending)))
	}

	if filter.Status != "" {
		condition = condition.AND(table.PharmaSheetWarehouseUsers.Status.EQ(postgres.NewEnumValue(string(filter.Status))))
	}

	if filter.Search != "" {
		search := postgres.String("%" + strings.ToLower(filter.Search) + "%")
		condition = condition.AND(postgres.LOWER(table.PharmaSheetWarehouses.Name).LIKE(search))
	}

	query, args := table.PharmaSheetWarehouses.
		LEFT_JOIN(table.PharmaSheetWarehouseUsers, table.PharmaSheetWarehouses.WarehouseID.EQ(table.PharmaSheetWarehouseUsers.WarehouseID).AND(table.PharmaSheetWarehouseUsers.UserID.EQ(postgres.UUID(uuid.MustParse(userProfile.UserID))))).
		SELECT(postgres.COUNT(table.PharmaSheetWarehouses.WarehouseID)).
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

	query, args = table.PharmaSheetWarehouses.
		LEFT_JOIN(table.PharmaSheetWarehouseUsers, table.PharmaSheetWarehouses.WarehouseID.EQ(table.PharmaSheetWarehouseUsers.WarehouseID).AND(table.PharmaSheetWarehouseUsers.UserID.EQ(postgres.UUID(uuid.MustParse(userProfile.UserID))))).
		SELECT(table.PharmaSheetWarehouses.WarehouseID, table.PharmaSheetWarehouses.Name, table.PharmaSheetWarehouseUsers.Role, table.PharmaSheetWarehouseUsers.Status).
		WHERE(condition).
		LIMIT(int64(filter.Limit)).
		OFFSET(int64(filter.Offset)).
		ORDER_BY(table.PharmaSheetWarehouses.Name.ASC()).
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
		warehouseData := genmodel.PharmaSheetWarehouses{
			WarehouseID: uuid.MustParse(warehouseID),
			Name:        req.Name,
			CreatedAt:   time.Now(),
		}

		warehouseUserData := genmodel.PharmaSheetWarehouseUsers{
			WarehouseID: uuid.MustParse(warehouseID),
			Role:        genmodel.PharmaSheetRole_Admin,
			UserID:      uuid.MustParse(userProfile.UserID),
			Status:      genmodel.PharmaSheetApprovalStatus_Approved,
			CreatedAt:   time.Now(),
		}

		sql, args := table.PharmaSheetWarehouses.
			INSERT(table.PharmaSheetWarehouses.WarehouseID, table.PharmaSheetWarehouses.Name, table.PharmaSheetWarehouses.CreatedAt).
			MODEL(warehouseData).
			Sql()
		_, err = tx.Exec(ctx, sql, args...)
		if err != nil {
			logger.Context(ctx).Error(err)
			return err
		}

		sql, args = table.PharmaSheetWarehouseUsers.
			INSERT(table.PharmaSheetWarehouseUsers.WarehouseID, table.PharmaSheetWarehouseUsers.UserID, table.PharmaSheetWarehouseUsers.Role, table.PharmaSheetWarehouseUsers.Status, table.PharmaSheetWarehouses.CreatedAt).
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
	warehouses := table.PharmaSheetWarehouses

	now := time.Now()
	warehouse := genmodel.PharmaSheetWarehouses{
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
	stmt, args := table.PharmaSheetWarehouses.DELETE().WHERE(table.PharmaSheetWarehouses.WarehouseID.EQ(postgres.UUID(uuid.MustParse(warehouseID)))).Sql()
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

func (r *warehouse) GetWarehouseRole(ctx context.Context, warehouseID, userID string) (role genmodel.PharmaSheetRole, err error) {
	query, args := table.PharmaSheetWarehouseUsers.
		SELECT(table.PharmaSheetWarehouseUsers.Role).
		WHERE(
			table.PharmaSheetWarehouseUsers.UserID.EQ(postgres.UUID(uuid.MustParse(userID))).AND(
				table.PharmaSheetWarehouseUsers.Status.EQ(enum.PharmaSheetApprovalStatus.Approved)).AND(
				table.PharmaSheetWarehouseUsers.WarehouseID.EQ(postgres.UUID(uuid.MustParse(warehouseID)))),
		).
		Sql()

	err = r.pgPool.QueryRow(ctx, query, args...).Scan(&role)
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}

	return role, nil
}

func (r *warehouse) CountWarehouseUserStatus(ctx context.Context, warehouseID string) (model.CountWarehouseUserStatus, error) {
	query, args := table.PharmaSheetWarehouseUsers.
		SELECT(
			table.PharmaSheetWarehouseUsers.UserID,
			table.PharmaSheetWarehouseUsers.Role,
			table.PharmaSheetWarehouseUsers.Status,
		).
		WHERE(table.PharmaSheetWarehouseUsers.WarehouseID.EQ(postgres.UUID(uuid.MustParse(warehouseID)))).Sql()

	var count model.CountWarehouseUserStatus
	rows, err := r.pgPool.Query(ctx, query, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return count, err
	}
	defer rows.Close()

	for rows.Next() {
		var warehouseUser model.WarehouseUser
		err = rows.Scan(
			&warehouseUser.UserID,
			&warehouseUser.Role,
			&warehouseUser.Status,
		)
		if err != nil {
			logger.Context(ctx).Error(err)
			return count, err
		}

		switch warehouseUser.Status {
		case genmodel.PharmaSheetApprovalStatus_Approved:
			count.TotalApproved++
		case genmodel.PharmaSheetApprovalStatus_Pending:
			count.TotalPending++
		}
	}

	return count, nil
}

func (r *warehouse) GetWarehouseUsers(ctx context.Context, warehouseID string, filter model.FilterWarehouseUser) (warehouseUsers []model.WarehouseUser, total uint64, err error) {
	condition := table.PharmaSheetWarehouseUsers.WarehouseID.EQ(postgres.UUID(uuid.MustParse(warehouseID)))

	if filter.Role != "" {
		condition = condition.AND(table.PharmaSheetWarehouseUsers.Role.EQ(postgres.NewEnumValue(string(filter.Role))))
	}
	if filter.Status != "" {
		condition = condition.AND(table.PharmaSheetWarehouseUsers.Status.EQ(postgres.NewEnumValue(string(filter.Status))))
	}
	if strings.TrimSpace(filter.Search) != "" {
		search := postgres.String("%" + strings.ToLower(filter.Search) + "%")
		condition = condition.AND(
			postgres.OR(
				postgres.LOWER(table.PharmaSheetUsers.DisplayName).LIKE(search),
				postgres.LOWER(table.PharmaSheetUsers.Email).LIKE(search),
			),
		)
	}

	query, args := table.PharmaSheetWarehouseUsers.
		INNER_JOIN(table.PharmaSheetUsers, table.PharmaSheetWarehouseUsers.UserID.EQ(table.PharmaSheetUsers.UserID)).
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

	query, args = table.PharmaSheetWarehouseUsers.
		INNER_JOIN(table.PharmaSheetUsers, table.PharmaSheetWarehouseUsers.UserID.EQ(table.PharmaSheetUsers.UserID)).
		SELECT(
			table.PharmaSheetWarehouseUsers.UserID,
			table.PharmaSheetWarehouseUsers.Role,
			table.PharmaSheetUsers.FirebaseUID,
			table.PharmaSheetUsers.Email,
			table.PharmaSheetUsers.DisplayName,
			table.PharmaSheetUsers.ImageURL,
			table.PharmaSheetWarehouseUsers.Status,
		).
		WHERE(condition).
		LIMIT(int64(filter.Limit)).
		OFFSET(int64(filter.Offset)).
		ORDER_BY(table.PharmaSheetUsers.Email.ASC()).
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

func (r *warehouse) GetWarehouseUserStatus(ctx context.Context, warehouseID, userID string) (status genmodel.PharmaSheetApprovalStatus, err error) {
	query, args := table.PharmaSheetWarehouseUsers.
		SELECT(table.PharmaSheetWarehouseUsers.Status).
		WHERE(table.PharmaSheetWarehouseUsers.UserID.EQ(postgres.UUID(uuid.MustParse(userID))).AND(table.PharmaSheetWarehouseUsers.WarehouseID.EQ(postgres.UUID(uuid.MustParse(warehouseID))))).
		Sql()

	err = r.pgPool.QueryRow(ctx, query, args...).Scan(&status)
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}

	return status, nil
}

func (r *warehouse) CreateWarehouseUser(ctx context.Context, warehouseID, userID string, role genmodel.PharmaSheetRole, status genmodel.PharmaSheetApprovalStatus) error {
	warehouseUsers := table.PharmaSheetWarehouseUsers

	warehouse := genmodel.PharmaSheetWarehouseUsers{
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

func (r *warehouse) UpdateWarehouseUser(ctx context.Context, warehouseUser genmodel.PharmaSheetWarehouseUsers) error {
	warehouseUsers := table.PharmaSheetWarehouseUsers

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
	warehouseUsers := table.PharmaSheetWarehouseUsers
	condition := warehouseUsers.WarehouseID.EQ(postgres.UUID(uuid.MustParse(warehouseID)))
	if userID != nil {
		condition = condition.AND(warehouseUsers.UserID.EQ(postgres.UUID(uuid.MustParse(*userID))))
	}
	stmt, args := table.PharmaSheetWarehouseUsers.DELETE().WHERE(condition).Sql()
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

func (r *warehouse) CheckConflictWarehouseSheet(ctx context.Context, warehouseID string, spreadsheetID string, sheetID int32) (bool, error) {
	query, args := table.PharmaSheetWarehouseSheets.
		SELECT(postgres.COUNT(postgres.STAR)).
		WHERE(
			table.PharmaSheetWarehouseSheets.WarehouseID.NOT_EQ(postgres.UUID(uuid.MustParse(warehouseID))).AND(
				table.PharmaSheetWarehouseSheets.SpreadsheetID.EQ(postgres.String(spreadsheetID)).AND(
					table.PharmaSheetWarehouseSheets.SheetID.EQ(postgres.Int32(sheetID))))).
		Sql()

	var count uint64
	err := r.pgPool.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		logger.Context(ctx).Error(err)
		return false, err
	}

	return count > 0, nil
}

func (r *warehouse) UpsertWarehouseSheet(ctx context.Context, warehouseSheet genmodel.PharmaSheetWarehouseSheets) error {
	warehouseSheet.LatestSyncedAt = time.Now()
	warehouseSheet.CreatedAt = time.Now()

	stmt, args := table.PharmaSheetWarehouseSheets.
		INSERT(table.PharmaSheetWarehouseSheets.WarehouseID, table.PharmaSheetWarehouseSheets.SpreadsheetID, table.PharmaSheetWarehouseSheets.SheetID, table.PharmaSheetWarehouseSheets.LatestSyncedAt, table.PharmaSheetWarehouseSheets.CreatedAt).
		MODEL(warehouseSheet).
		ON_CONFLICT(table.PharmaSheetWarehouseSheets.WarehouseID).
		DO_UPDATE(postgres.SET(
			table.PharmaSheetWarehouseSheets.SpreadsheetID.SET(postgres.String(warehouseSheet.SpreadsheetID)),
			table.PharmaSheetWarehouseSheets.SheetID.SET(postgres.Int32(warehouseSheet.SheetID)),
			table.PharmaSheetWarehouseSheets.LatestSyncedAt.SET(postgres.TimestampzT(time.Now())),
		)).
		Sql()
	_, err := r.pgPool.Exec(ctx, stmt, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return nil
}

func (r *warehouse) DeleteWarehouseSheet(ctx context.Context, warehouseID string) error {
	stmt, args := table.PharmaSheetWarehouseSheets.DELETE().WHERE(table.PharmaSheetWarehouseSheets.WarehouseID.EQ(postgres.UUID(uuid.MustParse(warehouseID)))).Sql()
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
