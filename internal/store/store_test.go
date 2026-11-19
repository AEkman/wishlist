package store

import (
	"bytes"
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"testing"
)

func TestOpenUpgradesImagesInExistingWishlists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wishlist.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = legacy.Close() })
	if _, err := legacy.ExecContext(t.Context(), schema); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.ExecContext(t.Context(), `
		INSERT INTO children (id, name) VALUES ('child', 'Alex');
		INSERT INTO wishlists (id, child_id, title, share_id) VALUES ('list', 'child', 'Jul', 'share');
		INSERT INTO wishes (id, wishlist_id, title, description, media_type, image, position, buyer)
		VALUES ('wish', 'list', 'LEGO', '', 'image/png', X'01', 0, 'Anna')
	`); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	s := openTestStore(t, path)
	before, err := s.Wish(t.Context(), "list", "wish")
	if err != nil || before.Buyer != "Anna" || before.ImageRevision != 0 {
		t.Fatalf("upgraded wish = %+v, %v", before, err)
	}
	after, err := s.UpdateWish(t.Context(), "list", "wish", "LEGO", "", "image/png", []byte{2})
	if err != nil || after.Buyer != "Anna" || after.ImageRevision != 1 {
		t.Fatalf("updated wish = %+v, %v", after, err)
	}
}

func TestWishlistSurvivesReopeningDatabase(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "wishlist.db")
	s := openTestStore(t, path)
	list := createList(t, s, "Alex", "Jul")
	bike := createWish(t, s, list.ID, "Bike")
	lego := createWish(t, s, list.ID, "LEGO")
	if err := s.Pick(ctx, list.ID, lego.ID, bike.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkBought(ctx, list.ID, lego.ID, "Anna"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestStore(t, path)
	shared, err := reopened.SharedWishlist(ctx, list.ShareID)
	if err != nil || shared != list {
		t.Fatalf("shared wishlist after reopening = %+v, %v, want %+v", shared, err, list)
	}
	lego.Buyer = "Anna"
	wishes, err := reopened.Wishes(ctx, list.ID)
	if want := []Wish{lego, bike}; err != nil || !slices.Equal(wishes, want) {
		t.Fatalf("wishes after reopening = %+v, %v, want %+v", wishes, err, want)
	}
	mediaType, image, err := reopened.Image(ctx, lego.ID)
	if err != nil || mediaType != "image/png" || !bytes.Equal(image, []byte("picture")) {
		t.Fatalf("image after reopening = %q, %q, %v", mediaType, image, err)
	}
}

func TestUpdateWishKeepsRankAndPurchaseMark(t *testing.T) {
	s := openMemoryStore(t)
	ctx := t.Context()
	list := createList(t, s, "Alex", "Jul")
	bike := createWish(t, s, list.ID, "Bike")
	lego := createWish(t, s, list.ID, "LEGO")
	createWish(t, s, list.ID, "Ball")
	if err := s.MarkBought(ctx, list.ID, lego.ID, "Anna"); err != nil {
		t.Fatal(err)
	}
	newPicture := []byte("new picture")
	updated, err := s.UpdateWish(ctx, list.ID, lego.ID, "Space LEGO", "The big set", "image/jpeg", newPicture)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Buyer != "Anna" || updated.Title != "Space LEGO" || updated.Description != "The big set" || updated.ID != lego.ID {
		t.Fatalf("unexpected edited wish: %+v", updated)
	}
	wishes, err := s.Wishes(ctx, list.ID)
	if err != nil || len(wishes) != 3 || wishes[0] != bike || wishes[1] != updated {
		t.Fatalf("wishes after edit = %+v, %v, want unchanged rank", wishes, err)
	}
	if _, err := s.UpdateWish(ctx, list.ID, lego.ID, "Space set", "", "", nil); err != nil {
		t.Fatal(err)
	}
	mediaType, image, err := s.Image(ctx, lego.ID)
	if err != nil || mediaType != "image/jpeg" || !bytes.Equal(image, newPicture) {
		t.Fatalf("updated image = %q, %q, %v, want %q, %q", mediaType, image, err, "image/jpeg", newPicture)
	}
}

func TestMarkAndClearPurchaseWithoutOverwritingBuyer(t *testing.T) {
	s := openMemoryStore(t)
	ctx := t.Context()
	list := createList(t, s, "Alex", "Jul")
	wish := createWish(t, s, list.ID, "LEGO")
	if err := s.MarkBought(ctx, list.ID, wish.ID, " "); !errors.Is(err, ErrInvalid) {
		t.Fatalf("mark with blank buyer = %v, want invalid input", err)
	}
	if err := s.MarkBought(ctx, list.ID, wish.ID, " Anna "); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkBought(ctx, list.ID, wish.ID, "Sam"); !errors.Is(err, ErrAlreadyBought) {
		t.Fatalf("mark an already bought wish = %v, want already bought", err)
	}
	marked, err := s.Wish(ctx, list.ID, wish.ID)
	if err != nil || marked.Buyer != "Anna" {
		t.Fatalf("marked wish = %+v, %v, want buyer Anna", marked, err)
	}
	if err := s.ClearBought(ctx, "another-list", wish.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("clear through another list = %v, want not found", err)
	}
	if err := s.ClearBought(ctx, list.ID, wish.ID); err != nil {
		t.Fatal(err)
	}
	saved, err := s.Wish(ctx, list.ID, wish.ID)
	if err != nil || saved.Buyer != "" {
		t.Fatalf("cleared wish = %+v, %v, want no buyer", saved, err)
	}
}

func TestPickMovesWinnerAboveLoserAndLeavesOtherWishesInOrder(t *testing.T) {
	s := openMemoryStore(t)
	ctx := t.Context()
	list := createList(t, s, "Alex", "Jul")
	bike := createWish(t, s, list.ID, "Bike")
	lego := createWish(t, s, list.ID, "LEGO")
	skates := createWish(t, s, list.ID, "Skates")
	ball := createWish(t, s, list.ID, "Ball")
	if err := s.Pick(ctx, list.ID, skates.ID, bike.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Pick(ctx, list.ID, bike.ID, lego.ID); err != nil {
		t.Fatal(err)
	}
	wishes, err := s.Wishes(ctx, list.ID)
	if want := []Wish{skates, bike, lego, ball}; err != nil || !slices.Equal(wishes, want) {
		t.Fatalf("wishes = %+v, %v, want %+v", wishes, err, want)
	}
	other := createList(t, s, "Sam", "Jul")
	otherWish := createWish(t, s, other.ID, "LEGO")
	if err := s.Pick(ctx, list.ID, skates.ID, otherWish.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("pick across lists = %v, want not found", err)
	}
	if err := s.Pick(ctx, list.ID, skates.ID, skates.ID); !errors.Is(err, ErrInvalid) {
		t.Fatalf("pick against the same wish = %v, want invalid input", err)
	}
}

func createWish(t *testing.T, s *Store, listID, title string) Wish {
	t.Helper()
	wish, err := s.CreateWish(t.Context(), listID, title, "", "image/png", []byte("picture"))
	if err != nil {
		t.Fatal(err)
	}
	return wish
}

func TestCreateWishAddsImageAndDetailsAtEndOfWishlist(t *testing.T) {
	s := openMemoryStore(t)
	ctx := t.Context()
	list := createList(t, s, "Alex", "Jul")
	picture := []byte("picture")
	first, err := s.CreateWish(ctx, list.ID, "Bike", "", "image/png", picture)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CreateWish(ctx, list.ID, " LEGO ", "Space set", "image/png", picture)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID == "" || second.WishlistID != list.ID || second.Title != "LEGO" || second.Description != "Space set" {
		t.Fatalf("unexpected wish: %+v", second)
	}
	wishes, err := s.Wishes(ctx, list.ID)
	if want := []Wish{first, second}; err != nil || !slices.Equal(wishes, want) {
		t.Fatalf("wishes = %+v, %v, want %+v", wishes, err, want)
	}
	saved, err := s.Wish(ctx, list.ID, second.ID)
	if err != nil || saved != second {
		t.Fatalf("wish = %+v, %v, want %+v", saved, err, second)
	}
	mediaType, image, err := s.Image(ctx, second.ID)
	if err != nil || mediaType != "image/png" || !bytes.Equal(image, picture) {
		t.Fatalf("image = %q, %q, %v, want %q, %q", mediaType, image, err, "image/png", picture)
	}
	if _, err := s.Wish(ctx, "another-list", second.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("looking up wish in a different list = %v, want not found", err)
	}
}

func createList(t *testing.T, s *Store, name, title string) Wishlist {
	t.Helper()
	child, err := s.CreateChild(t.Context(), name)
	if err != nil {
		t.Fatal(err)
	}
	list, err := s.CreateWishlist(t.Context(), child.ID, title)
	if err != nil {
		t.Fatal(err)
	}
	return list
}

func TestCreateWishlistKeepsChildrenSeparateAndUsesIndependentShareID(t *testing.T) {
	s := openMemoryStore(t)
	ctx := t.Context()
	alex, err := s.CreateChild(ctx, "Alex")
	if err != nil {
		t.Fatal(err)
	}
	sam, err := s.CreateChild(ctx, "Sam")
	if err != nil {
		t.Fatal(err)
	}
	christmas, err := s.CreateWishlist(ctx, alex.ID, " Jul ")
	if err != nil {
		t.Fatal(err)
	}
	birthday, err := s.CreateWishlist(ctx, alex.ID, "Födelsedag")
	if err != nil {
		t.Fatal(err)
	}
	samsList, err := s.CreateWishlist(ctx, sam.ID, "Jul")
	if err != nil {
		t.Fatal(err)
	}
	if christmas.Title != "Jul" || christmas.ShareID == "" || christmas.ShareID == christmas.ID {
		t.Fatalf("unexpected wishlist: %+v", christmas)
	}
	alexLists, err := s.Wishlists(ctx, alex.ID)
	if want := []Wishlist{christmas, birthday}; err != nil || !slices.Equal(alexLists, want) {
		t.Fatalf("Alex's lists = %+v, %v, want %+v", alexLists, err, want)
	}
	samLists, err := s.Wishlists(ctx, sam.ID)
	if want := []Wishlist{samsList}; err != nil || !slices.Equal(samLists, want) {
		t.Fatalf("Sam's lists = %+v, %v, want %+v", samLists, err, want)
	}
	saved, err := s.Wishlist(ctx, christmas.ID)
	if err != nil || saved != christmas {
		t.Fatalf("wishlist = %+v, %v, want %+v", saved, err, christmas)
	}
	shared, err := s.SharedWishlist(ctx, christmas.ShareID)
	if err != nil || shared != christmas {
		t.Fatalf("shared wishlist = %+v, %v, want %+v", shared, err, christmas)
	}
	if _, err := s.SharedWishlist(ctx, christmas.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("sharing by private ID = %v, want not found", err)
	}
	if _, err := s.CreateWishlist(ctx, "missing", "Jul"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("creating wishlist for missing child = %v, want not found", err)
	}
}

func TestCreateChildAppearsInChildren(t *testing.T) {
	s := openMemoryStore(t)
	child, err := s.CreateChild(t.Context(), "  Alex  ")
	if err != nil {
		t.Fatal(err)
	}
	if child.ID == "" || child.Name != "Alex" {
		t.Fatalf("unexpected child: %+v", child)
	}
	children, err := s.Children(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if want := []Child{child}; !slices.Equal(children, want) {
		t.Fatalf("children = %+v, want %+v", children, want)
	}
	saved, err := s.Child(t.Context(), child.ID)
	if err != nil || saved != child {
		t.Fatalf("child = %+v, %v, want %+v", saved, err, child)
	}
}

func openMemoryStore(t *testing.T) *Store {
	t.Helper()
	return openTestStore(t, ":memory:")
}

func openTestStore(t *testing.T, dsn string) *Store {
	t.Helper()
	s, err := Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}
