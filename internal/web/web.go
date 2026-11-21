package web

import (
	"bytes"
	"cmp"

	"embed"
	"errors"
	"html/template"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/AEkman/wishlist/internal/store"
	"github.com/gofiber/fiber/v3"
)

//go:embed templates/*.html static
var assets embed.FS

const maxImageSize = 8 << 20

type breadcrumb struct {
	Label, URL string
}

type page struct {
	PreviewURL, PreviewImageURL        string
	FamilyURL, SharedListURL, BrandURL string
	SharedChildren                     []store.Child
	SharedLists                        []store.Wishlist
	SharedChildURLs, SharedListURLs    map[string]string
	Breadcrumbs                        []breadcrumb
	ChildURLs                          map[string]string
	View, Title, Error, ShareURL       string
	Next                               string
	Child                              store.Child
	Wishlist                           store.Wishlist
	Wish, Left, Right                  store.Wish
	Children                           []store.Child
	Wishlists                          []store.Wishlist
	Wishes                             []rankedWish
	RankingResult                      rankedWish
	NewFavorite                        bool
	HasPair, Upload, Completed, Focus  bool
	Insertion                          insertionState
	Remaining                          int
	RankingDebug                       bool
	Guided                             *store.GuidedProgress
	GuidedWish                         store.Wish
	GuidedLink                         *store.GuidedProgress
}

type rankedWish struct {
	store.Wish
	Rank int
}

type server struct {
	db           *store.Store
	pages        *template.Template
	baseURL      string
	draw         func(int) int
	rankingDebug bool
}

func New(db *store.Store, baseURL string) (*fiber.App, error) {
	pin := os.Getenv("FAMILY_PIN")
	if !validPIN(pin) {
		return nil, errors.New("FAMILY_PIN must contain 4 to 12 digits")
	}
	return newApp(db, baseURL, rand.IntN, os.Getenv("RANKING_DEBUG") == "1", pin)
}

func newApp(db *store.Store, baseURL string, draw func(int) int, rankingDebug bool, configuredPIN ...string) (*fiber.App, error) {
	pages, err := template.New("page.html").Funcs(template.FuncMap{"childPath": childPath}).ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, err
	}
	s := server{db: db, pages: pages, baseURL: strings.TrimRight(baseURL, "/"), draw: draw, rankingDebug: rankingDebug}
	app := fiber.New(fiber.Config{
		BodyLimit:    maxImageSize + (1 << 20),
		ErrorHandler: s.handleError,
	})
	app.Use(func(c fiber.Ctx) error {
		started := time.Now()
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("Referrer-Policy", "no-referrer")
		err := c.Next()
		requestErr := err
		if err != nil {
			err = s.handleError(c, err)
		}
		attributes := []any{"method", c.Method(), "route", c.Route().Path, "status", c.Response().StatusCode(), "duration", time.Since(started)}
		if c.Response().StatusCode() >= http.StatusInternalServerError {
			slog.Error("HTTP request", append(attributes, "error", requestErr)...)
		} else {
			slog.Info("HTTP request", attributes...)
		}
		return err
	})
	if len(configuredPIN) > 0 {
		pin := configuredPIN[0]
		app.Get("/unlock", s.unlockForm)
		app.Post("/unlock", s.unlock(pin))
		app.Use(s.requirePIN(pin))
	}
	app.Get("/", s.home)
	app.Get("/static/:file", s.asset)
	app.Post("/children", s.createChild)
	app.Get("/children/:child", s.child)
	app.Post("/children/:child/wishlists", s.createWishlist)
	app.Get("/lists/:list", s.compare)
	app.Post("/lists/:list/pick", s.pick)
	app.Post("/lists/:list/ranking/undo", s.guidedUndo)
	app.Post("/lists/:list/ranking/restart", s.guidedRestart)
	app.Get("/lists/:list/ranked", s.ranked)
	app.Post("/lists/:list/ranked/move", s.moveWish)
	app.Get("/lists/:list/wishes/new", s.addForm)
	app.Post("/lists/:list/wishes/new", s.addWish)
	app.Get("/lists/:list/wishes/:wish/edit", s.editForm)
	app.Post("/lists/:list/wishes/:wish/edit", s.editWish)
	app.Get("/images/:wish", s.image)
	app.Post("/lists/:list/sharing", s.updateSharing)
	app.Get("/family/:family", s.family)
	app.Get("/family/:family/children/:child", s.familyChild)
	app.Get("/family/:family/lists/:list", s.shared)
	app.Get("/family/:family/lists/:list/images/:wish", s.sharedImage)
	app.Get("/family/:family/lists/:list/wishes/:wish/buy", s.buyForm)
	app.Post("/family/:family/lists/:list/wishes/:wish/buy", s.buy)
	app.Get("/family/:family/lists/:list/wishes/:wish/clear", s.clearForm)
	app.Post("/family/:family/lists/:list/wishes/:wish/clear", s.clearPurchase)
	app.Get("/:share/images/:wish", s.sharedImage)
	app.Get("/:share", s.shared)
	app.Get("/:share/wishes/:wish/buy", s.buyForm)
	app.Post("/:share/wishes/:wish/buy", s.buy)
	app.Get("/:share/wishes/:wish/clear", s.clearForm)
	app.Post("/:share/wishes/:wish/clear", s.clearPurchase)
	return app, nil
}

func (s server) show(c fiber.Ctx, data page) error {
	baseURL := cmp.Or(s.baseURL, c.Scheme()+"://"+c.Host())
	data.RankingDebug = s.rankingDebug
	data.BrandURL = "/"
	if data.FamilyURL != "" {
		data.BrandURL = data.FamilyURL
	}
	if data.View == "children" || data.View == "lists" || data.View == "ranked" {
		token, err := s.db.FamilyShareID(c.Context())
		if err != nil {
			return err
		}
		data.FamilyURL = baseURL + "/family/" + token
	}
	if data.Wishlist.ID != "" {
		child, err := s.db.Child(c.Context(), data.Wishlist.ChildID)
		if err != nil {
			return err
		}
		data.Child = child
		if data.SharedListURL == "" && data.FamilyURL == "" {
			data.SharedListURL = "/" + data.Wishlist.ShareID
		}
	}
	if data.SharedListURL != "" && data.FamilyURL == "" && (data.View == "share" || data.View == "purchase" || data.View == "clear") {
		data.BrandURL = data.SharedListURL
	}
	previewPath := "/"
	switch data.View {
	case "share", "purchase", "clear":
		previewPath = data.SharedListURL
	case "family", "family-lists":
		previewPath = c.Path()
	}
	data.PreviewURL = baseURL + previewPath
	data.PreviewImageURL = baseURL + "/static/share-card-v1.jpg"
	children, err := s.db.Children(c.Context())
	if err != nil {
		return err
	}
	data.ChildURLs = make(map[string]string, len(children))
	counts := make(map[string]int, len(children))
	for _, child := range children {
		counts[childPath(child)]++
	}
	for _, child := range children {
		path := childPath(child)
		if counts[path] > 1 {
			path = "/children/" + child.ID
		}
		data.ChildURLs[child.ID] = path
	}
	shared := data.View == "share" || data.View == "purchase" || data.View == "clear"
	if data.View == "family" || data.View == "family-lists" || (shared && data.FamilyURL != "") {
		data.Breadcrumbs = []breadcrumb{{Label: "Familjens önskelistor", URL: data.FamilyURL}}
		if data.Child.ID != "" {
			data.Breadcrumbs = append(data.Breadcrumbs, breadcrumb{Label: data.Child.Name, URL: data.SharedChildURLs[data.Child.ID]})
		}
		if data.Wishlist.ID != "" {
			data.Breadcrumbs = append(data.Breadcrumbs, breadcrumb{Label: data.Wishlist.Title, URL: data.SharedListURL})
		}
	} else if shared {
		data.Breadcrumbs = []breadcrumb{{Label: data.Wishlist.Title, URL: "/" + data.Wishlist.ShareID}}
	} else {
		data.Breadcrumbs = []breadcrumb{{Label: "Alla barn", URL: "/"}}
		if data.Child.ID != "" {
			data.Breadcrumbs = append(data.Breadcrumbs, breadcrumb{Label: data.Child.Name, URL: data.ChildURLs[data.Child.ID]})
		}
		if data.Wishlist.ID != "" {
			listURL := "/lists/" + data.Wishlist.ID
			if data.View == "edit" {
				listURL += "/ranked"
			}
			data.Breadcrumbs = append(data.Breadcrumbs, breadcrumb{Label: data.Wishlist.Title, URL: listURL})
		}
	}
	switch data.View {
	case "ranked":
		data.Breadcrumbs = append(data.Breadcrumbs, breadcrumb{Label: "Önskelistan"})
	case "add", "edit", "error":
		data.Breadcrumbs = append(data.Breadcrumbs, breadcrumb{Label: data.Title})
	case "purchase":
		data.Breadcrumbs = append(data.Breadcrumbs, breadcrumb{Label: "Markera som köpt"})
	case "clear":
		data.Breadcrumbs = append(data.Breadcrumbs, breadcrumb{Label: "Avmarkera som köpt"})
	}
	data.Breadcrumbs[len(data.Breadcrumbs)-1].URL = ""
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
	case "htmx.min.js", "ranking-debug.js", "wish-camera.js", "child-dialog.js", "share-links.js", "image-preview.js", "wish-reorder.js":
		contentType = "text/javascript; charset=utf-8"
	case "htmx.LICENSE":
		contentType = "text/plain; charset=utf-8"
	case "share-card-v1.jpg":
		contentType = "image/jpeg"
	case "wishlist-hero.webp":
		contentType = "image/webp"
	case "favicon.svg":
		contentType = "image/svg+xml"
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
	return redirect(c, childPath(child))
}

func (s server) child(c fiber.Ctx) error {
	child, err := s.resolveChild(c)
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
	child, err := s.resolveChild(c)
	if err != nil {
		return err
	}
	list, err := s.db.CreateWishlist(c.Context(), child.ID, c.FormValue("title"))
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

func childPath(child store.Child) string {
	return "/children/" + child.ID
}

func (s server) resolveChild(c fiber.Ctx) (store.Child, error) {
	key, err := url.PathUnescape(c.Params("child"))
	if err != nil {
		return store.Child{}, store.ErrNotFound
	}
	if child, err := s.db.Child(c.Context(), key); err == nil {
		return child, nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return store.Child{}, err
	}
	return store.Child{}, store.ErrNotFound
}
