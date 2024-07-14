package peer

import (
	"log"
	"spotseek/src/slsk/messages/peerMessages"
)

func (peer *Peer) HandlePeerMessage(mr *peerMessages.PeerMessageReader) (map[string]interface{}, error) {
	log.Println("hey this is a peer msg lol")
	return nil, nil
}

// func (c *SlskClient) HandleFileSearchResponse(mr *messages.MessageReader) (map[string]interface{}, error) {
// }

// func (c *SlskClient) HandleSharedFileListResponse(mr *messages.MessageReader) (error) {

// }

// func (peer *Peer) HandleFileSearchResponse(mr *peerMessages.PeerMessageReader) (uint32, []shared.SearchResult) {

// 	// c.SearchResults[token] = append(c.SearchResults[token], results...)
// 	// return nil
// }
