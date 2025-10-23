package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"spotseek/config"
	"spotseek/logging"
	"spotseek/slsk/api"
	"spotseek/slsk/client"
	"spotseek/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-chi/chi/v5"
)

type CLI struct {
	output io.Writer
}

func (c *CLI) Run(args []string) error {
	if len(args) < 1 {
		fmt.Fprintln(c.output, "Usage:")
		fmt.Fprintln(c.output, "  serve                          - Start the HTTP server")
		fmt.Fprintln(c.output, "  tui                            - Start the TUI")
		return fmt.Errorf("no command provided")
	}

	command := args[0]
	switch command {
	case "serve":
		logger := logging.NewHandler(nil, os.Stdout)
		// logger := logging.NewHandler(&slog.HandlerOptions{
		// 	Level:     slog.LevelInfo,
		// 	AddSource: false,
		// }, os.Stdout)
		log := slog.New(logger)
		slskClient := client.NewSlskClient(config.SOULSEEK_USERNAME, "server.slsknet.org", 2242, log)
		err := slskClient.Connect(config.SOULSEEK_USERNAME, config.SOULSEEK_PASSWORD)
		if err != nil {
			logging.LogFatal(log, "Failed to connect to soulseek", "err", err)
		}
		return startServer(slskClient)

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
		slskClient := client.NewSlskClient(config.SOULSEEK_USERNAME, "server.slsknet.org", 2242, log)
		err = slskClient.Connect(config.SOULSEEK_USERNAME, config.SOULSEEK_PASSWORD)
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

func startServer(slskClient *client.SlskClient) error {
	mux := chi.NewRouter()
	handler := api.NewAPIHandler(slskClient)

	mux.Get("/connect/user/{username}/conn/{connType}", handler.ConnectToPeer)
	mux.Post("/search", handler.Search)
	mux.Get("/search/{id}", handler.GetSearch)
	mux.Get("/search/{id}/results", handler.GetSearchResults)
	mux.Get("/download", handler.Download)
	mux.Get("/join", handler.JoinRoom)
	mux.Get("/slsk-client", handler.GetSlskClient)
	mux.Get("/check-port", handler.CheckPort)

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
