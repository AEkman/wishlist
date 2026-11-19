package web

import (
	"bytes"
	"embed"
	"errors"
	"html/template"
	"math/rand/v2"
	"net/http"
	"os"
	"strings"

	"github.com/AEkman/wishlist/internal/store"
	"github.com/gofiber/fiber/v3"
)

//go:embed templates/*.html static
var assets embed.FS

const maxImageSize = 8 << 20

type page struct {
	View, Title, Error, ShareURL string
	Child                        store.Child
	Wishlist                     store.Wishlist
	Wish, Left, Right            store.Wish
	Children                     []store.Child
	Wishlists                    []store.Wishlist
	Wishes                       []rankedWish
	OtherWishes                  []store.Wish
	HasPair, Upload, Completed   bool
	Tried                        []string
	Remaining                    int
	RankingDebug                 bool
}

type rankedWish struct {
	store.Wish
	Rank  int
	Large bool
}

type server struct {
	db           *store.Store
	pages        *template.Template
	baseURL      string
	draw         func(int) int
	rankingDebug bool
}

func New(db *store.Store, baseURL string) (*fiber.App, error) {
	return newApp(db, baseURL, rand.IntN, os.Getenv("RANKING_DEBUG") == "1")
}

func newApp(db *store.Store, baseURL string, draw func(int) int, rankingDebug bool) (*fiber.App, error) {
	pages, err := template.ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, err
	}
	s := server{db: db, pages: pages, baseURL: strings.TrimRight(baseURL, "/"), draw: draw, rankingDebug: rankingDebug}
	app := fiber.New(fiber.Config{
		BodyLimit:    maxImageSize + (1 << 20),
		ErrorHandler: s.handleError,
	})
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("Referrer-Policy", "no-referrer")
		return c.Next()
	})
	app.Get("/", s.home)
	app.Get("/static/:file", s.asset)
	app.Post("/children", s.createChild)
	app.Get("/children/:child", s.child)
	app.Post("/children/:child/wishlists", s.createWishlist)
	app.Get("/lists/:list", s.compare)
	app.Post("/lists/:list/pick", s.pick)
	app.Get("/lists/:list/ranked", s.ranked)
	app.Get("/lists/:list/wishes/new", s.addForm)
	app.Post("/lists/:list/wishes/new", s.addWish)
	app.Get("/lists/:list/wishes/:wish/edit", s.editForm)
	app.Post("/lists/:list/wishes/:wish/edit", s.editWish)
	app.Get("/images/:wish", s.image)
	app.Get("/:share", s.shared)
	app.Get("/:share/wishes/:wish/buy", s.buyForm)
	app.Post("/:share/wishes/:wish/buy", s.buy)
	app.Get("/:share/wishes/:wish/clear", s.clearForm)
	app.Post("/:share/wishes/:wish/clear", s.clearPurchase)
	return app, nil
}

func (s server) show(c fiber.Ctx, data page) error {
	data.RankingDebug = s.rankingDebug
	var body bytes.Buffer
	if err := s.pages.ExecuteTemplate(&body, "page", data); err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
	return c.Send(body.Bytes())
}

func (s server) home(c fiber.Ctx) error {
	children, err := s.db.Children(c.Context())
	if err != nil {
		return err
	}
	return s.show(c, page{View: "children", Children: children})
}

func (s server) asset(c fiber.Ctx) error {
	file := c.Params("file")
	var contentType string
	switch file {
	case "app.css":
		contentType = "text/css; charset=utf-8"
	case "htmx.min.js", "ranking-debug.js":
		contentType = "text/javascript; charset=utf-8"
	case "htmx.LICENSE":
		contentType = "text/plain; charset=utf-8"
	default:
		return fiber.ErrNotFound
	}
	data, err := assets.ReadFile("static/" + file)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, contentType)
	return c.Send(data)
}

func (s server) createChild(c fiber.Ctx) error {
	child, err := s.db.CreateChild(c.Context(), c.FormValue("name"))
	if err != nil {
		return err
	}
	return redirect(c, "/children/"+child.ID)
}

func (s server) child(c fiber.Ctx) error {
	child, err := s.db.Child(c.Context(), c.Params("child"))
	if err != nil {
		return err
	}
	lists, err := s.db.Wishlists(c.Context(), child.ID)
	if err != nil {
		return err
	}
	return s.show(c, page{View: "lists", Title: child.Name, Child: child, Wishlists: lists})
}

func (s server) createWishlist(c fiber.Ctx) error {
	list, err := s.db.CreateWishlist(c.Context(), c.Params("child"), c.FormValue("title"))
	if err != nil {
		return err
	}
	return redirect(c, "/lists/"+list.ID)
}

func redirect(c fiber.Ctx, location string) error {
	return c.Redirect().Status(http.StatusSeeOther).To(location)
}

func (s server) handleError(c fiber.Ctx, err error) error {
	status, message := http.StatusInternalServerError, "Något gick fel."
	switch {
	case errors.Is(err, store.ErrNotFound):
		status, message = http.StatusNotFound, "Sidan finns inte."
	case errors.Is(err, store.ErrInvalid):
		status, message = http.StatusBadRequest, "Kontrollera uppgifterna och försök igen."
	default:
		if fiberErr, ok := errors.AsType[*fiber.Error](err); ok {
			status = fiberErr.Code
			if status == http.StatusNotFound {
				message = "Sidan finns inte."
			}
		}
	}
	c.Status(status)
	return s.show(c, page{View: "error", Title: message})
}
