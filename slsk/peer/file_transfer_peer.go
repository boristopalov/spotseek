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
	mb := messages.NewMessageBuilder()
	mb.AddInt32(token)
	err := peer.SendMessage(mb.Message)
	return err
}

func (peer *Peer) FileOffset(offset uint64) error {
	mb := messages.NewMessageBuilder()
	mb.AddInt64(offset)
	err := peer.SendMessage(mb.Message)
	return err
}
