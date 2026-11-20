package store

import (
	"context"
	"errors"
	"strings"
)

var ErrAlreadyBought = errors.New("wish already marked as bought")

func (s *Store) MarkBought(ctx context.Context, wishlistID, wishID, buyer string) error {
	buyer = strings.TrimSpace(buyer)
	if buyer == "" {
		return ErrInvalid
	}
	err := requireAffectedRow(s.execContext(ctx, `
		UPDATE wishes SET buyer = ? WHERE wishlist_id = ? AND id = ? AND buyer = ''
	`, buyer, wishlistID, wishID))
	if !errors.Is(err, ErrNotFound) {
		return err
	}
	if _, err := s.Wish(ctx, wishlistID, wishID); err != nil {
		return err
	}
	return ErrAlreadyBought
}

func (s *Store) ClearBought(ctx context.Context, wishlistID, wishID string) error {
	return requireAffectedRow(s.execContext(ctx, `
		UPDATE wishes SET buyer = '' WHERE wishlist_id = ? AND id = ?
	`, wishlistID, wishID))
}
