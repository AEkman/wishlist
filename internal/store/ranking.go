package store

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
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
	var winnerTitle, loserTitle string
	err = tx.QueryRowContext(ctx, `
		SELECT winner.position, loser.position, winner.title, loser.title
		FROM wishes AS winner JOIN wishes AS loser ON winner.wishlist_id = loser.wishlist_id
		WHERE winner.wishlist_id = ? AND winner.id = ? AND loser.id = ?
	`, wishlistID, winnerID, loserID).Scan(&winnerPosition, &loserPosition, &winnerTitle, &loserTitle)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	winnerAfter, loserAfter := winnerPosition, loserPosition
	if winnerPosition > loserPosition {
		_, err = tx.ExecContext(ctx, `
			UPDATE wishes
			SET position = CASE WHEN id = ? THEN ? ELSE position + 1 END
			WHERE wishlist_id = ? AND position BETWEEN ? AND ?
		`, winnerID, loserPosition, wishlistID, loserPosition, winnerPosition)
		if err != nil {
			return err
		}
		winnerAfter, loserAfter = loserPosition, loserPosition+1
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	slog.DebugContext(ctx, "ranking_pick",
		"list_id", wishlistID, "winner", winnerTitle, "loser", loserTitle,
		"winner_rank_before", winnerPosition+1, "winner_rank_after", winnerAfter+1,
		"loser_rank_before", loserPosition+1, "loser_rank_after", loserAfter+1,
	)
	return nil
}
