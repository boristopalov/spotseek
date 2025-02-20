package messages

type PeerInitMessageReader struct {
	*MessageReader
}

func (mr *PeerInitMessageReader) ParsePeerInit() (string, string, uint32) {
	return mr.ReadString(), mr.ReadString(), mr.ReadInt32()
}

func (mr *PeerInitMessageReader) ParsePierceFirewall() uint32 {
	return mr.ReadInt32()
}
