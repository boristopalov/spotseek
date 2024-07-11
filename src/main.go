package main

import (
	"context"
	"log"
	"net/http"
	"spotseek/src/config"
	"spotseek/src/slsk/api"
	"spotseek/src/slsk/client"
	"spotseek/src/slsk/utils"
	"spotseek/src/spotifyClient"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

type ContextKey string

func (c ContextKey) String() string {
	return string(c)
}

func main() {
	rdb := redis.NewClient(&redis.Options{
		Addr:     config.REDIS_URI,
		Password: config.REDIS_PASSWORD,
		DB:       config.REDIS_DB,
	})

	spotifyClient := spotifyClient.NewSpotifyClient(rdb)
	slskClient := client.NewSlskClient("server.slsknet.org", 2242)
	slskClient.Connect()
	log.Println(slskClient)

	mux := chi.NewRouter()

	// middleware to pass in slskCLient into the context
	// TODO: some other mapping to support multiple soulseek clients?
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ctx = context.WithValue(ctx, "slskClient", *slskClient)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	mux.Get("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("hey")
	})
	mux.Get("/login", spotifyClient.LoginHandler)
	mux.Get("/callback", spotifyClient.CallbackTokenHandler)
	mux.Get("/connect/user/{username}/conn/{connType}", api.ConnectToPeer)
	mux.Get("/slsk-client", utils.GetSlskClient)
	mux.Get("/check-port", utils.CheckPort)
	// mux.Get("/downloadPlaylist", spotifyClient.downloadPlaylistHandler)
	http.ListenAndServe("localhost:3000", mux)

}

// func withRedis(c *redis.Client, f func(c *redis.Client, w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
// return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { f(c, w, r) })
// }
