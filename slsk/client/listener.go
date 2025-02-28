package client

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"spotseek/slsk/messages"
)

// ListenForIncomingPeers listens for new peer connections
// TODO: maybe send these to peer manager channel instead of processing here
func (c *SlskClient) ListenForIncomingPeers() {
	for {
		peerConn, err := c.Listener.Accept()
		if err != nil {
			c.logger.Error("Error accepting peer connection", "err", err)
			continue
		}
		message, err := readPeerInitMessage(peerConn)
		if err != nil {
			c.logger.Error("Error reading peer message", "err", err)
			continue
		}

		peerMsgReader := messages.NewMessageReader(message)
		code := peerMsgReader.ReadInt8()

		var decoded map[string]any
		switch code {
		case 0:
			decoded, err = c.handlePierceFirewall(peerConn, peerMsgReader)
		case 1:
			decoded, err = c.handlePeerInit(peerConn, peerMsgReader)
		default:
			c.logger.Error("Unknown peer message code", "code", code)
		}
		if err != nil {
			c.logger.Error("Error handling peer message", "err", err)
			continue
		}
		c.logger.Info("Message from peer", "code", code, "message", decoded, "peerAddr", peerConn.RemoteAddr().String())
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

func (c *SlskClient) handlePierceFirewall(conn net.Conn, mr *messages.MessageReader) (map[string]any, error) {
	token := mr.ReadInt32()
	connInfo, ok := c.PendingTokens[token]
	if !ok {
		c.logger.Error("No pending connection for token", "token", token)
		return map[string]any{
			"token": token,
		}, nil
	}

	username := connInfo.username
	c.logger.Info("Received PierceFirewall", "peer", username)
	host, port, err := SplitHostPort(conn)
	if err != nil {
		return nil, err
	}
	c.RemovePendingPeer(connInfo.username)

	peer := c.PeerManager.AddPeer(username, connInfo.connType, host, port, token, connInfo.privileged)
	if peer == nil {
		return nil, fmt.Errorf("failed to connect to peer: %v", peer)
	}
	if connInfo.connType == "F" {
		key := fmt.Sprintf("%s_%d", username, token)
		transfer, exists := c.PendingTransferReqs[key]
		if !exists {
			return nil, fmt.Errorf("got F connection but no pending file transfers found")
		}
		peer.PeerInit(c.Username, "F", 0)
		go peer.UploadFile(transfer)
		delete(c.PendingTransferReqs, key) // TODO: maybe should wait for an upload complete event

	} else {
		go peer.Listen()
	}
	return map[string]any{
		"token": token,
	}, nil
}

func (c *SlskClient) handlePeerInit(conn net.Conn, mr *messages.MessageReader) (map[string]any, error) {
	username := mr.ReadString()
	connType := mr.ReadString()
	token := mr.ReadInt32()
	host, port, err := SplitHostPort(conn)
	if err != nil {
		return nil, err
	}
	peer := c.PeerManager.AddPeer(username, connType, host, port, 0, 0)
	if peer == nil {
		return nil, fmt.Errorf("failed to connect to peer: %v", peer)
	}
	go peer.Listen()
	return map[string]any{
		"username": username,
		"connType": connType,
		"token":    token,
	}, nil
}
