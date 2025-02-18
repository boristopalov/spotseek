package client

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"spotseek/src/slsk/messages"
)

// ListenForIncomingPeers listens for new peer connections
func (c *SlskClient) ListenForIncomingPeers() {
	for {
		peerConn, err := c.Listener.Accept()
		if err != nil {
			log.Error("Error accepting peer connection", "err", err)
			continue
		}
		message, err := readPeerInitMessage(peerConn)
		if err != nil {
			log.Error("Error reading peer message", "err", err)
			continue
		}

		peerMsgReader := messages.PeerInitMessageReader{MessageReader: messages.NewMessageReader(message)}
		code := peerMsgReader.ReadInt8()

		log.Error("Peer message code", "code", code, "peerAddr", peerConn.RemoteAddr().String())

		var decoded map[string]interface{}
		switch code {
		case 0:
			decoded, err = c.handlePierceFirewall(peerConn, &peerMsgReader)
		case 1:
			decoded, err = c.handlePeerInit(peerConn, &peerMsgReader)
		default:
			log.Error("Unknown peer message code", "code", code)
		}
		if err != nil {
			log.Error("Error handling peer message", "err", err)
			continue
		}
		log.Info("Message from peer", "message", decoded)
	}
}

func readPeerInitMessage(conn net.Conn) ([]byte, error) {
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

func (c *SlskClient) handlePierceFirewall(conn net.Conn, mr *messages.PeerInitMessageReader) (map[string]interface{}, error) {
	token := mr.ParsePierceFirewall()
	usernameAndConnType, ok := c.PendingPeerConnections[token]
	if !ok {
		log.Error("No pending connection for token", "token", token)
		return map[string]interface{}{
			"token": token,
		}, nil
	}

	log.Info("Received PierceFirewall", "username", usernameAndConnType.username)
	return map[string]interface{}{
		"token": token,
	}, nil

	// I don't think we need to do anything here?
	// ip, port, err := SplitHostPort(conn)
	// if err != nil {
	// 	return nil, fmt.Errorf("HandlePierceFirewall Error: %v", err)
	// }
	// err = c.PeerManager.ConnectToPeer(ip, port, usernameAndConnType.username, usernameAndConnType.connType, token)
	// if err != nil {
	// 	log.Printf("Error establishing connection to peer while handling PierceFirewall: %v", err)
	// 	return nil, fmt.Errorf("HandlePierceFirewall Error: %v", err)
	// }

	// c.mu.Lock()
	// c.ConnectedPeers[peer.Username] = *peer
	// delete(c.PendingTokenConnTypes, token)
	// c.mu.Unlock()

	// err = peer.PierceFirewall(token)
	// if err != nil {
	// 	log.Printf("Error sending PierceFirewall: %v", err)
	// 	return nil, fmt.Errorf("HandlePierceFirewall Error: %v", err)
	// }
	// c.mu.Lock()
	// delete(c.PendingPeerConnections, token)
	// c.mu.Unlock()
	// return map[string]interface{}{
	// 	"token": token,
	// }, nil
}

// Step 3 (User B)
// If User B receives the PeerInit message, a connection is established, and user A is free to send peer messages.
func (c *SlskClient) handlePeerInit(conn net.Conn, mr *messages.PeerInitMessageReader) (map[string]interface{}, error) {
	username, connType, token := mr.ParsePeerInit()
	// ip, port, err := SplitHostPort(conn)
	// if err != nil {
	// 	return nil, fmt.Errorf("HandlePeerInit Error: %v", err)
	// }
	// err = c.PeerManager.ConnectToPeer(ip, port, username, connType, token)
	// c.PendingPeerConnections[token] = PendingTokenConn{
	// 	username: username,
	// 	connType: connType,
	// }
	c.ConnectToPeer(username, connType)
	// if err != nil {
	// 	log.Printf("Error establishing connection to peer while handling PeerInit: %v", err)
	// 	return nil, err
	// }

	// peer, err := c.createPeerFromConnection(conn, username, connType, token)
	// if err != nil {
	// 	log.Printf("Error establishing connection to peer while handling PeerInit: %v", err)
	// 	return nil, err
	// }
	// c.mu.RLock()
	// connectedPeer, ok := c.ConnectedPeers[username]
	// c.mu.RUnlock()
	// if ok && connType == connectedPeer.ConnType {
	// 	log.Printf("Already connected to %s", username)
	// 	return nil, err
	// }

	// log.Printf("Connection established with peer %v", peer)
	// c.mu.Lock()
	// c.ConnectedPeers[username] = *peer
	// delete(c.PendingTokenConnTypes, token)
	// c.mu.Unlock()

	// go c.ListenForPeerMessages(peer)
	return map[string]interface{}{
		"username": username,
		"connType": connType,
		"token":    token,
	}, nil
}

// func (c *SlskClient) createPeerFromConnection(conn net.Conn, username, connType string, token uint32) (*peer.Peer, error) {
// 	ip, portStr, err := net.SplitHostPort(conn.RemoteAddr().String())
// 	if err != nil {
// 		return nil, fmt.Errorf("error getting IP and port: %w", err)
// 	}

// 	port, err := strconv.ParseUint(portStr, 10, 32)
// 	if err != nil {
// 		return nil, fmt.Errorf("error parsing port: %w", err)
// 	}

// 	peer, err := peer.NewPeer(username, connType, token, ip, uint32(port))
// 	if err != nil {
// 		return nil, err
// 	}
// 	return peer, nil
// }
