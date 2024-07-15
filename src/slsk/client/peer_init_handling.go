package client

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"spotseek/src/slsk/messages"
	"spotseek/src/slsk/peer"
	"strconv"
)

// ListenForIncomingPeers listens for new peer connections
func (c *SlskClient) ListenForIncomingPeers() {
	for {
		peerConn, err := c.Listener.Accept()
		if err != nil {
			log.Printf("Error accepting peer connection: %v", err)
			continue
		}

		go c.handlePeerConnection(peerConn)
	}
}

func (c *SlskClient) handlePeerConnection(peerConn net.Conn) (map[string]interface{}, error) {
	// defer peerConn.Close()
	message, err := c.readPeerInitMessage(peerConn)
	if err != nil {
		return nil, fmt.Errorf("error reading peer message: %v", err)
	}

	peerMsgReader := messages.PeerInitMessageReader{MessageReader: messages.NewMessageReader(message)}
	code := peerMsgReader.ReadInt8()

	log.Printf("Peer message: code %d; address %s", code, peerConn.RemoteAddr().String())

	var decoded map[string]interface{}
	switch code {
	case 0:
		decoded, err = c.handlePierceFirewall(peerConn, &peerMsgReader)
	case 1:
		decoded, err = c.handlePeerInit(peerConn, &peerMsgReader)
	default:
		return nil, fmt.Errorf("unknown peer message code: %d", code)
	}
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func (c *SlskClient) readPeerInitMessage(conn net.Conn) ([]byte, error) {
	sizeBuf := make([]byte, 4)
	_, err := io.ReadFull(conn, sizeBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to read message size: %w", err)
	}
	size := binary.LittleEndian.Uint32(sizeBuf)

	if size > 4096 {
		return nil, fmt.Errorf("message size too large: %d bytes", size)
	}

	message := make([]byte, size)
	_, err = io.ReadFull(conn, message)
	if err != nil {
		return nil, fmt.Errorf("failed to read message body: %w", err)
	}

	return message, nil
}

func (c *SlskClient) handlePierceFirewall(conn net.Conn, reader *messages.PeerInitMessageReader) (map[string]interface{}, error) {
	token := reader.ParsePierceFirewall()
	usernameAndConnType, ok := c.PendingTokenConnTypes[token]
	if !ok {
		log.Printf("No pending connection for token %d", token)
		return map[string]interface{}{
			"token": token,
		}, nil
	}

	log.Printf("received PierceFirewall from %s", usernameAndConnType.username)
	// return map[string]interface{}{
	// 	"token": token,
	// }, nil

	// I don't think we need to do anything here?
	peer, err := c.createPeerFromConnection(conn, usernameAndConnType.username, usernameAndConnType.connType, token)
	if err != nil {
		log.Printf("Error establishing connection to peer while handling PierceFirewall: %v", err)
		return nil, fmt.Errorf("HandlePierceFirewall Error: %v", err)
	}

	c.mu.Lock()
	c.ConnectedPeers[peer.Username] = *peer
	delete(c.PendingTokenConnTypes, token)
	c.mu.Unlock()

	// err = peer.PierceFirewall(token)
	// if err != nil {
	// 	log.Printf("Error sending PierceFirewall: %v", err)
	// 	return nil, fmt.Errorf("HandlePierceFirewall Error: %v", err)
	// }

	go c.ListenForPeerMessages(peer)
	return map[string]interface{}{
		"token": token,
	}, nil
}

// Step 3 (User B)
// If User B receives the PeerInit message, a connection is established, and user A is free to send peer messages.
func (c *SlskClient) handlePeerInit(conn net.Conn, reader *messages.PeerInitMessageReader) (map[string]interface{}, error) {
	username, connType, token := reader.ParsePeerInit()

	peer, err := c.createPeerFromConnection(conn, username, connType, token)
	if err != nil {
		log.Printf("Error establishing connection to peer while handling PeerInit: %v", err)
		return nil, err
	}
	c.mu.RLock()
	connectedPeer, ok := c.ConnectedPeers[username]
	c.mu.RUnlock()
	if ok && connType == connectedPeer.ConnType {
		log.Printf("Already connected to %s", username)
		return nil, err
	}

	log.Printf("Connection established with peer %v", peer)
	c.mu.Lock()
	c.ConnectedPeers[username] = *peer
	delete(c.PendingTokenConnTypes, token)
	c.mu.Unlock()

	go c.ListenForPeerMessages(peer)
	return map[string]interface{}{
		"username": username,
		"connType": connType,
		"token":    token,
	}, nil
}

func (c *SlskClient) createPeerFromConnection(conn net.Conn, username, connType string, token uint32) (*peer.Peer, error) {
	ip, portStr, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		return nil, fmt.Errorf("error getting IP and port: %w", err)
	}

	port, err := strconv.ParseUint(portStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("error parsing port: %w", err)
	}

	peer, err := peer.NewPeer(username, connType, token, ip, uint32(port))
	if err != nil {
		return nil, err
	}
	return peer, nil
}
