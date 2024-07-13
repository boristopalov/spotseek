package peerMessages

import (
	"encoding/binary"
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

func (mb *PeerMessageBuilder) PlaceInQueueRequest(filename string) []byte {
	mb.AddString(filename)
	return mb.Build(51)
}

// File Messages do not have codes
func (mb *PeerMessageBuilder) FileTransferInit(token uint32) []byte {
	mb.AddInt32(token)
	prefixBytes := make([]byte, 4)
	messageLength := uint32(len(mb.Message) + 4) // length of the message
	binary.LittleEndian.PutUint32(prefixBytes, messageLength)
	mb.Message = append(prefixBytes, mb.Message...) // append the message to the prefix bytes
	return mb.Message
}

func (mb *PeerMessageBuilder) FileOffset(offset uint64) []byte {
	mb.AddInt64(offset)
	prefixBytes := make([]byte, 4)
	messageLength := uint32(len(mb.Message) + 4) // length of the message
	binary.LittleEndian.PutUint32(prefixBytes, messageLength)
	mb.Message = append(prefixBytes, mb.Message...) // append the message to the prefix bytes
	return mb.Message
}
