package web

import (
	"net/url"
	"slices"
	"strconv"

	"github.com/AEkman/wishlist/internal/store"
	"github.com/gofiber/fiber/v3"
)

const focusedComparisons = 5

func (s server) focusComparison(c fiber.Ctx, data page, wishes []store.Wish, id string) error {
	index := findWishIndex(wishes, id)
	if index < 0 {
		return store.ErrNotFound
	}
	if len(wishes) < 2 {
		data.Wish = wishes[index]
		return s.show(c, data)
	}
	remaining, err := strconv.Atoi(c.Query("remaining", strconv.Itoa(focusedComparisons)))
	tried := triedIDs(c.Request().URI().QueryArgs().PeekMulti("tried"))
	if err != nil || remaining < 1 || remaining > focusedComparisons || len(tried) != focusedComparisons-remaining {
		return store.ErrInvalid
	}
	order := make([]string, len(wishes))
	for i, wish := range wishes {
		order[i] = wish.ID
	}
	opponent := uploadOpponent(order, id, c.Query("pivot"), c.Query("higher") == "1", tried, s.draw)
	if opponent == "" {
		// Start another cycle when fewer than five other wishes are available.
		// Keep the last opponent excluded so consecutive pairs differ when possible.
		var last []string
		if len(tried) > 0 && len(wishes) > 2 {
			last = tried[len(tried)-1:]
		}
		opponent = uploadOpponent(order, id, c.Query("pivot"), c.Query("higher") == "1", last, s.draw)
	}
	data.HasPair, data.Focus = true, true
	data.Left, data.Right = wishes[index], wishes[findWishIndex(wishes, opponent)]
	data.Remaining, data.Tried = remaining, tried
	return s.show(c, data)
}

func (s server) focusPick(c fiber.Ctx, listID, id, winner, loser string) (url.Values, error) {
	remaining, err := strconv.Atoi(c.FormValue("remaining"))
	tried := triedIDs(c.Request().PostArgs().PeekMulti("tried"))
	if err != nil || remaining < 1 || remaining > focusedComparisons || len(tried) != focusedComparisons-remaining || (id != winner && id != loser) {
		return nil, store.ErrInvalid
	}
	wishes, err := s.db.Wishes(c.Context(), listID)
	if err != nil {
		return nil, err
	}
	for _, previous := range tried {
		if previous == id || findWishIndex(wishes, previous) < 0 {
			return nil, store.ErrInvalid
		}
	}
	opponent := winner
	if id == winner {
		opponent = loser
	}
	eligible := make([]string, 0, len(wishes))
	for _, wish := range wishes {
		if wish.ID != id && !slices.Contains(tried, wish.ID) {
			eligible = append(eligible, wish.ID)
		}
	}
	if len(eligible) > 0 && !slices.Contains(eligible, opponent) {
		return nil, store.ErrInvalid
	}
	if len(eligible) == 0 && len(wishes) > 2 && len(tried) > 0 && opponent == tried[len(tried)-1] {
		return nil, store.ErrInvalid
	}
	next := url.Values{}
	if remaining > 1 {
		next.Set("focus", id)
		next.Set("pivot", winner)
		if id == winner {
			next.Set("higher", "1")
		}
		next.Set("remaining", strconv.Itoa(remaining-1))
		next["tried"] = append(tried, opponent)
	}
	return next, nil
}
