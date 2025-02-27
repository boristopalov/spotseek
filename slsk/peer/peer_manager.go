package peer

import (
	"fmt"
	"log/slog"
	"net"
	"spotseek/slsk/messages"
	"spotseek/slsk/shared"
	"sync"
	"time"
)

type PeerManager struct {
	peers         map[string]*Peer // username --> Peer info
	mu            sync.RWMutex
	peerCh        chan PeerEvent
	clientCh      chan<- PeerEvent
	children      []*DistributedPeer
	SearchResults map[uint32][]shared.SearchResult // token --> search results
	logger        *slog.Logger
}

func NewPeerManager(eventChan chan<- PeerEvent, logger *slog.Logger) *PeerManager {
	m := &PeerManager{
		peers:         make(map[string]*Peer),
		peerCh:        make(chan PeerEvent),
		clientCh:      eventChan,
		children:      make([]*DistributedPeer, 0),
		SearchResults: make(map[uint32][]shared.SearchResult),
		logger:        logger,
	}
	go m.listenForEvents()
	return m
}

func (manager *PeerManager) RemovePeer(peer *Peer) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	delete(manager.peers, peer.Username)
}

// for outgoing connection attempts
func (manager *PeerManager) ConnectToPeer(host string, port uint32, username string, connType string, token uint32, privileged uint8) error {
	peer := manager.AddPeer(username, connType, host, port, token, privileged)
	if peer == nil {
		return fmt.Errorf("cannot connect to user %s with connType %s", username, connType)
	}

	err := peer.PierceFirewall(peer.Token)
	if err != nil {
		return err
	}

	go peer.Listen()
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
		peer.ConnType = connType
		return peer
	}

	peer, err := newPeer(username, connType, token, ip, port, privileged, manager.peerCh, manager.logger)
	if err != nil {
		return nil
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.peers[username] = peer
	manager.logger.Info("connected to peer", "peer", peer)
	return peer
}

func newPeer(username string, connType string, token uint32, host string, port uint32, privileged uint8, peerCh chan<- PeerEvent, logger *slog.Logger) (*Peer, error) {
	c, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("unable to establish connection to peer %s: %v", username, err)
	} else {
		return &Peer{
			Username:       username,
			PeerConnection: &shared.Connection{Conn: c},
			ConnType:       connType,
			Token:          token,
			Host:           host,
			Port:           port,
			Privileged:     privileged,
			mgrCh:          peerCh,
			downloadFileCh: make(chan struct{}),
			logger:         logger,
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

// TODO: complete handling of PeerEvents
func (manager *PeerManager) listenForEvents() {
	for event := range manager.peerCh {
		switch event.Type {
		case PeerDisconnected:
			manager.logger.Warn("Stopped listening for messages from peer",
				"peer", event.Peer.Username)
			event.Peer.Close()
			manager.mu.Lock()
			manager.RemovePeer(event.Peer)
			manager.mu.Unlock()
		case FileSearchResponse:
			msg := event.Data.(FileSearchData)
			manager.mu.Lock()
			manager.SearchResults[msg.Token] =
				append(manager.SearchResults[msg.Token], msg.Results)
			manager.mu.Unlock()

			manager.clientCh <- event

		// Distributed Messages
		case DistribSearch:
			msg := event.Data.(DistribSearchMessage)
			mb := messages.NewMessageBuilder()
			mb.AddInt32(1) // unknown
			mb.AddString(msg.Username)
			mb.AddInt32(msg.Token)
			mb.AddString(msg.Query)
			data := mb.Build(3)
			for _, child := range manager.children {
				child.PeerConnection.SendMessage(data)
			}
		case BranchLevel:
			msg := event.Data.(BranchLevelMessage)
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
