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

type PeerManager struct {
	peers    map[string]*Peer // username --> Peer info
	mu       sync.RWMutex
	peerCh   chan PeerEvent
	clientCh chan<- PeerEvent
}

func NewPeerManager(eventChan chan<- PeerEvent) *PeerManager {
	m := &PeerManager{
		peers:    make(map[string]*Peer),
		peerCh:   make(chan PeerEvent),
		clientCh: eventChan,
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

func (manager *PeerManager) GetPeer(username string, connType string) *Peer {
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
		log.Warn("peer already connected", "peer", peer)
		return nil
	}

	peer, err := newPeer(username, connType, token, ip, port, privileged, manager.peerCh)
	if err != nil {
		return nil
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.peers[username] = peer
	log.Info("connected to peer", "peer", peer)
	return peer
}

func newPeer(username string, connType string, token uint32, host string, port uint32, privileged uint8, peerCh chan<- PeerEvent) (*Peer, error) {
	c, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 10*time.Second)
	if err != nil {
		log.Error("failed to connect to peer",
			"username", username,
			"connType", connType,
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
			mgrCh:          peerCh,
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
	for event := range manager.peerCh {
		switch event.Type {
		case PeerConnected:
		case PeerDisconnected:
			log.Warn("Stopped listening for messages from peer",
				"peer", event.Peer.Username)
			event.Peer.ClosePeer()
			manager.RemovePeer(event.Peer)
		}
	}
}
