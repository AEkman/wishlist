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

func TestRegularChoiceStartsGuidedRanking(t *testing.T) {
	db, app := testApp(t)
	child, _ := db.CreateChild(t.Context(), "Alex")
	list, _ := db.CreateWishlist(t.Context(), child.ID, "Jul")
	a, _ := db.CreateWish(t.Context(), list.ID, "A", "", "image/png", []byte{1})
	b, _ := db.CreateWish(t.Context(), list.ID, "B", "", "image/png", []byte{1})
	_, _ = db.CreateWish(t.Context(), list.ID, "C", "", "image/png", []byte{1})
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/lists/"+list.ID, nil))
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, response)
	if !strings.Contains(body, "1 av 3 önskningar placerade") || !strings.Contains(body, "Placera B") {
		t.Fatalf("missing guided progress: %s", body)
	}
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	forms := htmlNodes(doc, func(n *html.Node) bool { return isElement("form")(n) && hasClass(n, "pair-option") })
	if len(forms) != 2 {
		t.Fatalf("comparison count=%d", len(forms))
	}
	first, second := formValues(forms[0]), formValues(forms[1])
	if !first.Has("guided_revision") || first.Get("winner") != a.ID || second.Get("winner") != b.ID {
		t.Fatalf("guided choices=%v,%v", first, second)
	}
	if strings.Contains(body, ">Ångra senaste valet</button>") {
		t.Fatal("undo shown without any choice")
	}
}

func TestRegularRankingCompletesWithFullListAndNewFavorite(t *testing.T) {
	db, app := testApp(t)
	child, _ := db.CreateChild(t.Context(), "Alex")
	list, _ := db.CreateWishlist(t.Context(), child.ID, "Jul")
	_, _ = db.CreateWish(t.Context(), list.ID, "A", "", "image/png", []byte{1})
	b, _ := db.CreateWish(t.Context(), list.ID, "B", "", "image/png", []byte{1})
	path := "/lists/" + list.ID
	response, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, response)
	doc, _ := html.Parse(strings.NewReader(body))
	forms := htmlNodes(doc, func(n *html.Node) bool { return isElement("form")(n) && hasClass(n, "pair-option") })
	var values url.Values
	for _, form := range forms {
		if formValues(form).Get("winner") == b.ID {
			values = formValues(form)
		}
	}
	request := httptest.NewRequest(http.MethodPost, path+"/pick", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err = app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	response, err = app.Test(httptest.NewRequest(http.MethodGet, response.Header.Get("Location"), nil))
	if err != nil {
		t.Fatal(err)
	}
	body = readBody(t, response)
	if !strings.Contains(body, "Rangordningen är klar") || !strings.Contains(body, "Ny favorit!") || strings.Count(body, `class="ranked-card"`) != 2 {
		t.Fatalf("missing completed ranked list: %s", body)
	}
}

func TestUndoFromCompletedRankingReturnsToPreviousChoice(t *testing.T) {
	db, app := testApp(t)
	child, _ := db.CreateChild(t.Context(), "Alex")
	list, _ := db.CreateWishlist(t.Context(), child.ID, "Jul")
	a, _ := db.CreateWish(t.Context(), list.ID, "A", "", "image/png", []byte{1})
	b, _ := db.CreateWish(t.Context(), list.ID, "B", "", "image/png", []byte{1})
	progress, _ := db.GuidedRanking(t.Context(), list.ID)
	_, err := db.PickGuided(t.Context(), list.ID, progress.Revision, b.ID, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	path := "/lists/" + list.ID
	response, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := html.Parse(strings.NewReader(readBody(t, response)))
	forms := htmlNodes(doc, func(n *html.Node) bool {
		return isElement("form")(n) && strings.HasSuffix(htmlAttr(n, "action"), "/ranking/undo")
	})
	if len(forms) != 1 {
		t.Fatalf("want completion undo control, got %d", len(forms))
	}
	request := httptest.NewRequest(http.MethodPost, htmlAttr(forms[0], "action"), strings.NewReader(formValues(forms[0]).Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err = app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("undo status=%d", response.StatusCode)
	}
	response, err = app.Test(httptest.NewRequest(http.MethodGet, response.Header.Get("Location"), nil))
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, response)
	if !strings.Contains(body, "1 av 2 önskningar placerade") || strings.Contains(body, "Rangordningen är klar") {
		t.Fatalf("undo did not return previous comparison: %s", body)
	}
	wishes, _ := db.Wishes(t.Context(), list.ID)
	if wishes[0].ID != a.ID {
		t.Fatal("undo did not restore first wish")
	}
}

func TestRankedViewOffersResumeAndRestartOfGuidedProgress(t *testing.T) {
	db, app := testApp(t)
	child, _ := db.CreateChild(t.Context(), "Alex")
	list, _ := db.CreateWishlist(t.Context(), child.ID, "Jul")
	a, _ := db.CreateWish(t.Context(), list.ID, "A", "", "image/png", []byte{1})
	b, _ := db.CreateWish(t.Context(), list.ID, "B", "", "image/png", []byte{1})
	_, _ = db.CreateWish(t.Context(), list.ID, "C", "", "image/png", []byte{1})
	progress, _ := db.GuidedRanking(t.Context(), list.ID)
	_, err := db.PickGuided(t.Context(), list.ID, progress.Revision, b.ID, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	path := "/lists/" + list.ID
	response, err := app.Test(httptest.NewRequest(http.MethodGet, path+"/ranked", nil))
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, response)
	if !strings.Contains(body, ">Fortsätt rangordna</a>") {
		t.Fatalf("missing resume link: %s", body)
	}
	response, err = app.Test(httptest.NewRequest(http.MethodGet, path, nil))
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := html.Parse(strings.NewReader(readBody(t, response)))
	forms := htmlNodes(doc, func(n *html.Node) bool {
		return isElement("form")(n) && strings.HasSuffix(htmlAttr(n, "action"), "/ranking/restart")
	})
	if len(forms) != 1 {
		t.Fatalf("restart forms=%d", len(forms))
	}
	request := httptest.NewRequest(http.MethodPost, htmlAttr(forms[0], "action"), strings.NewReader(formValues(forms[0]).Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err = app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	restarted, err := db.GuidedRanking(t.Context(), list.ID)
	if err != nil || restarted.Placed != 1 || restarted.CanUndo {
		t.Fatalf("restart progress=%+v,%v", restarted, err)
	}
}

func TestStaleGuidedChoiceShowsFreshComparisonWithoutSaving(t *testing.T) {
	db, app := testApp(t)
	child, _ := db.CreateChild(t.Context(), "Alex")
	list, _ := db.CreateWishlist(t.Context(), child.ID, "Jul")
	a, _ := db.CreateWish(t.Context(), list.ID, "A", "", "image/png", []byte{1})
	b, _ := db.CreateWish(t.Context(), list.ID, "B", "", "image/png", []byte{1})
	_, _ = db.CreateWish(t.Context(), list.ID, "C", "", "image/png", []byte{1})
	initial, _ := db.GuidedRanking(t.Context(), list.ID)
	changed, err := db.PickGuided(t.Context(), list.ID, initial.Revision, b.ID, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	values := url.Values{"guided_revision": {strconv.Itoa(initial.Revision)}, "winner": {b.ID}, "loser": {a.ID}}
	request := httptest.NewRequest(http.MethodPost, "/lists/"+list.ID+"/pick", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, response)
	if response.StatusCode != http.StatusConflict || !strings.Contains(body, "Vyn var inte aktuell") || !strings.Contains(body, "2 av 3 önskningar placerade") {
		t.Fatalf("stale response status=%d,body=%s", response.StatusCode, body)
	}
	progress, _ := db.GuidedRanking(t.Context(), list.ID)
	if progress.Revision != changed.Revision {
		t.Fatal("stale request saved another choice")
	}
}

func TestGuidedPickReportsRankingDebugForActualChoice(t *testing.T) {
	t.Setenv("RANKING_DEBUG", "1")
	db, app := testApp(t)
	child, _ := db.CreateChild(t.Context(), "Alex")
	list, _ := db.CreateWishlist(t.Context(), child.ID, "Jul")
	a, _ := db.CreateWish(t.Context(), list.ID, "A", "", "image/png", []byte{1})
	b, _ := db.CreateWish(t.Context(), list.ID, "B", "", "image/png", []byte{1})
	progress, _ := db.GuidedRanking(t.Context(), list.ID)
	values := url.Values{"guided_revision": {strconv.Itoa(progress.Revision)}, "winner": {b.ID}, "loser": {a.ID}}
	request := httptest.NewRequest(http.MethodPost, "/lists/"+list.ID+"/pick", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("HX-Request", "true")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.Contains(response.Header.Get("HX-Trigger"), `"winnerAfter":1`) || !strings.Contains(response.Header.Get("HX-Trigger"), `"winnerBefore":2`) || response.Header.Get("HX-Location") == "" {
		t.Fatalf("debug status=%d,headers=%v", response.StatusCode, response.Header)
	}
}

func TestRankedViewShowsWhenChangedListResetsGuidedProgress(t *testing.T) {
	db, app := testApp(t)
	child, _ := db.CreateChild(t.Context(), "Alex")
	list, _ := db.CreateWishlist(t.Context(), child.ID, "Jul")
	a, _ := db.CreateWish(t.Context(), list.ID, "A", "", "image/png", []byte{1})
	b, _ := db.CreateWish(t.Context(), list.ID, "B", "", "image/png", []byte{1})
	_, _ = db.CreateWish(t.Context(), list.ID, "C", "", "image/png", []byte{1})
	initial, _ := db.GuidedRanking(t.Context(), list.ID)
	if _, err := db.PickGuided(t.Context(), list.ID, initial.Revision, b.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	_, _ = db.CreateWish(t.Context(), list.ID, "D", "", "image/png", []byte{1})
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/lists/"+list.ID+"/ranked", nil))
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, response)
	if !strings.Contains(body, "Listan har ändrats. Rangordningen börjar om med den aktuella ordningen.") {
		t.Fatal("ranked view silently consumed guided reset notice")
	}
}
