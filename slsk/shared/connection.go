package shared

import (
	"errors"
	"net"
)

type Connection struct {
	net.Conn
}

func (conn *Connection) SendMessage(message []byte) error {
	if conn == nil {
		return errors.New("connection is not established")
	}
	_, err := conn.Write(message)
	if err != nil {
		return err
	}
	return nil
}
