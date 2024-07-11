package peerMessages

import "spotseek/src/slsk/messages"

type PeerInitMessageReader struct {
	*messages.MessageReader
}

type PeerInitMessageBuilder struct {
	*messages.MessageBuilder
}

func (mr *PeerInitMessageReader) ParsePeerInit() (string, string, uint32) {
	return mr.ReadString(), mr.ReadString(), mr.ReadInt32()
}

func (mr *PeerInitMessageReader) ParsePierceFirewall() uint32 {
	return mr.ReadInt32()
}

// feel like its a bad idea to be able to build messages using the same type and same code
// i.e. you can build a message to both server and peer using the code '1'
// maybe different type for each message and check the type before sending?
// but the functionality is exact same so idk
func (mb *PeerInitMessageBuilder) PierceFirewall(token uint32) []byte {
	mb.AddInt32(token)
	return mb.Build(0)
}

func (mb *PeerInitMessageBuilder) PeerInit(username string, connType string, token uint32) []byte {
	mb.AddString(username)
	mb.AddString(connType)
	mb.AddInt32(token) // token - value is always 0
	// mb.AddInt32(0) // token - value is always 0
	return mb.Build(1)
}
