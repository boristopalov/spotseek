package peer

import (
	"spotseek/src/slsk/messages"
)

type DistributedPeer interface {
	BasePeer
	SearchRequest()
	BranchLevel()
	BranchRoot()
	DistributedMessage()
}

func (peer *Peer) SearchRequest(username string, token uint32, query string) error {
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

func (peer *Peer) BranchLevel(branchLevel uint32) error {
	mb := messages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	mb.AddInt32(branchLevel)
	msg := mb.Build(4)
	err := peer.SendMessage(msg)
	return err
}

func (peer *Peer) BranchRoot(branchRoot string) error {
	mb := messages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	mb.AddString(branchRoot)
	msg := mb.Build(5)
	err := peer.SendMessage(msg)
	return err
}

// https://nicotine-plus.org/doc/SLSKPROTOCOL.html#distributed-code-93
func (peer *Peer) DistriubtedMessage(embeddedMessage string) error {
	mb := messages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	mb.AddInt8(3) // the only type of distributed message sent currently is SearchRequest
	mb.AddString(embeddedMessage)
	msg := mb.Build(93)
	err := peer.SendMessage(msg)
	return err
}
