package repository

import (
	"context"
	"errors"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	genmodel "github.com/kinkando/pharma-sheet-service/.gen/postgresql_kinkando/public/model"
	"github.com/kinkando/pharma-sheet-service/.gen/postgresql_kinkando/public/table"
	"github.com/kinkando/pharma-sheet-service/model"
	"github.com/kinkando/pharma-sheet-service/pkg/generator"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
)

type Locker interface {
	GetLockers(ctx context.Context, warehouseID string) ([]genmodel.PharmaSheetLockers, error)
	CreateLocker(ctx context.Context, req genmodel.PharmaSheetLockers) (string, error)
	UpdateLocker(ctx context.Context, req genmodel.PharmaSheetLockers) error
	DeleteLocker(ctx context.Context, filter model.DeleteLockerFilter) (int64, error)
}

type locker struct {
	pgPool *pgxpool.Pool
}

func NewLockerRepository(pgPool *pgxpool.Pool) Locker {
	return &locker{pgPool: pgPool}
}

func (r *locker) GetLockers(ctx context.Context, warehouseID string) (lockers []genmodel.PharmaSheetLockers, err error) {
	query, args := table.PharmaSheetLockers.
		SELECT(table.PharmaSheetLockers.LockerID, table.PharmaSheetLockers.WarehouseID, table.PharmaSheetLockers.Name).
		WHERE(table.PharmaSheetLockers.WarehouseID.EQ(postgres.UUID(uuid.MustParse(warehouseID)))).
		ORDER_BY(table.PharmaSheetLockers.CreatedAt.ASC()).
		Sql()

	rows, err := r.pgPool.Query(ctx, query, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var locker genmodel.PharmaSheetLockers
		err = rows.Scan(&locker.LockerID, &locker.WarehouseID, &locker.Name)
		if err != nil {
			logger.Context(ctx).Error(err)
			return nil, err
		}
		lockers = append(lockers, locker)
	}

	return lockers, nil
}

func (r *locker) CreateLocker(ctx context.Context, locker genmodel.PharmaSheetLockers) (string, error) {
	lockers := table.PharmaSheetLockers

	locker.LockerID = uuid.MustParse(generator.UUID())
	locker.CreatedAt = time.Now()

	sql, args := lockers.INSERT(lockers.LockerID, lockers.WarehouseID, lockers.Name, lockers.CreatedAt).MODEL(locker).Sql()
	_, err := r.pgPool.Exec(ctx, sql, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return "", err
	}

	return locker.LockerID.String(), nil
}

func (r *locker) UpdateLocker(ctx context.Context, locker genmodel.PharmaSheetLockers) error {
	lockers := table.PharmaSheetLockers

	now := time.Now()
	locker.UpdatedAt = &now

	sql, args := lockers.
		UPDATE(lockers.Name, lockers.UpdatedAt).
		WHERE(lockers.LockerID.EQ(postgres.UUID(locker.LockerID))).
		MODEL(locker).
		Sql()
	_, err := r.pgPool.Exec(ctx, sql, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return nil
}

func (r *locker) DeleteLocker(ctx context.Context, filter model.DeleteLockerFilter) (int64, error) {
	var condition postgres.BoolExpression
	if filter.LockerID != "" {
		condition = table.PharmaSheetLockers.LockerID.EQ(postgres.UUID(uuid.MustParse(filter.LockerID)))
	} else if filter.WarehouseID != "" {
		condition = table.PharmaSheetLockers.WarehouseID.EQ(postgres.UUID(uuid.MustParse(filter.WarehouseID)))
	} else {
		return 0, errors.New("filter is invalid")
	}

	lockers := table.PharmaSheetLockers
	stmt, args := lockers.
		DELETE().
		WHERE(condition).
		Sql()
	result, err := r.pgPool.Exec(ctx, stmt, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return 0, err
	}

	return result.RowsAffected(), nil
}
