package web

import (
	"bytes"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRankingDebugLogsShowPairAndRankChanges(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	previousWriter, previousFlags := log.Writer(), log.Flags()
	t.Cleanup(func() {
		slog.SetDefault(previous)
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	})
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})))

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
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("compare screen status = %d, want 200", res.StatusCode)
	}
	for range 2 {
		body := url.Values{"winner": {lego.ID}, "loser": {bike.ID}}.Encode()
		req := httptest.NewRequest(http.MethodPost, path+"/pick", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusSeeOther {
			t.Fatalf("comparison vote status = %d, want 303", res.StatusCode)
		}
	}
	logged := output.String()
	for _, expected := range []string{
		"msg=ranking_pair", "mode=regular",
		"msg=ranking_pick", "winner=LEGO", "loser=Bike",
		"winner_rank_before=2 winner_rank_after=1",
		"winner_rank_before=1 winner_rank_after=1",
		"loser_rank_before=1 loser_rank_after=2",
	} {
		if !strings.Contains(logged, expected) {
			t.Errorf("ranking log missing %q:\n%s", expected, logged)
		}
	}
}
