package store

import "context"

func (s *Store) FamilyShareID(ctx context.Context) (string, error) {
	var shareID string
	err := s.queryRowContext(ctx, "SELECT share_id FROM family_sharing WHERE id = 1").Scan(&shareID)
	return shareID, err
}

func (s *Store) SetFamilyVisible(ctx context.Context, listID string, visible bool) error {
	visibility := 0
	if visible {
		visibility = 1
	}
	return requireAffectedRow(s.execContext(ctx, "UPDATE wishlists SET family_visible = ? WHERE id = ?", visibility, listID))
}

func (s *Store) FamilyWishlists(ctx context.Context) ([]Wishlist, error) {
	rows, err := s.queryContext(ctx, "SELECT id, child_id, title, share_id, family_visible FROM wishlists WHERE family_visible = 1 ORDER BY "+s.wishlistOrder())
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
