package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestFavoriteChoicesShowPictureAndTitleWithoutDescription(t *testing.T) {
	db, app := testApp(t)
	child, err := db.CreateChild(t.Context(), "Alex")
	if err != nil {
		t.Fatal(err)
	}
	list, err := db.CreateWishlist(t.Context(), child.ID, "Jul")
	if err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Cykel", "Boll"} {
		_, err := db.CreateWish(t.Context(), list.ID, title, "Hemlig beskrivning", "image/png", []byte{1})
		if err != nil {
			t.Fatal(err)
		}
	}
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/lists/"+list.ID, nil))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := html.Parse(strings.NewReader(readBody(t, res)))
	if err != nil {
		t.Fatal(err)
	}
	cards := htmlNodes(doc, func(n *html.Node) bool { return hasClass(n, "pick-card") })
	if len(cards) != 2 {
		t.Fatalf("got %d choices, want 2", len(cards))
	}
	for _, card := range cards {
		if len(htmlNodes(card, isElement("img"))) != 1 || strings.TrimSpace(htmlText(card)) == "" {
			t.Fatal("each choice must show a picture and title")
		}
		if strings.Contains(htmlText(card), "Hemlig beskrivning") {
			t.Fatal("favorite choices must not show descriptions")
		}
	}
}
