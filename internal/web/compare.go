package web

import (
	"encoding/json"
	"net/http"

	"github.com/AEkman/wishlist/internal/store"
	"github.com/gofiber/fiber/v3"
)

func (s server) pick(c fiber.Ctx) error {
	listID := c.Params("list")
	winner, loser := c.FormValue("winner"), c.FormValue("loser")
	focus, upload := c.FormValue("focus"), c.FormValue("upload")
	if focus != "" && upload != "" {
		return store.ErrInvalid
	}
	if focus != "" {
		return s.insertionPick(c, listID, focus, winner, loser, "focus")
	}
	if upload != "" {
		return s.insertionPick(c, listID, upload, winner, loser, "upload")
	}
	result, err := s.db.Pick(c.Context(), listID, winner, loser)
	if err != nil {
		return err
	}
	path := "/lists/" + listID
	if s.rankingDebug && c.Get("HX-Request") == "true" {
		// HTMX handles these headers before requesting the next comparison.
		event, err := json.Marshal(map[string]store.PickResult{"wishlist:ranking-pick": result})
		if err != nil {
			return err
		}
		location, err := json.Marshal(map[string]string{
			"path": path, "target": "#screen", "select": "#screen", "swap": "outerHTML swap:150ms show:top",
		})
		if err != nil {
			return err
		}
		c.Set("HX-Trigger", string(event))
		c.Set("HX-Location", string(location))
		return c.SendStatus(http.StatusOK)
	}
	return redirect(c, path)
}
