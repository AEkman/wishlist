package store

import (
	"errors"
	"fmt"
	"math/bits"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func TestGuidedRankingResumesAfterReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guided.sqlite")
	s := openTestStore(t, path)
	list := createList(t, s, "Alex", "Jul")
	a := createWish(t, s, list.ID, "A")
	b := createWish(t, s, list.ID, "B")
	c := createWish(t, s, list.ID, "C")
	progress, err := s.GuidedRanking(t.Context(), list.ID)
	if err != nil {
		t.Fatal(err)
	}
	if progress.Placed != 1 || progress.Candidate != b.ID || progress.Opponent != a.ID {
		t.Fatalf("initial progress = %+v", progress)
	}
	progress, err = s.PickGuided(t.Context(), list.ID, progress.Revision, b.ID, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if progress.Placed != 2 || progress.Candidate != c.ID || !progress.CanUndo {
		t.Fatalf("after choice = %+v", progress)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestStore(t, path)
	resumed, err := reopened.GuidedRanking(t.Context(), list.ID)
	if err != nil || !reflect.DeepEqual(progress, resumed) {
		t.Fatalf("resumed = %+v, %v; want %+v", resumed, err, progress)
	}
	wishes, err := reopened.Wishes(t.Context(), list.ID)
	if err != nil || wishes[0].ID != b.ID || wishes[1].ID != a.ID {
		t.Fatalf("saved order = %v, %v", wishes, err)
	}
}

func TestGuidedUndoRestoresLastChoiceAfterCompletion(t *testing.T) {
	s := openMemoryStore(t)
	list := createList(t, s, "Alex", "Jul")
	a := createWish(t, s, list.ID, "A")
	b := createWish(t, s, list.ID, "B")
	initial, err := s.GuidedRanking(t.Context(), list.ID)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := s.PickGuided(t.Context(), list.ID, initial.Revision, b.ID, a.ID)
	if err != nil || !completed.Complete || completed.NewFavorite != b.ID {
		t.Fatalf("completed=%+v,%v", completed, err)
	}
	if _, err := s.UpdateWish(t.Context(), list.ID, b.ID, "Changed title", "description", "", nil); err != nil {
		t.Fatal(err)
	}
	undone, err := s.UndoGuided(t.Context(), list.ID, completed.Revision)
	if err != nil {
		t.Fatal(err)
	}
	wishes, err := s.Wishes(t.Context(), list.ID)
	if err != nil || wishes[0].ID != a.ID || wishes[1].ID != b.ID || wishes[1].Title != "Changed title" {
		t.Fatalf("undo order=%v,%v", wishes, err)
	}
	if undone.Complete || undone.CanUndo || undone.Candidate != initial.Candidate || undone.Opponent != initial.Opponent || undone.Placed != initial.Placed || undone.Revision <= completed.Revision {
		t.Fatalf("undo=%+v, initial=%+v", undone, initial)
	}
}

func TestGuidedRestartDiscardsProgressAndKeepsCurrentOrder(t *testing.T) {
	s := openMemoryStore(t)
	list := createList(t, s, "Alex", "Jul")
	a := createWish(t, s, list.ID, "A")
	b := createWish(t, s, list.ID, "B")
	_ = createWish(t, s, list.ID, "C")
	initial, err := s.GuidedRanking(t.Context(), list.ID)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := s.PickGuided(t.Context(), list.ID, initial.Revision, b.ID, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := s.RestartGuided(t.Context(), list.ID, changed.Revision)
	if err != nil || restarted.Placed != 1 || restarted.CanUndo || restarted.Candidate != a.ID || restarted.Opponent != b.ID || restarted.Revision <= changed.Revision {
		t.Fatalf("restart=%+v,%v", restarted, err)
	}
}

func TestGuidedRankingCoversWholeListAndCanUndoEveryChoice(t *testing.T) {
	for _, count := range []int{2, 4, 12, 31} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			s := openMemoryStore(t)
			list := createList(t, s, "Alex", "Jul")
			var original []string
			for i := range count {
				wish := createWish(t, s, list.ID, fmt.Sprint(i))
				original = append(original, wish.ID)
			}
			desired := slices.Clone(original)
			slices.Reverse(desired)
			progress, err := s.GuidedRanking(t.Context(), list.ID)
			if err != nil {
				t.Fatal(err)
			}
			candidates := map[string]bool{original[0]: true}
			picks := 0
			for !progress.Complete {
				candidates[progress.Candidate] = true
				winner, loser := progress.Candidate, progress.Opponent
				if slices.Index(desired, winner) > slices.Index(desired, loser) {
					winner, loser = loser, winner
				}
				progress, err = s.PickGuided(t.Context(), list.ID, progress.Revision, winner, loser)
				if err != nil {
					t.Fatal(err)
				}
				picks++
				if picks > count*bits.Len(uint(count)) {
					t.Fatal("guided ranking did not converge")
				}
			}
			if len(candidates) != count {
				t.Fatalf("only covered %d of %d wishes", len(candidates), count)
			}
			wishes, _ := s.Wishes(t.Context(), list.ID)
			var got []string
			for _, wish := range wishes {
				got = append(got, wish.ID)
			}
			if !slices.Equal(got, desired) {
				t.Fatalf("final order=%v,want %v", got, desired)
			}
			for range picks {
				progress, err = s.UndoGuided(t.Context(), list.ID, progress.Revision)
				if err != nil {
					t.Fatal(err)
				}
			}
			wishes, _ = s.Wishes(t.Context(), list.ID)
			got = nil
			for _, wish := range wishes {
				got = append(got, wish.ID)
			}
			if progress.Placed != 1 || progress.CanUndo || !slices.Equal(got, original) {
				t.Fatalf("full undo progress=%+v,order=%v", progress, got)
			}
		})
	}
}

func TestGuidedRankingRejectsStaleChoicesAndRestartsAfterExternalChange(t *testing.T) {
	s := openMemoryStore(t)
	list := createList(t, s, "Alex", "Jul")
	a := createWish(t, s, list.ID, "A")
	b := createWish(t, s, list.ID, "B")
	c := createWish(t, s, list.ID, "C")
	initial, _ := s.GuidedRanking(t.Context(), list.ID)
	if err := s.MoveWish(t.Context(), list.ID, c.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PickGuided(t.Context(), list.ID, initial.Revision, b.ID, a.ID); !errors.Is(err, ErrInvalid) {
		t.Fatalf("stale pick=%v", err)
	}
	if _, err := s.UndoGuided(t.Context(), list.ID, initial.Revision); !errors.Is(err, ErrInvalid) {
		t.Fatalf("stale undo=%v", err)
	}
	progress, err := s.GuidedRanking(t.Context(), list.ID)
	if err != nil || !progress.Restarted || progress.Revision <= initial.Revision || progress.Opponent != c.ID || progress.Candidate != a.ID {
		t.Fatalf("restarted=%+v,%v", progress, err)
	}
	fresh, err := s.GuidedRanking(t.Context(), list.ID)
	if err != nil || fresh.Restarted {
		t.Fatalf("restart notice repeats:%+v,%v", fresh, err)
	}
	next, err := s.PickGuided(t.Context(), list.ID, progress.Revision, a.ID, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PickGuided(t.Context(), list.ID, progress.Revision, a.ID, c.ID); !errors.Is(err, ErrInvalid) {
		t.Fatalf("replayed choice=%v,next=%+v", err, next)
	}
}

func TestPostgresGuidedProgressAndUndoSurviveReopen(t *testing.T) {
	dsn := postgresTestURL(t)
	s := openTestStore(t, dsn)
	list := createList(t, s, "Alex", "Jul")
	a := createWish(t, s, list.ID, "A")
	b := createWish(t, s, list.ID, "B")
	initial, err := s.GuidedRanking(t.Context(), list.ID)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := s.PickGuided(t.Context(), list.ID, initial.Revision, b.ID, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestStore(t, dsn)
	resumed, err := reopened.GuidedRanking(t.Context(), list.ID)
	if err != nil || !resumed.Complete || !resumed.CanUndo || resumed.Revision != completed.Revision {
		t.Fatalf("resumed=%+v,%v", resumed, err)
	}
	undone, err := reopened.UndoGuided(t.Context(), list.ID, resumed.Revision)
	if err != nil || undone.Complete || undone.Candidate != b.ID || undone.Opponent != a.ID {
		t.Fatalf("undo=%+v,%v", undone, err)
	}
	wishes, _ := reopened.Wishes(t.Context(), list.ID)
	if wishes[0].ID != a.ID {
		t.Fatal("undo failed to restore order")
	}
}
