package web

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUploadFirstWishStoresImageAndOffersNoComparison(t *testing.T) {
	db, app := testApp(t)
	child, err := db.CreateChild(t.Context(), "Alex")
	if err != nil {
		t.Fatal(err)
	}
	list, err := db.CreateWishlist(t.Context(), child.ID, "Jul")
	if err != nil {
		t.Fatal(err)
	}
	path := "/lists/" + list.ID
	request := wishUploadRequest(t, path, func(file io.Writer) error {
		return png.Encode(file, image.NewRGBA(image.Rect(0, 0, 1, 1)))
	})
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("upload returned %d, want 303", response.StatusCode)
	}
	response, err = app.Test(httptest.NewRequest(http.MethodGet, response.Header.Get("Location"), nil))
	if err != nil {
		t.Fatal(err)
	}
	view := readBody(t, response)
	if response.StatusCode != http.StatusOK || !strings.Contains(view, "LEGO") || strings.Contains(view, "Välj din favorit") {
		t.Fatalf("first wish returned %d, want a page showing LEGO without a comparison", response.StatusCode)
	}
	wishes, err := db.Wishes(t.Context(), list.ID)
	if err != nil || len(wishes) != 1 {
		t.Fatalf("stored wishes = %v, %v", wishes, err)
	}
	response, err = app.Test(httptest.NewRequest(http.MethodGet, "/images/"+wishes[0].ID, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("image returned %d with type %q", response.StatusCode, response.Header.Get("Content-Type"))
	}
	if _, err := png.Decode(response.Body); err != nil {
		t.Fatal(err)
	}
}

func TestUploadRejectsCorruptImage(t *testing.T) {
	db, app := testApp(t)
	child, err := db.CreateChild(t.Context(), "Alex")
	if err != nil {
		t.Fatal(err)
	}
	list, err := db.CreateWishlist(t.Context(), child.ID, "Jul")
	if err != nil {
		t.Fatal(err)
	}
	request := wishUploadRequest(t, "/lists/"+list.ID, func(file io.Writer) error {
		_, err := io.WriteString(file, "\x89PNG\r\n\x1a\nnot a PNG image")
		return err
	})
	res, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("corrupt image returned %d, want 400", res.StatusCode)
	}
	wishes, err := db.Wishes(t.Context(), list.ID)
	if err != nil || len(wishes) != 0 {
		t.Fatalf("stored wishes = %v, %v", wishes, err)
	}
}

func wishUploadRequest(t *testing.T, path string, writeImage func(io.Writer) error) *http.Request {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	if err := form.WriteField("title", "LEGO"); err != nil {
		t.Fatal(err)
	}
	file, err := form.CreateFormFile("image", "lego.png")
	if err != nil {
		t.Fatal(err)
	}
	if err := writeImage(file); err != nil {
		t.Fatal(err)
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path+"/wishes/new", &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	return request
}
