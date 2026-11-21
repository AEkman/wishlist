package store

import (
	"context"
	"database/sql"
	"errors"
)

// MoveWish moves a wish to a one-based rank, retaining the order of other wishes.
func (s *Store) MoveWish(ctx context.Context, wishlistID, wishID string, rank int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.lockWishlist(ctx, tx, wishlistID); err != nil {
		return err
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
