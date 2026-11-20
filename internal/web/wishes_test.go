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

	"golang.org/x/net/html"
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
	request := wishUploadRequest(t, path+"/wishes/new", func(file io.Writer) error {
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
	request := wishUploadRequest(t, "/lists/"+list.ID+"/wishes/new", func(file io.Writer) error {
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

func TestEditImageChangesItsURL(t *testing.T) {
	db, app := testApp(t)
	child, err := db.CreateChild(t.Context(), "Alex")
	if err != nil {
		t.Fatal(err)
	}
	list, err := db.CreateWishlist(t.Context(), child.ID, "Jul")
	if err != nil {
		t.Fatal(err)
	}
	var original bytes.Buffer
	if err := png.Encode(&original, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	wish, err := db.CreateWish(t.Context(), list.ID, "LEGO", "", "image/png", original.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	path := "/lists/" + list.ID
	imageURL := func() string {
		t.Helper()
		res, err := app.Test(httptest.NewRequest(http.MethodGet, path+"/ranked", nil))
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		doc, err := html.Parse(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		images := htmlNodes(doc, isElement("img"))
		if len(images) != 1 {
			t.Fatalf("ranked page has %d images, want one", len(images))
		}
		return htmlAttr(images[0], "src")
	}
	before := imageURL()
	request := wishUploadRequest(t, path+"/wishes/"+wish.ID+"/edit", func(file io.Writer) error {
		return png.Encode(file, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	})
	res, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("image edit returned %d, want 303", res.StatusCode)
	}
	if after := imageURL(); before == after {
		t.Fatalf("image URL stayed at %q after editing", after)
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
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	return request
}
