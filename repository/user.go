package repository

import (
	"context"
	"errors"
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
	GetUser(ctx context.Context, user model.Users) (model.Users, error)
	CreateUser(ctx context.Context, user model.Users) (string, error)
}

type user struct {
	pgPool *pgxpool.Pool
}

func NewUserRepository(pgPool *pgxpool.Pool) User {
	return &user{
		pgPool: pgPool,
	}
}

func (r *user) GetUser(ctx context.Context, filter model.Users) (user model.Users, err error) {
	users := table.Users

	var condition postgres.BoolExpression
	if filter.UserID != uuid.Nil {
		condition = users.UserID.EQ(postgres.UUID(filter.UserID))
	} else if filter.FirebaseUID != "" {
		condition = users.FirebaseUID.EQ(postgres.String(filter.FirebaseUID))
	} else if filter.Email != "" {
		condition = users.Email.EQ(postgres.String(filter.Email))
	} else {
		err = errors.New("filter must be provided")
		logger.Context(ctx).Error(err)
		return
	}

	query, args := users.SELECT(users.UserID, users.FirebaseUID, users.Email).WHERE(condition).Sql()
	err = r.pgPool.QueryRow(ctx, query, args...).Scan(&user.UserID, &user.FirebaseUID, &user.Email)
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}

	return user, nil
}

func (r *user) CreateUser(ctx context.Context, user model.Users) (string, error) {
	users := table.Users

	user.UserID = uuid.MustParse(generator.UUID())
	user.CreatedAt = time.Now()

	sql, args := users.INSERT(users.UserID, users.FirebaseUID, users.Email, users.CreatedAt).MODEL(user).Sql()
	_, err := r.pgPool.Exec(ctx, sql, args...)
	if err != nil {
		logger.Context(ctx).Error(err)
		return "", err
	}

	return user.UserID.String(), nil
}
