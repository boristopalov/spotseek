package main

import (
	"context"
	"net/http"
	"spotseek/logging"
	"spotseek/src/config"
	"spotseek/src/slsk/api"
	"spotseek/src/slsk/client"

	"github.com/go-chi/chi/v5"
)

var log = logging.GetLogger()

type ContextKey string

func (c ContextKey) String() string {
	return string(c)
}

func main() {

	slskClient := client.NewSlskClient("server.slsknet.org", 2242)
	err := slskClient.Connect(config.SOULSEEK_USERNAME, config.SOULSEEK_PASSWORD)
	if err != nil {
		logging.LogFatal(log, "Failed to connect to soulseek", "err", err)
	}
	mux := chi.NewRouter()

	// middleware to pass in slskCLient into the context
	// should delete
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ctx = context.WithValue(ctx, "slskClient", slskClient)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	// rdb := redis.NewClient(&redis.Options{
	// 		Addr:     config.REDIS_URI,
	// 		Password: config.REDIS_PASSWORD,
	// 		DB:       config.REDIS_DB,
	// 	})
	// spotifyClient := spotify.NewSpotifyClient(rdb)
	// mux.Get("/login", spotifyClient.LoginHandler)
	// mux.Get("/callback", spotifyClient.CallbackTokenHandler)

	mux.Get("/connect/user/{username}/conn/{connType}", api.ConnectToPeer)
	mux.Get("/search", api.Search)
	mux.Get("/download", api.Download)
	mux.Get("/join", api.JoinRoom)

	// print out info about the slsk client
	mux.Get("/slsk-client", api.GetSlskClient)

	// check if port is open to receive messages from soulseek
	mux.Get("/check-port", api.CheckPort)

	// mux.Get("/downloadPlaylist", spotifyClient.downloadPlaylistHandler)
	http.ListenAndServe("localhost:3000", mux)

}

// func withRedis(c *redis.Client, f func(c *redis.Client, w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
// return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { f(c, w, r) })
// }
