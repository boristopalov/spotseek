package messages

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
)

type FileInfo struct {
	Filename   string
	Size       uint64
	Extension  string
	Attributes []FileAttribute
}

type FileAttribute struct {
	Type  uint32
	Value uint32
}

type PeerMessageBuilder struct {
	*MessageBuilder
}

func (mb *PeerMessageBuilder) QueueUpload(filename string) []byte {
	mb.AddString(filename)
	return mb.Build(43)
}

func (mb *PeerMessageBuilder) SharedFileListRequest() []byte {
	return mb.Build(4)
}

func (mb *PeerMessageBuilder) SharedFileListResponse() []byte {

	return mb.Build(5)
}

func (mb *PeerMessageBuilder) UserInfoRequest() []byte {
	return mb.Build(15)
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

func (mb *PeerMessageBuilder) FileSearchResponse(files []FileInfo, token uint32, user string) []byte {
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)

	mb.AddString(user)
	mb.AddInt32(token)
	mb.AddInt32(uint32(len(files)))

	for _, file := range files {
		mb.AddString(file.Filename)
		mb.AddInt64(file.Size)
		mb.AddString(file.Extension)
		mb.AddInt32(uint32(len(file.Attributes)))
		for _, attr := range file.Attributes {
			mb.AddInt32(attr.Type)
			mb.AddInt32(attr.Value)
		}
	}

	zw.Write(mb.Message)
	zw.Close()

	mb.Message = buf.Bytes()
	return mb.Build(9)
}
