package web

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestRequestLogsStatusWithoutSharedToken(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	db, app := testApp(t)
	child, _ := db.CreateChild(t.Context(), "Ella")
	list, _ := db.CreateWishlist(t.Context(), child.ID, "Jul")
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/"+list.ShareID+"?private=query", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	entry := output.String()
	for _, want := range []string{`msg="HTTP request"`, "method=GET", "route=/:share", "status=200", "duration="} {
		if !strings.Contains(entry, want) {
			t.Errorf("missing %q in request log: %s", want, entry)
		}
	}
	if strings.Contains(entry, list.ShareID) || strings.Contains(entry, "private=query") {
		t.Errorf("request log exposes shared URL: %s", entry)
	}
}

func TestRequestLogsServerErrorsAndFinalStatus(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	_, app := testApp(t)
	app.Get("/diagnostics/fail", func(c fiber.Ctx) error {
		return errors.New("database operation failed")
	})
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/diagnostics/fail", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body := readBody(t, response)
	if response.StatusCode != http.StatusInternalServerError || strings.Contains(body, "database operation failed") {
		t.Fatal("server errors must return a generic 500 page")
	}
	entry := output.String()
	for _, want := range []string{"level=ERROR", "status=500", `error="database operation failed"`} {
		if !strings.Contains(entry, want) {
			t.Errorf("missing %q in error log: %s", want, entry)
		}
	}
}
