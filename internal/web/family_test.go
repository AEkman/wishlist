package web

import (
	"errors"
	"github.com/AEkman/wishlist/internal/store"
	"github.com/gofiber/fiber/v3"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFamilyRoutesArePublicButNotAdminRoutes(t *testing.T) {
	token := "94f62f17-aa73-4815-bdaf-80f56eb225e8"
	for _, path := range []string{"/family/" + token, "/family/" + token + "/children/alex", "/family/" + token + "/lists/list", "/family/" + token + "/lists/list/images/wish", "/family/" + token + "/lists/list/wishes/wish/buy", "/family/" + token + "/lists/list/wishes/wish/clear"} {
		if !isSharedRoute(path) {
			t.Errorf("public family path rejected: %s", path)
		}
	}
	for _, path := range []string{"/family/no-token", "/family/" + token + "/lists/list/sharing", "/family/" + token + "/lists/list/wishes/new", "/lists/list/sharing"} {
		if isSharedRoute(path) {
			t.Errorf("admin or malformed path accepted: %s", path)
		}
	}
	_, app := testApp(t)
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/family/"+token, nil))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("unknown family token returned %d", res.StatusCode)
	}
	res.Body.Close()
}

func TestSharedListIdentifiesChildWithoutSiblingNavigation(t *testing.T) {
	db, app := testApp(t)
	child, _ := db.CreateChild(t.Context(), "Ella")
	list, _ := db.CreateWishlist(t.Context(), child.ID, "Jul 2026")
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/"+list.ShareID, nil))
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, res)
	if !strings.Contains(body, "Ella") || !strings.Contains(body, "Jul 2026") {
		t.Fatal("shared list must identify both child and occasion")
	}
	if strings.Contains(body, "/family/") {
		t.Fatal("standalone list must not expose family navigation")
	}
}

func TestListsExplainRanking(t *testing.T) {
	db, app := testApp(t)
	child, _ := db.CreateChild(t.Context(), "Ella")
	list, _ := db.CreateWishlist(t.Context(), child.ID, "Jul")
	for _, path := range []string{"/lists/" + list.ID + "/ranked", "/" + list.ShareID} {
		res, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatal(err)
		}
		body := readBody(t, res)
		if !strings.Contains(body, "Önskningarna visas i barnets favoritordning.") || strings.Contains(body, "står överst") {
			t.Errorf("%s missing ranking explanation", path)
		}
	}
}

func TestFamilyShowsOnlyPublishedChildrenAndLists(t *testing.T) {
	db, app := testApp(t)
	ella, _ := db.CreateChild(t.Context(), "Ella")
	leo, _ := db.CreateChild(t.Context(), "Leo")
	list, _ := db.CreateWishlist(t.Context(), ella.ID, "Jul 2026")
	db.CreateWishlist(t.Context(), leo.ID, "Hemlig lista")
	if err := db.SetFamilyVisible(t.Context(), list.ID, true); err != nil {
		t.Fatal(err)
	}
	token, err := db.FamilyShareID(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	base := "/family/" + token
	res, err := app.Test(httptest.NewRequest(http.MethodGet, base, nil))
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, res)
	if res.StatusCode != http.StatusOK || !strings.Contains(body, "Ella") || strings.Contains(body, "Leo") || strings.Contains(body, "Hemlig lista") {
		t.Fatalf("family status %d body %s", res.StatusCode, body)
	}
	res, err = app.Test(httptest.NewRequest(http.MethodGet, base+"/children/"+ella.ID, nil))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != base+"/lists/"+list.ID {
		t.Fatal("single visible list must open directly")
	}
	res, err = app.Test(httptest.NewRequest(http.MethodGet, base+"/lists/"+list.ID, nil))
	if err != nil {
		t.Fatal(err)
	}
	body = readBody(t, res)
	if res.StatusCode != http.StatusOK || !strings.Contains(body, "favoritordning") || !strings.Contains(body, "Ella") {
		t.Fatalf("shared family list status %d body %s", res.StatusCode, body)
	}
}

func TestFamilyPurchasesImagesAndVisibilityBoundaries(t *testing.T) {
	db, app := testApp(t)
	child, _ := db.CreateChild(t.Context(), "Ella")
	list, _ := db.CreateWishlist(t.Context(), child.ID, "Jul")
	wish, _ := db.CreateWish(t.Context(), list.ID, "Cykel", "", "image/png", []byte{1, 2, 3})
	other, _ := db.CreateWishlist(t.Context(), child.ID, "Privat")
	otherWish, _ := db.CreateWish(t.Context(), other.ID, "Hemlig", "", "image/png", []byte{4})
	db.SetFamilyVisible(t.Context(), list.ID, true)
	token, _ := db.FamilyShareID(t.Context())
	base := "/family/" + token + "/lists/" + list.ID
	request := func(method, path, form string) *http.Response {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(form))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	res := request(http.MethodGet, base+"/images/"+wish.ID, "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("family image returned %d", res.StatusCode)
	}
	res.Body.Close()
	for _, action := range []string{"buy", "clear"} {
		path := base + "/wishes/" + wish.ID + "/" + action
		res = request(http.MethodGet, path, "")
		body := readBody(t, res)
		if res.StatusCode != http.StatusOK || !strings.Contains(body, `action="`+path+`"`) || !strings.Contains(body, `href="`+base+`"`) {
			t.Fatalf("purchase loses family context: %d %s", res.StatusCode, body)
		}
		res = request(http.MethodPost, path, "name=Anna")
		res.Body.Close()
		if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != base {
			t.Fatalf("purchase redirect %d %s", res.StatusCode, res.Header.Get("Location"))
		}
		saved, _ := db.Wish(t.Context(), list.ID, wish.ID)
		if (saved.Buyer == "Anna") != (action == "buy") {
			t.Fatalf("unexpected purchase state: %q", saved.Buyer)
		}
	}
	for _, path := range []string{base + "/images/" + otherWish.ID, base + "/wishes/" + otherWish.ID + "/buy", "/family/" + token + "/lists/" + other.ID} {
		res = request(http.MethodGet, path, "")
		res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("cross-list/hidden access %s: %d", path, res.StatusCode)
		}
	}
	db.SetFamilyVisible(t.Context(), list.ID, false)
	for _, suffix := range []string{"", "/images/" + wish.ID, "/wishes/" + wish.ID + "/buy", "/wishes/" + wish.ID + "/clear"} {
		res = request(http.MethodGet, base+suffix, "")
		res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("revoked access %s: %d", suffix, res.StatusCode)
		}
	}
	res = request(http.MethodPost, base+"/wishes/"+wish.ID+"/buy", "name=Anna")
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatal("revoked family purchase allowed")
	}
	res = request(http.MethodGet, "/"+list.ShareID, "")
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatal("standalone sharing changed by family visibility")
	}
}

func TestFamilyListNavigationShowsSelectedChildLists(t *testing.T) {
	db, app := testApp(t)
	ella, _ := db.CreateChild(t.Context(), "Ella")
	leo, _ := db.CreateChild(t.Context(), "Leo")
	jul, _ := db.CreateWishlist(t.Context(), ella.ID, "Ellas jul")
	birthday, _ := db.CreateWishlist(t.Context(), ella.ID, "Ellas födelsedag")
	sibling, _ := db.CreateWishlist(t.Context(), leo.ID, "Leos jul")
	for _, list := range []string{jul.ID, birthday.ID, sibling.ID} {
		db.SetFamilyVisible(t.Context(), list, true)
	}
	token, _ := db.FamilyShareID(t.Context())
	base := "/family/" + token
	res, err := app.Test(httptest.NewRequest(http.MethodGet, base+"/lists/"+jul.ID, nil))
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, res)
	if strings.Contains(body, "Leos jul") || !strings.Contains(body, "Ellas födelsedag") || !strings.Contains(body, base+"/children/"+leo.ID) {
		t.Fatal("list selector must show own lists and child selector must retain siblings")
	}
	res, err = app.Test(httptest.NewRequest(http.MethodGet, base+"/children/"+ella.ID, nil))
	if err != nil {
		t.Fatal(err)
	}
	body = readBody(t, res)
	if res.StatusCode != http.StatusOK || !strings.Contains(body, "Ellas födelsedag") || strings.Contains(body, "Leos jul") {
		t.Fatal("multiple lists must show selection for chosen child")
	}
}

func TestFamilySharingRequiresPINForVisibilityChanges(t *testing.T) {
	t.Setenv("FAMILY_PIN", "123456")
	db, _ := testApp(t)
	child, _ := db.CreateChild(t.Context(), "Ella")
	list, _ := db.CreateWishlist(t.Context(), child.ID, "Jul")
	token, _ := db.FamilyShareID(t.Context())
	app, err := New(db, "https://wishlist.example.org")
	if err != nil {
		t.Fatal(err)
	}
	post := func(path, form string, cookie *http.Cookie) *http.Response {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	path := "/lists/" + list.ID + "/sharing"
	res := post(path, "visible=true", nil)
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatal("visibility change bypassed PIN")
	}
	res, err = app.Test(httptest.NewRequest(http.MethodGet, "/family/"+token, nil))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatal("family overview requires PIN")
	}
	res = post("/unlock", "pin=123456", nil)
	res.Body.Close()
	cookie := res.Cookies()[0]
	for _, form := range []string{"visible=true", ""} {
		res = post(path, form, cookie)
		res.Body.Close()
		if res.StatusCode != http.StatusSeeOther {
			t.Fatalf("visibility save returned %d", res.StatusCode)
		}
		saved, err := db.Wishlist(t.Context(), list.ID)
		if err != nil {
			t.Fatal(err)
		}
		if saved.FamilyVisible != (form != "") {
			t.Fatal("visibility setting not persisted")
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/lists/"+list.ID+"/ranked", nil)
	req.AddCookie(cookie)
	res, err = app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readBody(t, res), "https://wishlist.example.org/family/"+token) {
		t.Fatal("admin missing stable absolute family link")
	}
}

func TestStandaloneBrandStaysOnSharedList(t *testing.T) {
	db, app := testApp(t)
	child, _ := db.CreateChild(t.Context(), "Ella")
	list, _ := db.CreateWishlist(t.Context(), child.ID, "Jul")
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/"+list.ShareID, nil))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readBody(t, res), `class="brand" href="/`+list.ShareID+`"`) {
		t.Fatal("standalone brand should return to same shared list")
	}
}

func TestFamilyShareLinkUsesRequestOriginWithoutBaseURL(t *testing.T) {
	_, app := testApp(t)
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "http://localhost:8080/", nil))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readBody(t, res), `value="http://localhost:8080/family/`) {
		t.Fatal("copyable family link must contain request origin")
	}
}

func TestFamilyNavigationRejectsListHiddenAfterLookup(t *testing.T) {
	db, _ := testApp(t)
	child, err := db.CreateChild(t.Context(), "Ella")
	if err != nil {
		t.Fatal(err)
	}
	list, err := db.CreateWishlist(t.Context(), child.ID, "Jul")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SetFamilyVisible(t.Context(), list.ID, true); err != nil {
		t.Fatal(err)
	}
	list, err = db.Wishlist(t.Context(), list.ID)
	if err != nil {
		t.Fatal(err)
	}
	token, err := db.FamilyShareID(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SetFamilyVisible(t.Context(), list.ID, false); err != nil {
		t.Fatal(err)
	}
	s := server{db: db}
	app := fiber.New()
	app.Get("/family/:family/lists/:list", func(c fiber.Ctx) error {
		data := page{View: "share", Wishlist: list}
		err := s.familyNavigation(c, &data)
		if !errors.Is(err, store.ErrNotFound) {
			t.Errorf("hidden list navigation returned %v; must reject stale lookup", err)
		}
		if err != nil {
			return c.SendStatus(http.StatusNotFound)
		}
		return c.SendStatus(http.StatusOK)
	})
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/family/"+token+"/lists/"+list.ID, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("stale visible lookup returned %d", res.StatusCode)
	}
}

func TestVisibilityCheckboxSavesImmediately(t *testing.T) {
	db, app := testApp(t)
	child, err := db.CreateChild(t.Context(), "Ella")
	if err != nil {
		t.Fatal(err)
	}
	list, err := db.CreateWishlist(t.Context(), child.ID, "Jul")
	if err != nil {
		t.Fatal(err)
	}
	path := "/lists/" + list.ID + "/sharing"
	for _, view := range []string{"/children/" + child.ID, "/lists/" + list.ID + "/ranked"} {
		res, err := app.Test(httptest.NewRequest(http.MethodGet, view, nil))
		if err != nil {
			t.Fatal(err)
		}
		body := readBody(t, res)
		if !strings.Contains(body, `hx-post="`+path+`"`) || !strings.Contains(body, `hx-trigger="change"`) || !strings.Contains(body, `hx-push-url="false"`) {
			t.Fatalf("%s must save checkbox changes without navigation", view)
		}
		if strings.Contains(body, "Spara synlighet") {
			t.Fatal("visibility must not require a save button")
		}
	}
	for _, form := range []string{"visible=true", ""} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusNoContent || res.Header.Get("Location") != "" {
			t.Fatalf("checkbox change returned %d with redirect %q", res.StatusCode, res.Header.Get("Location"))
		}
		saved, err := db.Wishlist(t.Context(), list.ID)
		if err != nil {
			t.Fatal(err)
		}
		if saved.FamilyVisible != (form != "") {
			t.Fatal("checkbox change was not saved")
		}
	}
}
