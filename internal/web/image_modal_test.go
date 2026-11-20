package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestWishlistImagesOpenAccessiblePreview(t *testing.T) {
	db, app := testApp(t)
	child, _ := db.CreateChild(t.Context(), "Ella")
	list, _ := db.CreateWishlist(t.Context(), child.ID, "Jul")
	wish, err := db.CreateWish(t.Context(), list.ID, "Cykel", "", "image/png", []byte{1})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SetFamilyVisible(t.Context(), list.ID, true); err != nil {
		t.Fatal(err)
	}
	token, err := db.FamilyShareID(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/lists/" + list.ID, "/family/" + token + "/lists/" + list.ID, "/lists/" + list.ID + "/ranked", "/" + list.ShareID} {
		response, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatal(err)
		}
		doc, err := html.Parse(strings.NewReader(readBody(t, response)))
		if err != nil {
			t.Fatal(err)
		}
		scripts := htmlNodes(doc, func(n *html.Node) bool {
			return isElement("script")(n) && htmlAttr(n, "src") == "/static/image-preview.js" && htmlAttr(n, "type") == "module"
		})
		if len(scripts) != 1 {
			t.Error("preview module must load on the page")
		}
		links := htmlNodes(doc, func(n *html.Node) bool { return isElement("a")(n) && hasHTMLAttr(n, "data-image-preview") })
		if len(links) != 1 {
			t.Errorf("%s: expected one image preview link", path)
			continue
		}
		images := htmlNodes(links[0], isElement("img"))
		if len(images) != 1 || htmlAttr(links[0], "href") != htmlAttr(images[0], "src") || !strings.Contains(htmlAttr(links[0], "href"), wish.ID) || htmlAttr(links[0], "hx-boost") != "false" || htmlAttr(links[0], "aria-label") == "" {
			t.Error("preview must retain the image URL and accessible no-JS fallback")
		}
		dialogs := htmlNodes(doc, func(n *html.Node) bool {
			return isElement("dialog")(n) && htmlAttr(n, "id") == "wish-image-dialog" && htmlAttr(n, "aria-labelledby") == "wish-image-title"
		})
		if len(dialogs) != 1 {
			t.Error("page must contain a labelled preview dialog")
		}
	}
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/static/image-preview.js", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/javascript") {
		t.Error("preview module must be served as JavaScript")
	}
}
