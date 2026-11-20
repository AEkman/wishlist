package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

var (
	ErrNotFound = errors.New("not found")
	ErrInvalid  = errors.New("invalid input")
)

type Store struct {
	db       *sql.DB
	postgres bool
}

type Child struct {
	ID     string
	Name   string
	Number int64
}

func Open(dsn string) (*Store, error) {
	postgres := strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://")
	driver := "sqlite"
	if postgres {
		driver = "pgx"
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	// Keep one connection for the in-memory database and serialize writes.
	db.SetMaxOpenConns(1)
	initializeDB := initialize
	if postgres {
		initializeDB = initializePostgres
	}
	if err := initializeDB(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize wishlist database: %w", err)
	}
	return &Store{db: db, postgres: postgres}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) CreateChild(ctx context.Context, name string) (Child, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Child{}, ErrInvalid
	}
	id := ChildID(name)
	child := Child{ID: id, Name: name}
	query := "INSERT INTO children (id, name, number) SELECT ?, ?, (SELECT COALESCE(MAX(number), 0) + 1 FROM children) WHERE NOT EXISTS (SELECT 1 FROM children WHERE id = ?) RETURNING number"
	args := []any{id, name, id}
	if s.postgres {
		query = "INSERT INTO children (id, name) VALUES (?, ?) ON CONFLICT (id) DO NOTHING RETURNING number"
		args = []any{id, name}
	}
	err := s.queryRowContext(ctx, query, args...).Scan(&child.Number)
	if errors.Is(err, sql.ErrNoRows) {
		return Child{}, ErrInvalid
	}
	return child, err
}

func newID() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func requireAffectedRow(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Children(ctx context.Context) ([]Child, error) {
	rows, err := s.queryContext(ctx, "SELECT id, name, number FROM children ORDER BY number")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var children []Child
	for rows.Next() {
		var child Child
		if err := rows.Scan(&child.ID, &child.Name, &child.Number); err != nil {
			return nil, err
		}
		children = append(children, child)
	}
	return children, rows.Err()
}

func (s *Store) Child(ctx context.Context, id string) (Child, error) {
	var child Child
	err := s.queryRowContext(ctx, "SELECT id, name, number FROM children WHERE id = ?", id).Scan(&child.ID, &child.Name, &child.Number)
	if errors.Is(err, sql.ErrNoRows) {
		return Child{}, ErrNotFound
	}
	return child, err
}

func (s *Store) ChildByNumber(ctx context.Context, number int64) (Child, error) {
	var child Child
	err := s.queryRowContext(ctx, "SELECT id, name, number FROM children WHERE number = ?", number).Scan(&child.ID, &child.Name, &child.Number)
	if errors.Is(err, sql.ErrNoRows) {
		return Child{}, ErrNotFound
	}
	return child, err
}

// ChildID creates a readable, stable key from a child's name.
func ChildID(name string) string {
	var slug strings.Builder
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			slug.WriteRune(r)
		} else if slug.Len() > 0 {
			slug.WriteByte('-')
		}
	}
	id := strings.Trim(slug.String(), "-")
	if id == "" {
		return "barn"
	}
	return id
}
