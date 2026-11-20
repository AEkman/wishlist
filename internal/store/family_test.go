package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestFamilyShareIDSurvivesMigrationAndReopening(t *testing.T) {
	path := filepath.Join(t.TempDir(), "family.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.ExecContext(t.Context(), schema); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.ExecContext(t.Context(), `INSERT INTO children (id,name) VALUES ('alex','Alex'); INSERT INTO wishlists (id,child_id,title,share_id) VALUES ('list','alex','Jul','original-share')`); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	s := openTestStore(t, path)
	share, err := s.FamilyShareID(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uuid.Parse(share); err != nil {
		t.Fatalf("family share ID = %q, want UUID: %v", share, err)
	}
	if share == "original-share" {
		t.Fatal("family token reuses list token")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestStore(t, path)
	again, err := reopened.FamilyShareID(t.Context())
	if err != nil || again != share {
		t.Fatalf("reopened token = %q, %v, want %q", again, err, share)
	}
	list, err := reopened.SharedWishlist(t.Context(), "original-share")
	if err != nil || list.ID != "list" || list.FamilyVisible {
		t.Fatalf("existing list = %+v, %v", list, err)
	}
}

func TestFamilySharingRequiresExplicitVisibility(t *testing.T) {
	s := openMemoryStore(t)
	first := createList(t, s, "Ella", "Jul")
	other := createList(t, s, "Leo", "Födelsedag")
	if first.FamilyVisible || other.FamilyVisible {
		t.Fatal("new lists must be hidden")
	}
	lists, err := s.FamilyWishlists(t.Context())
	if err != nil || len(lists) != 0 {
		t.Fatalf("initial family lists = %+v, %v", lists, err)
	}
	if err := s.SetFamilyVisible(t.Context(), other.ID, true); err != nil {
		t.Fatal(err)
	}
	lists, err = s.FamilyWishlists(t.Context())
	if err != nil || len(lists) != 1 || lists[0].ID != other.ID || !lists[0].FamilyVisible {
		t.Fatalf("visible family lists = %+v, %v", lists, err)
	}
	for _, read := range []func() (Wishlist, error){
		func() (Wishlist, error) { return s.Wishlist(t.Context(), other.ID) },
		func() (Wishlist, error) { return s.SharedWishlist(t.Context(), other.ShareID) },
	} {
		list, err := read()
		if err != nil || !list.FamilyVisible {
			t.Fatalf("saved visibility = %+v, %v", list, err)
		}
	}
	childLists, err := s.Wishlists(t.Context(), other.ChildID)
	if err != nil || len(childLists) != 1 || !childLists[0].FamilyVisible {
		t.Fatalf("child lists = %+v, %v", childLists, err)
	}
	if err := s.SetFamilyVisible(t.Context(), first.ID, true); err != nil {
		t.Fatal(err)
	}
	lists, err = s.FamilyWishlists(t.Context())
	if err != nil || len(lists) != 2 || lists[0].ID != first.ID || lists[1].ID != other.ID {
		t.Fatalf("ordered family lists = %+v, %v", lists, err)
	}
	if err := s.SetFamilyVisible(t.Context(), other.ID, false); err != nil {
		t.Fatal(err)
	}
	lists, err = s.FamilyWishlists(t.Context())
	if err != nil || len(lists) != 1 || lists[0].ID != first.ID {
		t.Fatalf("after hiding = %+v, %v", lists, err)
	}
	if _, err := s.SharedWishlist(t.Context(), other.ShareID); err != nil {
		t.Fatalf("hidden list remains directly shared: %v", err)
	}
}

func TestFamilyVisibilitySurvivesReopening(t *testing.T) {
	path := filepath.Join(t.TempDir(), "family.db")
	s := openTestStore(t, path)
	list := createList(t, s, "Ella", "Jul")
	wish := createWish(t, s, list.ID, "LEGO")
	if err := s.MarkBought(t.Context(), list.ID, wish.ID, "Anna"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetFamilyVisible(t.Context(), "missing", true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing list visibility = %v, want not found", err)
	}
	if err := s.SetFamilyVisible(t.Context(), list.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestStore(t, path)
	lists, err := reopened.FamilyWishlists(t.Context())
	if err != nil || len(lists) != 1 || lists[0].ID != list.ID || !lists[0].FamilyVisible {
		t.Fatalf("reopened visible lists = %+v, %v", lists, err)
	}
	bought, err := reopened.Wish(t.Context(), list.ID, wish.ID)
	if err != nil || bought.Buyer != "Anna" {
		t.Fatalf("purchase after visibility change = %+v, %v", bought, err)
	}
}
