package messages

import "encoding/binary"

type PeerInitMessageBuilder struct {
	*MessageBuilder
}

func (mb *PeerInitMessageBuilder) PeerInit(username string, connType string, token uint32) []byte {
	mb.AddString(username)
	mb.AddString(connType)
	mb.AddInt32(token) // token - value is always 0
	// mb.AddInt32(0) // token - value is always 0
	return mb.Build(1)
}

func (mb *PeerInitMessageBuilder) PierceFirewall(token uint32) []byte {
	mb.AddInt32(token)
	return mb.Build(0)
}

// Build overrides MessageBuilder.Build to use 1-byte message codes for peer messages
func (mb *PeerInitMessageBuilder) Build(code uint32) []byte {
	prefixBytes := make([]byte, 4)
	MessageLength := uint32(len(mb.Message) + 1) // length of the Message + 1 byte for code
	binary.LittleEndian.PutUint32(prefixBytes, MessageLength)
	prefixBytes = append(prefixBytes, byte(code)) // code is 1 byte for peer init messages
	mb.Message = append(prefixBytes, mb.Message...)
	return mb.Message
}
