package messages

import (
	"encoding/binary"
	"strconv"
)


type MessageReader struct { 
	message []byte
	pointer uint32 // current position in the message
}

func NewMessageReader(msg []byte) *MessageReader {
	return &MessageReader{
		message: msg,
		pointer: 0,
	}
}


func (mr *MessageReader) ReadBool() bool {
	success := mr.message[mr.pointer] == 1
	mr.IncrementPointer(1)
	return success
}

func (mr *MessageReader) ReadString() string { 
	length := mr.ReadInt32()
	content := string(mr.message[mr.pointer:mr.pointer + length])
	mr.IncrementPointer(length)
	return content
}

func (mr *MessageReader) ReadInt8() uint8 { 
	num := uint8(mr.message[mr.pointer])
	mr.IncrementPointer(1)
	return num
}


func (mr *MessageReader) ReadInt32() uint32 { 
	num := binary.LittleEndian.Uint32(mr.message[mr.pointer:mr.pointer+4])
	mr.IncrementPointer(4)
	return num
}


func (mr *MessageReader) ReadInt64() uint64 { 
	num := binary.LittleEndian.Uint64(mr.message[mr.pointer:mr.pointer+8])
	mr.IncrementPointer(8)
	return num
}

func (mr *MessageReader) ReadIp() string { 
	ip4 := strconv.Itoa(int(mr.ReadInt8()))
	ip3 := strconv.Itoa(int(mr.ReadInt8()))
	ip2 := strconv.Itoa(int(mr.ReadInt8()))
	ip1 := strconv.Itoa(int(mr.ReadInt8()))
	return ip1 + "." + ip2 + "." + ip3 + "." + ip4
}


func (mr *MessageReader) IncrementPointer(move uint32) { 
	mr.pointer += move
}
