package peer

import (
	"fmt"
	"spotseek/logging"
	"spotseek/src/config"
	"sync"
)

var log = logging.GetLogger()

type PeerManager struct {
	peers         map[string]*Peer  // username --> Peer info
	pendingPeers  map[string]string // username --> connType
	mu            sync.RWMutex
	eventEmitter  chan<- PeerEvent
	eventListener <-chan PeerEvent
}

func NewPeerManager(eventChan chan PeerEvent) *PeerManager {
	m := &PeerManager{
		peers:         make(map[string]*Peer),
		pendingPeers:  make(map[string]string),
		eventEmitter:  eventChan,
		eventListener: eventChan,
	}
	go m.listenForEvents()
	return m
}

func (manager *PeerManager) RemovePeer(peer *Peer) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	delete(manager.peers, peer.Username)
}

func (manager *PeerManager) AddPeer(peer *Peer) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.peers[peer.Username] = peer
}

func (manager *PeerManager) ConnectToPeer(ip string, port uint32, username, connType string, token uint32) error {
	peer := manager.GetPeer(username)
	if peer != nil && connType == peer.ConnType {
		log.Warn("already connected to peer",
			"peer", username,
			"connType", connType,
		)
		return nil
	}

	pendingConnType := manager.GetPendingPeer(username)
	if pendingConnType == "" {
		return fmt.Errorf("no pending connection for %s", username)
	}
	peer, err := newPeer(username, pendingConnType, token, ip, port, manager.eventEmitter)
	if err != nil {
		return err
	}
	manager.AddPeer(peer)
	manager.RemovePendingPeer(username)
	// Step 2: Send Peer Init
	err = peer.PeerInit(config.SOULSEEK_USERNAME, pendingConnType, token)
	if err != nil {
		return err
	}
	go peer.ListenForMessages()
	return nil
}

func (manager *PeerManager) listenForEvents() {
	for event := range manager.eventListener {
		switch event.Type {
		case PeerConnected:
		case PeerDisconnected:
			manager.RemovePeer(event.Peer)
		}
	}
}

func (manager *PeerManager) GetPeer(username string) *Peer {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	peer, ok := manager.peers[username]
	if !ok {
		return nil
	}
	return peer
}

func (manager *PeerManager) AddPendingPeer(username, connType string) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.pendingPeers[username] = connType
}

func (manager *PeerManager) RemovePendingPeer(username string) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	delete(manager.pendingPeers, username)
}

func (manager *PeerManager) GetPendingPeer(username string) string {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	connType, ok := manager.pendingPeers[username]
	if !ok {
		return ""
	}
	return connType
}

func (manager *PeerManager) GetNumConnections() int {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return len(manager.peers)
}
