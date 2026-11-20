package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestOwnerPagesRequirePIN(t *testing.T) {
	t.Setenv("FAMILY_PIN", "123456")
	db, _ := testApp(t)
	app, err := New(db, "")
	if err != nil {
		t.Fatal(err)
	}
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/unlock?next=%2F" {
		t.Fatalf("unauthenticated home returned %d, location %q", res.StatusCode, res.Header.Get("Location"))
	}
	_ = res.Body.Close()
}

func TestPINUnlockPersistsAndSharedPagesStayPublic(t *testing.T) {
	t.Setenv("FAMILY_PIN", "123456")
	db, _ := testApp(t)
	child, err := db.CreateChild(t.Context(), "Alex")
	if err != nil {
		t.Fatal(err)
	}
	list, err := db.CreateWishlist(t.Context(), child.ID, "Jul")
	if err != nil {
		t.Fatal(err)
	}
	wish, err := db.CreateWish(t.Context(), list.ID, "Bil", "", "image/png", []byte{1})
	if err != nil {
		t.Fatal(err)
	}
	app, err := New(db, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "/children/" + child.ID, "/lists/" + list.ID, "/lists/" + list.ID + "/ranked", "/lists/" + list.ID + "/wishes/new", "/lists/" + list.ID + "/wishes/" + wish.ID + "/edit", "/images/" + wish.ID} {
		res, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusSeeOther {
			t.Fatalf("%s returned %d without PIN", path, res.StatusCode)
		}
	}
	for _, path := range []string{"/" + list.ShareID, "/" + list.ShareID + "/images/" + wish.ID, "/" + list.ShareID + "/wishes/" + wish.ID + "/buy"} {
		res, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("public %s returned %d", path, res.StatusCode)
		}
	}
	submit := func(pin string) *http.Response {
		req := httptest.NewRequest(http.MethodPost, "/unlock", strings.NewReader(url.Values{"pin": {pin}, "next": {"/children/" + child.ID}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	wrong := submit("999999")
	_ = wrong.Body.Close()
	if wrong.StatusCode != http.StatusUnauthorized || len(wrong.Cookies()) != 0 {
		t.Fatal("incorrect PIN unlocked app")
	}
	success := submit("123456")
	_ = success.Body.Close()
	if success.StatusCode != http.StatusSeeOther || success.Header.Get("Location") != "/children/"+child.ID {
		t.Fatalf("unlock returned %d %s", success.StatusCode, success.Header.Get("Location"))
	}
	cookies := success.Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].MaxAge < 300*24*60*60 {
		t.Fatal("expected long-lived HTTP-only device cookie")
	}
	restarted, err := New(db, "")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookies[0])
	res, err := restarted.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("remembered device returned %d", res.StatusCode)
	}
}

func TestPINProtectsWritesAndSharedImageScope(t *testing.T) {
	t.Setenv("FAMILY_PIN", "123456")
	db, _ := testApp(t)
	child, err := db.CreateChild(t.Context(), "Alex")
	if err != nil {
		t.Fatal(err)
	}
	first, err := db.CreateWishlist(t.Context(), child.ID, "Jul")
	if err != nil {
		t.Fatal(err)
	}
	second, err := db.CreateWishlist(t.Context(), child.ID, "Birthday")
	if err != nil {
		t.Fatal(err)
	}
	wish, err := db.CreateWish(t.Context(), second.ID, "Bil", "", "image/png", []byte{1})
	if err != nil {
		t.Fatal(err)
	}
	app, err := New(db, "")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/children", strings.NewReader("name=Stranger"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized write returned %d", res.StatusCode)
	}
	res, err = app.Test(httptest.NewRequest(http.MethodGet, "/"+first.ShareID+"/images/"+wish.ID, nil))
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-list shared image returned %d", res.StatusCode)
	}
}

func TestMissingPINStopsStartup(t *testing.T) {
	t.Setenv("FAMILY_PIN", "")
	db, _ := testApp(t)
	if _, err := New(db, ""); err == nil {
		t.Fatal("startup without PIN must fail")
	}
}

func TestExpiredHTMXWriteRedirectsToUnlock(t *testing.T) {
	t.Setenv("FAMILY_PIN", "123456")
	db, _ := testApp(t)
	app, err := New(db, "")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/lists/list-id/pick", strings.NewReader("winner=a&loser=b"))
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Current-URL", "http://example.com/lists/list-id")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if got := res.Header.Get("HX-Redirect"); got != "/unlock?next=%2Flists%2Flist-id" {
		t.Fatalf("HTMX redirect = %q", got)
	}
}
