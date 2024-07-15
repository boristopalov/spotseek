package client

import (
	"net"
	"spotseek/src/slsk/listen"
	"spotseek/src/slsk/peer"
	"spotseek/src/slsk/shared"
	"sync"
)

type IP struct {
	IP   string
	port uint32
}

type PendingTokenConn struct {
	username string
	connType string
}

type SlskClient struct {
	Host                     string
	Port                     int
	ServerConnection         *listen.Connection
	Listener                 net.Listener
	ConnectedPeers           map[string]peer.Peer // username --> Peer
	mu                       sync.RWMutex
	fileMutex                sync.RWMutex
	User                     string               // the user that is logged in
	UsernameIps              map[string]IP        // username -> IP address
	PendingPeerInits         map[string]peer.Peer // username -> Peer
	PendingUsernameConnTypes map[string]string
	PendingTokenConnTypes    map[uint32]PendingTokenConn // token --> connType
	TokenSearches            map[uint32]string
	ConnectionToken          uint32
	SearchToken              uint32
	SearchResults            map[uint32][]shared.SearchResult // token --> search results
	DownloadQueue            map[string]*Transfer
	UploadQueue              map[string]*Transfer
	TransferListeners        []TransferListener
	JoinedRooms              map[string][]string // room name --> users in room
}

type Transfer struct {
	Username string
	Filename string
	Size     int64
	Progress int64
	Status   string
}

type TransferListener func(transfer *Transfer)
