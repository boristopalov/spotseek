package client

import (
	"encoding/binary"
	"io"
	"log"
	"net"
	"spotseek/src/slsk/messages"
	"spotseek/src/slsk/peer"
	"spotseek/src/slsk/shared"
)

func (c *SlskClient) ListenForPeerMessages(p *peer.Peer) {
	readBuffer := make([]byte, 4096)
	var currentMessage []byte
	var messageLength uint32

	defer func() {
		log.Printf("Stopped listening for messages from peer %s", p.Username)
		p.PeerConnection.Close()
		c.mu.Lock()
		delete(c.ConnectedPeers, p.Username)
		c.mu.Unlock()
	}()

	for {
		n, err := p.PeerConnection.Read(readBuffer)
		if err != nil {
			if err == io.EOF {
				log.Printf("Peer %s closed the connection", p.Username)
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				log.Printf("Timeout reading from peer %s, retrying...", p.Username)
				continue
			}
			log.Printf("Error reading from peer %s: %v", p.Username, err)
			return
		}

		currentMessage = append(currentMessage, readBuffer[:n]...)
		currentMessage = c.processPeerMessages(p, currentMessage, &messageLength)
	}
}

func (c *SlskClient) processPeerMessages(p *peer.Peer, data []byte, messageLength *uint32) []byte {
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
			log.Printf("Error reading message from peer %s: %v", p.Username, err)
		} else {
			log.Printf("Message from peer %s: %v", p.Username, msg)
			if msg["type"] == "FileSearchResponse" {
				c.fileMutex.Lock()
				c.SearchResults[msg["token"].(uint32)] = msg["results"].([]shared.SearchResult)
				c.fileMutex.Unlock()
			}
		}

		data = data[*messageLength:]
		*messageLength = 0
	}
}
