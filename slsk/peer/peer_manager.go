package peer

import (
	"fmt"
	"log/slog"
	"net"
	"spotseek/slsk/fileshare"
	"spotseek/slsk/messages"
	"spotseek/slsk/shared"
	"sync"
	"time"
)

type PeerManager struct {
	username        string
	peers           map[string]*Peer // username --> Peer info
	mu              sync.RWMutex
	peerCh          chan PeerEvent
	distribSearchCh chan DistribSearchMessage
	children        []*DistributedPeer
	SearchResults   map[uint32][]shared.SearchResult // token --> search results
	SearchRequests  map[string][]shared.SearchResult // peer username --> incoming distributed search requests
	transferReqCh   chan FileTransfer                // used when peers request files from us
	logger          *slog.Logger
	shares          *fileshare.Shared
}

func NewPeerManager(username string, distribSearchCh chan DistribSearchMessage, transferReqCh chan FileTransfer, shares *fileshare.Shared, logger *slog.Logger) *PeerManager {
	m := &PeerManager{
		username:        username,
		peers:           make(map[string]*Peer),
		peerCh:          make(chan PeerEvent),
		distribSearchCh: distribSearchCh,
		transferReqCh:   transferReqCh,
		children:        make([]*DistributedPeer, 0),
		SearchResults:   make(map[uint32][]shared.SearchResult),
		logger:          logger,
		shares:          shares,
	}
	go m.listenForEvents()
	go m.listenForDistribMessages()
	return m
}

func (manager *PeerManager) RemovePeer(peer *Peer) {
	manager.mu.Lock()
	delete(manager.peers, peer.Username)
	manager.mu.Unlock()
}

func (manager *PeerManager) ConnectToPeerDistrib(ownUser string, host string, port uint32, peerUsername string, connType string, token uint32, privileged uint8) error {
	peer := manager.AddPeer(peerUsername, connType, host, port, token, privileged)
	if peer == nil {
		return fmt.Errorf("cannot connect to user %s with connType %s", peerUsername, connType)
	}

	peer.PeerInit(ownUser, peer.ConnType, peer.Token)
	peer.PierceFirewall(peer.Token)
	go peer.Listen()
	return nil
}

func (manager *PeerManager) ConnectToPeer(host string, port uint32, username string, connType string, token uint32, privileged uint8) error {
	peer := manager.AddPeer(username, connType, host, port, token, privileged)
	if peer == nil {
		return fmt.Errorf("cannot connect to user %s with connType %s", username, connType)
	}

	if peer.ConnType == "F" {
		filePeer := NewFileTransferPeer(peer)
		peer.PierceFirewall(peer.Token)
		go filePeer.FileListen()
		manager.logger.Info("Listening for file transfer", "peer", peer.Username)
		return nil
	}

	peer.PierceFirewall(peer.Token)
	go peer.Listen()
	manager.logger.Info("Connected to peer", "username", username)
	return nil
}

func (manager *PeerManager) GetPeer(username string) *Peer {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	peer, exists := manager.peers[username]
	if exists {
		return peer
	}
	return nil
}

func (manager *PeerManager) AddPeer(username string, connType string, ip string, port uint32, token uint32, privileged uint8) *Peer {
	manager.mu.RLock()
	peer, exists := manager.peers[username]
	manager.mu.RUnlock()

	if exists {
		if peer.ConnType == connType {
			manager.logger.Warn("peer already connected", "peer", peer)
			return nil
		}

		peer.mu.Lock()
		peer.ConnType = connType
		peer.Token = token
		peer.mu.Unlock()

		// close the existing connection
		peer.Close()
		// open a new connection
		c, err := net.DialTimeout("tcp", net.JoinHostPort(ip, fmt.Sprintf("%d", port)), 10*time.Second)
		if err != nil {
			return nil
		}
		peer.mu.Lock()
		peer.PeerConnection = &shared.Connection{Conn: c}
		peer.mu.Unlock()
		return peer
	}

	peer, err := newPeer(username, connType, token, ip, port, privileged, manager.peerCh, manager.distribSearchCh, manager.logger)
	if err != nil {
		return nil
	}

	// only update the peer if it doesn't exist
	// that way we don't lose track of any pending transfers
	manager.mu.Lock()
	manager.peers[username] = peer
	manager.mu.Unlock()
	return peer
}

func newPeer(username string, connType string, token uint32, host string, port uint32, privileged uint8, peerCh chan<- PeerEvent, distribSearchCh chan<- DistribSearchMessage, logger *slog.Logger) (*Peer, error) {
	logger.Info("Connecting to peer via TCP", "username", username, "host", host, "port", port)
	c, err := net.DialTimeout("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)), 10*time.Second)
	if err != nil {
		logger.Error("unable to establish connection to peer", "username", username, "error", err)
		return nil, fmt.Errorf("unable to establish connection to peer %s: %v", username, err)
	} else {
		return &Peer{
			Username:        username,
			PeerConnection:  &shared.Connection{Conn: c},
			ConnType:        connType,
			Token:           token,
			Host:            host,
			Port:            port,
			Privileged:      privileged,
			mgrCh:           peerCh,
			distribSearchCh: distribSearchCh,
			fileTransferCh:  make(chan struct{}),
			logger:          logger,
		}, nil
	}
}

func (peer *Peer) PeerInit(username string, connType string, token uint32) error {

	mb := messages.PeerInitMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.PeerInit(username, connType, token)
	err := peer.SendMessage(msg)
	return err
}

func (peer *Peer) PierceFirewall(token uint32) error {
	mb := messages.PeerInitMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.PierceFirewall(token)
	err := peer.SendMessage(msg)
	return err
}

func (manager *PeerManager) listenForDistribMessages() {
	for msg := range manager.distribSearchCh {
		// send to client
		manager.distribSearchCh <- msg

		// forward to children
		mb := messages.NewMessageBuilder()
		mb.AddInt32(1) // unknown
		mb.AddString(msg.Username)
		mb.AddInt32(msg.Token)
		mb.AddString(msg.Query)
		data := mb.Build(3)
		for _, child := range manager.children {
			child.PeerConnection.SendMessage(data)
		}
	}
}

// TODO: complete handling of PeerEvents
func (manager *PeerManager) listenForEvents() {
	for event := range manager.peerCh {
		switch event.Type {
		case PeerDisconnected:
			event.Peer.Close()
			manager.RemovePeer(event.Peer)

		// we are ready to start uploading a file
		// client will send a new ConnectToPeer with type "F"
		// see client.listenForTransferRequests()
		case TransferRequest:
			msg := event.Data.(TransferRequestMessage)
			manager.transferReqCh <- FileTransfer{
				Filename:     msg.Filename,
				Token:        msg.Token,
				Size:         msg.Size,
				PeerUsername: msg.PeerUsername,
				Offset:       0,
				Buffer:       nil,
			}
			// idk about this
			event.Peer.Close()
			// event.Peer.PeerInit(manager.username, "F", msg.Token)

		case SharedFileListRequest:
			event.Peer.SharedFileListResponse(manager.shares)

		// peer wants to download a file
		case UploadRequest:
			msg := event.Data.(UploadRequestMessage)
			res := manager.shares.Search(msg.Filename)
			if len(res) > 0 {
				event.Peer.TransferRequest(1, res[0].Value.Path, uint64(res[0].Value.Size))
			} else {
				event.Peer.UploadDenied(msg.Filename, "File not shared.")
			}

		// incoming file search response
		case FileSearchResponse:
			msg := event.Data.(FileSearchData)
			manager.SearchResults[msg.Token] =
				append(manager.SearchResults[msg.Token], msg.Results)
			// manager.clientCh <- event

		case UploadComplete:
			manager.RemovePeer(event.Peer)
			event.Peer.Close()

		case BranchLevel:
			msg := event.Data.(BranchLevelMessage)
			if msg.BranchLevel == 0 {
				// TODO
			}
			mb := messages.NewMessageBuilder()
			mb.AddInt32(msg.BranchLevel)
			data := mb.Build(4)
			for _, child := range manager.children {
				child.PeerConnection.SendMessage(data)
			}
		case BranchRoot:
			msg := event.Data.(BranchRootMessage)
			mb := messages.NewMessageBuilder()
			mb.AddString(msg.BranchRootUsername)
			data := mb.Build(5)
			for _, child := range manager.children {
				child.PeerConnection.SendMessage(data)
			}
		}
	}
}
