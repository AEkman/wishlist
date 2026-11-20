package web

import (
	"errors"
	"net/http"

	"github.com/AEkman/wishlist/internal/store"
	"github.com/gofiber/fiber/v3"
)

func (s server) shared(c fiber.Ctx) error {
	list, err := s.sharedWishlist(c)
	if err != nil {
		return err
	}
	return s.showShared(c, list, "")
}

func (s server) showShared(c fiber.Ctx, list store.Wishlist, message string) error {
	wishes, err := s.db.Wishes(c.Context(), list.ID)
	if err != nil {
		return err
	}
	data := page{View: "share", Title: list.Title, Wishlist: list, Error: message, Wishes: inRankOrder(wishes)}
	if err := s.familyNavigation(c, &data); err != nil {
		return err
	}
	return s.show(c, data)
}

func (s server) buyForm(c fiber.Ctx) error {
	return s.showPurchaseForm(c, "purchase")
}

func (s server) clearForm(c fiber.Ctx) error {
	return s.showPurchaseForm(c, "clear")
}

func (s server) showPurchaseForm(c fiber.Ctx, view string) error {
	data, err := s.purchasePage(c, view)
	if err != nil {
		return err
	}
	return s.show(c, data)
}

func (s server) buy(c fiber.Ctx) error {
	data, err := s.purchasePage(c, "purchase")
	if err != nil {
		return err
	}
	err = s.db.MarkBought(c.Context(), data.Wishlist.ID, data.Wish.ID, c.FormValue("name"))
	switch {
	case errors.Is(err, store.ErrInvalid):
		data.Error = "Ange ditt namn."
		c.Status(http.StatusBadRequest)
		return s.show(c, data)
	case errors.Is(err, store.ErrAlreadyBought):
		c.Status(http.StatusConflict)
		return s.showShared(c, data.Wishlist, "Någon har redan markerat önskan som köpt.")
	case err != nil:
		return err
	}
	return redirect(c, data.SharedListURL)
}

func (s server) clearPurchase(c fiber.Ctx) error {
	data, err := s.purchasePage(c, "clear")
	if err != nil {
		return err
	}
	if err := s.db.ClearBought(c.Context(), data.Wishlist.ID, data.Wish.ID); err != nil {
		return err
	}
	return redirect(c, data.SharedListURL)
}

func (s server) purchasePage(c fiber.Ctx, view string) (page, error) {
	list, err := s.sharedWishlist(c)
	if err != nil {
		return page{}, err
	}
	wish, err := s.db.Wish(c.Context(), list.ID, c.Params("wish"))
	if err != nil {
		return page{}, err
	}
	data := page{View: view, Title: list.Title, Wishlist: list, Wish: wish, SharedListURL: "/" + list.ShareID}
	if err := s.familyNavigation(c, &data); err != nil {
		return page{}, err
	}
	return data, nil
}
