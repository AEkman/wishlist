package store

import (
	"context"
	"database/sql"
	"errors"
	"slices"
)

// MoveWish moves a wish to a one-based rank, retaining the order of other wishes.
func (s *Store) MoveWish(ctx context.Context, wishlistID, wishID string, rank int) error {
	return s.moveWish(ctx, wishlistID, wishID, rank, nil)
}

// PlaceWish moves a wish only if the complete list still has the expected order.
func (s *Store) PlaceWish(ctx context.Context, wishlistID, wishID string, rank int, expected []string) error {
	if len(expected) == 0 {
		return ErrInvalid
	}
	return s.moveWish(ctx, wishlistID, wishID, rank, expected)
}

func (s *Store) moveWish(ctx context.Context, wishlistID, wishID string, rank int, expected []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.lockWishlist(ctx, tx, wishlistID); err != nil {
		return err
	}
	if expected != nil {
		rows, err := tx.QueryContext(ctx, s.bind("SELECT id FROM wishes WHERE wishlist_id = ? ORDER BY position"), wishlistID)
		if err != nil {
			return err
		}
		var actual []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			actual = append(actual, id)
		}
		readErr := rows.Err()
		closeErr := rows.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return err
		}
		if !slices.Equal(actual, expected) {
			return ErrInvalid
		}
	}
	var position, count int
	err = tx.QueryRowContext(ctx, s.bind(`SELECT position, (SELECT COUNT(*) FROM wishes WHERE wishlist_id = ?) FROM wishes WHERE wishlist_id = ? AND id = ?`), wishlistID, wishlistID, wishID).Scan(&position, &count)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if rank < 1 || rank > count {
		return ErrInvalid
	}
	target := rank - 1
	if target < position {
		_, err = tx.ExecContext(ctx, s.bind(`UPDATE wishes SET position = CASE WHEN id = ? THEN ? ELSE position + 1 END WHERE wishlist_id = ? AND position BETWEEN ? AND ?`), wishID, target, wishlistID, target, position)
	} else if target > position {
		_, err = tx.ExecContext(ctx, s.bind(`UPDATE wishes SET position = CASE WHEN id = ? THEN ? ELSE position - 1 END WHERE wishlist_id = ? AND position BETWEEN ? AND ?`), wishID, target, wishlistID, position, target)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
