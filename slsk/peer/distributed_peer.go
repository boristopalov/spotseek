package peer

import (
	"spotseek/slsk/messages"
)

type DistributedPeer interface {
	BasePeer
	SearchRequest(username string, token uint32, query string) error
	BranchLevel(branchLevel uint32) error
	BranchRoot(branchRoot string) error
	DistributedMessage(embeddedMessage string) error
}

type DistributedPeerImpl struct {
	*Peer
	branchLevel int
	branchRoot  string
	childDepth  int
}

func NewDistributedPeerImpl(peer *Peer) *DistributedPeerImpl {
	return &DistributedPeerImpl{
		Peer:        peer,
		branchLevel: -1,
		branchRoot:  "",
		childDepth:  -1,
	}
}

func (peer *DistributedPeerImpl) SearchRequest(username string, token uint32, query string) error {
	mb := messages.NewMessageBuilder()
	mb.AddInt32(1) // unknown
	mb.AddString(username)
	mb.AddString(query)
	mb.AddInt32(token)
	msg := mb.Build(3)
	err := peer.SendMessage(msg)
	return err
}

func (peer *DistributedPeerImpl) BranchLevel(branchLevel uint32) error {
	mb := messages.NewMessageBuilder()
	mb.AddInt32(branchLevel)
	msg := mb.Build(4)
	err := peer.SendMessage(msg)
	return err
}

func (peer *DistributedPeerImpl) BranchRoot(branchRoot string) error {
	mb := messages.NewMessageBuilder()
	mb.AddString(branchRoot)
	msg := mb.Build(5)
	err := peer.SendMessage(msg)
	return err
}

// https://nicotine-plus.org/doc/SLSKPROTOCOL.html#distributed-code-93
func (peer *DistributedPeerImpl) DistributedMessage(embeddedMessage string) error {
	mb := messages.NewMessageBuilder()
	mb.AddInt8(3) // the only type of distributed message sent currently is SearchRequest
	mb.AddString(embeddedMessage)
	msg := mb.Build(93)
	err := peer.SendMessage(msg)
	return err
}
