package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestBreadcrumbsShowLocationAndParentLinks(t *testing.T) {
	db, app := testApp(t)
	child, err := db.CreateChild(t.Context(), "Alex")
	if err != nil {
		t.Fatal(err)
	}
	list, err := db.CreateWishlist(t.Context(), child.ID, "Jul")
	if err != nil {
		t.Fatal(err)
	}
	wish, err := db.CreateWish(t.Context(), list.ID, "Cykel", "", "image/png", []byte{1})
	if err != nil {
		t.Fatal(err)
	}
	base := "/lists/" + list.ID
	for _, tc := range []struct {
		path   string
		labels string
		parent string
	}{
		{"/children/" + child.ID, "Alla barn|Alex", "/"},
		{base, "Alla barn|Alex|Jul", childPath(child)},
		{base + "/ranked", "Alla barn|Alex|Jul|Önskelistan", base},
		{base + "/wishes/new", "Alla barn|Alex|Jul|Lägg till en önskan", base},
		{base + "/wishes/" + wish.ID + "/edit", "Alla barn|Alex|Jul|Ändra önskan", base + "/ranked"},
		{"/" + list.ShareID, "Jul", ""},
		{"/" + list.ShareID + "/wishes/" + wish.ID + "/buy", "Jul|Markera som köpt", "/" + list.ShareID},
		{"/" + list.ShareID + "/wishes/" + wish.ID + "/clear", "Jul|Avmarkera som köpt", "/" + list.ShareID},
	} {
		t.Run(tc.path, func(t *testing.T) {
			res, err := app.Test(httptest.NewRequest(http.MethodGet, tc.path, nil))
			if err != nil {
				t.Fatal(err)
			}
			doc, err := html.Parse(strings.NewReader(readBody(t, res)))
			if err != nil {
				t.Fatal(err)
			}
			navs := htmlNodes(doc, func(n *html.Node) bool { return isElement("nav")(n) && htmlAttr(n, "aria-label") == "Brödsmulor" })
			if len(navs) != 1 {
				t.Fatalf("want one breadcrumb navigation, got %d", len(navs))
			}
			items := htmlNodes(navs[0], isElement("li"))
			var labels []string
			for _, item := range items {
				labels = append(labels, strings.TrimSpace(htmlText(item)))
			}
			if strings.Join(labels, "|") != tc.labels {
				t.Fatalf("got %v, want %s", labels, tc.labels)
			}
			current := htmlNodes(navs[0], func(n *html.Node) bool { return htmlAttr(n, "aria-current") == "page" })
			if len(current) != 1 || strings.TrimSpace(htmlText(current[0])) != labels[len(labels)-1] {
				t.Fatal("current page must be identified")
			}
			if tc.parent != "" {
				links := htmlNodes(navs[0], isElement("a"))
				if len(links) == 0 || htmlAttr(links[len(links)-1], "href") != tc.parent {
					t.Fatalf("missing parent link %s", tc.parent)
				}
			}
			if strings.Contains(tc.path, list.ShareID) && len(htmlNodes(navs[0], func(n *html.Node) bool { return strings.HasPrefix(htmlAttr(n, "href"), "/children/") })) > 0 {
				t.Fatal("shared page exposes owner navigation")
			}
		})
	}
}

func TestRankedActionsUseMatchingButtons(t *testing.T) {
	db, app := testApp(t)
	child, err := db.CreateChild(t.Context(), "Alex")
	if err != nil {
		t.Fatal(err)
	}
	list, err := db.CreateWishlist(t.Context(), child.ID, "Jul")
	if err != nil {
		t.Fatal(err)
	}
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/lists/"+list.ID+"/ranked", nil))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := html.Parse(strings.NewReader(readBody(t, res)))
	if err != nil {
		t.Fatal(err)
	}
	var add, share *html.Node
	for _, n := range htmlNodes(doc, func(n *html.Node) bool { return isElement("a")(n) || isElement("summary")(n) }) {
		switch strings.TrimSpace(htmlText(n)) {
		case "Lägg till en önskan":
			add = n
		case "Dela önskelistan":
			share = n
		}
	}
	if add == nil || share == nil || !hasClass(add, "button") || htmlAttr(add, "class") != htmlAttr(share, "class") {
		t.Fatal("add and share must have the same button style")
	}
	if add.Parent != share.Parent.Parent || !hasClass(add.Parent, "actions") {
		t.Fatal("add and share must be grouped in the heading actions")
	}
	details := share.Parent
	if !isElement("details")(details) || len(htmlNodes(details, func(n *html.Node) bool {
		return isElement("input")(n) && htmlAttr(n, "id") == "share-url" && htmlAttr(n, "value") != ""
	})) != 1 {
		t.Fatal("share must still reveal the share link")
	}
}

func TestHomeOmitsCurrentPageNavigation(t *testing.T) {
	_, app := testApp(t)
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := html.Parse(strings.NewReader(readBody(t, res)))
	if err != nil {
		t.Fatal(err)
	}
	if len(htmlNodes(doc, func(n *html.Node) bool {
		return hasClass(n, "brand") || (isElement("nav")(n) && htmlAttr(n, "aria-label") == "Brödsmulor")
	})) != 0 {
		t.Fatal("home page must not show links or breadcrumbs for the current page")
	}
}

func TestShareAllButtonJoinsRankedActions(t *testing.T) {
	db, app := testApp(t)
	child, err := db.CreateChild(t.Context(), "Ella")
	if err != nil {
		t.Fatal(err)
	}
	list, err := db.CreateWishlist(t.Context(), child.ID, "Jul")
	if err != nil {
		t.Fatal(err)
	}
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/lists/"+list.ID+"/ranked", nil))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := html.Parse(strings.NewReader(readBody(t, res)))
	if err != nil {
		t.Fatal(err)
	}
	actions := htmlNodes(doc, func(n *html.Node) bool { return hasClass(n, "ranked-actions") })
	if len(actions) != 1 {
		t.Fatal("missing ranked actions")
	}
	buttons := htmlNodes(actions[0], func(n *html.Node) bool {
		return isElement("summary")(n) && strings.TrimSpace(htmlText(n)) == "Dela alla" && hasClass(n, "button")
	})
	if len(buttons) != 1 {
		t.Fatal("Dela alla must be a button next to the other ranked actions")
	}
	inputs := htmlNodes(buttons[0].Parent, func(n *html.Node) bool { return htmlAttr(n, "id") == "family-share-url" })
	if len(inputs) != 1 || !strings.Contains(htmlAttr(inputs[0], "value"), "/family/") {
		t.Fatal("Dela alla must reveal the family link")
	}
}
