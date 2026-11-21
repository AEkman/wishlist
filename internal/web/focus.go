package web

import (
	"crypto/sha256"
	"encoding/hex"
	"math/bits"
	"net/url"
	"strconv"
	"strings"

	"github.com/AEkman/wishlist/internal/store"
	"github.com/gofiber/fiber/v3"
)

type insertionState struct {
	Low, High int
	Order     string
}

func insertionRange(low, high string, size int) (insertionState, error) {
	lower, err := strconv.Atoi(low)
	if err != nil {
		return insertionState{}, store.ErrInvalid
	}
	upper, err := strconv.Atoi(high)
	if err != nil || lower < 0 || lower >= upper || upper > size {
		return insertionState{}, store.ErrInvalid
	}
	return insertionState{Low: lower, High: upper}, nil
}

func insertionWishes(wishes []store.Wish, id string) (store.Wish, []store.Wish, error) {
	index := findWishIndex(wishes, id)
	if index < 0 {
		return store.Wish{}, nil, store.ErrNotFound
	}
	others := make([]store.Wish, 0, len(wishes)-1)
	others = append(others, wishes[:index]...)
	others = append(others, wishes[index+1:]...)
	return wishes[index], others, nil
}

func insertionOrder(wishes []store.Wish) string {
	ids := make([]string, len(wishes))
	for i, wish := range wishes {
		ids[i] = wish.ID
	}
	sum := sha256.Sum256([]byte(strings.Join(ids, ",")))
	return hex.EncodeToString(sum[:])
}

func (s server) insertionComparison(c fiber.Ctx, data page, wishes []store.Wish, id, mode string) error {
	wish, others, err := insertionWishes(wishes, id)
	if err != nil {
		return err
	}
	if len(others) == 0 {
		data.Wish = wish
		return s.show(c, data)
	}
	order := insertionOrder(others)
	if (c.Query("low") != "" || c.Query("high") != "" || c.Query("order") != "") && c.Query("order") != order {
		return store.ErrInvalid
	}
	state, err := insertionRange(c.Query("low", "0"), c.Query("high", strconv.Itoa(len(others))), len(others))
	if err != nil {
		return err
	}
	data.HasPair, data.Focus, data.Upload = true, mode == "focus", mode == "upload"
	data.Left, data.Right = wish, others[(state.Low+state.High)/2]
	state.Order = order
	data.Insertion = state
	data.Remaining = bits.Len(uint(state.High - state.Low))
	return s.show(c, data)
}

func (s server) insertionPick(c fiber.Ctx, listID, id, winner, loser, mode string) error {
	wishes, err := s.db.Wishes(c.Context(), listID)
	if err != nil {
		return err
	}
	_, others, err := insertionWishes(wishes, id)
	if err != nil {
		return err
	}
	order := insertionOrder(others)
	if c.FormValue("order") != order {
		return store.ErrInvalid
	}
	state, err := insertionRange(c.FormValue("low"), c.FormValue("high"), len(others))
	if err != nil {
		return err
	}
	middle := (state.Low + state.High) / 2
	opponent := others[middle].ID
	if winner == id && loser == opponent {
		state.High = middle
	} else if loser == id && winner == opponent {
		state.Low = middle + 1
	} else {
		return store.ErrInvalid
	}
	path := "/lists/" + listID
	if state.Low == state.High {
		expected := make([]string, len(wishes))
		for i, wish := range wishes {
			expected[i] = wish.ID
		}
		if err := s.db.PlaceWish(c.Context(), listID, id, state.Low+1, expected); err != nil {
			return err
		}
		completed := url.Values{"ranked": {id}}
		if state.Low == 0 && wishes[0].ID != id {
			completed.Set("newfavorite", "1")
		}
		return redirect(c, path+"/ranked?"+completed.Encode())
	}
	next := url.Values{mode: {id}, "low": {strconv.Itoa(state.Low)}, "high": {strconv.Itoa(state.High)}, "order": {order}}
	return redirect(c, path+"?"+next.Encode())
}
