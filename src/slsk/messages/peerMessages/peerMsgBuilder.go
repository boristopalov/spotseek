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

func (mb *PeerMessageBuilder) PeerInit(username string, connType string, token uint32) []byte {
	mb.AddString(username)
	mb.AddString(connType)
	mb.AddInt32(token) // token - value is always 0
	// mb.AddInt32(0) // token - value is always 0
	return mb.Build(1)
}

func (mb *PeerMessageBuilder) PierceFirewall(token uint32) []byte {
	mb.AddInt32(token)
	return mb.Build(0)
}
