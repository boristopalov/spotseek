package network

import (
	"encoding/binary"
	"errors"
	"log"
	"net"
)

type Connection struct {
	net.Conn
}

func (listener *Connection) SendMessage(message []byte) error {
	if listener == nil {
		return errors.New("connection is not established")
	}
	_, err := listener.Write(message)
	if err != nil {
		log.Println(err)
		return err
	}

	log.Printf("------------------- Sent message to %s ----------------", listener.RemoteAddr().String())
	log.Printf("Sent code %d; Message: %s", binary.LittleEndian.Uint32(message[4:8]), string(message[8:]))
	log.Println("------------------- End of message ----------------")
	return nil
}
