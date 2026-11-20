package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

type Wishlist struct {
	FamilyVisible bool
	ID            string
	ChildID       string
	Title         string
	ShareID       string
}

func (s *Store) CreateWishlist(ctx context.Context, childID, title string) (Wishlist, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Wishlist{}, ErrInvalid
	}
	id, err := newID()
	if err != nil {
		return Wishlist{}, err
	}
	shareID, err := newID()
	if err != nil {
		return Wishlist{}, err
	}
	err = requireAffectedRow(s.execContext(ctx, `
		INSERT INTO wishlists (id, child_id, title, share_id)
		SELECT ?, id, ?, ? FROM children WHERE id = ?
	`, id, title, shareID, childID))
	if err != nil {
		return Wishlist{}, err
	}
	return Wishlist{ID: id, ChildID: childID, Title: title, ShareID: shareID}, nil
}

func (s *Store) Wishlists(ctx context.Context, childID string) ([]Wishlist, error) {
	rows, err := s.queryContext(ctx, `
		SELECT id, child_id, title, share_id, family_visible FROM wishlists WHERE child_id = ? ORDER BY `+s.wishlistOrder(), childID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lists []Wishlist
	for rows.Next() {
		var list Wishlist
		if err := rows.Scan(&list.ID, &list.ChildID, &list.Title, &list.ShareID, &list.FamilyVisible); err != nil {
			return nil, err
		}
		lists = append(lists, list)
	}
	return lists, rows.Err()
}

func (s *Store) Wishlist(ctx context.Context, id string) (Wishlist, error) {
	return scanWishlist(s.queryRowContext(ctx, "SELECT id, child_id, title, share_id, family_visible FROM wishlists WHERE id = ?", id))
}

func (s *Store) SharedWishlist(ctx context.Context, shareID string) (Wishlist, error) {
	return scanWishlist(s.queryRowContext(ctx, "SELECT id, child_id, title, share_id, family_visible FROM wishlists WHERE share_id = ?", shareID))
}

func scanWishlist(row *sql.Row) (Wishlist, error) {
	var list Wishlist
	err := row.Scan(&list.ID, &list.ChildID, &list.Title, &list.ShareID, &list.FamilyVisible)
	if errors.Is(err, sql.ErrNoRows) {
		return Wishlist{}, ErrNotFound
	}
	return list, err
}
