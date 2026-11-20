package web

import (
	"net/http"

	"github.com/AEkman/wishlist/internal/store"
	"github.com/gofiber/fiber/v3"
)

func (s server) family(c fiber.Ctx) error {
	data := page{View: "family", Title: "Familjens önskelistor"}
	if err := s.familyNavigation(c, &data); err != nil {
		return err
	}
	return s.show(c, data)
}

func (s server) familyChild(c fiber.Ctx) error {
	data := page{View: "family-lists"}
	if err := s.familyNavigation(c, &data); err != nil {
		return err
	}
	child, err := s.db.Child(c.Context(), c.Params("child"))
	if err != nil {
		return err
	}
	data.Child, data.Title = child, child.Name
	var lists []store.Wishlist
	for _, list := range data.SharedLists {
		if list.ChildID == child.ID {
			lists = append(lists, list)
		}
	}
	if len(lists) == 0 {
		return store.ErrNotFound
	}
	if len(lists) == 1 {
		return redirect(c, data.SharedListURLs[lists[0].ID])
	}
	data.SharedLists = lists
	return s.show(c, data)
}

func (s server) updateSharing(c fiber.Ctx) error {
	if err := s.db.SetFamilyVisible(c.Context(), c.Params("list"), c.FormValue("visible") == "true"); err != nil {
		return err
	}
	if c.Get("HX-Request") == "true" {
		return c.SendStatus(http.StatusNoContent)
	}
	return redirect(c, "/lists/"+c.Params("list")+"/ranked")
}

func (s server) familyNavigation(c fiber.Ctx, data *page) error {
	token := c.Params("family")
	if token == "" {
		return nil
	}
	expected, err := s.db.FamilyShareID(c.Context())
	if err != nil {
		return err
	}
	if token != expected {
		return store.ErrNotFound
	}
	lists, err := s.db.FamilyWishlists(c.Context())
	if err != nil {
		return err
	}
	children, err := s.db.Children(c.Context())
	if err != nil {
		return err
	}
	data.FamilyURL = "/family/" + token
	data.SharedLists = lists
	data.SharedChildURLs = make(map[string]string)
	data.SharedListURLs = make(map[string]string)
	visibleChildren := make(map[string]bool)
	for _, list := range lists {
		visibleChildren[list.ChildID] = true
		data.SharedListURLs[list.ID] = data.FamilyURL + "/lists/" + list.ID
	}
	for _, child := range children {
		if visibleChildren[child.ID] {
			data.SharedChildren = append(data.SharedChildren, child)
			data.SharedChildURLs[child.ID] = data.FamilyURL + "/children/" + child.ID
		}
	}
	if data.Wishlist.ID != "" {
		listURL, visible := data.SharedListURLs[data.Wishlist.ID]
		if !visible {
			return store.ErrNotFound
		}
		data.SharedListURL = listURL
		data.SharedLists = nil
		for _, list := range lists {
			if list.ChildID == data.Wishlist.ChildID {
				data.SharedLists = append(data.SharedLists, list)
			}
		}
	}
	return nil
}

func (s server) sharedWishlist(c fiber.Ctx) (store.Wishlist, error) {
	if c.Params("family") == "" {
		return s.db.SharedWishlist(c.Context(), c.Params("share"))
	}
	token, err := s.db.FamilyShareID(c.Context())
	if err != nil {
		return store.Wishlist{}, err
	}
	if c.Params("family") != token {
		return store.Wishlist{}, store.ErrNotFound
	}
	list, err := s.db.Wishlist(c.Context(), c.Params("list"))
	if err != nil {
		return store.Wishlist{}, err
	}
	if !list.FamilyVisible {
		return store.Wishlist{}, store.ErrNotFound
	}
	return list, nil
}
