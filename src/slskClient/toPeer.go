package slskClient

import (
	"spotseek/src/messages"
	"fmt"
	"net"
	"time"
)

type Peer struct { 
	Username string
	Listener net.Listener
	Conn *Server
	ConnType string
	Token uint32
	Host string
	Port uint32
}


func NewPeer(username string, listener net.Listener, connType string, token uint32, host string, port uint32) *Peer {
	c, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 10*time.Second)
	if err != nil {
	  fmt.Println(err)
		return nil 
	} else { 
		fmt.Printf("established TCP connection to %s:%d\n", host, port)
	}
	return &Peer{
		Username: username,
		Conn: &Server{Conn: c},
		Listener: listener,
		ConnType: connType,
		Token: token,
		Host: host,
		Port: port,
	}
}

func (peer *Peer) PeerInit(username string, connType string, token uint32) error { 
	msg := messages.NewMessageBuilder().PeerInit(username, connType, token)
	err := peer.Conn.SendMessage(msg)
	return err
}

func (peer *Peer) PierceFirewall(token uint32) error { 
	msg := messages.NewMessageBuilder().PierceFirewall(token)
	err := peer.Conn.SendMessage(msg)
	return err
}

func (peer *Peer) QueueUpload(filename string) error { 
	msg := messages.NewMessageBuilder().QueueUpload(filename)
	err := peer.Conn.SendMessage(msg)
	return err
}

func (peer *Peer) UserInfoRequest() error { 
	msg := messages.NewMessageBuilder().UserInfoRequest()
	err := peer.Conn.SendMessage(msg)
	return err
}
