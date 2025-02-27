package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"spotseek/config"
	"spotseek/logging"
	"spotseek/slsk/client"
	"spotseek/slsk/tui"

	tea "github.com/charmbracelet/bubbletea"
)

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
		fmt.Fprintln(c.output, "  serve                          - Start the HTTP server")
		fmt.Fprintln(c.output, "  tui                            - Start the TUI")
		return fmt.Errorf("no command provided")
	}

	command := args[0]
	switch command {
	// case "serve":
	// log := logging.GetLogger()
	// slskClient := client.NewSlskClient("server.slsknet.org", 2242)
	// err := slskClient.Connect(config.SOULSEEK_USERNAME, config.SOULSEEK_PASSWORD)
	// if err != nil {
	// 	logging.LogFatal(log, "Failed to connect to soulseek", "err", err)
	// }
	// return startServer(slskClient)

	case "tui":
		logFile, err := os.OpenFile("spotseek.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		defer logFile.Close()

		// Create handler with the file as writer
		fileHandler := logging.NewHandler(&slog.HandlerOptions{
			Level:     slog.LevelInfo,
			AddSource: false,
		}, logFile)

		log := slog.New(fileHandler)

		// Set up soulseek client
		slskClient := client.NewSlskClient("server.slsknet.org", 2242, log)
		err = slskClient.Connect(config.SOULSEEK_USERNAME, config.SOULSEEK_PASSWORD)
		if err != nil {
			logging.LogFatal(log, "Failed to connect to soulseek", "err", err)
		}

		tuiModel := tui.NewModel(slskClient)
		p := tea.NewProgram(tuiModel)

		// Start event handling in background
		go func() {
			for event := range slskClient.PeerEventCh {
				p.Send(event)
			}
		}()

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error running tui: %w", err)
		}
		return nil

	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// func startServer(slskClient *client.SlskClient) error {
// 	mux := chi.NewRouter()

// 	mux.Use(func(next http.Handler) http.Handler {
// 		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 			ctx := r.Context()
// 			ctx = context.WithValue(ctx, SlskClientKey, slskClient)
// 			next.ServeHTTP(w, r.WithContext(ctx))
// 		})
// 	})

// 	mux.Get("/connect/user/{username}/conn/{connType}", api.ConnectToPeer)
// 	mux.Get("/search", api.Search)
// 	mux.Get("/download", api.Download)
// 	mux.Get("/join", api.JoinRoom)
// 	mux.Get("/slsk-client", api.GetSlskClient)
// 	mux.Get("/check-port", api.CheckPort)

// 	fmt.Println("Starting HTTP server on localhost:3000")
// 	return http.ListenAndServe("localhost:3000", mux)
// }

func main() {
	cli := &CLI{
		output: os.Stdout,
	}

	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
