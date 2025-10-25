package client

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"spotseek/config"
	"spotseek/slsk/downloads"
	"spotseek/slsk/fileshare"
	nc "spotseek/slsk/net"
	"spotseek/slsk/peer"
	"sync"
	"time"
)

type IP struct {
	IP   string
	port uint32
}

type PendingTokenConn struct {
	username   string
	connType   string
	privileged uint8
	token      uint32
	isParent   bool
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

type distribSearchResult struct {
	token uint32
	files []fileshare.SharedFile
}

type SlskClient struct {
	Username         string // username of user logged in TODO: do i need this
	Host             string
	Port             int
	ServerConnection *nc.Connection
	Listener         net.Listener
	mu               sync.RWMutex
	User             string                      // the user that is logged in
	PendingUsers     map[string]PendingTokenConn // username --> connection info
	PendingTokens    map[uint32]PendingTokenConn // token --> connection info
	JoinedRooms      map[string]*Room            // room name --> users in room
	PeerManager      *peer.PeerManager
	SearchManager    *SearchManager
	DownloadManager  *downloads.DownloadManager

	ParentUsername       string // username of our parent, if we have one
	ParentIp             IP
	DistribSearchCh      chan peer.DistribSearchMsg       // used for incoming distributed msgs
	DistribSearchResults map[string][]distribSearchResult // username -> token and results

	TransferReqCh       chan peer.FileTransfer       // used for when peers request files from us
	PendingTransferReqs map[string]peer.FileTransfer // username_token -> file

	logger *slog.Logger
	shares *fileshare.Shared
}

func NewSlskClient(username string, host string, port int, logger *slog.Logger) *SlskClient {
	if logger == nil {
		return nil
	}
	shares := fileshare.NewShared(config.GetSettings(), logger)
	distributedsearchCh := make(chan peer.DistribSearchMsg)
	transferReqCh := make(chan peer.FileTransfer)
	searchManager := NewSearchManager(10*time.Minute, logger)
	downloadManager := downloads.NewDownloadManager(10*time.Minute, logger)

	client := &SlskClient{
		Username:             username,
		Host:                 host,
		Port:                 port,
		PendingUsers:         make(map[string]PendingTokenConn),
		PendingTokens:        make(map[uint32]PendingTokenConn),
		JoinedRooms:          make(map[string]*Room),
		DistribSearchCh:      distributedsearchCh,
		DistribSearchResults: make(map[string][]distribSearchResult),
		TransferReqCh:        transferReqCh,
		PendingTransferReqs:  make(map[string]peer.FileTransfer),
		SearchManager:        searchManager,
		DownloadManager:      downloadManager,
		logger:               logger,
		shares:               shares,
	}

	client.PeerManager = peer.NewPeerManager(username, distributedsearchCh, transferReqCh, shares, searchManager, downloadManager, logger)
	return client
}

// Connect to soulseek server and login
func (c *SlskClient) Connect(username, pw string) error {
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
	listener, err := net.Listen("tcp", ":2234")
	if err != nil {
		return errors.New("unable to listen on port 2234; " + err.Error())
	}
	c.ServerConnection = &nc.Connection{Conn: conn}
	c.Listener = listener

	c.User = username

	go c.ListenForServerMessages()
	go c.ListenForIncomingPeers()

	c.Login(username, pw) // see c.HandleLogin() for response handling
	c.SetWaitPort(2234)
	c.logger.Info("Listening on port 2234")

	go c.listenForDistribSearches()
	go c.listenForTransferRequests()

	// c.JoinRoom("nicotine")
	// c.JoinRoom("The Lobby")
	return nil
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

func (c *SlskClient) AddPendingPeer(username string, token uint32, connType string, privileged uint8) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, found := c.PendingUsers[username]
	if found {
		c.logger.Warn("Peer connection already pending",
			"token", token,
			"username", username,
		)
		return
	}
	connInfo := PendingTokenConn{username: username, connType: connType, token: token, privileged: privileged}
	c.PendingUsers[username] = connInfo
	c.PendingTokens[token] = connInfo
}

func (c *SlskClient) GetPendingPeer(username string) (PendingTokenConn, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	pendingToken, found := c.PendingUsers[username]
	if !found {
		return PendingTokenConn{}, false // shitty workaround but prolly fine for now
	}
	return pendingToken, true
}

func (c *SlskClient) RemovePendingPeer(username string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	connInfo := c.PendingUsers[username]
	delete(c.PendingUsers, username)
	delete(c.PendingTokens, connInfo.token)
}

// messages get sent to DistribSearchCh when distributed peers send us queries
func (c *SlskClient) listenForDistribSearches() {
	for msg := range c.DistribSearchCh {
		if len(msg.Query) < 4 {
			continue
		}
		res := c.shares.Search(msg.Query)
		if len(res) > 0 {
			c.logger.Info("distributed search match", "query", msg.Query, "username", msg.Username)
			if peer := c.PeerManager.GetPeer(msg.Username); peer != nil {
				peer.FileSearchResponse(msg.Username, msg.Token, res)
			} else {
				// store the search result
				// when we get a response for peer address, we will try sending search result
				c.DistribSearchResults[msg.Username] = append(
					c.DistribSearchResults[msg.Username],
					distribSearchResult{token: msg.Token, files: res})

				c.logger.Info("added distrib search result", "username", msg.Username, "token", msg.Token, "results", res)
				c.ConnectToPeer(msg.Username, "P", msg.Token)
			}
		}
	}
}

// messages get sent to TransferRequestCh when peers request a file from us
func (c *SlskClient) listenForTransferRequests() {
	for msg := range c.TransferReqCh {
		key := fmt.Sprintf("%s_%d", msg.PeerUsername, msg.Token)
		c.PendingTransferReqs[key] = msg
		c.logger.Info("attempting to start file transfer", "transfer", msg)
		c.AddPendingPeer(msg.PeerUsername, msg.Token, "F", 0)
		c.ConnectToPeer(msg.PeerUsername, "F", msg.Token)
	}
}
