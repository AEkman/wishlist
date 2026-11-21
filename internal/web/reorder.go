package web

import (
	"net/http"
	"strconv"

	"github.com/AEkman/wishlist/internal/store"
	"github.com/gofiber/fiber/v3"
)

func (s server) moveWish(c fiber.Ctx) error {
	rank, err := strconv.Atoi(c.FormValue("rank"))
	if err != nil {
		return store.ErrInvalid
	}
	if err := s.db.MoveWish(c.Context(), c.Params("list"), c.FormValue("wish"), rank); err != nil {
		return err
	}
	return c.SendStatus(http.StatusNoContent)
}
