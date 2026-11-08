// Command forage-server serves the Forage PWA and its API.
//
// Environment:
//
//	FORAGE_API_KEY  required; bearer token for /api routes
//	FORAGE_DIR      data directory (default /home/sprite/forage)
//	PORT            listen port (default 8080)
//	FORAGE_TZ       IANA time zone for "today" (e.g. America/New_York);
//	                default is the system zone, which is UTC on the Sprite
//	FORAGE_DEV_WWW  development only: serve PWA files from this directory
//	                instead of the embedded copy, so edits show without a rebuild
package main

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // FORAGE_TZ works even without system zone files

	"forage/internal/api"
	"forage/internal/pwa"
	"forage/internal/storage"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	forageDir := envOr("FORAGE_DIR", "/home/sprite/forage")
	port := envOr("PORT", "8080")
	apiKey := os.Getenv("FORAGE_API_KEY")
	if apiKey == "" {
		log.Fatal("FORAGE_API_KEY is required")
	}

	if tz := os.Getenv("FORAGE_TZ"); tz != "" {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			log.Fatalf("FORAGE_TZ: %v", err)
		}
		time.Local = loc
	}

	var static fs.FS = pwa.Assets()
	staticDesc := "embedded"
	if dir := os.Getenv("FORAGE_DEV_WWW"); dir != "" {
		static = os.DirFS(dir)
		staticDesc = dir
		// A "dev" build makes the service worker fetch every file fresh,
		// so edits show on reload.
		version = "dev-" + version
	}

	store, err := storage.New(forageDir)
	if err != nil {
		log.Fatalf("opening store: %v", err)
	}
	defer store.Close()

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           api.NewHandler(store, apiKey, version, static),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("forage-server %s listening on :%s (data=%s, static=%s, tz=%s)", version, port, forageDir, staticDesc, time.Local)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Print("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
