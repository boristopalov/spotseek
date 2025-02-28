package peer

import (
	"spotseek/slsk/messages"
)

type DistributedPeer struct {
	*Peer
	branchLevel int
	branchRoot  string
	childDepth  int
}

func NewDistributedPeer(peer *Peer) *DistributedPeer {
	return &DistributedPeer{
		Peer:        peer,
		branchLevel: -1,
		branchRoot:  "",
		childDepth:  -1,
	}
}

func (peer *DistributedPeer) handleMessage(messageData []byte) error {
	mr := messages.NewMessageReader(messageData)
	code := mr.ReadInt8()
	peer.logger.Info("received distributed message", "code", code)
	var result map[string]any
	var err error
	switch code {
	case 3:
		result, err = peer.handleSearch(mr)
	case 4:
		result, err = peer.handleBranchLevel(mr)
	case 5:
		result, err = peer.handleBranchRoot(mr)
	case 93:
		result, err = peer.handleDistributedMessage(mr)
	}
	if err != nil {
		peer.logger.Error("error handling distributed message", "error", err)
	}
	peer.logger.Info("distributed message handled", "result", result)
	return err
}

func (peer *DistributedPeer) handleSearch(mr *messages.MessageReader) (map[string]any, error) {
	mr.ReadInt32() // unknown field
	username := mr.ReadString()
	token := mr.ReadInt32()
	query := mr.ReadString()
	peer.mgrCh <- PeerEvent{
		Type: DistribSearch,
		Peer: peer.Peer,
		Data: DistribSearchMessage{
			Username: username,
			Token:    token,
			Query:    query,
		},
	}
	return map[string]any{
		"type":     "Search",
		"username": username,
		"token":    token,
		"query":    query,
	}, nil
}

func (peer *DistributedPeer) Search(username string, token uint32, query string) (map[string]any, error) {
	mb := messages.NewMessageBuilder()
	mb.AddInt32(1) // unknown
	mb.AddString(username)
	mb.AddString(query)
	mb.AddInt32(token)
	msg := mb.Build(3)
	peer.SendMessage(msg)
	peer.logger.Info("sent distributedsearch message", "username", username, "token", token, "query", query)
	return map[string]any{
		"type":     "Search",
		"username": username,
		"token":    token,
		"query":    query,
	}, nil
}

func (peer *DistributedPeer) handleBranchLevel(mr *messages.MessageReader) (map[string]any, error) {
	level := mr.ReadInt32()
	peer.mgrCh <- PeerEvent{
		Type: BranchLevel,
		Peer: peer.Peer,
		Data: BranchLevelMessage{
			BranchLevel: level,
		},
	}
	return map[string]any{
		"type":  "BranchLevel",
		"level": level,
	}, nil
}

func (peer *DistributedPeer) BranchLevel(branchLevel uint32) (map[string]any, error) {
	mb := messages.NewMessageBuilder()
	mb.AddInt32(branchLevel)
	msg := mb.Build(4)
	peer.SendMessage(msg)
	peer.logger.Info("sent distributed branch level message", "level", branchLevel)
	return map[string]any{
		"type":  "BranchLevel",
		"level": branchLevel,
	}, nil
}

func (peer *DistributedPeer) handleBranchRoot(mr *messages.MessageReader) (map[string]any, error) {
	branchRootUsername := mr.ReadString()
	peer.mgrCh <- PeerEvent{
		Type: BranchRoot,
		Peer: peer.Peer,
		Data: BranchRootMessage{
			BranchRootUsername: branchRootUsername,
		},
	}
	return map[string]any{
		"type":               "BranchRoot",
		"branchRootUsername": branchRootUsername,
	}, nil
}

func (peer *DistributedPeer) BranchRoot(branchRoot string) (map[string]any, error) {
	mb := messages.NewMessageBuilder()
	mb.AddString(branchRoot)
	msg := mb.Build(5)
	peer.SendMessage(msg)
	peer.logger.Info("sent distributed branch root message", "branchRoot", branchRoot)
	return map[string]any{
		"type":               "BranchRoot",
		"branchRootUsername": branchRoot,
	}, nil
}

func (peer *DistributedPeer) handleDistributedMessage(mr *messages.MessageReader) (map[string]any, error) {
	peer.handleMessage(mr.Message)
	return map[string]any{
		"type": "DistributedMessage",
	}, nil
}

func (peer *DistributedPeer) DistributedMessage(code uint8, data []byte) (map[string]any, error) {
	mb := messages.NewMessageBuilder()
	mb.AddInt8(code)
	mb.Message = append(mb.Message, data...)
	msg := mb.Build(93)
	peer.SendMessage(msg)
	peer.logger.Info("sent distributed message", "code", code)
	return map[string]any{
		"type": "DistributedMessage",
	}, nil
}
