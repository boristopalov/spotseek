package peer

import (
	"spotseek/slsk/messages"
)

type FileTransferPeer interface {
	BasePeer
	FileTransferInit(token uint32) error
	FileOffset(offset uint64) error
}

func (peer *Peer) FileTransferInit(token uint32) error {
	mb := messages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.FileTransferInit(token)
	err := peer.SendMessage(msg)
	return err
}

func (peer *Peer) FileOffset(offset uint64) error {
	mb := messages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.FileOffset(offset)
	err := peer.SendMessage(msg)
	return err
}
