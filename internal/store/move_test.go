package store

import (
	"errors"
	"slices"
	"testing"
)

func TestMoveWishChangesRankAndPreservesOtherOrder(t *testing.T) {
	s := openMemoryStore(t)
	list := createList(t, s, "Alex", "Jul")
	first := createWish(t, s, list.ID, "First")
	second := createWish(t, s, list.ID, "Second")
	third := createWish(t, s, list.ID, "Third")
	fourth := createWish(t, s, list.ID, "Fourth")
	for _, tc := range []struct {
		wish string
		rank int
		want []Wish
	}{
		{third.ID, 1, []Wish{third, first, second, fourth}},
		{first.ID, 4, []Wish{third, second, fourth, first}},
	} {
		if err := s.MoveWish(t.Context(), list.ID, tc.wish, tc.rank); err != nil {
			t.Fatal(err)
		}
		got, err := s.Wishes(t.Context(), list.ID)
		if err != nil || !slices.Equal(got, tc.want) {
			t.Fatalf("wishes = %v, %v, want %v", got, err, tc.want)
		}
	}
}

func TestMoveWishRejectsInvalidRanksAndOtherLists(t *testing.T) {
	s := openMemoryStore(t)
	list := createList(t, s, "Alex", "Jul")
	first := createWish(t, s, list.ID, "First")
	second := createWish(t, s, list.ID, "Second")
	other := createList(t, s, "Sam", "Jul")
	for _, rank := range []int{0, -1, 3} {
		if err := s.MoveWish(t.Context(), list.ID, first.ID, rank); !errors.Is(err, ErrInvalid) {
			t.Fatalf("rank %d error = %v, want invalid", rank, err)
		}
	}
	if err := s.MoveWish(t.Context(), other.ID, first.ID, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-list move = %v, want not found", err)
	}
	if err := s.MoveWish(t.Context(), list.ID, "missing", 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing wish move = %v, want not found", err)
	}
	wishes, err := s.Wishes(t.Context(), list.ID)
	if err != nil || !slices.Equal(wishes, []Wish{first, second}) {
		t.Fatalf("invalid move changed order: %v, %v", wishes, err)
	}
}

func TestPostgresMoveWishPersistsOrder(t *testing.T) {
	dsn := postgresTestURL(t)
	s := openTestStore(t, dsn)
	list := createList(t, s, "Alex", "Jul")
	first := createWish(t, s, list.ID, "First")
	second := createWish(t, s, list.ID, "Second")
	third := createWish(t, s, list.ID, "Third")
	if err := s.MoveWish(t.Context(), list.ID, third.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveWish(t.Context(), list.ID, first.ID, 3); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestStore(t, dsn)
	wishes, err := reopened.Wishes(t.Context(), list.ID)
	if err != nil || !slices.Equal(wishes, []Wish{third, second, first}) {
		t.Fatalf("persisted order = %v, %v", wishes, err)
	}
}

func TestPlaceWishChecksExpectedOrder(t *testing.T) {
	for _, database := range []string{"sqlite", "postgres"} {
		t.Run(database, func(t *testing.T) {
			s := openMemoryStore(t)
			if database == "postgres" {
				s = openTestStore(t, postgresTestURL(t))
			}
			list := createList(t, s, "Alex", "Jul")
			first := createWish(t, s, list.ID, "First")
			second := createWish(t, s, list.ID, "Second")
			wish := createWish(t, s, list.ID, "Focus")
			expected := []string{first.ID, second.ID, wish.ID}
			if err := s.MoveWish(t.Context(), list.ID, second.ID, 1); err != nil {
				t.Fatal(err)
			}
			if err := s.PlaceWish(t.Context(), list.ID, wish.ID, 2, expected); !errors.Is(err, ErrInvalid) {
				t.Fatalf("stale placement = %v, want invalid", err)
			}
			after, err := s.Wishes(t.Context(), list.ID)
			if err != nil || !slices.Equal(after, []Wish{second, first, wish}) {
				t.Fatalf("stale placement changed order: %v, %v", after, err)
			}
			if err := s.PlaceWish(t.Context(), list.ID, wish.ID, 2, []string{second.ID, first.ID, wish.ID}); err != nil {
				t.Fatal(err)
			}
			after, err = s.Wishes(t.Context(), list.ID)
			if err != nil || !slices.Equal(after, []Wish{second, wish, first}) {
				t.Fatalf("valid placement = %v, %v", after, err)
			}
		})
	}
}
