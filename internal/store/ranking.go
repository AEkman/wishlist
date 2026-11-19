package store

import (
	"context"
	"database/sql"
	"errors"
)

func (s *Store) Pick(ctx context.Context, wishlistID, winnerID, loserID string) error {
	if winnerID == loserID {
		return ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var winnerPosition, loserPosition int
	err = tx.QueryRowContext(ctx, `
		SELECT winner.position, loser.position
		FROM wishes AS winner JOIN wishes AS loser ON winner.wishlist_id = loser.wishlist_id
		WHERE winner.wishlist_id = ? AND winner.id = ? AND loser.id = ?
	`, wishlistID, winnerID, loserID).Scan(&winnerPosition, &loserPosition)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if winnerPosition < loserPosition {
		return nil
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE wishes
		SET position = CASE WHEN id = ? THEN ? ELSE position + 1 END
		WHERE wishlist_id = ? AND position BETWEEN ? AND ?
	`, winnerID, loserPosition, wishlistID, loserPosition, winnerPosition)
	if err != nil {
		return err
	}
	return tx.Commit()
}
