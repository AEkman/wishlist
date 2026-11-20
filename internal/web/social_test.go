package web

import (
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestAnonymousPagesHaveSocialPreviewMetadata(t *testing.T) {
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
	wish, err := db.CreateWish(t.Context(), list.ID, "Hemlig önskan", "", "image/png", []byte{1})
	if err != nil {
		t.Fatal(err)
	}
	token, err := db.FamilyShareID(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	const origin = "https://wishlist.andreasekman.com"
	app, err := newApp(db, origin, func(int) int { return 0 }, false, "123456")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ path, canonical string }{
		{"/unlock?next=/lists/private", "/"},
		{"/" + list.ShareID, "/" + list.ShareID},
		{"/family/" + token, "/family/" + token},
		{"/family/" + token + "/lists/" + list.ID, "/family/" + token + "/lists/" + list.ID},
		{"/family/" + token + "/lists/" + list.ID + "/wishes/" + wish.ID + "/buy", "/family/" + token + "/lists/" + list.ID},
	} {
		t.Run(tc.path, func(t *testing.T) {
			res, err := app.Test(httptest.NewRequest(http.MethodGet, tc.path, nil))
			if err != nil {
				t.Fatal(err)
			}
			if res.StatusCode != http.StatusOK {
				t.Fatalf("anonymous preview returned %d", res.StatusCode)
			}
			doc, err := html.Parse(strings.NewReader(readBody(t, res)))
			if err != nil {
				t.Fatal(err)
			}
			metadata := make(map[string]string)
			for _, node := range htmlNodes(doc, isElement("meta")) {
				key := htmlAttr(node, "property")
				if key == "" {
					key = htmlAttr(node, "name")
				}
				if _, exists := metadata[key]; exists && strings.HasPrefix(key, "og:") {
					t.Fatalf("duplicate metadata: %s", key)
				}
				metadata[key] = htmlAttr(node, "content")
			}
			for key, want := range map[string]string{
				"og:title":        "Önskelistan",
				"og:type":         "website",
				"og:site_name":    "Önskelistan",
				"og:locale":       "sv_SE",
				"og:url":          origin + tc.canonical,
				"og:image":        origin + "/static/share-card-v1.jpg",
				"og:image:type":   "image/jpeg",
				"og:image:width":  "1200",
				"og:image:height": "630",
				"twitter:card":    "summary_large_image",
				"twitter:image":   origin + "/static/share-card-v1.jpg",
			} {
				if metadata[key] != want {
					t.Errorf("%s = %q, want %q", key, metadata[key], want)
				}
			}
			for _, key := range []string{"og:title", "og:description", "og:image:alt"} {
				if strings.Contains(metadata[key], child.Name) || strings.Contains(metadata[key], wish.Title) {
					t.Fatal("preview metadata contains list contents")
				}
			}
			if metadata["og:description"] == "" || metadata["og:image:alt"] == "" {
				t.Fatal("preview needs a description and image alternative text")
			}
		})
	}
}

func TestSocialPreviewImageIsPublicAndMatchesMetadata(t *testing.T) {
	db, _ := testApp(t)
	app, err := newApp(db, "https://wishlist.andreasekman.com", func(int) int { return 0 }, false, "123456")
	if err != nil {
		t.Fatal(err)
	}
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/static/share-card-v1.jpg", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("preview image returned %d %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
	cfg, err := jpeg.DecodeConfig(res.Body)
	if err != nil || cfg.Width != 1200 || cfg.Height != 630 {
		t.Fatalf("preview dimensions = %+v, %v", cfg, err)
	}
}

func TestSocialPreviewUsesRequestOriginWithoutBaseURL(t *testing.T) {
	_, app := testApp(t)
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "http://preview.example/", nil))
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, res)
	if !strings.Contains(body, `property="og:image" content="http://preview.example/static/share-card-v1.jpg"`) || !strings.Contains(body, `property="og:url" content="http://preview.example/"`) {
		t.Fatal("preview URLs must use the request origin when BASE_URL is unset")
	}
}
