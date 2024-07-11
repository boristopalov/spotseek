package peerMessages

import (
	"spotseek/src/slsk/messages"
)

type PeerMessageBuilder struct {
	*messages.MessageBuilder
}

func (mb *PeerMessageBuilder) QueueUpload(filename string) []byte {
	mb.AddString(filename)
	return mb.Build(43)
}

func (mb *PeerMessageBuilder) UserInfoRequest() []byte {
	return mb.Build(15)
}
