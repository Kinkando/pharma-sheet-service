package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	genmodel "github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/model"
	"github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/table"
	"github.com/kinkando/pharma-sheet-service/model"
	"github.com/kinkando/pharma-sheet-service/pkg/generator"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
)

type Medicine interface {
	GetMedicineRole(ctx context.Context, medicineID, userID string) (genmodel.Role, error)
	GetMedicine(ctx context.Context, medicineID string) (model.Medicine, error)
	GetMedicines(ctx context.Context, filter model.FilterMedicine) (data []model.Medicine, total uint64, err error)
	CreateMedicine(ctx context.Context, req model.CreateMedicineRequest) (medicineID string, err error)
	UpdateMedicine(ctx context.Context, req model.UpdateMedicineRequest) error
	DeleteMedicine(ctx context.Context, medicineID string) error
}

type medicine struct {
	pgPool *pgxpool.Pool
}

func NewMedicineRepository(pgPool *pgxpool.Pool) Medicine {
	return &medicine{pgPool: pgPool}
}

func (r *medicine) GetMedicineRole(ctx context.Context, medicineID, userID string) (role genmodel.Role, err error) {
	query, args := table.WarehouseUsers.
		INNER_JOIN(table.Medicines, table.WarehouseUsers.WarehouseID.EQ(table.Medicines.WarehouseID)).
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

func (r *medicine) GetMedicine(ctx context.Context, medicineID string) (medicine model.Medicine, err error) {
	medicines := table.Medicines
	query, args := medicines.
		LEFT_JOIN(table.Lockers, medicines.LockerID.EQ(table.Lockers.LockerID)).
		SELECT(
			medicines.MedicineID,
			medicines.WarehouseID,
			medicines.LockerID,
			table.Lockers.Name,
			medicines.Floor,
			medicines.No,
			medicines.Address,
			medicines.Description,
			medicines.MedicalName,
			medicines.Label,
			medicines.ImageURL,
		).
		WHERE(medicines.MedicineID.EQ(postgres.UUID(uuid.MustParse(medicineID)))).
		Sql()

	err = r.pgPool.QueryRow(ctx, query, args...).Scan(
		&medicine.MedicineID,
		&medicine.WarehouseID,
		&medicine.LockerID,
		&medicine.LockerName,
		&medicine.Floor,
		&medicine.No,
		&medicine.Address,
		&medicine.Description,
		&medicine.MedicalName,
		&medicine.Label,
		&medicine.ImageURL,
	)
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}

	return medicine, err
}

func (r *medicine) GetMedicines(ctx context.Context, filter model.FilterMedicine) (data []model.Medicine, total uint64, err error) {
	medicines := table.Medicines
	condition := medicines.WarehouseID.EQ(postgres.UUID(uuid.MustParse(filter.WarehouseID)))

	if filter.Search != "" {
		search := postgres.String("%" + strings.ToLower(filter.Search) + "%")
		condition = condition.AND(
			postgres.OR(
				postgres.LOWER(medicines.Description).LIKE(search),
				postgres.LOWER(medicines.MedicalName).LIKE(search),
				postgres.LOWER(medicines.Label).LIKE(search),
			),
		)
	}

	query, args := medicines.
		SELECT(postgres.COUNT(medicines.MedicineID)).
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

	query, args = medicines.
		LEFT_JOIN(table.Lockers, medicines.LockerID.EQ(table.Lockers.LockerID)).
		SELECT(
			medicines.MedicineID,
			medicines.WarehouseID,
			medicines.LockerID,
			table.Lockers.Name,
			medicines.Floor,
			medicines.No,
			medicines.Address,
			medicines.Description,
			medicines.MedicalName,
			medicines.Label,
			medicines.ImageURL,
		).
		WHERE(condition).
		LIMIT(int64(filter.Limit)).
		OFFSET(int64(filter.Offset)).
		ORDER_BY(medicines.Description.ASC()).
		Sql()

	rows, err := r.pgPool.Query(ctx, query, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var medicine model.Medicine
		err = rows.Scan(
			&medicine.MedicineID,
			&medicine.WarehouseID,
			&medicine.LockerID,
			&medicine.LockerName,
			&medicine.Floor,
			&medicine.No,
			&medicine.Address,
			&medicine.Description,
			&medicine.MedicalName,
			&medicine.Label,
			&medicine.ImageURL,
		)
		if err != nil {
			logger.Context(ctx).Error(err)
			return nil, 0, err
		}
		data = append(data, medicine)
	}

	return data, total, nil
}

func (r *medicine) CreateMedicine(ctx context.Context, req model.CreateMedicineRequest) (medicineID string, err error) {
	medicines := table.Medicines

	medicine := genmodel.Medicines{
		MedicineID:  uuid.MustParse(generator.UUID()),
		WarehouseID: uuid.MustParse(req.WarehouseID),
		LockerID:    uuid.MustParse(req.LockerID),
		Floor:       req.Floor,
		No:          req.No,
		Address:     req.Address,
		Description: req.Description,
		MedicalName: req.MedicalName,
		Label:       req.Label,
		ImageURL:    req.ImageURL,
		CreatedAt:   time.Now(),
	}

	sql, args := medicines.INSERT(
		medicines.MedicineID,
		medicines.WarehouseID,
		medicines.LockerID,
		medicines.Floor,
		medicines.No,
		medicines.Address,
		medicines.Description,
		medicines.MedicalName,
		medicines.Label,
		medicines.ImageURL,
		medicines.CreatedAt,
	).
		MODEL(medicine).
		Sql()

	_, err = r.pgPool.Exec(ctx, sql, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return "", err
	}

	return medicine.MedicineID.String(), nil
}

func (r *medicine) UpdateMedicine(ctx context.Context, req model.UpdateMedicineRequest) error {
	medicines := table.Medicines

	columnNames := postgres.ColumnList{
		medicines.LockerID,
		medicines.Floor,
		medicines.No,
		medicines.Address,
		medicines.Description,
		medicines.MedicalName,
		medicines.Label,
		medicines.UpdatedAt,
	}

	columnValues := []any{
		postgres.UUID(uuid.MustParse(req.LockerID)),
		postgres.Int32(req.Floor),
		postgres.Int32(req.No),
		postgres.String(req.Address),
		postgres.String(req.Description),
		postgres.String(req.MedicalName),
		postgres.String(req.Label),
		postgres.TimestampzT(time.Now()),
	}

	if req.ImageURL != nil && *req.ImageURL == "null" {
		columnNames = append(columnNames, medicines.ImageURL)
		columnValues = append(columnValues, postgres.NULL)
	} else if req.ImageURL != nil {
		columnNames = append(columnNames, medicines.ImageURL)
		columnValues = append(columnValues, postgres.String(*req.ImageURL))
	}

	sql, args := medicines.
		UPDATE(columnNames).
		SET(columnValues[0], columnValues[1:]...).
		WHERE(medicines.MedicineID.EQ(postgres.UUID(uuid.MustParse(req.MedicineID)))).
		Sql()
	_, err := r.pgPool.Exec(ctx, sql, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return nil
}

func (r *medicine) DeleteMedicine(ctx context.Context, medicineID string) error {
	stmt, args := table.Medicines.DELETE().WHERE(table.Medicines.MedicineID.EQ(postgres.UUID(uuid.MustParse(medicineID)))).Sql()
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
