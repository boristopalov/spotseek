package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"spotseek/db"
	"spotseek/spotify"
	"spotseek/worker"
	"syscall"

	"github.com/go-chi/chi/v5"
)

const spotseekApiUri = "http://localhost:3000"

func main() {
	// Initialize database
	database, err := db.NewDB(db.DefaultDBPath())
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	log.Println("Database initialized at", db.DefaultDBPath())

	// Initialize Spotify client
	spotifyClient := spotify.NewSpotifyClient()

	// Initialize track repository
	trackRepo := db.NewTrackRepository(database)

	// Initialize and start worker
	// The worker will call the Soulseek API at localhost:3000
	downloadWorker := worker.NewDownloadWorker(spotifyClient, trackRepo, spotseekApiUri)
	downloadWorker.Start()
	defer downloadWorker.Stop()

	log.Println("Download worker started")

	// Start HTTP server for Spotify OAuth
	go func() {
		if err := startSpotifyServer(spotifyClient); err != nil {
			log.Fatalf("Failed to start Spotify server: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
}

func startSpotifyServer(spotifyClient *spotify.SpotifyClient) error {
	mux := chi.NewRouter()

	// Spotify OAuth routes
	mux.Get("/login", spotifyClient.LoginHandler)
	mux.Get("/callback", spotifyClient.CallbackTokenHandler)

	// Health check
	mux.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	fmt.Println("Starting Spotify OAuth server on localhost:3001")
	fmt.Println("To authenticate, visit: http://localhost:3001/login")
	return http.ListenAndServe("localhost:3001", mux)
}
