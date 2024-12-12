package postgresql

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Commit(ctx context.Context, pgp *pgxpool.Pool, txFunc func(context.Context, pgx.Tx) error) error {
	txCtx, txCtxCancel := context.WithTimeout(ctx, 10*time.Second)
	defer txCtxCancel()

	tx, err := pgp.Begin(txCtx)
	if err != nil {
		return err
	}

	err = txFunc(txCtx, tx)
	if err != nil {
		if errRollback := tx.Rollback(txCtx); errRollback != nil {
			return fmt.Errorf("%w: postgresql: rollback: %v", err, errRollback)
		}
		return err
	}

	return tx.Commit(txCtx)
}
