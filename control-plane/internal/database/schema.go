package database

import (
	"context"
	_ "embed"

	"github.com/jackc/pgx/v5/pgxpool"
)

// schema is the initial database shape. Versioned migrations begin only after
// an externally released schema exists.
//
//go:embed schema.sql
var schema string

func InitializeSchema(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, schema)
	return err
}
