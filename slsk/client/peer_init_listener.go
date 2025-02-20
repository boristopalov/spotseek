package client

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"spotseek/slsk/messages"
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
		log.Info("Message from peer", "code", code, "message", decoded, "peerAddr", peerConn.RemoteAddr().String())
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
	usernameAndConnType, ok := c.PendingOutgoingPeerConnectionTokens[token]
	if !ok {
		log.Error("No pending connection for token", "token", token)
		return map[string]interface{}{
			"token": token,
		}, nil
	}

	log.Info("Received PierceFirewall", "username", usernameAndConnType.username)
	host, port, err := SplitHostPort(conn)
	if err != nil {
		return nil, err
	}
	c.RemovePendingPeer(usernameAndConnType.username)
	delete(c.PendingOutgoingPeerConnectionTokens, token)

	peer := c.PeerManager.AddPeer(usernameAndConnType.username, usernameAndConnType.connType, host, port, token, 0)
	if peer == nil {
		return nil, fmt.Errorf("failed to connect to peer: %v", peer)
	}
	switch usernameAndConnType.connType {
	case "P":
		go peer.ListenForPeerMessages()
	case "D":
		go peer.ListenForDistributedMessages()
	case "F":
		go peer.ListenForFileTransferMessages()
	}
	return map[string]interface{}{
		"token": token,
	}, nil

}

func (c *SlskClient) handlePeerInit(conn net.Conn, mr *messages.PeerInitMessageReader) (map[string]interface{}, error) {
	username, connType, token := mr.ParsePeerInit()
	host, port, err := SplitHostPort(conn)
	if err != nil {
		return nil, err
	}
	peer := c.PeerManager.AddPeer(username, connType, host, port, 0, 0)
	if peer == nil {
		return nil, fmt.Errorf("failed to connect to peer: %v", peer)
	}
	switch connType {
	case "P":
		go peer.ListenForPeerMessages()
	case "D":
		go peer.ListenForDistributedMessages()
	case "F":
		go peer.ListenForFileTransferMessages()
	default:
		log.Error("Unknown peer connection type", "peer", peer, "connType", connType)
	}
	return map[string]interface{}{
		"username": username,
		"connType": connType,
		"token":    token,
	}, nil
}
