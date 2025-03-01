package messages

import "encoding/binary"

type PeerInitMessageBuilder = MessageBuilder

// Build overrides MessageBuilder.Build to use 1-byte message codes for peer messages
func (mb *PeerInitMessageBuilder) BuildPeerInit(code uint32) []byte {
	prefixBytes := make([]byte, 4)
	MessageLength := uint32(len(mb.Message) + 1) // length of the Message + 1 byte for code
	binary.LittleEndian.PutUint32(prefixBytes, MessageLength)
	prefixBytes = append(prefixBytes, byte(code)) // code is 1 byte for peer init messages
	mb.Message = append(prefixBytes, mb.Message...)
	return mb.Message
}
