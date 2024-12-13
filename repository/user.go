package repository

import (
	"context"
	"errors"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kinkando/pharma-sheet/.gen/pharma_sheet/public/model"
	"github.com/kinkando/pharma-sheet/.gen/pharma_sheet/public/table"
	"github.com/kinkando/pharma-sheet/pkg/logger"
)

type User interface {
	GetUser(ctx context.Context, user model.Users) (model.Users, error)
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
	if filter.UserID.String() != "" {
		condition = users.UserID.EQ(postgres.String(filter.UserID.String()))
	} else if filter.FirebaseUID != "" {
		condition = users.FirebaseUID.EQ(postgres.String(filter.FirebaseUID))
	} else {
		err = errors.New("filter must be provided")
		logger.Context(ctx).Error(err)
		return
	}

	query, args := users.SELECT(users.UserID, users.FirebaseUID).WHERE(condition).Sql()
	err = r.pgPool.QueryRow(ctx, query, args...).Scan(&user.UserID, &user.FirebaseUID)
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}

	return user, nil
}
