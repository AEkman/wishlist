package web

import (
	"net/url"
	"slices"
	"strconv"

	"github.com/AEkman/wishlist/internal/store"
	"github.com/gofiber/fiber/v3"
)

const maxUploadPicks = 3

func uploadPicks(wishCount int) int {
	if wishCount < 5 {
		return max(0, min(2, wishCount-1))
	}
	return maxUploadPicks
}

func (s server) uploadComparison(c fiber.Ctx, data page, wishes []store.Wish, id string) error {
	index := findWishIndex(wishes, id)
	if index < 0 {
		return store.ErrNotFound
	}
	remaining, err := strconv.Atoi(c.Query("remaining", strconv.Itoa(uploadPicks(len(wishes)))))
	if err != nil || remaining < 1 || remaining > maxUploadPicks {
		return store.ErrInvalid
	}
	tried := triedIDs(c.Request().URI().QueryArgs().PeekMulti("tried"))
	order := make([]string, len(wishes))
	for i, wish := range wishes {
		order[i] = wish.ID
	}
	opponent := findWishIndex(wishes, uploadOpponent(order, id, c.Query("pivot"), c.Query("higher") == "1", tried, s.draw))
	if opponent < 0 {
		return redirect(c, "/lists/"+data.Wishlist.ID+"?"+url.Values{"done": {id}}.Encode())
	}
	data.HasPair, data.Upload = true, true
	data.Left, data.Right = wishes[index], wishes[opponent]
	data.Tried, data.Remaining = tried, remaining
	return s.show(c, data)
}

func (s server) pick(c fiber.Ctx) error {
	listID := c.Params("list")
	winner, loser := c.FormValue("winner"), c.FormValue("loser")
	next := url.Values{}
	if id := c.FormValue("upload"); id != "" {
		remaining, err := strconv.Atoi(c.FormValue("remaining"))
		tried := triedIDs(c.Request().PostArgs().PeekMulti("tried"))
		if err != nil || remaining < 1 || len(tried)+remaining > maxUploadPicks || (id != winner && id != loser) {
			return store.ErrInvalid
		}
		opponent := winner
		if id == winner {
			opponent = loser
		}
		if slices.Contains(tried, opponent) {
			return store.ErrInvalid
		}
		if remaining == 1 {
			next.Set("done", id)
		} else {
			next.Set("upload", id)
			next.Set("remaining", strconv.Itoa(remaining-1))
			next["tried"] = append(tried, opponent)
			next.Set("pivot", winner)
			if id == winner {
				next.Set("higher", "1")
			}
		}
	}
	if err := s.db.Pick(c.Context(), listID, winner, loser); err != nil {
		return err
	}
	path := "/lists/" + listID
	if len(next) > 0 {
		path += "?" + next.Encode()
	}
	return redirect(c, path)
}

func triedIDs(values [][]byte) []string {
	ids := make([]string, len(values))
	for i, value := range values {
		ids[i] = string(value)
	}
	return ids
}
