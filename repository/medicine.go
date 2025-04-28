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
	"github.com/kinkando/pharma-sheet-service/pkg/profile"
	"github.com/sourcegraph/conc/pool"
)

type Medicine interface {
	GetMedicineRole(ctx context.Context, medicationID, userID string) (genmodel.PharmaSheetRole, error)
	GetMedicine(ctx context.Context, medicationID string) (model.Medicine, error)
	GetMedicines(ctx context.Context, filter model.FilterMedicine) (data []model.Medicine, total uint64, err error)
	ListMedicines(ctx context.Context, filter model.ListMedicine) ([]model.Medicine, error)
	ListMedicinesMaster(ctx context.Context) ([]model.Medicine, error)
	CreateMedicine(ctx context.Context, req model.CreateMedicineRequest) (medicationID string, err error)
	UpdateMedicine(ctx context.Context, req model.UpdateMedicineRequest) error
	DeleteMedicine(ctx context.Context, filter model.DeleteMedicineFilter) (int64, error)

	GetMedicineHouses(ctx context.Context, filter model.FilterMedicineHouse) ([]model.MedicineHouse, error)
	ListMedicineHouses(ctx context.Context, filter model.ListMedicineHouse) (data []model.MedicineHouse, total uint64, err error)
	CreateMedicineHouse(ctx context.Context, req model.CreateMedicineHouseRequest) (string, error)
	UpdateMedicineHouse(ctx context.Context, req model.UpdateMedicineHouseRequest) error
	DeleteMedicineHouse(ctx context.Context, filter model.DeleteMedicineHouseFilter) (int64, error)

	GetMedicineBrands(ctx context.Context, req model.FilterMedicineBrand) ([]model.MedicineBrand, error)
	GetMedicineWithBrands(ctx context.Context, filter model.FilterMedicineWithBrand) (data []model.Medicine, total uint64, err error)
	CreateMedicineBrand(ctx context.Context, req model.CreateMedicineBrandRequest) (string, error)
	UpdateMedicineBrand(ctx context.Context, req model.UpdateMedicineBrandRequest) error
	DeleteMedicineBrand(ctx context.Context, filter model.DeleteMedicineBrandFilter) (int64, error)

	GetMedicineBlisterChangeDateHistory(ctx context.Context, id uuid.UUID) (model.MedicineBlisterDateHistory, error)
	ListMedicineBlisterChangeDateHistory(ctx context.Context, filter model.FilterMedicineBrandBlisterDateHistory) ([]model.MedicineBlisterDateHistory, error)
	CreateMedicineBlisterChangeDateHistory(ctx context.Context, req model.CreateMedicineBlisterChangeDateHistoryRequest) (string, error)
	DeleteMedicineBlisterChangeDateHistory(ctx context.Context, id uuid.UUID) error
}

type medicine struct {
	pgPool *pgxpool.Pool
}

func NewMedicineRepository(pgPool *pgxpool.Pool) Medicine {
	return &medicine{pgPool: pgPool}
}

func (r *medicine) GetMedicineRole(ctx context.Context, medicationID, userID string) (role genmodel.PharmaSheetRole, err error) {
	query, args := table.PharmaSheetMedicines.
		LEFT_JOIN(table.PharmaSheetMedicineHouses, table.PharmaSheetMedicineHouses.MedicationID.EQ(table.PharmaSheetMedicines.MedicationID)).
		LEFT_JOIN(table.PharmaSheetWarehouseUsers, table.PharmaSheetWarehouseUsers.WarehouseID.EQ(table.PharmaSheetMedicineHouses.WarehouseID)).
		SELECT(table.PharmaSheetWarehouseUsers.UserID, table.PharmaSheetWarehouseUsers.Role, table.PharmaSheetWarehouseUsers.Status).
		WHERE(table.PharmaSheetMedicines.MedicationID.EQ(postgres.String(medicationID))).
		GROUP_BY(table.PharmaSheetWarehouseUsers.UserID, table.PharmaSheetWarehouseUsers.Role, table.PharmaSheetWarehouseUsers.Status).
		Sql()

	rows, err := r.pgPool.Query(ctx, query, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}
	defer rows.Close()

	var warehouseUsers []genmodel.PharmaSheetWarehouseUsers
	for rows.Next() {
		var userID *uuid.UUID
		var userRole *genmodel.PharmaSheetRole
		var status *genmodel.PharmaSheetApprovalStatus
		if err = rows.Scan(&userID, &userRole, &status); err != nil {
			logger.Context(ctx).Error(err)
			return
		}

		if userID != nil && userRole != nil && status != nil {
			warehouseUsers = append(warehouseUsers, genmodel.PharmaSheetWarehouseUsers{
				UserID: *userID,
				Role:   *userRole,
				Status: *status,
			})
		}
	}

	if len(warehouseUsers) == 0 {
		return genmodel.PharmaSheetRole_Admin, nil
	}

	for _, warehouseUser := range warehouseUsers {
		if warehouseUser.UserID.String() == userID && warehouseUser.Status == genmodel.PharmaSheetApprovalStatus_Approved {
			return warehouseUser.Role, nil
		}
	}

	return role, model.ErrResourceNotAllowed
}

func (r *medicine) GetMedicine(ctx context.Context, medicationID string) (medicine model.Medicine, err error) {
	userProfile, err := profile.UseProfile(ctx)
	if err != nil {
		return
	}

	query, args := table.PharmaSheetMedicines.
		SELECT(table.PharmaSheetMedicines.MedicationID, table.PharmaSheetMedicines.MedicalName).
		WHERE(table.PharmaSheetMedicines.MedicationID.EQ(postgres.String(medicationID))).
		Sql()
	err = r.pgPool.QueryRow(ctx, query, args...).Scan(&medicine.MedicationID, &medicine.MedicalName)
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}

	query, args = table.PharmaSheetMedicineHouses.
		INNER_JOIN(table.PharmaSheetWarehouses, table.PharmaSheetMedicineHouses.WarehouseID.EQ(table.PharmaSheetWarehouses.WarehouseID)).
		INNER_JOIN(table.PharmaSheetWarehouseUsers, table.PharmaSheetMedicineHouses.WarehouseID.EQ(table.PharmaSheetWarehouseUsers.WarehouseID)).
		SELECT(
			table.PharmaSheetMedicineHouses.ID,
			table.PharmaSheetMedicineHouses.WarehouseID,
			table.PharmaSheetMedicineHouses.MedicationID,
			table.PharmaSheetMedicineHouses.Locker,
			table.PharmaSheetMedicineHouses.Floor,
			table.PharmaSheetMedicineHouses.No,
			table.PharmaSheetMedicineHouses.Label,
			table.PharmaSheetWarehouses.Name,
		).
		WHERE(table.PharmaSheetMedicineHouses.MedicationID.EQ(postgres.String(medicationID)).AND(
			table.PharmaSheetWarehouseUsers.UserID.EQ(postgres.UUID(uuid.MustParse(userProfile.UserID))).AND(table.PharmaSheetWarehouseUsers.Status.EQ(enum.PharmaSheetApprovalStatus.Approved)),
		)).
		ORDER_BY(table.PharmaSheetWarehouses.WarehouseID.ASC(), table.PharmaSheetMedicineHouses.Locker.ASC(), table.PharmaSheetMedicineHouses.Floor.ASC(), table.PharmaSheetMedicineHouses.No.ASC()).
		Sql()
	rows, err := r.pgPool.Query(ctx, query, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}
	defer rows.Close()

	var medicineHouses []model.MedicineHouse
	for rows.Next() {
		var medicineHouse model.MedicineHouse
		err = rows.Scan(
			&medicineHouse.ID,
			&medicineHouse.WarehouseID,
			&medicineHouse.MedicationID,
			&medicineHouse.Locker,
			&medicineHouse.Floor,
			&medicineHouse.No,
			&medicineHouse.Label,
			&medicineHouse.WarehouseName,
		)
		if err != nil {
			logger.Context(ctx).Error(err)
			return
		}
		medicineHouses = append(medicineHouses, medicineHouse)
	}

	query, args = table.PharmaSheetMedicineBrands.
		SELECT(
			table.PharmaSheetMedicineBrands.ID,
			table.PharmaSheetMedicineBrands.TradeID,
			table.PharmaSheetMedicineBrands.MedicationID,
			table.PharmaSheetMedicineBrands.TradeName,
			table.PharmaSheetMedicineBrands.BlisterImageURL,
			table.PharmaSheetMedicineBrands.TabletImageURL,
			table.PharmaSheetMedicineBrands.BoxImageURL,
		).
		WHERE(table.PharmaSheetMedicineBrands.MedicationID.EQ(postgres.String(medicationID))).
		ORDER_BY(table.PharmaSheetMedicineBrands.TradeID.ASC()).
		Sql()
	rows, err = r.pgPool.Query(ctx, query, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}
	defer rows.Close()

	var medicineBrands []genmodel.PharmaSheetMedicineBrands
	for rows.Next() {
		var medicineBrand genmodel.PharmaSheetMedicineBrands
		err = rows.Scan(
			&medicineBrand.ID,
			&medicineBrand.TradeID,
			&medicineBrand.MedicationID,
			&medicineBrand.TradeName,
			&medicineBrand.BlisterImageURL,
			&medicineBrand.TabletImageURL,
			&medicineBrand.BoxImageURL,
		)
		if err != nil {
			logger.Context(ctx).Error(err)
			return
		}
		medicineBrands = append(medicineBrands, medicineBrand)
	}

	query, args = table.PharmaSheetMedicineBlisterDateHistories.
		INNER_JOIN(table.PharmaSheetWarehouses, table.PharmaSheetMedicineBlisterDateHistories.WarehouseID.EQ(table.PharmaSheetWarehouses.WarehouseID)).
		INNER_JOIN(table.PharmaSheetWarehouseUsers, table.PharmaSheetMedicineBlisterDateHistories.WarehouseID.EQ(table.PharmaSheetWarehouseUsers.WarehouseID)).
		LEFT_JOIN(table.PharmaSheetMedicineBrands, table.PharmaSheetMedicineBlisterDateHistories.BrandID.EQ(table.PharmaSheetMedicineBrands.ID)).
		SELECT(
			table.PharmaSheetMedicineBlisterDateHistories.ID,
			table.PharmaSheetMedicineBlisterDateHistories.WarehouseID,
			table.PharmaSheetMedicineBlisterDateHistories.MedicationID,
			table.PharmaSheetMedicineBlisterDateHistories.BrandID,
			table.PharmaSheetMedicineBrands.TradeID,
			table.PharmaSheetMedicineBlisterDateHistories.BlisterChangeDate,
			table.PharmaSheetWarehouses.Name,
			table.PharmaSheetMedicineBrands.TradeName,
		).
		WHERE(table.PharmaSheetMedicineBlisterDateHistories.MedicationID.EQ(postgres.String(medicationID)).AND(
			table.PharmaSheetWarehouseUsers.UserID.EQ(postgres.UUID(uuid.MustParse(userProfile.UserID))).AND(table.PharmaSheetWarehouseUsers.Status.EQ(enum.PharmaSheetApprovalStatus.Approved)),
		)).
		ORDER_BY(table.PharmaSheetMedicineBlisterDateHistories.WarehouseID.ASC(), table.PharmaSheetMedicineBrands.TradeID.ASC(), table.PharmaSheetMedicineBlisterDateHistories.BlisterChangeDate.ASC()).
		Sql()
	rows, err = r.pgPool.Query(ctx, query, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}
	defer rows.Close()

	var medicineBlisterDateHistories []model.MedicineBlisterDateHistory
	for rows.Next() {
		var medicineBlisterDateHistory model.MedicineBlisterDateHistory
		err = rows.Scan(
			&medicineBlisterDateHistory.ID,
			&medicineBlisterDateHistory.WarehouseID,
			&medicineBlisterDateHistory.MedicationID,
			&medicineBlisterDateHistory.BrandID,
			&medicineBlisterDateHistory.TradeID,
			&medicineBlisterDateHistory.BlisterChangeDate,
			&medicineBlisterDateHistory.WarehouseName,
			&medicineBlisterDateHistory.TradeName,
		)
		if err != nil {
			logger.Context(ctx).Error(err)
			return
		}
		medicineBlisterDateHistories = append(medicineBlisterDateHistories, medicineBlisterDateHistory)
	}

	for _, medicineBrand := range medicineBrands {
		medicine.Brands = append(medicine.Brands, model.MedicineBrand{
			ID:              medicineBrand.ID,
			TradeID:         medicineBrand.TradeID,
			TradeName:       medicineBrand.TradeName,
			BlisterImageURL: medicineBrand.BlisterImageURL,
			TabletImageURL:  medicineBrand.TabletImageURL,
			BoxImageURL:     medicineBrand.BoxImageURL,
		})
	}

	for _, medicineHouse := range medicineHouses {
		houseDetail := model.MedicineHouseDetailView{
			ID:     medicineHouse.ID,
			Locker: medicineHouse.Locker,
			Floor:  medicineHouse.Floor,
			No:     medicineHouse.No,
			Label:  medicineHouse.Label,
		}
		isFound := false
		for index, house := range medicine.Houses {
			if house.WarehouseID == medicineHouse.WarehouseID {
				medicine.Houses[index].Addresses = append(house.Addresses, houseDetail)
				isFound = true
				break
			}
		}
		if !isFound {
			medicine.Houses = append(medicine.Houses, model.MedicineHouseView{
				WarehouseID:   medicineHouse.WarehouseID,
				WarehouseName: medicineHouse.WarehouseName,
				Addresses:     []model.MedicineHouseDetailView{houseDetail},
			})
		}
	}

	for _, medicineBlisterDateHistory := range medicineBlisterDateHistories {
		isFound := false
		detail := model.MedicineBrandBlisterDateHistoryView{
			TradeID:   medicineBlisterDateHistory.TradeID,
			TradeName: medicineBlisterDateHistory.TradeName,
			BlisterChanges: []model.MedicineBrandBlisterDateDetailHistoryView{{
				ID:   medicineBlisterDateHistory.ID,
				Date: medicineBlisterDateHistory.BlisterChangeDate.Format(time.DateOnly),
			}},
		}
		for historyIndex, history := range medicine.BlisterDateHistories {
			if history.WarehouseID == medicineBlisterDateHistory.WarehouseID {
				isFound = true
				isFoundBrand := false
				for brandIndex, brand := range history.Brands {
					var brandTradeID, medicineHistoryTradeID string
					if brand.TradeID != nil {
						brandTradeID = *brand.TradeID
					}
					if medicineBlisterDateHistory.TradeID != nil {
						medicineHistoryTradeID = *medicineBlisterDateHistory.TradeID
					}
					if brandTradeID == medicineHistoryTradeID {
						isFoundBrand = true
						medicine.BlisterDateHistories[historyIndex].Brands[brandIndex].BlisterChanges = append(medicine.BlisterDateHistories[historyIndex].Brands[brandIndex].BlisterChanges, detail.BlisterChanges...)
						break
					}
				}
				if !isFoundBrand {
					medicine.BlisterDateHistories[historyIndex].Brands = append(medicine.BlisterDateHistories[historyIndex].Brands, detail)
				}
				break
			}
		}
		if !isFound {
			medicine.BlisterDateHistories = append(medicine.BlisterDateHistories, model.MedicineBlisterDateHistoryView{
				WarehouseID:   medicineBlisterDateHistory.WarehouseID,
				WarehouseName: medicineBlisterDateHistory.WarehouseName,
				Brands:        []model.MedicineBrandBlisterDateHistoryView{detail},
			})
		}
	}

	return medicine, err
}

func (r *medicine) GetMedicines(ctx context.Context, filter model.FilterMedicine) (sortedData []model.Medicine, total uint64, err error) {
	condition := postgres.Bool(true)
	if filter.WarehouseID != "" {
		condition = condition.AND(table.PharmaSheetMedicineHouses.ID.IS_NOT_NULL().AND(table.PharmaSheetMedicineHouses.WarehouseID.EQ(postgres.String(filter.WarehouseID))))
	}

	if search := strings.TrimSpace(filter.Search); search != "" {
		search := postgres.String("%" + strings.ToLower(search) + "%")
		address := postgres.CONCAT(table.PharmaSheetMedicineHouses.Locker, postgres.String("-"), table.PharmaSheetMedicineHouses.Floor, postgres.String("-"), table.PharmaSheetMedicineHouses.No)
		condition = condition.AND(
			postgres.OR(
				postgres.LOWER(table.PharmaSheetMedicineBrands.TradeID).LIKE(search),
				postgres.LOWER(table.PharmaSheetMedicineBrands.TradeName).LIKE(search),
				postgres.LOWER(table.PharmaSheetMedicines.MedicalName).LIKE(search),
				postgres.LOWER(table.PharmaSheetMedicines.MedicationID).LIKE(search),
				postgres.LOWER(table.PharmaSheetMedicineHouses.Label).LIKE(search),
				postgres.LOWER(address).LIKE(search),
			),
		)
	}

	sortBy := filter.SortBy("medical_name ASC")
	sorts := strings.Split(sortBy, " ")
	order := sorts[1]
	switch sorts[0] {
	case "medication_id":
		sortBy = fmt.Sprintf("%s.medication_id %s", table.PharmaSheetMedicines.TableName(), order)
	case "address":
		sortBy = fmt.Sprintf("locker %s, floor %s, no %s", order, order, order)
	}

	query, args := table.PharmaSheetMedicines.
		LEFT_JOIN(table.PharmaSheetMedicineHouses, table.PharmaSheetMedicines.MedicationID.EQ(table.PharmaSheetMedicineHouses.MedicationID)).
		LEFT_JOIN(table.PharmaSheetMedicineBrands, table.PharmaSheetMedicines.MedicationID.EQ(table.PharmaSheetMedicineBrands.MedicationID)).
		SELECT(postgres.COUNT(postgres.DISTINCT(table.PharmaSheetMedicines.MedicationID)).AS("total")).
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

	query, args = table.PharmaSheetMedicines.
		LEFT_JOIN(table.PharmaSheetMedicineHouses, table.PharmaSheetMedicines.MedicationID.EQ(table.PharmaSheetMedicineHouses.MedicationID)).
		LEFT_JOIN(table.PharmaSheetMedicineBrands, table.PharmaSheetMedicines.MedicationID.EQ(table.PharmaSheetMedicineBrands.MedicationID)).
		SELECT(
			table.PharmaSheetMedicines.MedicationID,
			table.PharmaSheetMedicines.MedicalName,
			table.PharmaSheetMedicineHouses.WarehouseID,
			table.PharmaSheetMedicineHouses.Locker,
			table.PharmaSheetMedicineHouses.Floor,
			table.PharmaSheetMedicineHouses.No,
			table.PharmaSheetMedicineHouses.Label,
			table.PharmaSheetMedicineBrands.TradeID,
			table.PharmaSheetMedicineBrands.TradeName,
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

	var medicines []model.MedicineView
	medicineID := make(map[string]bool)
	for rows.Next() {
		var medicine model.MedicineView
		err = rows.Scan(
			&medicine.MedicationID,
			&medicine.MedicalName,
			&medicine.WarehouseID,
			&medicine.Locker,
			&medicine.Floor,
			&medicine.No,
			&medicine.Label,
			&medicine.TradeID,
			&medicine.TradeName,
		)
		if err != nil {
			logger.Context(ctx).Error(err)
			return nil, 0, err
		}
		if _, ok := medicineID[medicine.MedicationID]; !ok {
			medicineID[medicine.MedicationID] = true
			medicines = append(medicines, medicine)
		}
	}

	var data []model.Medicine
	conc := pool.New().WithContext(ctx).WithMaxGoroutines(10)
	for _, medicine := range medicines {
		medicine := medicine
		conc.Go(func(ctx context.Context) error {
			medicine, err := r.GetMedicine(ctx, medicine.MedicationID)
			if err != nil {
				logger.Context(ctx).Error(err)
				return err
			}
			data = append(data, medicine)
			return nil
		})
	}
	if err := conc.Wait(); err != nil {
		return nil, 0, err
	}

	for _, medicine := range medicines {
		for _, d := range data {
			if medicine.MedicationID == d.MedicationID {
				sortedData = append(sortedData, d)
				break
			}
		}
	}

	if len(sortedData) != len(medicines) {
		return nil, 0, errors.New("data length mismatch")
	}

	return sortedData, total, nil
}

func (r *medicine) ListMedicines(ctx context.Context, filter model.ListMedicine) (data []model.Medicine, err error) {
	var condition postgres.BoolExpression
	if filter.WarehouseID != "" {
		condition = table.PharmaSheetMedicineHouses.WarehouseID.EQ(postgres.String(filter.WarehouseID))
	} else {
		return nil, errors.New("filter is invalid")
	}

	query, args := table.PharmaSheetMedicineHouses.SELECT(postgres.DISTINCT(table.PharmaSheetMedicineHouses.MedicationID)).WHERE(condition).Sql()
	rows, err := r.pgPool.Query(ctx, query, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return nil, err
	}
	defer rows.Close()

	var medicationIDs []string
	for rows.Next() {
		var medicationID string
		err = rows.Scan(&medicationID)
		if err != nil {
			logger.Context(ctx).Error(err)
			return nil, err
		}
		medicationIDs = append(medicationIDs, medicationID)
	}

	conc := pool.New().WithContext(ctx).WithMaxGoroutines(10)
	for _, medicationID := range medicationIDs {
		medicationID := medicationID
		conc.Go(func(ctx context.Context) error {
			medicine, err := r.GetMedicine(ctx, medicationID)
			if err != nil {
				logger.Context(ctx).Error(err)
				return err
			}
			data = append(data, medicine)
			return nil
		})
	}
	if err := conc.Wait(); err != nil {
		return nil, err
	}

	if len(data) != len(medicationIDs) {
		return nil, errors.New("data length mismatch")
	}

	return data, nil
}

func (r *medicine) ListMedicinesMaster(ctx context.Context) ([]model.Medicine, error) {
	query, args := table.PharmaSheetMedicines.
		SELECT(table.PharmaSheetMedicines.MedicationID, table.PharmaSheetMedicines.MedicalName).
		ORDER_BY(table.PharmaSheetMedicines.MedicationID.ASC()).
		Sql()

	rows, err := r.pgPool.Query(ctx, query, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return nil, err
	}
	defer rows.Close()

	var medicines []model.Medicine
	for rows.Next() {
		var medicine model.Medicine
		err = rows.Scan(&medicine.MedicationID, &medicine.MedicalName)
		if err != nil {
			logger.Context(ctx).Error(err)
			return nil, err
		}
		medicines = append(medicines, medicine)
	}

	return medicines, nil
}

func (r *medicine) CreateMedicine(ctx context.Context, req model.CreateMedicineRequest) (medicationID string, err error) {
	medicines := table.PharmaSheetMedicines

	if req.MedicalName == nil || *req.MedicalName == "" {
		req.MedicalName = &req.MedicationID
	}

	now := time.Now()
	medicine := genmodel.PharmaSheetMedicines{
		MedicationID: req.MedicationID,
		MedicalName:  *req.MedicalName,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	sql, args := medicines.
		INSERT(
			medicines.MedicationID,
			medicines.MedicalName,
			medicines.CreatedAt,
			medicines.UpdatedAt,
		).
		MODEL(medicine).
		Sql()

	_, err = r.pgPool.Exec(ctx, sql, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return "", err
	}

	return req.MedicationID, nil
}

func (r *medicine) UpdateMedicine(ctx context.Context, req model.UpdateMedicineRequest) error {
	medicines := table.PharmaSheetMedicines

	if req.MedicalName == nil || *req.MedicalName == "" {
		req.MedicalName = &req.MedicationID
	}

	sql, args := medicines.
		UPDATE(medicines.MedicalName, medicines.UpdatedAt).
		SET(postgres.String(*req.MedicalName), postgres.TimestampzT(time.Now())).
		WHERE(medicines.MedicationID.EQ(postgres.String(req.MedicationID))).
		Sql()
	_, err := r.pgPool.Exec(ctx, sql, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return nil
}

func (r *medicine) DeleteMedicine(ctx context.Context, filter model.DeleteMedicineFilter) (int64, error) {
	var (
		stmt string
		args []any
	)
	if filter.MedicationID != "" {
		stmt, args = table.PharmaSheetMedicines.DELETE().WHERE(table.PharmaSheetMedicines.MedicationID.EQ(postgres.String(filter.MedicationID))).Sql()

	} else if filter.WarehouseID != "" {
		stmt, args = table.PharmaSheetMedicines.DELETE().
			USING(table.PharmaSheetMedicineHouses).
			WHERE(table.PharmaSheetMedicines.MedicationID.EQ(table.PharmaSheetMedicineHouses.MedicationID).AND(table.PharmaSheetMedicineHouses.WarehouseID.EQ(postgres.String(filter.WarehouseID)))).Sql()

	} else {
		return 0, errors.New("filter is invalid")
	}
	result, err := r.pgPool.Exec(ctx, stmt, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return 0, err
	}
	return result.RowsAffected(), nil
}

func (r *medicine) GetMedicineHouses(ctx context.Context, filter model.FilterMedicineHouse) (houses []model.MedicineHouse, err error) {
	var condition postgres.BoolExpression
	if filter.MedicationID != "" {
		condition = table.PharmaSheetMedicineHouses.MedicationID.EQ(postgres.String(filter.MedicationID))
	} else if filter.WarehouseID != "" {
		condition = table.PharmaSheetMedicineHouses.WarehouseID.EQ(postgres.String(filter.WarehouseID))
	} else if filter.ID != uuid.Nil {
		condition = table.PharmaSheetMedicineHouses.ID.EQ(postgres.UUID(filter.ID))
	} else {
		return nil, errors.New("filter is invalid")
	}

	query, args := table.PharmaSheetMedicineHouses.
		SELECT(
			table.PharmaSheetMedicineHouses.ID,
			table.PharmaSheetMedicineHouses.WarehouseID,
			table.PharmaSheetMedicineHouses.MedicationID,
			table.PharmaSheetMedicineHouses.Locker,
			table.PharmaSheetMedicineHouses.Floor,
			table.PharmaSheetMedicineHouses.No,
			table.PharmaSheetMedicineHouses.Label,
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
		var house model.MedicineHouse
		err = rows.Scan(
			&house.ID,
			&house.WarehouseID,
			&house.MedicationID,
			&house.Locker,
			&house.Floor,
			&house.No,
			&house.Label,
		)
		if err != nil {
			logger.Context(ctx).Error(err)
			return nil, err
		}
		houses = append(houses, house)
	}

	return houses, nil
}

func (r *medicine) ListMedicineHouses(ctx context.Context, filter model.ListMedicineHouse) (data []model.MedicineHouse, total uint64, err error) {
	sortBy := filter.SortBy(table.PharmaSheetMedicineHouses.TableName() + ".medication_id ASC")
	sorts := strings.Split(sortBy, " ")
	order := sorts[1]
	switch sorts[0] {
	case "medication_id":
		sortBy = fmt.Sprintf("%s.medication_id %s", table.PharmaSheetMedicines.TableName(), order)
	case "address":
		sortBy = fmt.Sprintf("locker %s, floor %s, no %s", order, order, order)
	}

	condition := postgres.Bool(true)
	if filter.WarehouseID != "" {
		condition = condition.AND(table.PharmaSheetMedicineHouses.WarehouseID.EQ(postgres.String(filter.WarehouseID)))
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		search := postgres.String("%" + strings.ToLower(search) + "%")
		address := postgres.CONCAT(table.PharmaSheetMedicineHouses.Locker, postgres.String("-"), table.PharmaSheetMedicineHouses.Floor, postgres.String("-"), table.PharmaSheetMedicineHouses.No)
		condition = condition.AND(
			postgres.OR(
				postgres.LOWER(table.PharmaSheetMedicines.MedicalName).LIKE(search),
				postgres.LOWER(table.PharmaSheetMedicineHouses.MedicationID).LIKE(search),
				postgres.LOWER(table.PharmaSheetMedicineHouses.Label).LIKE(search),
				postgres.LOWER(address).LIKE(search),
			),
		)
	}

	query, args := table.PharmaSheetMedicineHouses.
		INNER_JOIN(table.PharmaSheetMedicines, table.PharmaSheetMedicines.MedicationID.EQ(table.PharmaSheetMedicineHouses.MedicationID)).
		SELECT(postgres.COUNT(postgres.DISTINCT(table.PharmaSheetMedicineHouses.ID)).AS("total")).
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

	query, args = table.PharmaSheetMedicineHouses.
		INNER_JOIN(table.PharmaSheetMedicines, table.PharmaSheetMedicines.MedicationID.EQ(table.PharmaSheetMedicineHouses.MedicationID)).
		SELECT(
			table.PharmaSheetMedicineHouses.ID,
			table.PharmaSheetMedicineHouses.WarehouseID,
			table.PharmaSheetMedicineHouses.MedicationID,
			table.PharmaSheetMedicineHouses.Locker,
			table.PharmaSheetMedicineHouses.Floor,
			table.PharmaSheetMedicineHouses.No,
			table.PharmaSheetMedicineHouses.Label,
			table.PharmaSheetMedicines.MedicalName,
		).
		WHERE(condition).
		LIMIT(int64(filter.Limit)).
		OFFSET(int64(filter.Offset)).
		GROUP_BY(
			table.PharmaSheetMedicineHouses.ID,
			table.PharmaSheetMedicineHouses.WarehouseID,
			table.PharmaSheetMedicineHouses.MedicationID,
			table.PharmaSheetMedicineHouses.Locker,
			table.PharmaSheetMedicineHouses.Floor,
			table.PharmaSheetMedicineHouses.No,
			table.PharmaSheetMedicineHouses.Label,
			table.PharmaSheetMedicines.MedicalName,
		).
		ORDER_BY(postgres.Raw(sortBy)).
		Sql()

	rows, err := r.pgPool.Query(ctx, query, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return nil, 0, err
	}
	defer rows.Close()

	var medicineHouses []model.MedicineHouse
	for rows.Next() {
		var medicineHouse model.MedicineHouse
		err = rows.Scan(
			&medicineHouse.ID,
			&medicineHouse.WarehouseID,
			&medicineHouse.MedicationID,
			&medicineHouse.Locker,
			&medicineHouse.Floor,
			&medicineHouse.No,
			&medicineHouse.Label,
			&medicineHouse.MedicalName,
		)
		if err != nil {
			logger.Context(ctx).Error(err)
			return nil, 0, err
		}
		medicineHouses = append(medicineHouses, medicineHouse)
	}

	return medicineHouses, total, nil
}

func (r *medicine) CreateMedicineHouse(ctx context.Context, req model.CreateMedicineHouseRequest) (string, error) {
	medicineHouses := table.PharmaSheetMedicineHouses

	now := time.Now()
	medicineHouse := genmodel.PharmaSheetMedicineHouses{
		ID:           uuid.MustParse(generator.UUID()),
		MedicationID: req.MedicationID,
		WarehouseID:  req.WarehouseID,
		Locker:       req.Locker,
		Floor:        req.Floor,
		No:           req.No,
		Label:        req.Label,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	sql, args := medicineHouses.
		INSERT(
			medicineHouses.ID,
			medicineHouses.MedicationID,
			medicineHouses.WarehouseID,
			medicineHouses.Locker,
			medicineHouses.Floor,
			medicineHouses.No,
			medicineHouses.Label,
			medicineHouses.CreatedAt,
			medicineHouses.UpdatedAt,
		).
		MODEL(medicineHouse).
		Sql()

	_, err := r.pgPool.Exec(ctx, sql, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return "", err
	}

	return medicineHouse.ID.String(), nil
}

func (r *medicine) UpdateMedicineHouse(ctx context.Context, req model.UpdateMedicineHouseRequest) error {
	label := postgres.NULL
	if req.Label != nil && *req.Label != "" {
		label = postgres.String(*req.Label)
	}

	medicineHouses := table.PharmaSheetMedicineHouses
	sql, args := medicineHouses.
		UPDATE(
			medicineHouses.MedicationID,
			medicineHouses.Locker,
			medicineHouses.Floor,
			medicineHouses.No,
			medicineHouses.Label,
			medicineHouses.UpdatedAt,
		).
		SET(
			postgres.String(req.MedicationID),
			postgres.String(req.Locker),
			postgres.Int32(req.Floor),
			postgres.Int32(req.No),
			label,
			postgres.TimestampzT(time.Now()),
		).
		WHERE(medicineHouses.ID.EQ(postgres.UUID(req.ID))).
		Sql()
	_, err := r.pgPool.Exec(ctx, sql, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return nil
}

func (r *medicine) DeleteMedicineHouse(ctx context.Context, filter model.DeleteMedicineHouseFilter) (int64, error) {
	var condition postgres.BoolExpression
	if filter.MedicationID != "" {
		condition = table.PharmaSheetMedicineHouses.MedicationID.EQ(postgres.String(filter.MedicationID))
	} else if filter.WarehouseID != "" {
		condition = table.PharmaSheetMedicineHouses.WarehouseID.EQ(postgres.String(filter.WarehouseID))
	} else if filter.ID != uuid.Nil {
		condition = table.PharmaSheetMedicineHouses.ID.EQ(postgres.UUID(filter.ID))
	} else {
		return 0, errors.New("filter is invalid")
	}
	stmt, args := table.PharmaSheetMedicineHouses.DELETE().WHERE(condition).Sql()
	result, err := r.pgPool.Exec(ctx, stmt, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return 0, err
	}
	return result.RowsAffected(), nil
}

func (r *medicine) GetMedicineBrands(ctx context.Context, req model.FilterMedicineBrand) (brands []model.MedicineBrand, err error) {
	var condition postgres.BoolExpression
	if req.MedicationID != "" {
		condition = table.PharmaSheetMedicineBrands.MedicationID.EQ(postgres.String(req.MedicationID))
	} else if req.WarehouseID != "" {
		condition = table.PharmaSheetMedicineHouses.WarehouseID.EQ(postgres.String(req.WarehouseID))
	} else if req.BrandID != uuid.Nil {
		condition = table.PharmaSheetMedicineBrands.ID.EQ(postgres.UUID(req.BrandID))
	} else {
		return nil, errors.New("filter is invalid")
	}

	query, args := table.PharmaSheetMedicineBrands.
		LEFT_JOIN(table.PharmaSheetMedicineHouses, table.PharmaSheetMedicineBrands.MedicationID.EQ(table.PharmaSheetMedicineHouses.MedicationID)).
		SELECT(
			table.PharmaSheetMedicineBrands.ID,
			table.PharmaSheetMedicineBrands.MedicationID,
			table.PharmaSheetMedicineBrands.TradeID,
			table.PharmaSheetMedicineBrands.TradeName,
			table.PharmaSheetMedicineBrands.BlisterImageURL,
			table.PharmaSheetMedicineBrands.TabletImageURL,
			table.PharmaSheetMedicineBrands.BoxImageURL,
		).
		WHERE(condition).
		GROUP_BY(
			table.PharmaSheetMedicineBrands.ID,
			table.PharmaSheetMedicineBrands.MedicationID,
			table.PharmaSheetMedicineBrands.TradeID,
			table.PharmaSheetMedicineBrands.TradeName,
			table.PharmaSheetMedicineBrands.BlisterImageURL,
			table.PharmaSheetMedicineBrands.TabletImageURL,
			table.PharmaSheetMedicineBrands.BoxImageURL,
		).
		Sql()

	rows, err := r.pgPool.Query(ctx, query, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var brand model.MedicineBrand
		err = rows.Scan(
			&brand.ID,
			&brand.MedicationID,
			&brand.TradeID,
			&brand.TradeName,
			&brand.BlisterImageURL,
			&brand.TabletImageURL,
			&brand.BoxImageURL,
		)
		if err != nil {
			logger.Context(ctx).Error(err)
			return nil, err
		}
		brands = append(brands, brand)
	}

	return brands, nil
}

func (r *medicine) GetMedicineWithBrands(ctx context.Context, filter model.FilterMedicineWithBrand) (data []model.Medicine, total uint64, err error) {
	sortBy := filter.SortBy(table.PharmaSheetMedicines.TableName() + ".medication_id ASC")
	sorts := strings.Split(sortBy, " ")
	order := sorts[1]
	switch sorts[0] {
	case "medication_id":
		sortBy = fmt.Sprintf("%s.medication_id %s", table.PharmaSheetMedicines.TableName(), order)
	}

	condition := postgres.Bool(true)
	if search := strings.TrimSpace(filter.Search); search != "" {
		search := postgres.String("%" + strings.ToLower(search) + "%")
		condition = condition.AND(
			postgres.OR(
				postgres.LOWER(table.PharmaSheetMedicineBrands.TradeID).LIKE(search),
				postgres.LOWER(table.PharmaSheetMedicineBrands.TradeName).LIKE(search),
				postgres.LOWER(table.PharmaSheetMedicines.MedicalName).LIKE(search),
				postgres.LOWER(table.PharmaSheetMedicines.MedicationID).LIKE(search),
			),
		)
	}

	query, args := table.PharmaSheetMedicines.
		LEFT_JOIN(table.PharmaSheetMedicineBrands, table.PharmaSheetMedicines.MedicationID.EQ(table.PharmaSheetMedicineBrands.MedicationID)).
		SELECT(postgres.COUNT(postgres.DISTINCT(table.PharmaSheetMedicines.MedicationID)).AS("total")).
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

	query, args = table.PharmaSheetMedicines.
		LEFT_JOIN(table.PharmaSheetMedicineBrands, table.PharmaSheetMedicines.MedicationID.EQ(table.PharmaSheetMedicineBrands.MedicationID)).
		SELECT(table.PharmaSheetMedicines.MedicationID, table.PharmaSheetMedicines.MedicalName).
		WHERE(condition).
		GROUP_BY(table.PharmaSheetMedicines.MedicationID, table.PharmaSheetMedicines.MedicalName).
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

	var medicationIDs []postgres.Expression
	for rows.Next() {
		var medicine model.Medicine
		err = rows.Scan(&medicine.MedicationID, &medicine.MedicalName)
		if err != nil {
			logger.Context(ctx).Error(err)
			return nil, 0, err
		}
		data = append(data, medicine)
		medicationIDs = append(medicationIDs, postgres.String(medicine.MedicationID))
	}

	if len(data) > 0 {
		query, args = table.PharmaSheetMedicineBrands.
			SELECT(
				table.PharmaSheetMedicineBrands.ID,
				table.PharmaSheetMedicineBrands.MedicationID,
				table.PharmaSheetMedicineBrands.TradeID,
				table.PharmaSheetMedicineBrands.TradeName,
				table.PharmaSheetMedicineBrands.BlisterImageURL,
				table.PharmaSheetMedicineBrands.TabletImageURL,
				table.PharmaSheetMedicineBrands.BoxImageURL,
			).
			WHERE(table.PharmaSheetMedicineBrands.MedicationID.IN(medicationIDs...)).
			ORDER_BY(table.PharmaSheetMedicineBrands.TradeID).
			Sql()

		rows, err := r.pgPool.Query(ctx, query, args...)
		if err != nil {
			logger.Context(ctx).Error(err)
			return nil, 0, err
		}
		defer rows.Close()

		for rows.Next() {
			var medicineBrand model.MedicineBrand
			err = rows.Scan(
				&medicineBrand.ID,
				&medicineBrand.MedicationID,
				&medicineBrand.TradeID,
				&medicineBrand.TradeName,
				&medicineBrand.BlisterImageURL,
				&medicineBrand.TabletImageURL,
				&medicineBrand.BoxImageURL,
			)
			if err != nil {
				logger.Context(ctx).Error(err)
				return nil, 0, err
			}
			for index := range data {
				if data[index].MedicationID == medicineBrand.MedicationID {
					data[index].Brands = append(data[index].Brands, medicineBrand)
					break
				}
			}
		}
	}

	return
}

func (r *medicine) CreateMedicineBrand(ctx context.Context, req model.CreateMedicineBrandRequest) (string, error) {
	medicineBrands := table.PharmaSheetMedicineBrands

	now := time.Now()
	medicineBrand := genmodel.PharmaSheetMedicineBrands{
		ID:              uuid.MustParse(generator.UUID()),
		TradeID:         req.TradeID,
		TradeName:       req.TradeName,
		MedicationID:    req.MedicationID,
		BlisterImageURL: req.BlisterImageURL,
		TabletImageURL:  req.TabletImageURL,
		BoxImageURL:     req.BoxImageURL,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	sql, args := medicineBrands.
		INSERT(
			medicineBrands.ID,
			medicineBrands.MedicationID,
			medicineBrands.TradeID,
			medicineBrands.TradeName,
			medicineBrands.BlisterImageURL,
			medicineBrands.TabletImageURL,
			medicineBrands.BoxImageURL,
			medicineBrands.CreatedAt,
			medicineBrands.UpdatedAt,
		).
		MODEL(medicineBrand).
		Sql()

	_, err := r.pgPool.Exec(ctx, sql, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return "", err
	}

	return medicineBrand.ID.String(), nil
}

func (r *medicine) UpdateMedicineBrand(ctx context.Context, req model.UpdateMedicineBrandRequest) error {
	medicineBrands := table.PharmaSheetMedicineBrands

	columnNames := postgres.ColumnList{medicineBrands.UpdatedAt, medicineBrands.TradeName}
	columnValues := []any{postgres.TimestampzT(time.Now())}

	if req.TradeName != nil {
		columnValues = append(columnValues, *req.TradeName)
	} else {
		columnValues = append(columnValues, postgres.NULL)
	}

	if req.BlisterImageURL != nil && *req.BlisterImageURL == "null" {
		columnNames = append(columnNames, medicineBrands.BlisterImageURL)
		columnValues = append(columnValues, postgres.NULL)
	} else if req.BlisterImageURL != nil {
		columnNames = append(columnNames, medicineBrands.BlisterImageURL)
		columnValues = append(columnValues, postgres.String(*req.BlisterImageURL))
	}

	if req.TabletImageURL != nil && *req.TabletImageURL == "null" {
		columnNames = append(columnNames, medicineBrands.TabletImageURL)
		columnValues = append(columnValues, postgres.NULL)
	} else if req.TabletImageURL != nil {
		columnNames = append(columnNames, medicineBrands.TabletImageURL)
		columnValues = append(columnValues, postgres.String(*req.TabletImageURL))
	}

	if req.BoxImageURL != nil && *req.BoxImageURL == "null" {
		columnNames = append(columnNames, medicineBrands.BoxImageURL)
		columnValues = append(columnValues, postgres.NULL)
	} else if req.BoxImageURL != nil {
		columnNames = append(columnNames, medicineBrands.BoxImageURL)
		columnValues = append(columnValues, postgres.String(*req.BoxImageURL))
	}

	sql, args := medicineBrands.
		UPDATE(columnNames).
		SET(columnValues[0], columnValues[1:]...).
		WHERE(medicineBrands.ID.EQ(postgres.UUID(req.BrandID))).
		Sql()
	_, err := r.pgPool.Exec(ctx, sql, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return nil
}

func (r *medicine) DeleteMedicineBrand(ctx context.Context, filter model.DeleteMedicineBrandFilter) (int64, error) {
	var condition postgres.BoolExpression
	if filter.MedicationID != "" {
		condition = table.PharmaSheetMedicineBrands.MedicationID.EQ(postgres.String(filter.MedicationID))
	} else if filter.TradeID != "" {
		condition = table.PharmaSheetMedicineBrands.TradeID.EQ(postgres.String(filter.TradeID))
	} else if filter.BrandID != uuid.Nil {
		condition = table.PharmaSheetMedicineBrands.ID.EQ(postgres.UUID(filter.BrandID))
	} else {
		return 0, errors.New("filter is invalid")
	}
	stmt, args := table.PharmaSheetMedicineBrands.DELETE().WHERE(condition).Sql()
	result, err := r.pgPool.Exec(ctx, stmt, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return 0, err
	}
	return result.RowsAffected(), nil
}

func (r *medicine) GetMedicineBlisterChangeDateHistory(ctx context.Context, id uuid.UUID) (medicineBlisterDateHistory model.MedicineBlisterDateHistory, err error) {
	query, args := table.PharmaSheetMedicineBlisterDateHistories.
		SELECT(
			table.PharmaSheetMedicineBlisterDateHistories.ID,
			table.PharmaSheetMedicineBlisterDateHistories.WarehouseID,
			table.PharmaSheetMedicineBlisterDateHistories.MedicationID,
			table.PharmaSheetMedicineBlisterDateHistories.BrandID,
			table.PharmaSheetMedicineBlisterDateHistories.BlisterChangeDate,
		).
		WHERE(table.PharmaSheetMedicineBlisterDateHistories.ID.EQ(postgres.UUID(id))).
		Sql()

	err = r.pgPool.QueryRow(ctx, query, args...).Scan(
		&medicineBlisterDateHistory.ID,
		&medicineBlisterDateHistory.WarehouseID,
		&medicineBlisterDateHistory.MedicationID,
		&medicineBlisterDateHistory.BrandID,
		&medicineBlisterDateHistory.BlisterChangeDate,
	)
	if err != nil {
		logger.Context(ctx).Error(err)
		return medicineBlisterDateHistory, err
	}

	return medicineBlisterDateHistory, nil
}

func (r *medicine) ListMedicineBlisterChangeDateHistory(ctx context.Context, filter model.FilterMedicineBrandBlisterDateHistory) ([]model.MedicineBlisterDateHistory, error) {
	condition := postgres.Bool(true)
	validCondition := false
	if filter.MedicationID != nil {
		condition = condition.AND(table.PharmaSheetMedicineBlisterDateHistories.MedicationID.EQ(postgres.String(*filter.MedicationID)))
		validCondition = true
	}
	if filter.BrandID != nil {
		condition = condition.AND(table.PharmaSheetMedicineBlisterDateHistories.BrandID.IS_NOT_NULL()).AND(table.PharmaSheetMedicineBlisterDateHistories.BrandID.EQ(postgres.UUID(filter.BrandID)))
		validCondition = true
	}
	if !validCondition {
		return nil, errors.New("filter is invalid")
	}

	query, args := table.PharmaSheetMedicineBlisterDateHistories.
		SELECT(
			table.PharmaSheetMedicineBlisterDateHistories.ID,
			table.PharmaSheetMedicineBlisterDateHistories.WarehouseID,
			table.PharmaSheetMedicineBlisterDateHistories.MedicationID,
			table.PharmaSheetMedicineBlisterDateHistories.BrandID,
			table.PharmaSheetMedicineBlisterDateHistories.BlisterChangeDate,
		).
		WHERE(condition).
		Sql()

	rows, err := r.pgPool.Query(ctx, query, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return nil, err
	}
	defer rows.Close()

	var medicineBlisterDateHistories []model.MedicineBlisterDateHistory
	for rows.Next() {
		var medicineBlisterDateHistory model.MedicineBlisterDateHistory
		err = rows.Scan(
			&medicineBlisterDateHistory.ID,
			&medicineBlisterDateHistory.WarehouseID,
			&medicineBlisterDateHistory.MedicationID,
			&medicineBlisterDateHistory.BrandID,
			&medicineBlisterDateHistory.BlisterChangeDate,
		)
		if err != nil {
			logger.Context(ctx).Error(err)
			return nil, err
		}
		medicineBlisterDateHistories = append(medicineBlisterDateHistories, medicineBlisterDateHistory)
	}

	return medicineBlisterDateHistories, nil
}

func (r *medicine) CreateMedicineBlisterChangeDateHistory(ctx context.Context, req model.CreateMedicineBlisterChangeDateHistoryRequest) (string, error) {
	medcineHistoryTable := table.PharmaSheetMedicineBlisterDateHistories
	medicineHistory := genmodel.PharmaSheetMedicineBlisterDateHistories{
		ID:                uuid.MustParse(generator.UUID()),
		WarehouseID:       req.WarehouseID,
		MedicationID:      req.MedicationID,
		BrandID:           req.BrandID,
		BlisterChangeDate: req.BlisterChangeDate,
		CreatedAt:         time.Now(),
	}

	sql, args := medcineHistoryTable.
		INSERT(
			medcineHistoryTable.ID,
			medcineHistoryTable.WarehouseID,
			medcineHistoryTable.MedicationID,
			medcineHistoryTable.BrandID,
			medcineHistoryTable.BlisterChangeDate,
			medcineHistoryTable.CreatedAt,
		).
		MODEL(medicineHistory).
		Sql()

	_, err := r.pgPool.Exec(ctx, sql, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return "", err
	}

	return medicineHistory.ID.String(), nil
}

func (r *medicine) DeleteMedicineBlisterChangeDateHistory(ctx context.Context, id uuid.UUID) error {
	stmt, args := table.PharmaSheetMedicineBlisterDateHistories.DELETE().WHERE(table.PharmaSheetMedicineBlisterDateHistories.ID.EQ(postgres.UUID(id))).Sql()
	_, err := r.pgPool.Exec(ctx, stmt, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}
	return nil
}
