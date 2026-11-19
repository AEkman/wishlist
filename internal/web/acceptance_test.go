package web

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/AEkman/wishlist/internal/store"
	"github.com/cucumber/godog"
	"github.com/gofiber/fiber/v3"
	"golang.org/x/net/html"
)

const acceptanceBaseURL = "https://wishlist.example"

type acceptance struct {
	app      *fiber.App
	db       *store.Store
	ctx      context.Context
	draws    []int
	response *http.Response
	body     string
	doc      *html.Node
	child    store.Child
	wishlist store.Wishlist
}

func TestAcceptance(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			a := new(acceptance)
			sc.Before(a.start)
			sc.After(a.stop)
			a.registerNavigationSteps(sc)
			a.registerRankingSteps(sc)
			a.registerUploadSteps(sc)
			a.registerSharingSteps(sc)
		},
		Options: &godog.Options{
			Paths: []string{
				"../../features/wishlist_access.feature",
				"../../features/wishlist_ranking.feature",
				"../../features/wish_upload.feature",
				"../../features/wishlist_sharing.feature",
			},
			Tags:           "~@wip",
			Format:         "progress",
			Strict:         true,
			NoColors:       true,
			DefaultContext: t.Context(),
			TestingT:       t,
		},
	}
	if suite.Run() != 0 {
		t.Error("wishlist acceptance scenarios failed")
	}
}

func (a *acceptance) start(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
	a.ctx = ctx
	var err error
	a.db, err = store.Open(":memory:")
	if err != nil {
		return ctx, err
	}
	a.app, err = newApp(a.db, acceptanceBaseURL, func(_ int) int {
		if len(a.draws) == 0 {
			return 0
		}
		draw := a.draws[0]
		a.draws = a.draws[1:]
		return draw
	})
	return ctx, err
}

func (a *acceptance) stop(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
	if a.db == nil {
		return ctx, nil
	}
	return ctx, a.db.Close()
}

func (a *acceptance) request(method, path string, body io.Reader, contentType string) error {
	request := httptest.NewRequestWithContext(a.ctx, method, path, body)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := a.app.Test(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	a.response, a.body, a.doc = response, string(data), nil
	mediaType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if mediaType == "text/html" {
		a.doc, err = html.Parse(bytes.NewReader(data))
	}
	return err
}

func (a *acceptance) get(path string) error {
	if err := a.request(http.MethodGet, path, nil, ""); err != nil {
		return err
	}
	return a.expectStatus(http.StatusOK)
}

func (a *acceptance) postForm(path string, values url.Values) error {
	return a.request(http.MethodPost, path, strings.NewReader(values.Encode()), "application/x-www-form-urlencoded")
}

func (a *acceptance) follow() error {
	if err := a.expectStatus(http.StatusSeeOther); err != nil {
		return err
	}
	location := a.response.Header.Get("Location")
	if location == "" {
		return fmt.Errorf("redirect has no Location header")
	}
	return a.get(location)
}

func (a *acceptance) expectStatus(want int) error {
	if a.response == nil {
		return fmt.Errorf("no HTTP response")
	}
	if a.response.StatusCode != want {
		return fmt.Errorf("HTTP status %d, want %d", a.response.StatusCode, want)
	}
	return nil
}

func (a *acceptance) seedWishlist(childName, title string) error {
	var err error
	a.child, err = a.db.CreateChild(a.ctx, childName)
	if err != nil {
		return err
	}
	a.wishlist, err = a.db.CreateWishlist(a.ctx, a.child.ID, title)
	return err
}

func (a *acceptance) seedWish(title string) (store.Wish, error) {
	var picture bytes.Buffer
	if err := png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		return store.Wish{}, err
	}
	return a.db.CreateWish(a.ctx, a.wishlist.ID, title, "", "image/png", picture.Bytes())
}

func (a *acceptance) findWish(title string) (store.Wish, error) {
	wishes, err := a.db.Wishes(a.ctx, a.wishlist.ID)
	if err != nil {
		return store.Wish{}, err
	}
	for _, wish := range wishes {
		if wish.Title == title {
			return wish, nil
		}
	}
	return store.Wish{}, fmt.Errorf("wish %q not found", title)
}

func (a *acceptance) listPath() string {
	return "/lists/" + a.wishlist.ID
}

func (a *acceptance) clickLink(label string) error {
	for _, link := range htmlNodes(a.doc, isElement("a")) {
		if htmlText(link) == label {
			return a.get(htmlAttr(link, "href"))
		}
	}
	return fmt.Errorf("link %q not found", label)
}

func htmlNodes(node *html.Node, match func(*html.Node) bool) []*html.Node {
	if node == nil {
		return nil
	}
	var nodes []*html.Node
	for descendant := range node.Descendants() {
		if match(descendant) {
			nodes = append(nodes, descendant)
		}
	}
	return nodes
}

func isElement(tag string) func(*html.Node) bool {
	return func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == tag
	}
}

func htmlAttr(node *html.Node, key string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == key {
			return attribute.Val
		}
	}
	return ""
}

func htmlText(node *html.Node) string {
	var text strings.Builder
	for descendant := range node.Descendants() {
		if descendant.Type == html.TextNode {
			text.WriteString(descendant.Data)
		}
	}
	return strings.Join(strings.Fields(text.String()), " ")
}
