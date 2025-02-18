package peer

import (
	"fmt"
	"net"
	"spotseek/config"
	"spotseek/logging"
	"spotseek/slsk/messages"
	"spotseek/slsk/shared"
	"sync"
	"time"
)

var log = logging.GetLogger()

type PeerManager struct {
	peers         map[string]*Peer // username --> Peer info
	mu            sync.RWMutex
	eventEmitter  chan<- PeerEvent
	eventListener <-chan PeerEvent
}

func NewPeerManager(eventChan chan PeerEvent) *PeerManager {
	m := &PeerManager{
		peers:         make(map[string]*Peer),
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

// for outgoing connection attempts
func (manager *PeerManager) ConnectToPeer(ip string, port uint32, username, connType string, token uint32) error {
	peer := manager.GetPeer(username)
	if peer != nil && connType == peer.ConnType {
		log.Warn("already connected to peer",
			"peer", username,
			"connType", connType,
		)
		return nil
	}

	err := peer.PeerInit(config.SOULSEEK_USERNAME, connType, token)
	if err != nil {
		return err
	}

	manager.AddPeer(peer.Username, peer.ConnType, peer.Host, peer.Port, peer.Token, peer.Privileged)
	go peer.ListenForMessages()
	return nil
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

func (manager *PeerManager) AddPeer(username string, connType string, ip string, port uint32, token uint32, privileged uint8) *Peer {
	peer := manager.GetPeer(username)
	if peer != nil && peer.ConnType == connType {
		log.Warn("peer already connected", "peer", peer)
		return nil
	}
	peer, err := NewPeer(username, connType, token, ip, port, privileged, manager.eventEmitter)
	if err != nil {
		return nil
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.peers[username] = peer
	return peer
}

func (manager *PeerManager) GetNumConnections() int {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return len(manager.peers)
}

func NewPeer(username string, connType string, token uint32, host string, port uint32, privileged uint8, eventEmitter chan<- PeerEvent) (*Peer, error) {
	c, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 10*time.Second)
	if err != nil {
		log.Error("failed to connect to peer",
			"username", username,
			"connType", connType,
			"token", token,
			"host", host,
			"port", port,
			"privileged", privileged,
			"err", err,
		)
		return nil, fmt.Errorf("unable to establish connection to peer %s: %v", username, err)
	} else {
		log.Info("established TCP connection to peer", "username", username, "peerHost", host, "peerPort", port)
		return &Peer{
			Username:       username,
			PeerConnection: &shared.Connection{Conn: c},
			ConnType:       connType,
			Token:          token,
			Host:           host,
			Port:           port,
			Privileged:     privileged,
			EventEmitter:   eventEmitter,
		}, nil
	}
}

func (mgr *PeerManager) HandleNewPeer(username string, connType string, ip string, port uint32, token uint32, privileged uint8) error {
	peer := mgr.AddPeer(username, connType, ip, port, token, privileged)
	if peer == nil {
		return fmt.Errorf("Failed to connect to peer")
	}
	err := peer.PierceFirewall(peer.Token)
	if err != nil {
		log.Error("Failed to send PierceFirewall",
			"peer", peer,
			"err", err,
		)
	}
	log.Info("Connected to peer", "peer", peer)
	go peer.ListenForMessages()
	return nil
}

func (peer *Peer) PeerInit(username string, connType string, token uint32) error {

	mb := messages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.PeerInit(username, connType, token)
	err := peer.SendMessage(msg)
	return err
}

func (peer *Peer) PierceFirewall(token uint32) error {
	mb := messages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.PierceFirewall(token)
	err := peer.SendMessage(msg)
	return err
}

func (manager *PeerManager) listenForEvents() {
	for event := range manager.eventListener {
		switch event.Type {
		case PeerConnected:
		case PeerDisconnected:
			event.Peer.ClosePeer()
			manager.RemovePeer(event.Peer)
		}
	}
}
