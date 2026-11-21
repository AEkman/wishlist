package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestMoveWishFromRankedViewPersistsOrder(t *testing.T) {
	db, app := testApp(t)
	child, _ := db.CreateChild(t.Context(), "Alex")
	list, _ := db.CreateWishlist(t.Context(), child.ID, "Jul")
	first, _ := db.CreateWish(t.Context(), list.ID, "First", "", "image/png", []byte{1})
	second, _ := db.CreateWish(t.Context(), list.ID, "Second", "", "image/png", []byte{1})
	path := "/lists/" + list.ID + "/ranked"
	request := httptest.NewRequest(http.MethodPost, path+"/move", strings.NewReader(url.Values{"wish": {second.ID}, "rank": {"1"}}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("move returned %d, want 204", response.StatusCode)
	}
	wishes, err := db.Wishes(t.Context(), list.ID)
	if err != nil || len(wishes) != 2 || wishes[0].ID != second.ID || wishes[1].ID != first.ID {
		t.Fatalf("order after move = %v, %v", wishes, err)
	}
	response, err = app.Test(httptest.NewRequest(http.MethodGet, path, nil))
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, response)
	if !strings.Contains(body, `data-reorder-url="`+path+`/move"`) || !strings.Contains(body, `data-wish-id="`+second.ID+`"`) || !strings.Contains(body, `data-drag-handle`) {
		t.Fatal("ranked view needs a reorder URL, wish IDs and drag handles")
	}
}
