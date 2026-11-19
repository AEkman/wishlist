package web

import (
	"bytes"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/AEkman/wishlist/internal/store"
	"github.com/gofiber/fiber/v3"
	_ "golang.org/x/image/webp"
)

const maxImagePixels = 24_000_000

var errMultipleImages = errors.New("multiple images submitted")

func (s server) compare(c fiber.Ctx) error {
	list, err := s.db.Wishlist(c.Context(), c.Params("list"))
	if err != nil {
		return err
	}
	wishes, err := s.db.Wishes(c.Context(), list.ID)
	if err != nil {
		return err
	}
	data := page{View: "compare", Title: list.Title, Wishlist: list}
	if id := c.Query("done"); id != "" {
		wish, err := s.db.Wish(c.Context(), list.ID, id)
		if err != nil {
			return err
		}
		data.Wish, data.Completed = wish, true
		return s.show(c, data)
	}
	if id := c.Query("upload"); id != "" && len(wishes) > 1 {
		return s.uploadComparison(c, data, wishes, id)
	}
	switch len(wishes) {
	case 0:
	case 1:
		data.Wish = wishes[0]
	default:
		left, right := randomPair(len(wishes), s.draw)
		data.Left, data.Right, data.HasPair = wishes[left], wishes[right], true
	}
	return s.show(c, data)
}

func (s server) addForm(c fiber.Ctx) error {
	list, err := s.db.Wishlist(c.Context(), c.Params("list"))
	if err != nil {
		return err
	}
	return s.show(c, page{View: "add", Title: "Lägg till en önskan", Wishlist: list})
}

func (s server) addWish(c fiber.Ctx) error {
	list, err := s.db.Wishlist(c.Context(), c.Params("list"))
	if err != nil {
		return err
	}
	title, description := c.FormValue("title"), c.FormValue("description")
	data := page{View: "add", Title: "Lägg till en önskan", Wishlist: list, Wish: store.Wish{Title: title, Description: description}}
	image, mediaType, err := readImage(c, true)
	if err != nil {
		return s.wishFormError(c, data, err)
	}
	wish, err := s.db.CreateWish(c.Context(), list.ID, title, description, mediaType, image)
	if err != nil {
		return s.wishFormError(c, data, err)
	}
	return redirect(c, "/lists/"+list.ID+"?"+url.Values{"upload": {wish.ID}}.Encode())
}

func (s server) editForm(c fiber.Ctx) error {
	data, err := s.editPage(c)
	if err != nil {
		return err
	}
	return s.show(c, data)
}

func (s server) editWish(c fiber.Ctx) error {
	data, err := s.editPage(c)
	if err != nil {
		return err
	}
	data.Wish.Title, data.Wish.Description = c.FormValue("title"), c.FormValue("description")
	image, mediaType, err := readImage(c, false)
	if err != nil {
		return s.wishFormError(c, data, err)
	}
	if _, err := s.db.UpdateWish(c.Context(), data.Wishlist.ID, data.Wish.ID, data.Wish.Title, data.Wish.Description, mediaType, image); err != nil {
		return s.wishFormError(c, data, err)
	}
	return redirect(c, "/lists/"+data.Wishlist.ID+"/ranked")
}

func (s server) editPage(c fiber.Ctx) (page, error) {
	list, err := s.db.Wishlist(c.Context(), c.Params("list"))
	if err != nil {
		return page{}, err
	}
	wish, err := s.db.Wish(c.Context(), list.ID, c.Params("wish"))
	if err != nil {
		return page{}, err
	}
	return page{View: "edit", Title: "Ändra önskan", Wishlist: list, Wish: wish}, nil
}

const personalRankLimit = 10
const featuredWishCount = 5

func (s server) ranked(c fiber.Ctx) error {
	list, err := s.db.Wishlist(c.Context(), c.Params("list"))
	if err != nil {
		return err
	}
	wishes, err := s.db.Wishes(c.Context(), list.ID)
	if err != nil {
		return err
	}
	baseURL := s.baseURL
	if baseURL == "" {
		baseURL = c.Scheme() + "://" + c.Host()
	}
	featured := min(len(wishes), personalRankLimit)
	return s.show(c, page{
		View: "ranked", Title: list.Title, Wishlist: list,
		ShareURL:    baseURL + "/" + list.ShareID,
		Wishes:      inRankOrder(wishes[:featured]),
		OtherWishes: wishes[featured:],
	})
}

func inRankOrder(wishes []store.Wish) []rankedWish {
	ranks := make([]rankedWish, len(wishes))
	for i, wish := range wishes {
		ranks[i] = rankedWish{Wish: wish, Rank: i + 1, Large: i < featuredWishCount}
	}
	return ranks
}

func findWishIndex(wishes []store.Wish, id string) int {
	return slices.IndexFunc(wishes, func(wish store.Wish) bool { return wish.ID == id })
}

func (s server) image(c fiber.Ctx) error {
	mediaType, image, err := s.db.Image(c.Context(), c.Params("wish"))
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, mediaType)
	return c.Send(image)
}

func readImage(c fiber.Ctx, required bool) ([]byte, string, error) {
	form, err := c.MultipartForm()
	if err != nil {
		return nil, "", store.ErrInvalid
	}
	files := form.File["image"]
	cameraFiles := form.File["camera"]
	if len(files)+len(cameraFiles) > 1 {
		return nil, "", errMultipleImages
	}
	if len(cameraFiles) > 0 {
		files = cameraFiles
	}
	if len(files) == 0 && !required {
		return nil, "", nil
	}
	if len(files) != 1 || files[0].Size > maxImageSize {
		return nil, "", store.ErrInvalid
	}
	file, err := files[0].Open()
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	payload, err := io.ReadAll(io.LimitReader(file, maxImageSize+1))
	if err != nil {
		return nil, "", err
	}
	if len(payload) > maxImageSize {
		return nil, "", store.ErrInvalid
	}
	mediaType := http.DetectContentType(payload)
	switch mediaType {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
	default:
		return nil, "", store.ErrInvalid
	}

	config, _, err := image.DecodeConfig(bytes.NewReader(payload))
	if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > maxImagePixels/config.Height {
		return nil, "", store.ErrInvalid
	}
	if _, _, err := image.Decode(bytes.NewReader(payload)); err != nil {
		return nil, "", store.ErrInvalid
	}
	return payload, mediaType, nil
}

func (s server) wishFormError(c fiber.Ctx, data page, err error) error {
	switch {
	case errors.Is(err, errMultipleImages):
		data.Error = "Välj en bild. Använd antingen Ta bild eller Välj bild."
	case errors.Is(err, store.ErrInvalid):
		data.Error = "Välj en giltig JPEG-, PNG-, GIF- eller WebP-bild (högst 8 MB och 24 megapixlar)."
		if strings.TrimSpace(data.Wish.Title) == "" {
			data.Error = "Ange en titel."
		}
	default:
		return err
	}
	c.Status(http.StatusBadRequest)
	return s.show(c, data)
}
