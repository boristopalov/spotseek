package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"spotseek/config"
	"spotseek/logging"
	"spotseek/slsk/api"
	"spotseek/slsk/client"
	"spotseek/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

type CLI struct {
	output io.Writer
}

func (c *CLI) Run(args []string) error {
	if len(args) < 1 {
		fmt.Fprintln(c.output, "Usage:")
		fmt.Fprintln(c.output, "  serve [flags]                  - Start the HTTP server")
		fmt.Fprintln(c.output, "  tui [flags]                    - Start the TUI")
		fmt.Fprintln(c.output, "")
		fmt.Fprintln(c.output, "Flags:")
		fmt.Fprintln(c.output, "  -username string               - Soulseek username (default: from config)")
		fmt.Fprintln(c.output, "  -password string               - Soulseek password (default: from config)")
		return fmt.Errorf("no command provided")
	}

	command := args[0]

	// Create a FlagSet for parsing flags
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(c.output)

	username := fs.String("username", config.SOULSEEK_USERNAME, "Soulseek username")
	password := fs.String("password", config.SOULSEEK_PASSWORD, "Soulseek password")
	clientID := fs.String("client-id", "", "Client ID for multi-client setup")
	apiPort := fs.Int("api-port", 0, "API server port (0 = auto-discover)")
	peerPort := fs.Int("peer-port", 0, "Peer listener port (0 = auto-discover)")

	// Parse flags from remaining args
	if len(args) > 1 {
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
	}

	switch command {
	case "serve":
		logger := logging.NewHandler(nil, os.Stdout)
		// logger := logging.NewHandler(&slog.HandlerOptions{
		// 	Level:     slog.LevelInfo,
		// 	AddSource: false,
		// }, os.Stdout)
		log := slog.New(logger)
		slskClient := client.NewSlskClient(*clientID, *username, "server.slsknet.org", 2242, log)
		err := slskClient.Connect(*username, *password, *peerPort)
		if err != nil {
			logging.LogFatal(log, "Failed to connect to soulseek", "err", err)
		}
		return startServer(slskClient, *apiPort)

	case "tui":
		logFile, err := os.OpenFile("spotseek.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		defer logFile.Close()

		// Create handler with the file as writer
		fileHandler := logging.NewHandler(&slog.HandlerOptions{
			Level:     slog.LevelInfo,
			AddSource: true,
		}, logFile)

		log := slog.New(fileHandler)

		// Set up soulseek client
		slskClient := client.NewSlskClient(*clientID, *username, "server.slsknet.org", 2242, log)
		err = slskClient.Connect(*username, *password, *peerPort)
		if err != nil {
			logging.LogFatal(log, "Failed to connect to soulseek", "err", err)
		}

		tuiModel := tui.NewModel(slskClient)
		p := tea.NewProgram(tuiModel)

		// Start event handling in background
		// go func() {
		// 	for event := range slskClient.PeerEventCh {
		// 		p.Send(event)
		// 	}
		// }()

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error running tui: %w", err)
		}
		return nil

	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func startServer(slskClient *client.SlskClient, apiPort int) error {
	mux := chi.NewRouter()

	// Add CORS middleware
	mux.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://*", "https://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	handler := api.NewAPIHandler(slskClient)

	mux.Get("/status", handler.Status)
	mux.Get("/connect/user/{username}/conn/{connType}", handler.ConnectToPeer)
	mux.Post("/search", handler.Search)
	mux.Get("/search/{id}", handler.GetSearch)
	mux.Get("/search/{id}/results", handler.GetSearchResults)
	mux.Post("/download", handler.Download)
	mux.Get("/download/{username}/{filename}", handler.GetDownload)
	mux.Get("/downloads", handler.ListDownloads)
	mux.Delete("/download/{username}/{filename}", handler.CancelDownload)
	mux.Get("/uploads", handler.ListUploads)
	mux.Get("/upload/{username}/{filename}", handler.GetUpload)
	mux.Delete("/upload/{username}/{filename}", handler.CancelUpload)
	mux.Get("/join", handler.JoinRoom)
	mux.Get("/peers", handler.GetPeers)
	mux.Get("/check-port", handler.CheckPort)

	if apiPort > 0 {
		// Use specified port
		addr := fmt.Sprintf("localhost:%d", apiPort)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("unable to listen on specified API port %d: %w", apiPort, err)
		}
		fmt.Printf("Starting HTTP server on %s\n", addr)
		return http.Serve(listener, mux)
	}

	// Auto-discover an available port
	startPort := 3000
	maxPort := 3000 + 100

	for port := startPort; port <= maxPort; port++ {
		addr := fmt.Sprintf("localhost:%d", port)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			// Found a free port
			fmt.Printf("Starting HTTP server on %s\n", addr)
			return http.Serve(listener, mux)
		}
	}

	return fmt.Errorf("no free port found in range %d-%d", startPort, maxPort)
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
