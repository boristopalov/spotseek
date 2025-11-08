package client

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"spotseek/config"
	db "spotseek/db"
	"spotseek/slsk/downloads"
	"spotseek/slsk/fileshare"
	nc "spotseek/slsk/net"
	"spotseek/slsk/peer"
	"spotseek/slsk/uploads"
	"sync"
	"time"
)

type IP struct {
	IP   string
	port uint32
}

type User struct {
	username   string
	status     uint32
	privileged bool
	avgSpeed   uint32
	uploadNum  uint32
	files      uint32
	dirs       uint32
	slotsFree  uint32
	country    string
}

func NewUser() *User {
	return &User{
		username:   "",
		status:     0,     // Default status
		privileged: false, // Default non-privileged
		avgSpeed:   0,     // Default speed
		uploadNum:  0,     // Default upload count
		files:      0,     // Default files count
		dirs:       0,     // Default directories count
		slotsFree:  0,     // Default slots free
		country:    "",    // Default country code
	}
}

type Room struct {
	users    []*User
	messages []string
}

type SearchResult struct {
	token uint32
	files []fileshare.SharedFile
}

type SlskClient struct {
	ClientID         string // identifier for multi-client setup
	Username         string // username of user logged in TODO: do i need this
	Host             string
	Port             int
	ServerConnection *nc.Connection
	Listener         net.Listener
	mu               sync.RWMutex
	User             string           // the user that is logged in
	JoinedRooms      map[string]*Room // room name --> users in room
	PeerManager      *peer.PeerManager
	SearchManager    *SearchManager
	DownloadManager  *downloads.DownloadManager
	UploadManager    *uploads.UploadManager

	ParentUsername  string // username of our parent, if we have one
	ParentIp        IP
	DistribSearchCh chan peer.DistribSearchMsg // used for incoming distributed msgs
	SearchResults   map[string][]SearchResult  // username -> token and results

	TransferReqCh chan peer.UploadStartMsg // used for when peers request files from us (we upload to them). Tokens are sent over this channel

	PeerLifecycleEventCh chan peer.PeerLifecycleEvent // used for peer lifecycle events (connect/disconnect)

	logger *slog.Logger
	shares *fileshare.Shared
}

func NewSlskClient(clientID string, username string, host string, port int, logger *slog.Logger) *SlskClient {
	if logger == nil {
		return nil
	}

	engine, err := db.NewSqliteDB(db.DefaultDBPath())
	if err != nil {
		logger.Error("failed to create db", "error", err.Error())
		return nil
	}

	shares := fileshare.NewShared(config.GetSettings(), logger)
	distributedsearchCh := make(chan peer.DistribSearchMsg)
	transferReqCh := make(chan peer.UploadStartMsg)
	lifecycleEventCh := make(chan peer.PeerLifecycleEvent)
	searchManager := NewSearchManager(10*time.Minute, logger)
	downloadRepo := db.NewDownloadRepository(engine)
	uploadRepo := db.NewUploadRepository(engine)
	downloadManager := downloads.NewDownloadManager(10*time.Minute, logger, downloadRepo)
	uploadManager := uploads.NewUploadManager(10*time.Minute, logger, uploadRepo)

	client := &SlskClient{
		ClientID:             clientID,
		Username:             username,
		Host:                 host,
		Port:                 port,
		JoinedRooms:          make(map[string]*Room),
		DistribSearchCh:      distributedsearchCh,
		SearchResults:        make(map[string][]SearchResult),
		TransferReqCh:        transferReqCh,
		PeerLifecycleEventCh: lifecycleEventCh,
		SearchManager:        searchManager,
		DownloadManager:      downloadManager,
		UploadManager:        uploadManager,
		logger:               logger,
		shares:               shares,
	}

	client.PeerManager = peer.NewPeerManager(username, distributedsearchCh, transferReqCh, lifecycleEventCh, shares, searchManager, downloadManager, uploadManager, logger)

	return client
}

// Connect to soulseek server and login
func (c *SlskClient) Connect(username, pw string, peerPort int) error {
	// Set up our shared files
	stats := c.shares.GetShareStats()
	c.logger.Info("share stats", "stats", stats)

	dialer := &net.Dialer{
		KeepAlive: 120 * time.Second,
	}
	conn, err := dialer.Dial("tcp", net.JoinHostPort(c.Host, fmt.Sprintf("%d", c.Port)))
	if err != nil {
		return errors.New("unable to dial tcp connection; " + err.Error())
	}

	// Set up peer listener
	var listener net.Listener
	actualPort := 0

	if peerPort > 0 {
		// Use specified port
		listener, err = net.Listen("tcp", fmt.Sprintf(":%d", peerPort))
		if err != nil {
			return fmt.Errorf("unable to listen on specified port %d: %w", peerPort, err)
		}
		actualPort = peerPort
		c.logger.Info("Listening on specified port", "port", actualPort)
	} else {
		// Auto-discover an available port starting from 2234
		startPort := 2234
		maxPort := startPort + 100 // Try up to 100 ports

		for port := startPort; port < maxPort; port++ {
			listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
			if err == nil {
				actualPort = port
				c.logger.Info("Listening on port", "port", actualPort)
				break
			}
		}

		if listener == nil {
			return fmt.Errorf("unable to find available port in range %d-%d", startPort, maxPort-1)
		}
	}

	c.ServerConnection = &nc.Connection{Conn: conn}
	c.Listener = listener

	c.User = username

	go func() {
		for {
			c.ListenForServerMessages()
			c.Login(username, pw)
			c.SetWaitPort(uint32(actualPort))
			c.logger.Error("Server listener stopped, restarting...")
		}
	}()

	c.Login(username, pw)
	c.SetWaitPort(uint32(actualPort))
	go c.ListenForIncomingPeers()
	go c.listenForDistribSearches()
	go c.listenForUploadRequests()
	go c.listenForPeerLifecycleEvents()

	// c.JoinRoom("nicotine")
	// c.JoinRoom("The Lobby")
	return nil
}

// listenForPeerLifecycleEvents listens for peer lifecycle events (for parent handling, etc.)
func (c *SlskClient) listenForPeerLifecycleEvents() {
	for event := range c.PeerLifecycleEventCh {
		if event.Event == peer.EventDisconnected {
			// Check if this was our parent peer
			if c.ParentIp.IP == event.Host {
				c.logger.Info("Parent disconnected", "username", event.Username)
				c.ParentIp = IP{}
				c.ParentUsername = ""
				c.HaveNoParent(1)
			}
		}
	}
}

func (c *SlskClient) Close() error {
	if c.ServerConnection == nil {
		return nil // Connection is already closed
	}

	err := c.ServerConnection.Close()
	if err != nil {
		return err
	}
	c.User = ""
	c.logger.Info("Connection closed")
	return nil
}

// messages get sent to DistribSearchCh when distributed peers send us queries
func (c *SlskClient) listenForDistribSearches() {
	for msg := range c.DistribSearchCh {
		if len(msg.Query) < 4 {
			continue
		}
		c.PeerManager.SendDistribSearch(msg.Username, msg.Token, msg.Query)
		res := c.shares.Search(msg.Query)
		if len(res) > 0 {
			c.logger.Info("distributed search match", "query", msg.Query, "username", msg.Username)
			c.SendSearchResults(msg.Username, msg.Token, res)
		}
	}
}

func (c *SlskClient) SendSearchResults(username string, token uint32, results []fileshare.SharedFile) {
	if peer := c.PeerManager.GetDefaultPeer(username); peer != nil {
		peer.FileSearchResponse(username, token, results)
	} else {
		// store the search result
		// when we get a response for peer address, we will try sending search result
		// name of this method is not accurate...
		// if we don't have a peer we don't actually send the results until a connection is established
		c.mu.Lock()
		c.SearchResults[username] = append(
			c.SearchResults[username],
			SearchResult{token: token, files: results})

		c.mu.Unlock()
		c.logger.Info("added search results", "username", username, "token", token, "numResults", len(results))
		c.RequestPeerConnection(username, "P", token, false)
	}
}

// messages get sent to TransferRequestCh when peers request a file from us
// Upload info is tracked in UploadManager
func (c *SlskClient) listenForUploadRequests() {
	for msg := range c.TransferReqCh {

		c.logger.Info("attempting to start file transfer", "transfer", msg)
		c.PeerManager.AddPendingConnection(msg.Username, msg.Token, "F", 0, false)
		c.RequestPeerConnection(msg.Username, "F", msg.Token, false)
	}
}
