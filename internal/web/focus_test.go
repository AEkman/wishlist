package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestFocusedRankingMakesFiveComparisonsAndReturnsToList(t *testing.T) {
	for _, wishCount := range []int{2, 4, 7} {
		t.Run(strconv.Itoa(wishCount)+" wishes", func(t *testing.T) {
			db, app := testApp(t)
			child, _ := db.CreateChild(t.Context(), "Ella")
			list, _ := db.CreateWishlist(t.Context(), child.ID, "Jul")
			focus, _ := db.CreateWish(t.Context(), list.ID, "Cykel", "", "image/png", []byte{1})
			for i := 1; i < wishCount; i++ {
				_, _ = db.CreateWish(t.Context(), list.ID, "Bok "+strconv.Itoa(i), "", "image/png", []byte{1})
			}
			path := "/lists/" + list.ID + "?focus=" + focus.ID
			for remaining := 5; remaining > 0; remaining-- {
				response, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
				if err != nil {
					t.Fatal(err)
				}
				body := readBody(t, response)
				doc, err := html.Parse(strings.NewReader(body))
				if err != nil {
					t.Fatal(err)
				}
				forms := htmlNodes(doc, func(n *html.Node) bool { return isElement("form")(n) && htmlAttr(n, "class") == "pair-option" })
				if len(forms) != 2 {
					t.Fatalf("want comparison, got %s", body)
				}
				values := url.Values{}
				for _, n := range htmlNodes(forms[remaining%2], isElement("input")) {
					values.Add(htmlAttr(n, "name"), htmlAttr(n, "value"))
				}
				if values.Get("focus") != focus.ID || values.Get("remaining") != strconv.Itoa(remaining) {
					t.Fatalf("focused comparison fields = %v", values)
				}
				if values.Get("winner") != focus.ID && values.Get("loser") != focus.ID {
					t.Fatal("comparison omitted selected wish")
				}
				request := httptest.NewRequest(http.MethodPost, "/lists/"+list.ID+"/pick", strings.NewReader(values.Encode()))
				request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				response, err = app.Test(request)
				if err != nil {
					t.Fatal(err)
				}
				response.Body.Close()
				if response.StatusCode != http.StatusSeeOther {
					t.Fatalf("pick status %d", response.StatusCode)
				}
				path = response.Header.Get("Location")
				if remaining > 1 {
					next, err := url.Parse(path)
					if err != nil {
						t.Fatal(err)
					}
					if next.Query().Get("pivot") != values.Get("winner") || (next.Query().Get("higher") == "1") != (values.Get("winner") == focus.ID) {
						t.Fatalf("next comparison lost preferred direction: %s", path)
					}
				}
			}
			if path != "/lists/"+list.ID+"/ranked" {
				t.Fatalf("finished at %s", path)
			}
		})
	}
}

func TestRankedImageOffersFocusedRankingOnlyForPrivateList(t *testing.T) {
	db, app := testApp(t)
	child, _ := db.CreateChild(t.Context(), "Ella")
	list, _ := db.CreateWishlist(t.Context(), child.ID, "Jul")
	wish, _ := db.CreateWish(t.Context(), list.ID, "Cykel", "", "image/png", []byte{1})
	_, _ = db.CreateWish(t.Context(), list.ID, "Bok", "", "image/png", []byte{1})
	for _, path := range []string{"/lists/" + list.ID + "/ranked", "/" + list.ShareID} {
		response, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatal(err)
		}
		body := readBody(t, response)
		want := strings.HasPrefix(path, "/lists/")
		if strings.Contains(body, `data-rank-url="/lists/`+list.ID+`?focus=`+wish.ID+`"`) != want {
			t.Fatalf("focus image action missing or exposed on %s", path)
		}
	}
}

func TestFocusedRankingNeedsAnotherWish(t *testing.T) {
	db, app := testApp(t)
	child, _ := db.CreateChild(t.Context(), "Ella")
	list, _ := db.CreateWishlist(t.Context(), child.ID, "Jul")
	wish, _ := db.CreateWish(t.Context(), list.ID, "Cykel", "", "image/png", []byte{1})
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/lists/"+list.ID+"?focus="+wish.ID, nil))
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, response)
	if response.StatusCode != http.StatusOK || !strings.Contains(body, "Lägg till en önskan till för att börja jämföra.") {
		t.Fatalf("single wish should explain missing opponent; status %d", response.StatusCode)
	}
}
