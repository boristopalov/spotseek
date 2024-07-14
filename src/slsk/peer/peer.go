package peer

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"spotseek/src/slsk/client/listen"
	"spotseek/src/slsk/messages"
	"spotseek/src/slsk/messages/peerMessages"
	"time"
)

type Peer struct {
	Username string
	Listener net.Listener
	Conn     *listen.Listener
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

func NewPeer(username string, listener net.Listener, connType string, token uint32, host string, port uint32) (*Peer, error) {
	c, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("unable to establish connection to peer %s: %v", username, err)
	} else {
		log.Printf("established TCP connection to %s:%d\n", host, port)
		return &Peer{
			Username: username,
			Conn:     &listen.Listener{Conn: c},
			Listener: listener,
			ConnType: connType,
			Token:    token,
			Host:     host,
			Port:     port,
		}, nil
	}
}

// PeerInit but whatever
func (peer *Peer) PeerInit(username string, connType string, token uint32) error {
	mb := peerMessages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.PeerInit(username, connType, token)
	err := peer.Conn.SendMessage(msg)
	return err
}

// PeerInit but whatever
func (peer *Peer) PierceFirewall(token uint32) error {
	mb := peerMessages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.PierceFirewall(token)
	err := peer.Conn.SendMessage(msg)
	return err
}

func (peer *Peer) QueueUpload(filename string) error {
	mb := peerMessages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.QueueUpload(filename)
	err := peer.Conn.SendMessage(msg)
	return err
}

func (peer *Peer) SharedFileListRequest() error {
	mb := peerMessages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.SharedFileListRequest()
	err := peer.Conn.SendMessage(msg)
	return err
}

func (peer *Peer) UserInfoRequest() error {
	mb := peerMessages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.UserInfoRequest()
	err := peer.Conn.SendMessage(msg)
	return err
}

func (peer *Peer) PlaceInQueueRequest(filename string) error {
	mb := peerMessages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.PlaceInQueueRequest(filename)
	err := peer.Conn.SendMessage(msg)
	return err
}

func (peer *Peer) FileTransferInit(token uint32) error {
	mb := peerMessages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.FileTransferInit(token)
	err := peer.Conn.SendMessage(msg)
	return err
}

func (peer *Peer) FileOffset(offset uint64) error {
	mb := peerMessages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.FileOffset(offset)
	err := peer.Conn.SendMessage(msg)
	return err
}
