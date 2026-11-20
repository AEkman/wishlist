package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/image/webp"
	"golang.org/x/net/html"
)

func TestHomeShowsHeroImage(t *testing.T) {
	_, app := testApp(t)
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := html.Parse(strings.NewReader(readBody(t, res)))
	if err != nil {
		t.Fatal(err)
	}
	images := htmlNodes(doc, isElement("img"))
	if len(images) != 1 || htmlAttr(images[0], "src") == "" {
		t.Fatal("home page should contain a hero image with a source")
	}

	res, err = app.Test(httptest.NewRequest(http.MethodGet, htmlAttr(images[0], "src"), nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "image/webp" {
		t.Fatalf("hero image returned status %d and content type %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
	if _, err := webp.Decode(res.Body); err != nil {
		t.Fatalf("decode hero image: %v", err)
	}
}

func TestHomeOffersChildCreationInDialog(t *testing.T) {
	for _, withChild := range []bool{false, true} {
		db, app := testApp(t)
		if withChild {
			if _, err := db.CreateChild(t.Context(), "Alex"); err != nil {
				t.Fatal(err)
			}
		}
		res, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
		if err != nil {
			t.Fatal(err)
		}
		doc, err := html.Parse(strings.NewReader(readBody(t, res)))
		if err != nil {
			t.Fatal(err)
		}
		cards := htmlNodes(doc, func(n *html.Node) bool {
			return isElement("button")(n) && htmlAttr(n, "aria-controls") == "add-child-dialog"
		})
		dialogs := htmlNodes(doc, func(n *html.Node) bool { return isElement("dialog")(n) && htmlAttr(n, "id") == "add-child-dialog" })
		if len(cards) != 1 || len(dialogs) != 1 {
			t.Fatalf("with child %v: expected a card button and dialog", withChild)
		}
		forms := htmlNodes(dialogs[0], func(n *html.Node) bool { return isElement("form")(n) && htmlAttr(n, "action") == "/children" })
		if len(forms) != 1 || len(htmlNodes(forms[0], func(n *html.Node) bool { return isElement("input")(n) && htmlAttr(n, "name") == "name" })) != 1 {
			t.Fatal("dialog must contain the child creation form")
		}
	}
}

func TestChildDialogScriptIsServed(t *testing.T) {
	_, app := testApp(t)
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/static/child-dialog.js", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/javascript") {
		t.Fatalf("dialog script returned status %d and content type %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
}

func TestPagesProvideFavicon(t *testing.T) {
	_, app := testApp(t)
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := html.Parse(strings.NewReader(readBody(t, res)))
	if err != nil {
		t.Fatal(err)
	}
	icons := htmlNodes(doc, func(n *html.Node) bool {
		return isElement("link")(n) && htmlAttr(n, "rel") == "icon"
	})
	if len(icons) != 1 || htmlAttr(icons[0], "href") == "" {
		t.Fatal("page must link to a favicon")
	}
	res, err = app.Test(httptest.NewRequest(http.MethodGet, htmlAttr(icons[0], "href"), nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "image/svg+xml") {
		t.Fatalf("favicon returned status %d and content type %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
}

func TestHtmxUsesCurrentMajorRelease(t *testing.T) {
	_, app := testApp(t)
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/static/htmx.min.js", nil))
	if err != nil {
		t.Fatal(err)
	}
	if body := readBody(t, res); !strings.Contains(body, `4.0.2`) {
		t.Fatal("htmx 4.0.2 must be served")
	}
}
