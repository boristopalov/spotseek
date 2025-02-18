package peer

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"spotseek/src/slsk/messages"
	listen "spotseek/src/slsk/network"
	"time"
)

type Event int

const (
	PeerConnected Event = iota
	PeerDisconnected
	FileSearchResponse
	CantConnectToPeer
)

type BasePeer interface {
	ReadMessage() ([]byte, error)
	SendMessage([]byte) error
}

type Peer struct {
	Username       string
	SlskListener   net.Listener
	PeerConnection *listen.Connection
	ConnType       string
	Token          uint32
	Host           string
	Port           uint32
	EventEmitter   chan<- PeerEvent
}

type PeerEvent struct {
	Type Event
	Peer *Peer
}

func (p *Peer) SendMessage(msg []byte) error {
	return p.PeerConnection.SendMessage(msg)
}

func newPeer(username string, connType string, token uint32, host string, port uint32, eventEmitter chan<- PeerEvent) (*Peer, error) {
	c, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("unable to establish connection to peer %s: %v", username, err)
	} else {
		log.Info("established TCP connection to peer", "username", username, "peerHost", host, "peerPort", port)
		return &Peer{
			Username:       username,
			PeerConnection: &listen.Connection{Conn: c},
			ConnType:       connType,
			Token:          token,
			Host:           host,
			Port:           port,
			EventEmitter:   eventEmitter,
		}, nil
	}
}

func (peer *Peer) ListenForMessages() {
	readBuffer := make([]byte, 4096)
	var currentMessage []byte
	var messageLength uint32

	defer func() {
		log.Warn("Stopped listening for messages from peer", "username", peer.Username)
		peer.ClosePeer()
		peer.EventEmitter <- PeerEvent{Type: PeerDisconnected, Peer: peer}
	}()

	for {
		n, err := peer.PeerConnection.Read(readBuffer)
		if err != nil {
			if err == io.EOF {
				log.Error("Error reading peer message; peer closed the connection", "username", peer.Username)
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				log.Error("Timeout reading from peer. retrying...", "username", peer.Username)
				continue
			}
			log.Error("Error reading from peer %s: %v", "username", peer.Username, "err", err)
			return
		}

		currentMessage = append(currentMessage, readBuffer[:n]...)
		currentMessage = peer.processMessage(currentMessage, &messageLength)
	}
}

func (peer *Peer) ClosePeer() {
	peer.PeerConnection.Close()

}

func (peer *Peer) processMessage(data []byte, messageLength *uint32) []byte {
	for {
		if *messageLength == 0 {
			if len(data) < 4 {
				return data // Not enough data to read message length
			}
			*messageLength = binary.LittleEndian.Uint32(data[:4])
			data = data[4:]
		}

		if uint32(len(data)) < *messageLength {
			return data // Not enough data for full message
		}

		message := data[:*messageLength]
		mr := messages.PeerMessageReader{MessageReader: messages.NewMessageReader(message)}
		msg, err := mr.HandlePeerMessage()
		if err != nil {
			log.Error("Error reading message from peer", "username", peer.Username, "err", err)
		} else {
			log.Info("Message from peer", "username", peer.Username, "message", msg)
			// TODO: Search Manager
			// if msg["type"] == "FileSearchResponse" {
			// 	peer.EventEmitter <- PeerEvent{Type: FileSearchResponse, Peer: peer}
			// 	// c.fileMutex.Lock()
			// 	// c.SearchResults[msg["token"].(uint32)] = msg["results"].([]shared.SearchResult)
			// 	// c.fileMutex.Unlock()
			// }
		}

		data = data[*messageLength:]
		*messageLength = 0
	}
}
