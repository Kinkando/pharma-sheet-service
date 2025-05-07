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
	"github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/model"
	"github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/table"
	"github.com/kinkando/pharma-sheet-service/pkg/generator"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
)

type User interface {
	GetUser(ctx context.Context, user model.PharmaSheetUsers) (model.PharmaSheetUsers, error)
	CreateUser(ctx context.Context, user model.PharmaSheetUsers) (string, error)
	UpdateUser(ctx context.Context, user model.PharmaSheetUsers) error
}

type user struct {
	pgPool *pgxpool.Pool
}

func NewUserRepository(pgPool *pgxpool.Pool) User {
	return &user{
		pgPool: pgPool,
	}
}

func (r *user) GetUser(ctx context.Context, filter model.PharmaSheetUsers) (user model.PharmaSheetUsers, err error) {
	users := table.PharmaSheetUsers

	var conditions []postgres.BoolExpression
	if filter.UserID != uuid.Nil {
		conditions = append(conditions, users.UserID.EQ(postgres.UUID(filter.UserID)))
	}
	if filter.FirebaseUID != nil {
		conditions = append(conditions, users.FirebaseUID.EQ(postgres.String(*filter.FirebaseUID)))
	}
	if filter.Email != "" {
		conditions = append(conditions, postgres.LOWER(users.Email).EQ(postgres.String(strings.ToLower(filter.Email))))
	}

	if len(conditions) == 0 {
		err = errors.New("filter is invalid")
		logger.Context(ctx).Error(err)
		return
	}

	query, args := users.SELECT(users.UserID, users.FirebaseUID, users.Email, users.DisplayName, users.ImageURL).WHERE(postgres.OR(conditions...)).Sql()
	err = r.pgPool.QueryRow(ctx, query, args...).Scan(&user.UserID, &user.FirebaseUID, &user.Email, &user.DisplayName, &user.ImageURL)
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}

	return user, nil
}

func (r *user) CreateUser(ctx context.Context, user model.PharmaSheetUsers) (string, error) {
	users := table.PharmaSheetUsers

	user.UserID = uuid.MustParse(generator.UUID())
	user.CreatedAt = time.Now()

	sql, args := users.INSERT(users.UserID, users.FirebaseUID, users.Email, users.ImageURL, users.CreatedAt).MODEL(user).Sql()
	_, err := r.pgPool.Exec(ctx, sql, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return "", err
	}

	return user.UserID.String(), nil
}

func (r *user) UpdateUser(ctx context.Context, user model.PharmaSheetUsers) error {
	users := table.PharmaSheetUsers

	columnNames := postgres.ColumnList{users.UpdatedAt}
	columnValues := []any{postgres.TimestampzT(time.Now())}

	if user.FirebaseUID != nil {
		columnNames = append(columnNames, users.FirebaseUID)
		columnValues = append(columnValues, postgres.String(*user.FirebaseUID))
	}

	if user.ImageURL != nil {
		columnNames = append(columnNames, users.ImageURL)
		columnValues = append(columnValues, postgres.String(*user.ImageURL))
	}

	if user.DisplayName != nil && *user.DisplayName != "" {
		columnNames = append(columnNames, users.DisplayName)
		columnValues = append(columnValues, postgres.String(*user.DisplayName))
	} else {
		columnNames = append(columnNames, users.DisplayName)
		columnValues = append(columnValues, postgres.NULL)
	}

	if len(columnValues) <= 1 {
		err := fmt.Errorf("no specific column would be updated")
		logger.Context(ctx).Error(err)
		return err
	}

	sql, args := users.
		UPDATE(columnNames).
		SET(columnValues[0], columnValues[1:]...).
		WHERE(users.UserID.EQ(postgres.UUID(user.UserID))).
		Sql()
	_, err := r.pgPool.Exec(ctx, sql, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return nil
}
