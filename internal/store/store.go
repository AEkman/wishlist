package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

var (
	ErrNotFound = errors.New("not found")
	ErrInvalid  = errors.New("invalid input")
)

type Store struct {
	db *sql.DB
}

type Child struct {
	ID   string
	Name string
}

const schema = `
	CREATE TABLE IF NOT EXISTS children (
		id TEXT NOT NULL PRIMARY KEY,
		name TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS wishlists (
		id TEXT NOT NULL PRIMARY KEY,
		child_id TEXT NOT NULL REFERENCES children(id),
		title TEXT NOT NULL,
		share_id TEXT NOT NULL UNIQUE
	);
	CREATE INDEX IF NOT EXISTS wishlists_child_id ON wishlists(child_id);
	CREATE TABLE IF NOT EXISTS wishes (
		id TEXT NOT NULL PRIMARY KEY,
		wishlist_id TEXT NOT NULL REFERENCES wishlists(id),
		title TEXT NOT NULL,
		description TEXT NOT NULL,
		media_type TEXT NOT NULL,
		image BLOB NOT NULL,
		position INTEGER NOT NULL,
		buyer TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS wishes_rank ON wishes(wishlist_id, position);
`

func Open(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// Keep one connection for the in-memory database and serialize writes.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize wishlist database: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) CreateChild(ctx context.Context, name string) (Child, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Child{}, ErrInvalid
	}
	id, err := newID()
	if err != nil {
		return Child{}, err
	}
	child := Child{ID: id, Name: name}
	_, err = s.db.ExecContext(ctx, "INSERT INTO children (id, name) VALUES (?, ?)", child.ID, child.Name)
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
	rows, err := s.db.QueryContext(ctx, "SELECT id, name FROM children ORDER BY rowid")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var children []Child
	for rows.Next() {
		var child Child
		if err := rows.Scan(&child.ID, &child.Name); err != nil {
			return nil, err
		}
		children = append(children, child)
	}
	return children, rows.Err()
}

func (s *Store) Child(ctx context.Context, id string) (Child, error) {
	var child Child
	err := s.db.QueryRowContext(ctx, "SELECT id, name FROM children WHERE id = ?", id).Scan(&child.ID, &child.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return Child{}, ErrNotFound
	}
	return child, err
}
