package web

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/AEkman/wishlist/internal/store"
	"github.com/gofiber/fiber/v3"
)

func TestRankedViewShowsWishesInUniformCards(t *testing.T) {
	db, app := testApp(t)
	child, err := db.CreateChild(t.Context(), "Alex")
	if err != nil {
		t.Fatal(err)
	}
	list, err := db.CreateWishlist(t.Context(), child.ID, "Jul")
	if err != nil {
		t.Fatal(err)
	}
	var last store.Wish
	for i := 1; i <= 11; i++ {
		last, err = db.CreateWish(t.Context(), list.ID, "Önskan "+strconv.Itoa(i), "", "image/png", []byte{1})
		if err != nil {
			t.Fatal(err)
		}
	}

	path := "/lists/" + list.ID
	res, err := app.Test(httptest.NewRequest(http.MethodGet, path+"/ranked", nil))
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, res)
	if !strings.Contains(body, `class="ranked-card" data-rank="11"`) || !strings.Contains(body, path+"/wishes/"+last.ID+"/edit") || !strings.Contains(body, "/images/"+last.ID) {
		t.Fatal("ranked view should show the eleventh wish in a standard card with its image and edit link")
	}
}

func TestCreateWishlistBelongsToSelectedChild(t *testing.T) {
	db, app := testApp(t)
	alex, err := db.CreateChild(t.Context(), "Alex")
	if err != nil {
		t.Fatal(err)
	}
	sam, err := db.CreateChild(t.Context(), "Sam")
	if err != nil {
		t.Fatal(err)
	}

	path := "/children/" + alex.ID + "/wishlists"
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(url.Values{"title": {"Födelsedag"}}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST %s returned %d, want %d", path, response.StatusCode, http.StatusSeeOther)
	}
	for _, tc := range []struct {
		childID string
		want    bool
	}{{alex.ID, true}, {sam.ID, false}} {
		response, err := app.Test(httptest.NewRequest(http.MethodGet, "/children/"+tc.childID, nil))
		if err != nil {
			t.Fatal(err)
		}
		body := readBody(t, response)
		if response.StatusCode != http.StatusOK || strings.Contains(body, "Födelsedag") != tc.want {
			t.Fatalf("child %s: got status %d and body %q", tc.childID, response.StatusCode, body)
		}
	}
}

func TestHomeShowsChildCard(t *testing.T) {
	db, app := testApp(t)
	child, err := db.CreateChild(t.Context(), "Alex")
	if err != nil {
		t.Fatal(err)
	}
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, res)
	if res.StatusCode != http.StatusOK || !strings.Contains(body, childPath(child)) || !strings.Contains(body, "Alex") {
		t.Fatalf("got status %d and body %q, want a card linking to Alex's page", res.StatusCode, body)
	}
}

func testApp(t *testing.T) (*store.Store, *fiber.App) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	app, err := newApp(db, "", func(int) int { return 0 }, os.Getenv("RANKING_DEBUG") == "1")
	if err != nil {
		t.Fatal(err)
	}
	return db, app
}

func readBody(t *testing.T, res *http.Response) string {
	t.Helper()
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestChildLinkUsesNameOnly(t *testing.T) {
	db, app := testApp(t)
	if _, err := db.CreateChild(t.Context(), "Alex"); err != nil {
		t.Fatal(err)
	}
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	if body := readBody(t, res); !strings.Contains(body, `href="/children/alex"`) {
		t.Fatal("missing name-only child link")
	}
	res, err = app.Test(httptest.NewRequest(http.MethodGet, "/children/alex", nil))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("name-only link returned %d", res.StatusCode)
	}
	_ = res.Body.Close()
}

func TestNamedChildRouteCreatesWishlist(t *testing.T) {
	db, app := testApp(t)
	child, err := db.CreateChild(t.Context(), "Åsa & Bo")
	if err != nil {
		t.Fatal(err)
	}
	path := childPath(child)
	request := httptest.NewRequest(http.MethodPost, path+"/wishlists", strings.NewReader(url.Values{"title": {"Jul"}}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("create wishlist returned %d", res.StatusCode)
	}
	lists, err := db.Wishlists(t.Context(), child.ID)
	if err != nil || len(lists) != 1 || lists[0].Title != "Jul" {
		t.Fatalf("got wishlists %v, error %v", lists, err)
	}
}

func TestDuplicateChildNameIsRejected(t *testing.T) {
	db, _ := testApp(t)
	if _, err := db.CreateChild(t.Context(), "Alex"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateChild(t.Context(), "Alex"); !errors.Is(err, store.ErrInvalid) {
		t.Fatalf("duplicate child: got %v", err)
	}
}
