package web

import (
	"fmt"
	"math/bits"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"

	"github.com/AEkman/wishlist/internal/store"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestRankingPlacesWishByBinaryInsertion(t *testing.T) {
	for _, mode := range []string{"focus", "upload"} {
		for _, count := range []int{2, 4, 8, 31} {
			for target := range count {
				for _, initial := range []int{1, (count + 1) / 2, count} {
					t.Run(fmt.Sprintf("%s/%d wishes/from %d/to %d", mode, count, initial, target+1), func(t *testing.T) {
						db, app := testApp(t)
						child, _ := db.CreateChild(t.Context(), "Ella")
						list, _ := db.CreateWishlist(t.Context(), child.ID, "Jul")
						var others []store.Wish
						for i := range count - 1 {
							wish, _ := db.CreateWish(t.Context(), list.ID, "Wish "+strconv.Itoa(i), "", "image/png", []byte{1})
							others = append(others, wish)
						}
						focus, _ := db.CreateWish(t.Context(), list.ID, "Focus", "", "image/png", []byte{1})
						if err := db.MoveWish(t.Context(), list.ID, focus.ID, initial); err != nil {
							t.Fatal(err)
						}
						path := "/lists/" + list.ID + "?" + mode + "=" + focus.ID
						low, high := 0, len(others)
						picks := 0
						for low < high {
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
								t.Fatalf("expected binary comparison, got status %d", response.StatusCode)
							}
							middle := (low + high) / 2
							if got := formValues(forms[0]).Get("loser"); got != others[middle].ID {
								t.Fatalf("opponent %s, want midpoint %s", got, others[middle].ID)
							}
							chosen := 0
							if target > middle {
								chosen = 1
								low = middle + 1
							} else {
								high = middle
							}
							values := formValues(forms[chosen])
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
							picks++
						}
						completed := url.Values{"ranked": {focus.ID}}
						newFavorite := target == 0 && initial != 1
						if newFavorite {
							completed.Set("newfavorite", "1")
						}
						destination := "/lists/" + list.ID + "/ranked?" + completed.Encode()
						if path != destination {
							t.Fatalf("finished at %s; want %s", path, destination)
						}
						response, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
						if err != nil {
							t.Fatal(err)
						}
						body := readBody(t, response)
						if !strings.Contains(body, fmt.Sprintf("Klart! Focus fick plats %d av %d.", target+1, count)) {
							t.Fatal("completion must show selected wish and actual final rank")
						}
						if strings.Contains(body, "Ny favorit!") != newFavorite {
							t.Fatalf("new-favorite celebration = %v, want %v", !newFavorite, newFavorite)
						}
						if picks > bits.Len(uint(count-1)) {
							t.Fatalf("used %d comparisons for %d wishes", picks, count)
						}
						expected := slices.Insert(slices.Clone(others), target, focus)
						got, err := db.Wishes(t.Context(), list.ID)
						if err != nil || !slices.Equal(got, expected) {
							t.Fatalf("final order = %v, %v; want %v", got, err, expected)
						}
					})
				}
			}
		}

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

func TestBinaryRankingRejectsStaleOrderWithoutChangingRanks(t *testing.T) {
	for _, change := range []string{"add", "reorder"} {
		t.Run(change, func(t *testing.T) {
			db, app := testApp(t)
			child, _ := db.CreateChild(t.Context(), "Ella")
			list, _ := db.CreateWishlist(t.Context(), child.ID, "Jul")
			var wishes []store.Wish
			for i := range 5 {
				wish, _ := db.CreateWish(t.Context(), list.ID, strconv.Itoa(i), "", "image/png", []byte{1})
				wishes = append(wishes, wish)
			}
			response, err := app.Test(httptest.NewRequest(http.MethodGet, "/lists/"+list.ID+"?focus="+wishes[4].ID, nil))
			if err != nil {
				t.Fatal(err)
			}
			doc, err := html.Parse(strings.NewReader(readBody(t, response)))
			if err != nil {
				t.Fatal(err)
			}
			forms := htmlNodes(doc, func(n *html.Node) bool { return isElement("form")(n) && htmlAttr(n, "class") == "pair-option" })
			values := formValues(forms[0])
			if change == "add" {
				_, err = db.CreateWish(t.Context(), list.ID, "Added", "", "image/png", []byte{1})
			} else {
				err = db.MoveWish(t.Context(), list.ID, wishes[0].ID, 2)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := db.Wishes(t.Context(), list.ID)
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodPost, "/lists/"+list.ID+"/pick", strings.NewReader(values.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			response, err = app.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusBadRequest {
				t.Fatalf("stale ranking returned %d, want 400", response.StatusCode)
			}
			after, err := db.Wishes(t.Context(), list.ID)
			if err != nil || !slices.Equal(before, after) {
				t.Fatalf("stale ranking changed order: %v, %v", after, err)
			}
		})
	}
}

func TestBinaryRankingRejectsInvalidBoundsAndOpponent(t *testing.T) {
	db, app := testApp(t)
	child, _ := db.CreateChild(t.Context(), "Ella")
	list, _ := db.CreateWishlist(t.Context(), child.ID, "Jul")
	var wishes []store.Wish
	for i := range 4 {
		wish, _ := db.CreateWish(t.Context(), list.ID, strconv.Itoa(i), "", "image/png", []byte{1})
		wishes = append(wishes, wish)
	}
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/lists/"+list.ID+"?focus="+wishes[3].ID, nil))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := html.Parse(strings.NewReader(readBody(t, response)))
	if err != nil {
		t.Fatal(err)
	}
	forms := htmlNodes(doc, func(n *html.Node) bool { return isElement("form")(n) && htmlAttr(n, "class") == "pair-option" })
	original := formValues(forms[0])
	for _, tc := range []struct{ field, value string }{{"low", "-1"}, {"high", "99"}, {"high", "0"}, {"low", "invalid"}, {"order", "changed"}, {"loser", wishes[0].ID}, {"upload", wishes[3].ID}} {
		t.Run(tc.field+"="+tc.value, func(t *testing.T) {
			values := original.Clone()
			values.Set(tc.field, tc.value)
			request := httptest.NewRequest(http.MethodPost, "/lists/"+list.ID+"/pick", strings.NewReader(values.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			response, err := app.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusBadRequest {
				t.Fatalf("invalid comparison returned %d", response.StatusCode)
			}
			after, err := db.Wishes(t.Context(), list.ID)
			if err != nil || !slices.Equal(wishes, after) {
				t.Fatalf("invalid request changed order: %v, %v", after, err)
			}
		})
	}
}
