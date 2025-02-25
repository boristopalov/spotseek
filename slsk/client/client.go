package client

import (
	"errors"
	"fmt"
	"net"
	"spotseek/config"
	"spotseek/logging"
	"spotseek/slsk/fileshare"
	"spotseek/slsk/peer"
	"spotseek/slsk/shared"
	"sync"
	"time"
)

var log = logging.GetLogger()

type IP struct {
	IP   string
	port uint32
}

type PendingTokenConn struct {
	username   string
	connType   string
	privileged uint8
	token      uint32
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

type SlskClient struct {
	Host                                string
	Port                                int
	ServerConnection                    *shared.Connection
	Listener                            net.Listener
	mu                                  sync.RWMutex
	User                                string                      // the user that is logged in
	PendingOutgoingPeerConnections      map[string]PendingTokenConn // username --> connection info
	PendingOutgoingPeerConnectionTokens map[uint32]PendingTokenConn // token --> connection info
	ConnectionToken                     uint32
	SearchToken                         uint32
	SearchResults                       map[uint32][]shared.SearchResult // token --> search results
	JoinedRooms                         map[string]*Room                 // room name --> users in room
	PeerManager                         *peer.PeerManager
	FileManager                         *fileshare.FileShareManager
	peerEventCh                         chan peer.PeerEvent
}

func NewSlskClient(host string, port int) *SlskClient {
	return &SlskClient{
		Host:                                host,
		Port:                                port,
		ConnectionToken:                     42,
		SearchToken:                         42,
		SearchResults:                       make(map[uint32][]shared.SearchResult),
		PendingOutgoingPeerConnections:      make(map[string]PendingTokenConn),
		PendingOutgoingPeerConnectionTokens: make(map[uint32]PendingTokenConn),
		JoinedRooms:                         make(map[string]*Room),
	}
}

// Connect to soulseek server and login
func (c *SlskClient) Connect(username, pw string) error {
	// Set up our shared files
	shares := fileshare.NewShared(config.GetSettings())
	stats := shares.GetShareStats()
	log.Info("share stats", "stats", stats)

	dialer := &net.Dialer{
		KeepAlive: 120 * time.Second,
	}
	conn, err := dialer.Dial("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port))
	if err != nil {
		return errors.New("unable to dial tcp connection; " + err.Error())
	}
	listener, err := net.Listen("tcp", ":2234")
	if err != nil {
		return errors.New("unable to listen on port 2234; " + err.Error())
	}
	c.ServerConnection = &shared.Connection{Conn: conn}
	c.Listener = listener

	c.Login(username, pw)
	c.SetWaitPort(2234)
	log.Info("Established connection to Soulseek server")
	log.Info("Listening on port 2234")
	c.User = username

	c.SharedFoldersFiles(stats.TotalFolders, stats.TotalFiles)

	// set up fileshare manager
	c.FileManager = fileshare.NewFileShareManager(shares)

	// Initialize peer event channel
	peerEventCh := make(chan peer.PeerEvent)
	c.peerEventCh = peerEventCh
	c.PeerManager = peer.NewPeerManager(peerEventCh)

	go c.ListenForServerMessages() // server messages sent to client
	go c.ListenForIncomingPeers()  // peer init

	go c.listenForPeerEvents() // peer manager events sent to client

	go func() {
		c.HaveNoParent(1)
		time.Sleep(2 * time.Second)
		c.FileSearch("norbak clima")
	}()

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
	log.Info("Connection closed")
	return nil
}

func (c *SlskClient) AddPendingPeer(token uint32, username string, connType string, privileged uint8) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, found := c.PendingOutgoingPeerConnections[username]
	if found {
		log.Warn("Peer connection already pending",
			"token", token,
			"username", username,
			"connType", connType,
		)
		return
	}
	log.Info("Pending peer connection added",
		"token", token,
		"username", username,
		"connType", connType,
	)
	c.PendingOutgoingPeerConnections[username] = PendingTokenConn{username: username, connType: connType, token: token, privileged: privileged}
	c.PendingOutgoingPeerConnectionTokens[token] = PendingTokenConn{username: username, connType: connType, token: token, privileged: privileged}
}

func (c *SlskClient) RemovePendingPeer(username string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.PendingOutgoingPeerConnections, username)
}

func (c *SlskClient) listenForPeerEvents() {
	for event := range c.peerEventCh {
		switch event.Type {
		case peer.PeerDisconnected:
			log.Info("Peer disconnected event received",
				"peer", event.Peer.Username)
			// Clean up any pending connections
			c.RemovePendingPeer(event.Peer.Username)

		case peer.PeerConnected:
			log.Info("Peer connected event received",
				"peer", event.Peer.Username)
			// Handle new peer connections if needed

		default:
			log.Warn("Unknown peer event received",
				"type", event.Type,
				"peer", event.Peer.Username)
		}
	}
}
