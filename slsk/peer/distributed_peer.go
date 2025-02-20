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

func (peer *DistributedPeer) SearchRequest(username string, token uint32, query string) error {
	mb := messages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	mb.AddInt32(1) // unknown
	mb.AddString(username)
	mb.AddString(query)
	mb.AddInt32(token)
	msg := mb.Build(3)
	err := peer.SendMessage(msg)
	return err
}

func (peer *DistributedPeer) BranchLevel(branchLevel uint32) error {
	mb := messages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	mb.AddInt32(branchLevel)
	msg := mb.Build(4)
	err := peer.SendMessage(msg)
	return err
}

func (peer *DistributedPeer) BranchRoot(branchRoot string) error {
	mb := messages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	mb.AddString(branchRoot)
	msg := mb.Build(5)
	err := peer.SendMessage(msg)
	return err
}

// https://nicotine-plus.org/doc/SLSKPROTOCOL.html#distributed-code-93
func (peer *DistributedPeer) DistributedMessage(embeddedMessage string) error {
	mb := messages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	mb.AddInt8(3) // the only type of distributed message sent currently is SearchRequest
	mb.AddString(embeddedMessage)
	msg := mb.Build(93)
	err := peer.SendMessage(msg)
	return err
}
