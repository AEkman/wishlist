package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

type Wish struct {
	ID          string
	WishlistID  string
	Title       string
	Description string
	Buyer       string
}

const wishColumns = "id, wishlist_id, title, description, buyer"

func (s *Store) UpdateWish(ctx context.Context, wishlistID, wishID, title, description, mediaType string, image []byte) (Wish, error) {
	title = strings.TrimSpace(title)
	if title == "" || (image != nil && (mediaType == "" || len(image) == 0)) {
		return Wish{}, ErrInvalid
	}
	query := "UPDATE wishes SET title = ?, description = ?"
	args := []any{title, description}
	if image != nil {
		query += ", media_type = ?, image = ?"
		args = append(args, mediaType, image)
	}
	query += " WHERE wishlist_id = ? AND id = ? RETURNING " + wishColumns
	args = append(args, wishlistID, wishID)
	return scanWish(s.db.QueryRowContext(ctx, query, args...))
}

func (s *Store) CreateWish(ctx context.Context, wishlistID, title, description, mediaType string, image []byte) (Wish, error) {
	title = strings.TrimSpace(title)
	if title == "" || mediaType == "" || len(image) == 0 {
		return Wish{}, ErrInvalid
	}
	id, err := newID()
	if err != nil {
		return Wish{}, err
	}
	err = requireAffectedRow(s.db.ExecContext(ctx, `
		INSERT INTO wishes (id, wishlist_id, title, description, media_type, image, position)
		SELECT ?, list.id, ?, ?, ?, ?,
			COALESCE((SELECT MAX(position) + 1 FROM wishes WHERE wishlist_id = list.id), 0)
		FROM wishlists AS list WHERE list.id = ?
	`, id, title, description, mediaType, image, wishlistID))
	if err != nil {
		return Wish{}, err
	}
	return Wish{ID: id, WishlistID: wishlistID, Title: title, Description: description}, nil
}

func (s *Store) Wishes(ctx context.Context, wishlistID string) ([]Wish, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+wishColumns+" FROM wishes WHERE wishlist_id = ? ORDER BY position", wishlistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var wishes []Wish
	for rows.Next() {
		var wish Wish
		if err := rows.Scan(&wish.ID, &wish.WishlistID, &wish.Title, &wish.Description, &wish.Buyer); err != nil {
			return nil, err
		}
		wishes = append(wishes, wish)
	}
	return wishes, rows.Err()
}

func (s *Store) Wish(ctx context.Context, wishlistID, wishID string) (Wish, error) {
	return scanWish(s.db.QueryRowContext(ctx,
		"SELECT "+wishColumns+" FROM wishes WHERE wishlist_id = ? AND id = ?", wishlistID, wishID))
}

func (s *Store) Image(ctx context.Context, wishID string) (string, []byte, error) {
	var mediaType string
	var image []byte
	err := s.db.QueryRowContext(ctx, "SELECT media_type, image FROM wishes WHERE id = ?", wishID).Scan(&mediaType, &image)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, ErrNotFound
	}
	return mediaType, image, err
}

func scanWish(row *sql.Row) (Wish, error) {
	var wish Wish
	err := row.Scan(&wish.ID, &wish.WishlistID, &wish.Title, &wish.Description, &wish.Buyer)
	if errors.Is(err, sql.ErrNoRows) {
		return Wish{}, ErrNotFound
	}
	return wish, err
}
