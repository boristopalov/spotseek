package peer

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"spotseek/src/slsk/listen"
	"time"
)

type BasePeer interface {
	ReadMessage() ([]byte, error)
	SendMessage([]byte) error
}

type Peer struct {
	Username       string
	SlskListener   net.Listener
	PeerConnection *listen.Connection
	ConnType       string
	Token          uint32
	Host           string
	Port           uint32
}

func (p *Peer) ReadMessage() ([]byte, error) {
	sizeBuf := make([]byte, 4)
	_, err := io.ReadFull(p.PeerConnection, sizeBuf)
	if err != nil {
		return nil, fmt.Errorf("error reading message size: %v", err)
	}
	size := binary.LittleEndian.Uint32(sizeBuf)

	message := make([]byte, size)
	_, err = io.ReadFull(p.PeerConnection, message)
	if err != nil {
		return nil, fmt.Errorf("error reading message: %v", err)
	}

	return message, nil
}

func (p *Peer) SendMessage(msg []byte) error {
	return p.PeerConnection.SendMessage(msg)
}

func NewPeer(username string, connType string, token uint32, host string, port uint32) (*Peer, error) {
	c, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("unable to establish connection to peer %s: %v", username, err)
	} else {
		log.Printf("established TCP connection to %s:%d\n", host, port)
		return &Peer{
			Username:       username,
			PeerConnection: &listen.Connection{Conn: c},
			ConnType:       connType,
			Token:          token,
			Host:           host,
			Port:           port,
		}, nil
	}
}
