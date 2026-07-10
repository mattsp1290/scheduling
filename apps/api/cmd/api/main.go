package main

import (
	"log"
	"net/http"
	"os"

	"github.com/mattsp1290/scheduling/apps/api/internal/app"
)

func main() {
	dbPath := getenv("DATABASE_PATH", "scheduling.db")
	addr := getenv("ADDR", ":8080")

	server, err := app.NewServer(dbPath)
	if err != nil {
		log.Fatalf("create server: %v", err)
	}
	defer server.Close()

	log.Printf("scheduling API listening on %s", addr)
	if err := http.ListenAndServe(addr, server.Routes()); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
