package web

import (
	"cmp"
	"errors"
	"net/http"
	"strconv"

	"github.com/AEkman/wishlist/internal/store"
	"github.com/gofiber/fiber/v3"
)

func (s server) guidedComparison(c fiber.Ctx, data page, wishes []store.Wish) error {
	progress, err := s.db.GuidedRanking(c.Context(), data.Wishlist.ID)
	if err != nil {
		return err
	}
	data.Guided = &progress
	if progress.Restarted {
		data.Error = "Listan har ändrats. Rangordningen börjar om med den aktuella ordningen."
	}
	if progress.Complete {
		wishes, err = s.db.Wishes(c.Context(), data.Wishlist.ID)
		if err != nil {
			return err
		}
		data.View = "ranked"
		data.Wishes = inRankOrder(wishes)
		data.ShareURL = cmp.Or(s.baseURL, c.Scheme()+"://"+c.Host()) + "/" + data.Wishlist.ShareID
		if progress.NewFavorite != "" && len(wishes) > 0 && wishes[0].ID == progress.NewFavorite {
			data.NewFavorite = true
			data.RankingResult = data.Wishes[0]
		}
	} else {
		candidate, err := s.db.Wish(c.Context(), data.Wishlist.ID, progress.Candidate)
		if err != nil {
			return err
		}
		opponent, err := s.db.Wish(c.Context(), data.Wishlist.ID, progress.Opponent)
		if err != nil {
			return err
		}
		data.Left, data.Right, data.GuidedWish, data.HasPair = opponent, candidate, candidate, true
		if s.draw(2) == 1 {
			data.Left, data.Right = data.Right, data.Left
		}
	}
	return s.show(c, data)
}

func (s server) guidedPick(c fiber.Ctx) error {
	revision, err := strconv.Atoi(c.FormValue("guided_revision"))
	if err != nil {
		return store.ErrInvalid
	}
	listID := c.Params("list")
	winner, loser := c.FormValue("winner"), c.FormValue("loser")
	var result store.PickResult
	debug := s.rankingDebug && c.Get("HX-Request") == "true"
	if debug {
		before, err := s.db.Wishes(c.Context(), listID)
		if err != nil {
			return err
		}
		result.WinnerBefore = findWishIndex(before, winner) + 1
		result.LoserBefore = findWishIndex(before, loser) + 1
	}
	if _, err := s.db.PickGuided(c.Context(), listID, revision, winner, loser); err != nil {
		return s.guidedActionError(c, err)
	}
	if debug {
		after, err := s.db.Wishes(c.Context(), listID)
		if err != nil {
			return err
		}
		result.WinnerAfter = findWishIndex(after, winner) + 1
		result.LoserAfter = findWishIndex(after, loser) + 1
	}
	return s.rankingRedirect(c, "/lists/"+listID, result)
}

func (s server) guidedUndo(c fiber.Ctx) error {
	revision, err := strconv.Atoi(c.FormValue("guided_revision"))
	if err != nil {
		return store.ErrInvalid
	}
	if _, err := s.db.UndoGuided(c.Context(), c.Params("list"), revision); err != nil {
		return s.guidedActionError(c, err)
	}
	return redirect(c, "/lists/"+c.Params("list"))
}

func (s server) guidedRestart(c fiber.Ctx) error {
	revision, err := strconv.Atoi(c.FormValue("guided_revision"))
	if err != nil {
		return store.ErrInvalid
	}
	if _, err := s.db.RestartGuided(c.Context(), c.Params("list"), revision); err != nil {
		return s.guidedActionError(c, err)
	}
	return redirect(c, "/lists/"+c.Params("list"))
}

func (s server) guidedActionError(c fiber.Ctx, err error) error {
	if !errors.Is(err, store.ErrInvalid) {
		return err
	}
	list, err := s.db.Wishlist(c.Context(), c.Params("list"))
	if err != nil {
		return err
	}
	wishes, err := s.db.Wishes(c.Context(), list.ID)
	if err != nil {
		return err
	}
	c.Status(http.StatusConflict)
	return s.guidedComparison(c, page{View: "compare", Title: list.Title, Wishlist: list, Error: "Vyn var inte aktuell. Ditt val kunde inte sparas. Försök igen."}, wishes)
}
