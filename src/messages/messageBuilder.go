package messages

import (
	"encoding/binary"
)

type MessageBuilder struct {
	message []byte
}


func NewMessageBuilder() *MessageBuilder {
	return &MessageBuilder{
		message: make([]byte, 0),
	}
}

// 1 byte so no need for little endian conversion
func (mb *MessageBuilder) AddInt8(val int8) {
	mb.message = append(mb.message, byte(val))
}

func (mb *MessageBuilder) AddInt32(val uint32) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, val)
	mb.message = append(mb.message, b...)
}

func (mb *MessageBuilder) AddInt64(val uint64) {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, val)
	mb.message = append(mb.message, b...)
}


func (mb *MessageBuilder) AddString(val string) {
	buf := []byte(val)
	mb.AddInt32(uint32(len(buf)))
	mb.message = append(mb.message, buf...)
}


// builds the message using the length of the message and the message code
// see soulseek protocol documentation for message codes
func (mb *MessageBuilder) Build(code uint32) []byte {
	prefixBytes := make([]byte, 4)
	messageLength := uint32(len(mb.message)+4) // length of the message
	binary.LittleEndian.PutUint32(prefixBytes, messageLength) 
	codeBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(codeBytes, code)  // prefixBytes contains the length of the message as well as the message code 
	prefixBytes = append(prefixBytes, codeBytes...)
	mb.message = append(prefixBytes, mb.message...) // append the message to the prefix bytes
	return mb.message
}


