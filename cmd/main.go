package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"spotseek/config"
	"spotseek/logging"
	"spotseek/slsk/api"
	"spotseek/slsk/client"

	"github.com/go-chi/chi/v5"
)

var log = logging.GetLogger()

type ContextKey string

func (c ContextKey) String() string {
	return string(c)
}

const SlskClientKey ContextKey = "slskClient"

type CLI struct {
	output io.Writer
}

func (c *CLI) Run(args []string) error {
	if len(args) < 1 {
		fmt.Fprintln(c.output, "Usage:")
		fmt.Fprintln(c.output, "  search <query>                  - Search for files")
		fmt.Fprintln(c.output, "  download <username> <filename>  - Download a file from a user")
		fmt.Fprintln(c.output, "  serve                          - Start the HTTP server")
		return fmt.Errorf("no command provided")
	}

	slskClient := client.NewSlskClient("server.slsknet.org", 2242)
	err := slskClient.Connect(config.SOULSEEK_USERNAME, config.SOULSEEK_PASSWORD)
	if err != nil {
		logging.LogFatal(log, "Failed to connect to soulseek", "err", err)
	}

	command := args[0]
	switch command {
	case "search":
		if len(args) < 2 {
			return fmt.Errorf("search requires a query")
		}
		query := args[1]
		slskClient.FileSearch(query)
		fmt.Fprintf(c.output, "Searching for: %s\n", query)

	case "download":
		if len(args) < 3 {
			return fmt.Errorf("download requires username and filename")
		}
		username := args[1]
		connType := args[2]
		filename := args[3]

		peer := slskClient.PeerManager.GetPeer(username, connType)
		if peer == nil {
			return fmt.Errorf("not connected to peer %s", username)
		}

		peer.QueueUpload(filename)
		fmt.Fprintf(c.output, "Download queued for %s from %s\n", filename, username)

	case "serve":
		return startServer(slskClient)

	default:
		return fmt.Errorf("unknown command: %s", command)
	}

	return nil
}

func startServer(slskClient *client.SlskClient) error {
	mux := chi.NewRouter()

	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ctx = context.WithValue(ctx, SlskClientKey, slskClient)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	mux.Get("/connect/user/{username}/conn/{connType}", api.ConnectToPeer)
	mux.Get("/search", api.Search)
	mux.Get("/download", api.Download)
	mux.Get("/join", api.JoinRoom)
	mux.Get("/slsk-client", api.GetSlskClient)
	mux.Get("/check-port", api.CheckPort)

	fmt.Println("Starting HTTP server on localhost:3000")
	return http.ListenAndServe("localhost:3000", mux)
}

func main() {
	cli := &CLI{
		output: os.Stdout,
	}

	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
