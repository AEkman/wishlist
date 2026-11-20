package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"io/fs"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed migrations/postgres/*.sql
var postgresMigrations embed.FS

func initializePostgres(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	migrations, err := fs.Sub(postgresMigrations, "migrations/postgres")
	if err != nil {
		return err
	}
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations, goose.WithSessionLocker(locker))
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}

// bind converts the application's fixed SQL queries to PostgreSQL placeholders.
func (s *Store) bind(query string) string {
	if !s.postgres {
		return query
	}
	var bound strings.Builder
	parameter := 0
	for _, r := range query {
		if r == '?' {
			parameter++
			bound.WriteByte('$')
			bound.WriteString(strconv.Itoa(parameter))
		} else {
			bound.WriteRune(r)
		}
	}
	return bound.String()
}

func (s *Store) execContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.db.ExecContext(ctx, s.bind(query), args...)
}

func (s *Store) queryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, s.bind(query), args...)
}

func (s *Store) queryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return s.db.QueryRowContext(ctx, s.bind(query), args...)
}

// lockWishlist serializes rank changes and additions across PostgreSQL connections.
func (s *Store) lockWishlist(ctx context.Context, tx *sql.Tx, listID string) error {
	if !s.postgres {
		return nil
	}
	var id string
	err := tx.QueryRowContext(ctx, "SELECT id FROM wishlists WHERE id = $1 FOR UPDATE", listID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Store) wishlistOrder() string {
	if s.postgres {
		return "created_order"
	}
	return "rowid"
}
