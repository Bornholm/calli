package store

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitemigration"
	"zombiezen.com/go/sqlite/sqlitex"
)

type Store struct {
	pool *sqlitemigration.Pool
}

var schema = sqlitemigration.Schema{
	Migrations: flatten(
		userMigrations,
		groupMigrations,
		ruleMigrations,
	),
	RepeatableMigration: strings.Join(
		flatten(
			repeatableGroupMigrations,
		), " ",
	),
}

func (s *Store) HealthCheck(ctx context.Context) error {
	conn, err := s.pool.Take(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	defer s.pool.Put(conn)

	if err := s.pool.CheckHealth(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (s *Store) Do(ctx context.Context, fn func(conn *sqlite.Conn) error) error {
	conn, err := s.pool.Take(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	defer s.pool.Put(conn)

	if err := fn(conn); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (s *Store) Tx(ctx context.Context, fn func(conn *sqlite.Conn) error) error {
	return errors.WithStack(s.Do(ctx, func(conn *sqlite.Conn) (err error) {
		defer sqlitex.Save(conn)(&err)
		err = fn(conn)
		return errors.WithStack(err)
	}))
}

func NewStore(uri string) *Store {
	pool := sqlitemigration.NewPool(uri, schema, sqlitemigration.Options{
		Flags: sqlite.OpenCreate | sqlite.OpenReadWrite | sqlite.OpenWAL,
		PrepareConn: func(conn *sqlite.Conn) error {
			return sqlitex.ExecuteTransient(conn, "PRAGMA foreign_keys = on", nil)
		},
	})

	return &Store{
		pool: pool,
	}
}

func flatten[T any](slices ...[]T) []T {
	flattened := make([]T, 0)
	for _, s := range slices {
		flattened = append(flattened, s...)
	}
	return flattened
}
