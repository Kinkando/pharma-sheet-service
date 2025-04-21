package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/enum"
	genmodel "github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/model"
	"github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/table"
	"github.com/kinkando/pharma-sheet-service/model"
	"github.com/kinkando/pharma-sheet-service/pkg/generator"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"github.com/kinkando/pharma-sheet-service/pkg/util"
)

type Medicine interface {
	GetMedicineRole(ctx context.Context, medicineID, userID string) (genmodel.PharmaSheetRole, error)
	GetMedicine(ctx context.Context, medicineID string) (model.Medicine, error)
	GetMedicines(ctx context.Context, filter model.FilterMedicine) (data []model.Medicine, total uint64, err error)
	ListMedicines(ctx context.Context, filter model.ListMedicine) ([]model.Medicine, error)
	CreateMedicine(ctx context.Context, req model.CreateMedicineRequest) (medicineID string, err error)
	UpdateMedicine(ctx context.Context, req model.UpdateMedicineRequest) error
	DeleteMedicine(ctx context.Context, filter model.DeleteMedicineFilter) (int64, error)
	UpsertMedicine(ctx context.Context, req model.Medicine) error
}

type medicine struct {
	pgPool *pgxpool.Pool
}

func NewMedicineRepository(pgPool *pgxpool.Pool) Medicine {
	return &medicine{pgPool: pgPool}
}

func (r *medicine) GetMedicineRole(ctx context.Context, medicineID, userID string) (role genmodel.PharmaSheetRole, err error) {
	query, args := table.PharmaSheetWarehouseUsers.
		INNER_JOIN(table.PharmaSheetMedicines, table.PharmaSheetWarehouseUsers.WarehouseID.EQ(table.PharmaSheetMedicines.WarehouseID)).
		SELECT(table.PharmaSheetWarehouseUsers.Role).
		WHERE(table.PharmaSheetWarehouseUsers.UserID.EQ(postgres.UUID(uuid.MustParse(userID))).AND(table.PharmaSheetWarehouseUsers.Status.EQ(enum.PharmaSheetApprovalStatus.Approved))).
		Sql()

	err = r.pgPool.QueryRow(ctx, query, args...).Scan(&role)
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}

	return role, nil
}

func (r *medicine) GetMedicine(ctx context.Context, medicineID string) (medicine model.Medicine, err error) {
	medicines := table.PharmaSheetMedicines
	query, args := medicines.
		LEFT_JOIN(table.PharmaSheetLockers, medicines.LockerID.EQ(table.PharmaSheetLockers.LockerID)).
		SELECT(
			medicines.MedicineID,
			medicines.WarehouseID,
			medicines.LockerID,
			table.PharmaSheetLockers.Name,
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
	medicines := table.PharmaSheetMedicines
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

	sortBy := "description ASC"
	if sorts := strings.Split(*filter.Sort, " "); filter.Sort != nil && *filter.Sort != "" && len(sorts) == 2 {
		sortBy = util.CamelToSnake(sorts[0]) + " " + sorts[1]
	}
	if !strings.HasSuffix(strings.ToUpper(sortBy), " ASC") && !strings.HasSuffix(strings.ToUpper(sortBy), " DESC") {
		sortBy = strings.Split(*filter.Sort, " ")[0] + " ASC"
	}
	if sorts := strings.Split(sortBy, " "); sorts[0] == "address" {
		order := sorts[1]
		sortBy = fmt.Sprintf("lockers.name %s, floor %s, no %s", order, order, order)
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
		LEFT_JOIN(table.PharmaSheetLockers, medicines.LockerID.EQ(table.PharmaSheetLockers.LockerID)).
		SELECT(
			medicines.MedicineID,
			medicines.WarehouseID,
			medicines.LockerID,
			table.PharmaSheetLockers.Name,
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
		ORDER_BY(postgres.Raw(sortBy)).
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

func (r *medicine) ListMedicines(ctx context.Context, filter model.ListMedicine) (data []model.Medicine, err error) {
	medicines := table.PharmaSheetMedicines

	var condition postgres.BoolExpression
	if filter.LockerID != "" {
		condition = medicines.LockerID.EQ(postgres.UUID(uuid.MustParse(filter.LockerID)))
	} else if filter.WarehouseID != "" {
		condition = medicines.WarehouseID.EQ(postgres.UUID(uuid.MustParse(filter.WarehouseID)))
	} else {
		return nil, errors.New("filter is invalid")
	}

	query, args := medicines.
		SELECT(
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
		).
		WHERE(condition).
		Sql()

	rows, err := r.pgPool.Query(ctx, query, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var medicine model.Medicine
		err = rows.Scan(
			&medicine.MedicineID,
			&medicine.WarehouseID,
			&medicine.LockerID,
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
			return nil, err
		}
		data = append(data, medicine)
	}

	return data, nil
}

func (r *medicine) CreateMedicine(ctx context.Context, req model.CreateMedicineRequest) (medicineID string, err error) {
	medicines := table.PharmaSheetMedicines

	medicine := genmodel.PharmaSheetMedicines{
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
	medicines := table.PharmaSheetMedicines

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

func (r *medicine) UpsertMedicine(ctx context.Context, req model.Medicine) error {
	medicines := table.PharmaSheetMedicines

	if req.MedicineID == "" {
		req.MedicineID = generator.UUID()
	}

	now := time.Now()
	medicine := genmodel.PharmaSheetMedicines{
		MedicineID:  uuid.MustParse(req.MedicineID),
		WarehouseID: uuid.MustParse(req.WarehouseID),
		LockerID:    uuid.MustParse(req.LockerID),
		Floor:       req.Floor,
		No:          req.No,
		Address:     req.Address,
		Description: req.Description,
		MedicalName: req.MedicalName,
		Label:       req.Label,
		ImageURL:    req.ImageURL,
		CreatedAt:   now,
	}

	sql, args := medicines.
		INSERT(
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
		ON_CONFLICT(medicines.WarehouseID, medicines.LockerID, medicines.Floor, medicines.No).
		DO_UPDATE(postgres.SET(
			medicines.WarehouseID.SET(postgres.UUID(medicine.WarehouseID)),
			medicines.LockerID.SET(postgres.UUID(medicine.LockerID)),
			medicines.Floor.SET(postgres.Int32(medicine.Floor)),
			medicines.No.SET(postgres.Int32(medicine.No)),
			medicines.Address.SET(postgres.String(medicine.Address)),
			medicines.Description.SET(postgres.String(medicine.Description)),
			medicines.MedicalName.SET(postgres.String(medicine.MedicalName)),
			medicines.Label.SET(postgres.String(medicine.Label)),
			medicines.UpdatedAt.SET(postgres.TimestampzT(now)),
		)).
		Sql()

	_, err := r.pgPool.Exec(ctx, sql, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return nil
}

func (r *medicine) DeleteMedicine(ctx context.Context, filter model.DeleteMedicineFilter) (int64, error) {
	var condition postgres.BoolExpression
	if filter.MedicineID != "" {
		condition = table.PharmaSheetMedicines.MedicineID.EQ(postgres.UUID(uuid.MustParse(filter.MedicineID)))
	} else if filter.LockerID != "" {
		condition = table.PharmaSheetMedicines.LockerID.EQ(postgres.UUID(uuid.MustParse(filter.LockerID)))
	} else if filter.WarehouseID != "" {
		condition = table.PharmaSheetMedicines.WarehouseID.EQ(postgres.UUID(uuid.MustParse(filter.WarehouseID)))
	} else {
		return 0, errors.New("filter is invalid")
	}
	stmt, args := table.PharmaSheetMedicines.DELETE().WHERE(condition).Sql()
	result, err := r.pgPool.Exec(ctx, stmt, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return 0, err
	}
	return result.RowsAffected(), nil
}
