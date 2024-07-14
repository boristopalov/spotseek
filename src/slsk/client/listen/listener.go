package listen

import (
	"encoding/binary"
	"errors"
	"log"
	"net"
)

type Listener struct {
	net.Conn
}

func (listener *Listener) SendMessage(message []byte) error {
	if listener == nil {
		return errors.New("connection is not established")
	}
	_, err := listener.Write(message)
	if err != nil {
		log.Println(err)
		return err
	}

	log.Println("------------------- Sent message to Soulseek server ----------------")
	log.Printf("Code %d; Message: %s", binary.LittleEndian.Uint32(message[4:8]), string(message[8:]))
	return nil
}

func NewServer() *Listener {
	return &Listener{}
}
