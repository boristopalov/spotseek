package peer

import (
	"encoding/binary"
	"io"
	"spotseek/slsk/messages"
)

// Listen method for DistributedPeer
func (peer *DistributedPeer) Listen() {
	readBuffer := make([]byte, 4096)
	var currentMessage []byte
	var messageLength uint32

	defer func() {
		peer.mgrCh <- PeerEvent{Type: PeerDisconnected, Username: peer.Username, Host: peer.Host, Port: peer.Port}
	}()

	for {
		n, err := peer.Conn.Read(readBuffer)
		if err != nil {
			if err == io.EOF {
				peer.logger.Warn("Distributed peer closed the connection",
					"peer", peer.Username)
				return
			}
			peer.logger.Error("Error reading from distributed peer",
				"peer", peer.Username,
				"err", err)
			return
		}
		peer.logger.Debug("received message from distributed peer",
			"length", n,
			"peer", peer.Username)
		currentMessage = append(currentMessage, readBuffer[:n]...)
		currentMessage, messageLength = peer.processMessage(currentMessage, messageLength)
	}
}

func (peer *DistributedPeer) processMessage(data []byte, messageLength uint32) ([]byte, uint32) {
	if len(data) == 0 {
		return data, messageLength
	}
	for {
		if messageLength == 0 {
			if len(data) < 4 {
				return data, messageLength
			}
			messageLength = binary.LittleEndian.Uint32(data[:4])
			data = data[4:]
		}

		if uint32(len(data)) < messageLength {
			return data, messageLength
		}

		peer.handleDistribMessage(data[:messageLength])

		data = data[messageLength:]
		messageLength = 0

		if len(data) == 0 {
			return data, messageLength
		}
	}
}

func (peer *DistributedPeer) handleDistribMessage(messageData []byte) {
	mr := messages.NewMessageReader(messageData)
	code := mr.ReadInt8()

	switch code {
	case 3:
		peer.handleSearch(mr)
	case 4:
		peer.handleBranchLevel(mr)
	case 5:
		peer.handleBranchRoot(mr)
	case 93:
		peer.handleDistributedMessage(mr)
	}
}

func (peer *DistributedPeer) handleSearch(mr *messages.MessageReader) {
	mr.ReadInt32() // unknown field
	username := mr.ReadString()
	token := mr.ReadInt32()
	query := mr.ReadString()
	peer.distribSearchCh <- DistribSearchMsg{
		Username: username,
		Token:    token,
		Query:    query,
	}
	// this log is noisy
	// peer.logger.Info("received DistributedSearchMessage", "username", username, "token", token, "query", query)
}

func (peer *DistributedPeer) handleBranchLevel(mr *messages.MessageReader) {
	level := mr.ReadInt32()
	peer.BranchLevel = level
	peer.logger.Info("Received DistributedBranchLevelMessage", "level", level, "peer", peer.Username)
}

func (peer *DistributedPeer) handleBranchRoot(mr *messages.MessageReader) (map[string]any, error) {
	branchRoot := mr.ReadString()
	peer.BranchRoot = branchRoot
	peer.logger.Info("Received DistributedBranchRootMessage", "branchRoot", branchRoot, "peer", peer.Username)
	return map[string]any{
		"type":               "BranchRoot",
		"branchRootUsername": branchRoot,
	}, nil
}

func (peer *DistributedPeer) handleDistributedMessage(mr *messages.MessageReader) {
	embeddedData := mr.Message
	if len(embeddedData) == 0 {
		return
	}

	// Prevent infinite recursion: embedded messages should not contain code 93
	code := embeddedData[0]
	if code == 93 {
		peer.logger.Warn("Received nested DistribEmbeddedMessage (code 93), ignoring to prevent infinite recursion",
			"peer", peer.Username)
		return
	}

	peer.handleDistribMessage(embeddedData)
}
