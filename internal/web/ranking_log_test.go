package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRankingDebugReportsEachPickToHTMX(t *testing.T) {
	t.Setenv("RANKING_DEBUG", "1")
	db, app := testApp(t)
	child, err := db.CreateChild(t.Context(), "Alex")
	if err != nil {
		t.Fatal(err)
	}
	list, err := db.CreateWishlist(t.Context(), child.ID, "Jul")
	if err != nil {
		t.Fatal(err)
	}
	bike, err := db.CreateWish(t.Context(), list.ID, "Bike", "", "image/png", []byte{1})
	if err != nil {
		t.Fatal(err)
	}
	lego, err := db.CreateWish(t.Context(), list.ID, "LEGO", "", "image/png", []byte{1})
	if err != nil {
		t.Fatal(err)
	}
	path := "/lists/" + list.ID
	res, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
	if err != nil {
		t.Fatal(err)
	}
	if body := readBody(t, res); !strings.Contains(body, "/static/ranking-debug.js") || res.Header.Get("HX-Trigger") != "" {
		t.Error("comparison page should load the debug script without logging a pick")
	}
	type ranks struct {
		WinnerBefore int `json:"winnerBefore"`
		WinnerAfter  int `json:"winnerAfter"`
		LoserBefore  int `json:"loserBefore"`
		LoserAfter   int `json:"loserAfter"`
	}
	for _, want := range []ranks{{2, 1, 1, 2}, {1, 1, 2, 2}} {
		body := url.Values{"winner": {lego.ID}, "loser": {bike.ID}}.Encode()
		req := httptest.NewRequest(http.MethodPost, path+"/pick", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("HTMX pick returned %d, want 200", res.StatusCode)
		}
		var destination struct{ Path, Target, Select string }
		if err := json.Unmarshal([]byte(res.Header.Get("HX-Location")), &destination); err != nil {
			t.Fatal(err)
		}
		if destination.Path != path || destination.Target != "#screen" || destination.Select != "#screen" {
			t.Fatalf("HTMX destination = %+v", destination)
		}
		var event map[string]json.RawMessage
		if err := json.Unmarshal([]byte(res.Header.Get("HX-Trigger")), &event); err != nil {
			t.Fatal(err)
		}
		var got ranks
		if len(event) != 1 || json.Unmarshal(event["wishlist:ranking-pick"], &got) != nil || got != want {
			t.Errorf("debug event = %s, want ranks %+v", res.Header.Get("HX-Trigger"), want)
		}
	}
}
