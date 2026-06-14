package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"scribblesplash/internal/comments"
	"scribblesplash/internal/server"
	"scribblesplash/internal/storage"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	store, err := storage.New("articles")
	if err != nil {
		log.Fatalf("error loading articles: %v", err)
	}
	log.Printf("Loaded %d articles", len(store.Articles))

	dataDir := "data"
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("error creating data dir: %v", err)
	}

	cm, err := comments.New(dataDir)
	if err != nil {
		log.Fatalf("error initializing comments: %v", err)
	}

	srv, err := server.New(store, cm, "templates")
	if err != nil {
		log.Fatalf("error creating server: %v", err)
	}

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Scribblesplash running on http://localhost%s", addr)

	if err := http.ListenAndServe(addr, srv.Routes()); err != nil {
		log.Fatalf("server error: %v", err)
	}
	_ = filepath.Join
}
