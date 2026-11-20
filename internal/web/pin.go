package web

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const deviceCookie = "wishlist_device"
const deviceLifetime = 365 * 24 * time.Hour

type pinAttempts struct {
	mu    sync.Mutex
	until time.Time
	count int
}

func validPIN(pin string) bool {
	if len(pin) < 4 || len(pin) > 12 {
		return false
	}
	for _, r := range pin {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func sessionHash(token, pin string) []byte {
	sum := sha256.Sum256([]byte(token + ":" + pin))
	return sum[:]
}

func (s server) requirePIN(pin string) fiber.Handler {
	return func(c fiber.Ctx) error {
		path := c.Path()
		if strings.HasPrefix(path, "/static/") || isSharedRoute(path) {
			return c.Next()
		}
		token := c.Cookies(deviceCookie)
		if len(token) == 43 {
			valid, err := s.db.ValidDeviceSession(c.Context(), sessionHash(token, pin), time.Now())
			if err != nil {
				return err
			}
			if valid {
				return c.Next()
			}
		}
		if c.Method() != http.MethodGet {
			if c.Get("HX-Request") == "true" {
				next := "/"
				if ref, err := url.Parse(c.Get("HX-Current-URL")); err == nil && (ref.Scheme == "http" || ref.Scheme == "https") && strings.EqualFold(ref.Host, c.Host()) {
					next = safeNext(ref.RequestURI())
				}
				c.Set("HX-Redirect", "/unlock?next="+url.QueryEscape(next))
				return c.SendStatus(http.StatusOK)
			}
			return fiber.ErrUnauthorized
		}
		return redirect(c, "/unlock?next="+url.QueryEscape(c.OriginalURL()))
	}
}

func isSharedRoute(path string) bool {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) == 0 {
		return false
	}
	if parts[0] == "family" {
		if len(parts) < 2 || uuid.Validate(parts[1]) != nil {
			return false
		}
		switch len(parts) {
		case 2:
			return true
		case 4:
			return parts[2] == "children" || parts[2] == "lists"
		case 6:
			return parts[2] == "lists" && parts[4] == "images"
		case 7:
			return parts[2] == "lists" && parts[4] == "wishes" && (parts[6] == "buy" || parts[6] == "clear")
		}
		return false
	}
	if _, err := uuid.Parse(parts[0]); err != nil {
		return false
	}
	if len(parts) == 1 {
		return true
	}
	return len(parts) == 4 && parts[1] == "wishes" && (parts[3] == "buy" || parts[3] == "clear" || parts[1] == "images") || len(parts) == 3 && parts[1] == "images"
}

func safeNext(next string) string {
	parsed, err := url.Parse(next)
	if err != nil || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || parsed.IsAbs() || parsed.Host != "" {
		return "/"
	}
	return parsed.RequestURI()
}

func (s server) unlockForm(c fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	return s.show(c, page{View: "unlock", Title: "Lås upp", Next: safeNext(c.Query("next"))})
}

func (s server) unlock(pin string) fiber.Handler {
	attempts := &pinAttempts{}
	return func(c fiber.Ctx) error {
		c.Set("Cache-Control", "no-store")
		attempts.mu.Lock()
		now := time.Now()
		if now.Before(attempts.until) {
			attempts.mu.Unlock()
			return c.Status(http.StatusTooManyRequests).SendString("För många försök. Försök igen senare.")
		}
		submitted, expected := sha256.Sum256([]byte(c.FormValue("pin"))), sha256.Sum256([]byte(pin))
		if subtle.ConstantTimeCompare(submitted[:], expected[:]) != 1 {
			attempts.count++
			if attempts.count >= 10 {
				attempts.count = 0
				attempts.until = now.Add(15 * time.Minute)
			}
			attempts.mu.Unlock()
			c.Status(http.StatusUnauthorized)
			return s.show(c, page{View: "unlock", Title: "Lås upp", Error: "Fel PIN-kod. Försök igen.", Next: safeNext(c.FormValue("next"))})
		}
		attempts.count = 0
		attempts.mu.Unlock()
		var raw [32]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return err
		}
		token := base64.RawURLEncoding.EncodeToString(raw[:])
		expires := now.Add(deviceLifetime)
		if err := s.db.SaveDeviceSession(c.Context(), sessionHash(token, pin), expires); err != nil {
			return err
		}
		c.Cookie(&fiber.Cookie{Name: deviceCookie, Value: token, Path: "/", Expires: expires, MaxAge: int(deviceLifetime.Seconds()), HTTPOnly: true, Secure: strings.HasPrefix(s.baseURL, "https://") || c.Protocol() == "https", SameSite: "Lax"})
		return redirect(c, safeNext(c.FormValue("next")))
	}
}

func (s server) sharedImage(c fiber.Ctx) error {
	list, err := s.sharedWishlist(c)
	if err != nil {
		return err
	}
	if _, err := s.db.Wish(c.Context(), list.ID, c.Params("wish")); err != nil {
		return err
	}
	return s.image(c)
}
