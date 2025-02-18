package client

import (
	"errors"
	"fmt"
	"net"
	"spotseek/logging"
	"spotseek/src/slsk/peer"
	"spotseek/src/slsk/shared"
	"sync"
)

var log = logging.GetLogger()

type IP struct {
	IP   string
	port uint32
}

type PendingTokenConn struct {
	username string
	connType string
}

type SlskClient struct {
	Host                   string
	Port                   int
	ServerConnection       *shared.Connection
	Listener               net.Listener
	mu                     sync.RWMutex
	fileMutex              sync.RWMutex
	User                   string                      // the user that is logged in
	PendingPeerConnections map[uint32]PendingTokenConn // token --> connType
	TokenSearches          map[uint32]string
	ConnectionToken        uint32
	SearchToken            uint32
	SearchResults          map[uint32][]shared.SearchResult // token --> search results
	DownloadQueue          map[string]*Transfer
	UploadQueue            map[string]*Transfer
	TransferListeners      []TransferListener
	JoinedRooms            map[string][]string // room name --> users in room
	// DistributedNetworkManager *network.DistributedNetworkManager
	PeerManager *peer.PeerManager
	// FileTransferManager       *network.FileTransferManager
	// RoomManager               *rooms.RoomManager
}

type Transfer struct {
	Username string
	Filename string
	Size     int64
	Progress int64
	Status   string
}

type TransferListener func(transfer *Transfer)

func NewSlskClient(host string, port int) *SlskClient {
	return &SlskClient{
		Host:                   host,
		Port:                   port,
		ConnectionToken:        0,
		SearchToken:            0,
		DownloadQueue:          make(map[string]*Transfer),
		UploadQueue:            make(map[string]*Transfer),
		TransferListeners:      make([]TransferListener, 0),
		SearchResults:          make(map[uint32][]shared.SearchResult),
		TokenSearches:          make(map[uint32]string),
		PendingPeerConnections: make(map[uint32]PendingTokenConn),
		// PendingUsernameConnTypes: make(map[string]string),
		// UsernameIps: make(map[string]IP),
		// JoinedRooms:              make(map[string][]string),
		// DistributedNetwork:       network.NewDistributedNetwork(),
		PeerManager: peer.NewPeerManager(make(chan peer.PeerEvent)),
		// TransferManager: transfers.NewTransferManager(),
		// SearchManager:   search.NewSearchManager(),
		// RoomManager: rooms.NewRoomManager()
	}
}

// Connect to soulseek server and login
func (c *SlskClient) Connect(username, pw string) error {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port))
	if err != nil {
		return errors.New("unable to dial tcp connection; " + err.Error())
	}
	listener, err := net.Listen("tcp", ":2234")
	if err != nil {
		return errors.New("unable to listen on port 2234; " + err.Error())
	}
	c.ServerConnection = &shared.Connection{Conn: conn}
	c.Listener = listener
	go c.ListenForServerMessages()
	go c.ListenForIncomingPeers()
	c.Login(username, pw)
	c.SetWaitPort(2234)
	log.Info("Established connection to Soulseek server")
	log.Info("Listening on port 2234")
	c.User = username
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
