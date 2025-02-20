package peer

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"spotseek/slsk/messages"
	"spotseek/slsk/shared"
)

type Event int

const (
	PeerConnected Event = iota
	PeerDisconnected
	FileSearchResponse
)

type BasePeer interface {
	ReadMessage() ([]byte, error)
	SendMessage([]byte) error
}

type Peer struct {
	Username       string             `json:"username"`
	SlskListener   net.Listener       `json:"-"` // Skip in JSON
	PeerConnection *shared.Connection `json:"-"` // Skip in JSON
	ConnType       string             `json:"connType"`
	Token          uint32             `json:"token"`
	Host           string             `json:"host"`
	Port           uint32             `json:"port"`
	Privileged     uint8              `json:"privileged"`
	EventEmitter   chan<- PeerEvent   `json:"-"` // Skip in JSON
}

type PeerEvent struct {
	Type Event
	Peer *Peer
}

func (p *Peer) SendMessage(msg []byte) error {
	if p == nil {
		return fmt.Errorf("Tried to send message to peer but peer is nil!")
	}
	if p.PeerConnection == nil {
		return fmt.Errorf("Cannot send message to peer. No active connection")
	}
	return p.PeerConnection.SendMessage(msg)
}

func (peer *Peer) ListenForPeerMessages() {
	readBuffer := make([]byte, 4096)
	var currentMessage []byte
	var messageLength uint32

	defer func() {
		log.Warn("Stopped listening for messages from peer", "peer", peer)
		peer.EventEmitter <- PeerEvent{Type: PeerDisconnected, Peer: peer}
	}()

	for {
		n, err := peer.PeerConnection.Read(readBuffer)
		if err != nil {
			if err == io.EOF {
				log.Error("Error reading peer message. Peer closed the connection", "peer", peer)
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				log.Error("Timeout reading from peer. retrying...", "", peer)
				continue
			}
			log.Error("Error reading peer message", "peer", peer, "err", err)
			return
		}

		currentMessage = append(currentMessage, readBuffer[:n]...)
		currentMessage = peer.processMessage(currentMessage, &messageLength)
	}
}

func (peer *Peer) ListenForDistributedMessages() {
	readBuffer := make([]byte, 4096)
	var currentMessage []byte
	var messageLength uint32

	defer func() {
		log.Warn("Stopped listening for messages from peer", "peer", peer)
		peer.EventEmitter <- PeerEvent{Type: PeerDisconnected, Peer: peer}
	}()

	for {
		n, err := peer.PeerConnection.Read(readBuffer)
		if err != nil {
			if err == io.EOF {
				log.Error("Error reading peer message. Peer closed the connection", "peer", peer)
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				log.Error("Timeout reading from peer. retrying...", "", peer)
				continue
			}
			log.Error("Error reading peer message", "peer", peer, "err", err)
			return
		}

		currentMessage = append(currentMessage, readBuffer[:n]...)
		currentMessage = peer.processMessage(currentMessage, &messageLength)
	}
}

func (peer *Peer) ListenForFileTransferMessages() {
	readBuffer := make([]byte, 4096)
	var currentMessage []byte
	var messageLength uint32

	defer func() {
		log.Warn("Stopped listening for messages from peer", "peer", peer)
		peer.EventEmitter <- PeerEvent{Type: PeerDisconnected, Peer: peer}
	}()

	for {
		n, err := peer.PeerConnection.Read(readBuffer)
		if err != nil {
			if err == io.EOF {
				log.Error("Error reading peer message. Peer closed the connection", "peer", peer)
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				log.Error("Timeout reading from peer. retrying...", "", peer)
				continue
			}
			log.Error("Error reading peer message", "peer", peer, "err", err)
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
		_, err := mr.HandlePeerMessage()
		if err != nil {
			log.Error("Error reading message from peer", "peer", peer, "err", err)
			return data
		}

		data = data[*messageLength:]
		*messageLength = 0
	}
}
