package serverListener

import (
	"errors"
	"log"
	"net"
)

type ServerListener struct {
	net.Conn
}

func (server *ServerListener) SendMessage(message []byte) error {
	if server == nil {
		return errors.New("connection is not established")
	}
	_, err := server.Write(message)
	if err != nil {
		log.Println(err)
		return err
	}
	log.Println("sent message to Soulseek server....", string(message))
	return nil
}

func NewServer() *ServerListener {
	return &ServerListener{}
}
