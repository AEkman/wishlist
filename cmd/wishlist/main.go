package main

import (
	"cmp"
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/AEkman/wishlist/internal/store"
	"github.com/AEkman/wishlist/internal/web"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL != "" && !strings.HasPrefix(databaseURL, "postgres://") && !strings.HasPrefix(databaseURL, "postgresql://") {
		return errors.New("DATABASE_URL must be a postgres:// or postgresql:// URL")
	}
	db, err := store.Open(cmp.Or(databaseURL, os.Getenv("DATABASE_PATH"), "wishlists.db"))
	if err != nil {
		return err
	}
	defer db.Close()

	app, err := web.New(db, os.Getenv("BASE_URL"))
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	listenErr := make(chan error, 1)
	go func() {
		listenErr <- app.Listen(cmp.Or(os.Getenv("LISTEN_ADDR"), ":8080"))
	}()

	select {
	case err := <-listenErr:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := app.ShutdownWithContext(shutdown); err != nil {
			return err
		}
		select {
		case err := <-listenErr:
			return err
		case <-shutdown.Done():
			return errors.New("HTTP server did not stop within the shutdown timeout")
		}
	}
}
