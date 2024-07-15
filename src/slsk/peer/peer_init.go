package peer

import (
	"spotseek/src/slsk/messages"
)

type PeerInit interface {
	BasePeer
	PeerInit(username, connType string, token uint32) error
	PierceFirewall(token uint32) error
}

func (peer *Peer) PeerInit(username string, connType string, token uint32) error {

	mb := messages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.PeerInit(username, connType, token)
	err := peer.SendMessage(msg)
	return err
}

func (peer *Peer) PierceFirewall(token uint32) error {
	mb := messages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.PierceFirewall(token)
	err := peer.SendMessage(msg)
	return err
}
