package messages

import (
	"encoding/binary"
)

type MessageBuilder struct {
	Message []byte
}

func NewMessageBuilder() *MessageBuilder {
	return &MessageBuilder{
		Message: make([]byte, 0),
	}
}

// 1 byte so no need for little endian conversion
func (mb *MessageBuilder) AddInt8(val uint8) {
	mb.Message = append(mb.Message, byte(val))
}

func (mb *MessageBuilder) AddInt32(val uint32) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, val)
	mb.Message = append(mb.Message, b...)
}

func (mb *MessageBuilder) AddInt64(val uint64) {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, val)
	mb.Message = append(mb.Message, b...)
}

func (mb *MessageBuilder) AddString(val string) {
	buf := []byte(val)
	mb.AddInt32(uint32(len(buf)))
	mb.Message = append(mb.Message, buf...)
}

// builds the Message using the length of the Message and the Message code
// see soulseek protocol documentation for Message codes
func (mb *MessageBuilder) Build(code uint32) []byte {
	prefixBytes := make([]byte, 4)
	MessageLength := uint32(len(mb.Message) + 4) // length of the Message
	binary.LittleEndian.PutUint32(prefixBytes, MessageLength)
	codeBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(codeBytes, code) // prefixBytes contains the length of the Message as well as the Message code
	prefixBytes = append(prefixBytes, codeBytes...)
	mb.Message = append(prefixBytes, mb.Message...) // append the Message to the prefix bytes
	return mb.Message
}
