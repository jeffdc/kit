package main

import (
	"log"
	"net/http"
	"os"

	"forage/internal/api"
	"forage/internal/storage"
)

func main() {
	forageDir := os.Getenv("FORAGE_DIR")
	if forageDir == "" {
		forageDir = "/home/sprite/forage"
	}

	apiKey := os.Getenv("FORAGE_API_KEY")
	if apiKey == "" {
		log.Fatal("FORAGE_API_KEY is required")
	}

	wwwDir := os.Getenv("FORAGE_WWW")
	if wwwDir == "" {
		wwwDir = "./www"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	store, err := storage.New(forageDir)
	if err != nil {
		log.Fatalf("opening store: %v", err)
	}
	defer store.Close()

	handler := api.NewHandler(store, apiKey, wwwDir)

	log.Printf("forage-server listening on :%s (data=%s, www=%s)", port, forageDir, wwwDir)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
