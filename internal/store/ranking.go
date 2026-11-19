package store

import (
	"context"
	"database/sql"
	"errors"
)

type PickResult struct {
	WinnerBefore int `json:"winnerBefore"`
	WinnerAfter  int `json:"winnerAfter"`
	LoserBefore  int `json:"loserBefore"`
	LoserAfter   int `json:"loserAfter"`
}

func (s *Store) Pick(ctx context.Context, wishlistID, winnerID, loserID string) (PickResult, error) {
	if winnerID == loserID {
		return PickResult{}, ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PickResult{}, err
	}
	defer tx.Rollback()

	var winnerPosition, loserPosition int
	err = tx.QueryRowContext(ctx, `
		SELECT winner.position, loser.position
		FROM wishes AS winner JOIN wishes AS loser ON winner.wishlist_id = loser.wishlist_id
		WHERE winner.wishlist_id = ? AND winner.id = ? AND loser.id = ?
	`, wishlistID, winnerID, loserID).Scan(&winnerPosition, &loserPosition)
	if errors.Is(err, sql.ErrNoRows) {
		return PickResult{}, ErrNotFound
	}
	if err != nil {
		return PickResult{}, err
	}
	winnerAfter, loserAfter := winnerPosition, loserPosition
	if winnerPosition > loserPosition {
		_, err = tx.ExecContext(ctx, `
			UPDATE wishes
			SET position = CASE WHEN id = ? THEN ? ELSE position + 1 END
			WHERE wishlist_id = ? AND position BETWEEN ? AND ?
		`, winnerID, loserPosition, wishlistID, loserPosition, winnerPosition)
		if err != nil {
			return PickResult{}, err
		}
		winnerAfter, loserAfter = loserPosition, loserPosition+1
	}
	if err := tx.Commit(); err != nil {
		return PickResult{}, err
	}
	return PickResult{
		WinnerBefore: winnerPosition + 1,
		WinnerAfter:  winnerAfter + 1,
		LoserBefore:  loserPosition + 1,
		LoserAfter:   loserAfter + 1,
	}, nil
}
