package peer

import (
	"fmt"
	"log"
	"net"
	"spotseek/src/slsk/client/serverListener"
	"spotseek/src/slsk/messages"
	"spotseek/src/slsk/messages/peerMessages"
	"time"
)

func NewPeer(username string, listener net.Listener, connType string, token uint32, host string, port uint32) (*Peer, error) {
	c, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("unable to establish connection to peer %s: %v", username, err)
	} else {
		log.Printf("established TCP connection to %s:%d\n", host, port)
		return &Peer{
			Username: username,
			Conn:     &serverListener.ServerListener{Conn: c},
			Listener: listener,
			ConnType: connType,
			Token:    token,
			Host:     host,
			Port:     port,
		}, nil
	}
}

func (peer *Peer) PeerInit(username string, connType string, token uint32) error {
	mb := peerMessages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.PeerInit(username, connType, token)
	err := peer.Conn.SendMessage(msg)
	return err
}

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

func (peer *Peer) UserInfoRequest() error {
	mb := peerMessages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.UserInfoRequest()
	err := peer.Conn.SendMessage(msg)
	return err
}
