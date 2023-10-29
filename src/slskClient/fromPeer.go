package slskClient

import (
	"spotseek/src/messages"
	"fmt"
)



func (peer *Peer) HandlePeerMessage(mr *messages.MessageReader) (map[string]interface{}, error) { 
	fmt.Println("hey this is a peer msg lol")
	return nil, nil
}

// func (c *SlskClient) HandleFileSearchResponse(mr *messages.MessageReader) (map[string]interface{}, error) {
// }

// func (c *SlskClient) HandleSharedFileListResponse(mr *messages.MessageReader) (error) { 

// }
