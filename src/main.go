package main

import (
	"spotseek/src/config"
	"spotseek/src/slskClient"
	"spotseek/src/spotifyClient"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func main() {
	err := godotenv.Load()
	if err != nil { 
		log.Fatal("Cannot load .env file")
	}
	rdb := redis.NewClient(&redis.Options{
		Addr: config.REDIS_URI,
		Password: config.REDIS_PASSWORD,
		DB: config.REDIS_DB,
	})

	spotifyClient := spotifyClient.NewSpotifyClient(rdb)
	slskClient.NewSlskClient("server.slsknet.org", 2242).Connect()

	mux := chi.NewRouter()
	mux.Get("/", func(w http.ResponseWriter, r *http.Request) { 
		fmt.Println("hey")
	})
	mux.Get("/login", spotifyClient.LoginHandler)
	mux.Get("/callback", spotifyClient.CallbackTokenHandler)
	// mux.Get("/downloadPlaylist", spotifyClient.downloadPlaylistHandler)
	http.ListenAndServe("localhost:3000", mux)
	
}
// func withRedis(c *redis.Client, f func(c *redis.Client, w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	// return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { f(c, w, r) })
// }
