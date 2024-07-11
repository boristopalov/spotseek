package peer

import (
	"encoding/binary"
	"io"
	"net"
	"spotseek/src/slsk/client/serverListener"
)

type Peer struct {
	Username string
	Listener net.Listener
	Conn     *serverListener.ServerListener
	ConnType string
	Token    uint32
	Host     string
	Port     uint32
}

func (p *Peer) ReadMessage() ([]byte, error) {
	sizeBuf := make([]byte, 4)
	_, err := io.ReadFull(p.Conn, sizeBuf)
	if err != nil {
		return nil, err
	}
	size := binary.LittleEndian.Uint32(sizeBuf)

	message := make([]byte, size)
	_, err = io.ReadFull(p.Conn, message)
	if err != nil {
		return nil, err
	}

	return message, nil
}
