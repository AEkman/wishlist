package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDatabaseURLTakesPrecedenceWithoutSQLiteFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "must-not-be-created.db")
	t.Setenv("DATABASE_PATH", path)
	t.Setenv("DATABASE_URL", "postgres://wishlist@127.0.0.1:1/wishlist?sslmode=disable&connect_timeout=1")
	t.Setenv("FAMILY_PIN", "")
	if err := run(); err == nil {
		t.Fatal("unavailable PostgreSQL must fail startup")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("DATABASE_URL must not open or fall back to SQLite: %v", err)
	}
}

func TestDatabaseURLRejectsNonPostgresValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-postgres-url.db")
	t.Setenv("DATABASE_URL", path)
	t.Setenv("FAMILY_PIN", "")
	if err := run(); err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("invalid DATABASE_URL must fail explicitly: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("invalid DATABASE_URL opened SQLite: %v", err)
	}
}
