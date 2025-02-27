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
	return nil
}

func (peer *DistributedPeer) handleSearch(mr *messages.MessageReader) {
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
}

func (peer *DistributedPeer) Search(username string, token uint32, query string) error {
	mb := messages.NewMessageBuilder()
	mb.AddInt32(1) // unknown
	mb.AddString(username)
	mb.AddString(query)
	mb.AddInt32(token)
	msg := mb.Build(3)
	err := peer.SendMessage(msg)
	return err
}

func (peer *DistributedPeer) handleBranchLevel(mr *messages.MessageReader) {
	level := mr.ReadInt32()
	peer.mgrCh <- PeerEvent{
		Type: BranchLevel,
		Peer: peer.Peer,
		Data: BranchLevelMessage{
			BranchLevel: level,
		},
	}
}

func (peer *DistributedPeer) BranchLevel(branchLevel uint32) error {
	mb := messages.NewMessageBuilder()
	mb.AddInt32(branchLevel)
	msg := mb.Build(4)
	err := peer.SendMessage(msg)
	return err
}

func (peer *DistributedPeer) handleBranchRoot(mr *messages.MessageReader) {
	branchRootUsername := mr.ReadString()
	peer.mgrCh <- PeerEvent{
		Type: BranchRoot,
		Peer: peer.Peer,
		Data: BranchRootMessage{
			BranchRootUsername: branchRootUsername,
		},
	}
}

func (peer *DistributedPeer) BranchRoot(branchRoot string) error {
	mb := messages.NewMessageBuilder()
	mb.AddString(branchRoot)
	msg := mb.Build(5)
	err := peer.SendMessage(msg)
	return err
}

func (peer *DistributedPeer) handleDistributedMessage(mr *messages.MessageReader) {
	peer.handleMessage(mr.Message)
}

func (peer *DistributedPeer) DistributedMessage(code uint8, data []byte) error {
	mb := messages.NewMessageBuilder()
	mb.AddInt8(code)
	mb.Message = append(mb.Message, data...)
	msg := mb.Build(93)
	err := peer.SendMessage(msg)
	return err
}
