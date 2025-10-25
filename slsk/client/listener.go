package client

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"spotseek/slsk/messages"
	"time"
)

// ListenForIncomingPeers listens for new peer connections
func (c *SlskClient) ListenForIncomingPeers() {
	for {
		peerConn, err := c.Listener.Accept()
		if err != nil {
			c.logger.Error("Error accepting peer connection", "err", err)
			continue
		}

		// Set a timeout for the initial message
		_ = peerConn.SetReadDeadline(time.Now().Add(30 * time.Second))

		// Handle the connection in a separate goroutine
		go c.handleIncomingPeerConnection(peerConn)
	}
}

func (c *SlskClient) closePeerConnection(peerConn net.Conn) {
	host, port, err := SplitHostPort(peerConn)
	if err != nil {
		c.logger.Error("Failed to split host port")
	} else {
		ip := IP{IP: host, port: port}
		if c.ParentIp == ip {
			c.ParentIp = IP{}
			c.ParentUsername = ""
			c.HaveNoParent(1)
		}
	}
	peerConn.Close()
}

// New method to handle each incoming peer connection
func (c *SlskClient) handleIncomingPeerConnection(peerConn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Error("Recovered from panic in peer connection handler", "error", r)
			c.closePeerConnection(peerConn)
		}
	}()

	message, err := readPeerInitMessage(peerConn)
	if err != nil {
		c.logger.Error("Error reading peer message", "err", err)
		c.closePeerConnection(peerConn)
		return
	}

	peerMsgReader := messages.NewMessageReader(message)
	code := peerMsgReader.ReadInt8()

	var handleErr error
	switch code {
	case 0:
		_, handleErr = c.handlePierceFirewall(peerConn, peerMsgReader)
	case 1:
		_, handleErr = c.handlePeerInit(peerConn, peerMsgReader)
	}
	if handleErr != nil {
		c.closePeerConnection(peerConn)
	}
}

func readPeerInitMessage(conn net.Conn) ([]byte, error) {
	sizeBuf := make([]byte, 4)
	_, err := io.ReadFull(conn, sizeBuf)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return nil, fmt.Errorf("connection timed out waiting for data: %w", err)
		}
		return nil, fmt.Errorf("failed to read message size: %w", err)
	}
	msgLength := binary.LittleEndian.Uint32(sizeBuf)

	if msgLength > 4096 {
		return nil, fmt.Errorf("message size too large: %d bytes", msgLength)
	}

	message := make([]byte, msgLength)
	_, err = io.ReadFull(conn, message)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return nil, fmt.Errorf("connection timed out waiting for message body: %w", err)
		}
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

	peer := c.PeerManager.AddPeer(username, connInfo.connType, host, port, token, connInfo.privileged, conn)
	if peer == nil {
		return nil, fmt.Errorf("failed to connect to peer: %v", peer)
	}
	if connInfo.connType == "F" {
		key := fmt.Sprintf("%s_%d", username, token)
		transfer, exists := c.PendingTransferReqs[key]
		if !exists {
			return nil, fmt.Errorf("got F connection but no pending file transfers found")
		}
		go peer.UploadFile(transfer)
		delete(c.PendingTransferReqs, key) // TODO: maybe should wait for an upload complete event

	} else {
		// Send any stored search results for this peer
		storedSearchResultsForPeer := c.DistribSearchResults[username]
		for _, res := range storedSearchResultsForPeer {
			peer.FileSearchResponse(username, res.token, res.files)
		}
		delete(c.DistribSearchResults, username)

		// Send QueueUpload for any pending downloads
		pendingDownloads := c.DownloadManager.GetPendingForPeer(username)
		for _, dl := range pendingDownloads {
			peer.QueueUpload(dl.Filename)
			// Update download status from "connecting" to "queued"
			dl.UpdateStatus("queued")
		}
		c.DownloadManager.ClearPendingForPeer(username)

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
	peer := c.PeerManager.AddPeer(username, connType, host, port, 0, 0, conn)
	if peer == nil {
		return nil, fmt.Errorf("failed to connect to peer: %v", peer)
	}
	if connType == "F" {
		go peer.FileListen()
	} else {
		go peer.Listen()
	}
	return map[string]any{
		"username": username,
		"connType": connType,
		"token":    token,
	}, nil
}
