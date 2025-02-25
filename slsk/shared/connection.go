package shared

import (
	"errors"
	"net"
	"spotseek/logging"
)

var log = logging.GetLogger()

type Connection struct {
	net.Conn
}

func (conn *Connection) SendMessage(message []byte) error {
	if conn == nil {
		return errors.New("connection is not established")
	}
	_, err := conn.Write(message)
	if err != nil {
		log.Error("Error Sending Message", "err", err)
		return err
	}

	// log.Info("sending message",
	// 	"remoteAddr", conn.RemoteAddr().String(),
	// 	"code", binary.LittleEndian.Uint32(message[4:8]),
	// 	"length", binary.LittleEndian.Uint32(message[:4]),
	// 	"message", string(message[8:]))
	return nil
}
