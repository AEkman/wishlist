package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestSharingPanelsOfferCopyAndLoadSharingScript(t *testing.T) {
	db, app := testApp(t)
	child, _ := db.CreateChild(t.Context(), "Ella")
	list, _ := db.CreateWishlist(t.Context(), child.ID, "Jul")
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/lists/"+list.ID+"/ranked", nil))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := html.Parse(strings.NewReader(readBody(t, response)))
	if err != nil {
		t.Fatal(err)
	}
	panels := htmlNodes(doc, func(n *html.Node) bool {
		return isElement("details")(n) && strings.Contains(htmlAttr(n, "class"), "share-action")
	})
	if len(panels) != 2 {
		t.Fatalf("expected list and family sharing panels, got %d", len(panels))
	}
	for _, panel := range panels {
		buttons := htmlNodes(panel, func(n *html.Node) bool {
			return isElement("button")(n) && hasHTMLAttr(n, "data-copy-share")
		})
		statuses := htmlNodes(panel, func(n *html.Node) bool { return htmlAttr(n, "role") == "status" && hasHTMLAttr(n, "data-share-status") })
		if len(buttons) != 1 || len(statuses) != 1 {
			t.Error("sharing panel must offer a copy button and accessible status")
		}
	}
	scripts := htmlNodes(doc, func(n *html.Node) bool {
		return isElement("script")(n) && htmlAttr(n, "src") == "/static/share-links.js" && htmlAttr(n, "type") == "module"
	})
	if len(scripts) != 1 {
		t.Fatal("page must load sharing module")
	}
	response, err = app.Test(httptest.NewRequest(http.MethodGet, "/static/share-links.js", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/javascript") {
		t.Fatal("sharing module must be served as JavaScript")
	}
}

func hasHTMLAttr(n *html.Node, name string) bool {
	for _, attr := range n.Attr {
		if attr.Key == name {
			return true
		}
	}
	return false
}
