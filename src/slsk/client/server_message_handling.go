package client

import (
	"encoding/binary"
	"log"
	"spotseek/src/slsk/messages"
)

// SERVER MESSAGE HANDLING
func (c *SlskClient) ListenForServerMessages() {
	readBuffer := make([]byte, 4096)
	var currentMessage []byte
	var messageLength uint32

	for {
		n, err := c.ServerConnection.Read(readBuffer)
		if err != nil {
			log.Printf("Error reading from server connection: %v", err)
			return
		}

		currentMessage = append(currentMessage, readBuffer[:n]...)
		currentMessage = c.processServerMessages(currentMessage, &messageLength)
	}
}

func (c *SlskClient) processServerMessages(data []byte, messageLength *uint32) []byte {
	for {
		if *messageLength == 0 {
			if len(data) < 4 {
				return data // Not enough data to read message length
			}
			*messageLength = binary.LittleEndian.Uint32(data[:4])
			data = data[4:]
		}

		if uint32(len(data)) < *messageLength {
			return data // Not enough data for full message
		}

		c.handleServerMessage(data[:*messageLength])

		data = data[*messageLength:]
		*messageLength = 0
	}
}

func (c *SlskClient) handleServerMessage(messageData []byte) {
	mr := messages.NewMessageReader(messageData)
	serverMsgReader := messages.ServerMessageReader{MessageReader: mr}

	msg, err := c.HandleServerMessage(&serverMsgReader)
	if err != nil {
		log.Printf("Error decoding server message: %v", err)
	} else {
		log.Printf("Server message: message: %v", msg)
	}
	log.Println("--------------- End of message ----------------")
}
