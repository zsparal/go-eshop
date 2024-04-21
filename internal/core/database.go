package core

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrCouldNotStartTransaction = errors.New("could not start transaction")
var ErrCouldNotCommitTransaction = errors.New("could not commit transaction")

type DatabaseConnection interface {
	Begin(context.Context) (pgx.Tx, error)
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

func InTransaction[T any](ctx context.Context, conn DatabaseConnection, fn func(DatabaseConnection) (T, error)) (T, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return *new(T), ErrCouldNotStartTransaction
	}

	defer tx.Rollback(ctx)
	result, err := fn(tx)
	if err != nil {
		return *new(T), err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return *new(T), ErrCouldNotCommitTransaction
	}

	return result, nil
}
