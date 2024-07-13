package serverListener

import (
	"encoding/binary"
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

	log.Println("------------------- Sent message to Soulseek server ----------------")
	log.Printf("Code %d; Message: %s", binary.LittleEndian.Uint32(message[4:8]), string(message[8:]))
	return nil
}

func NewServer() *ServerListener {
	return &ServerListener{}
}
