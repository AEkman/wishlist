package store

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func postgresTestURL(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("WISHLIST_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set WISHLIST_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	schemaName := "wishlist_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.ExecContext(t.Context(), "CREATE SCHEMA "+schemaName); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP SCHEMA " + schemaName + " CASCADE"); err != nil {
			t.Error(err)
		}
	})
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schemaName)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func TestPostgresPersistsFamilyAndWishlist(t *testing.T) {
	dsn := postgresTestURL(t)
	s := openTestStore(t, dsn)
	token, err := s.FamilyShareID(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	list := createList(t, s, "Ella", "Jul")
	if list.FamilyVisible {
		t.Fatal("new list must be private")
	}
	if err := s.SetFamilyVisible(t.Context(), list.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestStore(t, dsn)
	saved, err := reopened.SharedWishlist(t.Context(), list.ShareID)
	if err != nil || saved.ID != list.ID || !saved.FamilyVisible {
		t.Fatalf("saved list = %+v, %v", saved, err)
	}
	again, err := reopened.FamilyShareID(t.Context())
	if err != nil || again != token {
		t.Fatalf("family token changed: %q, %v", again, err)
	}
	lists, err := reopened.FamilyWishlists(t.Context())
	if err != nil || len(lists) != 1 || lists[0] != saved {
		t.Fatalf("family lists = %+v, %v", lists, err)
	}
}

func TestPostgresWishesRankingAndSessions(t *testing.T) {
	s := openTestStore(t, postgresTestURL(t))
	list := createList(t, s, "Ella", "Jul")
	bike := createWish(t, s, list.ID, "Cykel")
	lego := createWish(t, s, list.ID, "LEGO")
	if _, err := s.Pick(t.Context(), list.ID, lego.ID, bike.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkBought(t.Context(), list.ID, lego.ID, "Anna"); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkBought(t.Context(), list.ID, lego.ID, "Bo"); !errors.Is(err, ErrAlreadyBought) {
		t.Fatalf("second buyer = %v", err)
	}
	updated, err := s.UpdateWish(t.Context(), list.ID, lego.ID, "LEGO rymd", "Blå", "image/jpeg", []byte{0, 1, 255})
	if err != nil || updated.ImageRevision != 1 || updated.Buyer != "Anna" {
		t.Fatalf("updated wish = %+v, %v", updated, err)
	}
	wishes, err := s.Wishes(t.Context(), list.ID)
	if err != nil || len(wishes) != 2 || wishes[0].ID != lego.ID || wishes[1].ID != bike.ID {
		t.Fatalf("ranked wishes = %+v, %v", wishes, err)
	}
	media, image, err := s.Image(t.Context(), lego.ID)
	if err != nil || media != "image/jpeg" || !bytes.Equal(image, []byte{0, 1, 255}) {
		t.Fatalf("image = %s %v %v", media, image, err)
	}
	if err := s.ClearBought(t.Context(), "another-list", lego.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-list purchase = %v", err)
	}
	if err := s.ClearBought(t.Context(), list.ID, lego.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	hash := []byte{1, 2, 3, 0, 255}
	if err := s.SaveDeviceSession(t.Context(), hash, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	valid, err := s.ValidDeviceSession(t.Context(), hash, now)
	if err != nil || !valid {
		t.Fatalf("session = %t, %v", valid, err)
	}
	valid, err = s.ValidDeviceSession(t.Context(), hash, now.Add(2*time.Hour))
	if err != nil || valid {
		t.Fatalf("expired session = %t, %v", valid, err)
	}
	if _, err := s.CreateChild(t.Context(), "Ella"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("duplicate child = %v", err)
	}
}

func TestPostgresConcurrentChildCreation(t *testing.T) {
	dsn := postgresTestURL(t)
	first := openTestStore(t, dsn)
	second := openTestStore(t, dsn)
	var wg sync.WaitGroup
	failures := make(chan error, 20)
	for i := range 20 {
		wg.Go(func() {
			s := first
			if i%2 == 0 {
				s = second
			}
			_, err := s.CreateChild(t.Context(), fmt.Sprintf("Barn %d", i))
			failures <- err
		})
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Error(err)
		}
	}
	children, err := first.Children(t.Context())
	if err != nil || len(children) != 20 {
		t.Fatalf("children = %d, %v", len(children), err)
	}
}

func TestPostgresConcurrentWishCreationKeepsPositionsUnique(t *testing.T) {
	dsn := postgresTestURL(t)
	first := openTestStore(t, dsn)
	second := openTestStore(t, dsn)
	list := createList(t, first, "Ella", "Jul")
	// Hold inserts briefly so both application connections read the same initial rank.
	if _, err := first.db.ExecContext(t.Context(), `
 CREATE FUNCTION delay_wish_insert() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN PERFORM pg_sleep(0.02); RETURN NEW; END $$;
 CREATE TRIGGER delay_wish_insert BEFORE INSERT ON wishes FOR EACH ROW EXECUTE FUNCTION delay_wish_insert();
 `); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	failures := make(chan error, 20)
	for i := range 20 {
		wg.Go(func() {
			s := first
			if i%2 == 0 {
				s = second
			}
			_, err := s.CreateWish(t.Context(), list.ID, fmt.Sprintf("Önskan %d", i), "", "image/png", []byte{1})
			failures <- err
		})
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	var positions int
	if err := first.queryRowContext(t.Context(), "SELECT COUNT(DISTINCT position) FROM wishes WHERE wishlist_id = ?", list.ID).Scan(&positions); err != nil {
		t.Fatal(err)
	}
	if positions != 20 {
		t.Fatalf("only %d distinct positions for 20 wishes", positions)
	}
}

func TestPostgresConcurrentPicksKeepRanksUnique(t *testing.T) {
	dsn := postgresTestURL(t)
	first := openTestStore(t, dsn)
	second := openTestStore(t, dsn)
	list := createList(t, first, "Ella", "Jul")
	a := createWish(t, first, list.ID, "A")
	b := createWish(t, first, list.ID, "B")
	c := createWish(t, first, list.ID, "C")
	if _, err := first.db.ExecContext(t.Context(), `
 CREATE FUNCTION delay_rank_update() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN PERFORM pg_sleep(0.02); RETURN NEW; END $$;
 CREATE TRIGGER delay_rank_update BEFORE UPDATE OF position ON wishes FOR EACH ROW EXECUTE FUNCTION delay_rank_update();
 `); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	failures := make(chan error, 2)
	wg.Go(func() { _, err := first.Pick(t.Context(), list.ID, c.ID, a.ID); failures <- err })
	wg.Go(func() { _, err := second.Pick(t.Context(), list.ID, b.ID, a.ID); failures <- err })
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	var positions int
	if err := first.queryRowContext(t.Context(), "SELECT COUNT(DISTINCT position) FROM wishes WHERE wishlist_id = ?", list.ID).Scan(&positions); err != nil {
		t.Fatal(err)
	}
	if positions != 3 {
		t.Fatalf("only %d distinct ranks for 3 wishes", positions)
	}
}

func TestPostgresRecordsGooseMigration(t *testing.T) {
	s := openTestStore(t, postgresTestURL(t))
	var version int
	if err := s.db.QueryRowContext(t.Context(), "SELECT MAX(version_id) FROM goose_db_version WHERE is_applied").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Fatalf("Goose version = %d, want 2", version)
	}
}

func TestPostgresGooseAdoptsExistingDatabase(t *testing.T) {
	dsn := postgresTestURL(t)
	legacy, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.Close()
	if _, err := legacy.ExecContext(t.Context(), legacyPostgresSchema+`
 CREATE TABLE wishlist_schema_migrations (version INTEGER PRIMARY KEY);
 INSERT INTO wishlist_schema_migrations VALUES (1);
 INSERT INTO children (id,name) VALUES ('ella','Ella');
 INSERT INTO wishlists (id,child_id,title,share_id,family_visible) VALUES ('list','ella','Jul','original-link',1);
 INSERT INTO wishes (id,wishlist_id,title,description,media_type,image,position,buyer,image_revision)
 VALUES ('wish','list','LEGO','','image/png',decode('00ff','hex'),0,'Anna',2);
 INSERT INTO family_sharing (id,share_id) VALUES (1,'original-family-link');
 `); err != nil {
		t.Fatal(err)
	}
	s := openTestStore(t, dsn)
	token, err := s.FamilyShareID(t.Context())
	if err != nil || token != "original-family-link" {
		t.Fatalf("token = %q, %v", token, err)
	}
	list, err := s.SharedWishlist(t.Context(), "original-link")
	if err != nil || !list.FamilyVisible {
		t.Fatalf("list = %+v, %v", list, err)
	}
	wish, err := s.Wish(t.Context(), list.ID, "wish")
	if err != nil || wish.Buyer != "Anna" || wish.ImageRevision != 2 {
		t.Fatalf("wish = %+v, %v", wish, err)
	}
	_, image, err := s.Image(t.Context(), wish.ID)
	if err != nil || !bytes.Equal(image, []byte{0, 255}) {
		t.Fatalf("image = %v, %v", image, err)
	}
	var version int
	if err := s.db.QueryRowContext(t.Context(), "SELECT MAX(version_id) FROM goose_db_version WHERE is_applied").Scan(&version); err != nil || version != 2 {
		t.Fatalf("Goose version = %d, %v", version, err)
	}
}

func TestPostgresConcurrentGooseStartup(t *testing.T) {
	dsn := postgresTestURL(t)
	type result struct {
		store *Store
		err   error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() { s, err := Open(dsn); results <- result{s, err} })
	}
	wg.Wait()
	close(results)
	token := ""
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		defer result.store.Close()
		got, err := result.store.FamilyShareID(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if token != "" && token != got {
			t.Fatal("concurrent startup changed family link")
		}
		token = got
	}
}
