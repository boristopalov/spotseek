package peerMessages

import "spotseek/src/slsk/messages"

type PeerInitMessageReader struct {
	*messages.MessageReader
}

// func (mr *PeerInitMessageReader) HandlePeerInitMessage() ([]byte, error)

func (mr *PeerInitMessageReader) ParsePeerInit() (string, string, uint32) {
	return mr.ReadString(), mr.ReadString(), mr.ReadInt32()
}

func (mr *PeerInitMessageReader) ParsePierceFirewall() uint32 {
	return mr.ReadInt32()
}
