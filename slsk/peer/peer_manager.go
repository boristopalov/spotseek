package peer

import (
	"fmt"
	"net"
	"spotseek/logging"
	"spotseek/slsk/messages"
	"spotseek/slsk/shared"
	"sync"
	"time"
)

var log = logging.GetLogger()

type peerKey = string

// Helper function to create peer key
func makePeerKey(username, connType string) peerKey {
	return username + "|" + connType
}

type PeerManager struct {
	peers         map[peerKey]*Peer // username|connType --> Peer info
	mu            sync.RWMutex
	eventEmitter  chan<- PeerEvent
	eventListener <-chan PeerEvent
}

func NewPeerManager(eventChan chan PeerEvent) *PeerManager {
	m := &PeerManager{
		peers:         make(map[peerKey]*Peer),
		eventEmitter:  eventChan,
		eventListener: eventChan,
	}
	go m.listenForEvents()
	return m
}

func (manager *PeerManager) RemovePeer(peer *Peer) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	key := makePeerKey(peer.Username, peer.ConnType)
	delete(manager.peers, key)
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

	switch connType {
	case "P":
		go peer.ListenForPeerMessages()
	case "D":
		go peer.ListenForDistributedMessages()
	case "F":
		go peer.ListenForFileTransferMessages()
	}
	return nil
}

func (manager *PeerManager) GetPeer(username string, connType string) *Peer {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	key := makePeerKey(username, connType)
	peer, exists := manager.peers[key]
	if exists {
		return peer
	}
	return nil
}

func (manager *PeerManager) AddPeer(username string, connType string, ip string, port uint32, token uint32, privileged uint8) *Peer {
	key := makePeerKey(username, connType)
	manager.mu.RLock()
	peer, exists := manager.peers[key]
	manager.mu.RUnlock()

	if exists {
		log.Warn("peer already connected", "peer", peer)
		return nil
	}

	peer, err := newPeer(username, connType, token, ip, port, privileged, manager.eventEmitter)
	if err != nil {
		return nil
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.peers[key] = peer
	log.Info("connected to peer", "peer", peer)
	return peer
}

func newPeer(username string, connType string, token uint32, host string, port uint32, privileged uint8, eventEmitter chan<- PeerEvent) (*Peer, error) {
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
