package peer

import (
	"log"
    "spotseek/src/slskClient/messages/peerMessages"
)


func (peer *Peer) HandlePeerMessage(mr *peerMessages.PeerMessageReader) (map[string]interface{}, error) { 
	log.Println("hey this is a peer msg lol")
	return nil, nil
}

// func (c *SlskClient) HandleFileSearchResponse(mr *messages.MessageReader) (map[string]interface{}, error) {
// }

// func (c *SlskClient) HandleSharedFileListResponse(mr *messages.MessageReader) (error) { 

// }
