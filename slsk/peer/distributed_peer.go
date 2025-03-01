package peer

import (
	"spotseek/slsk/messages"
)

type DistributedPeer interface {
	handleDistribMessage(messageData []byte)
	handleSearch(mr *messages.MessageReader)
	handleBranchLevel(mr *messages.MessageReader)
	handleDistributedMessage(mr *messages.MessageReader)
	SendMessage(msg []byte) error
}

type distributedPeer = Peer

func (peer *distributedPeer) handleDistribMessage(messageData []byte) {
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

func (peer *distributedPeer) handleSearch(mr *messages.MessageReader) {
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

func (peer *distributedPeer) handleBranchLevel(mr *messages.MessageReader) {
	level := mr.ReadInt32()
	peer.BranchLevel = level
	peer.logger.Info("Received DistributedBranchLevelMessage", "level", level, "peer", peer.Username)
}

func (peer *distributedPeer) handleBranchRoot(mr *messages.MessageReader) (map[string]any, error) {
	branchRoot := mr.ReadString()
	peer.BranchRoot = branchRoot
	peer.logger.Info("Received DIstributedBranchRootMessage", "branchRoot", branchRoot, "peer", peer.Username)
	return map[string]any{
		"type":               "BranchRoot",
		"branchRootUsername": branchRoot,
	}, nil
}

func (peer *distributedPeer) handleDistributedMessage(mr *messages.MessageReader) {
	peer.handleDistribMessage(mr.Message)
}
