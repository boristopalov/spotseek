package peerMessages

import (
	// "spotseek/src/slskClient/peer"
	"fmt"
	"log"
	"spotseek/src/slskClient/messages"
)

type PeerMessageReader struct {
	*messages.MessageReader
}

func (mr *PeerMessageReader) HandlePeerMessage() (map[string]interface{}, error) {
	messageLength := mr.ReadInt32()
	log.Println("Message length frmo peer", messageLength)
	code := mr.ReadInt32()
	if code < 1 {
		return nil, fmt.Errorf("invalid peer code. Received code %d", code)
	}
	var decoded map[string]interface{}
	// switch code {
	// case 1:
	// 	mb := NewMessageBuilder()
	// 	mb.ShareFileList().SendMessageToPeer()
	// case 4:
	// 	decoded = mr.HandleGetSharedFileList()
	// case 5:
	// 	decoded = mr.HandleSharedFileListResponse()
	// case 9:
	// 	decoded = mr.HandleFileSearchResponse()
	// case 15:
	// 	decoded = mr.HandleUserInfoRequest()
	// case 16:
	// 	decoded = mr.HandleUserInfoResponse()
	// case 36:
	// 	decoded = mr.HandleFolderContentsRequest()
	// case 37:
	// 	decoded = mr.HandleFolderContentsResponse()
	// case 40:
	// 	decoded = mr.HandleTransferRequest()
	// case 41:
	// 	decoded = mr.HandleUploadResponse()
	// case 43:
	// 	decoded = mr.HandleQueueUpload()
	// case 44:
	// 	decoded = mr.HandlePlaceInQueueResponse()
	// case 46:
	// 	decoded = mr.HandleUploadFailed()
	// case 50:
	// 	decoded = mr.HandleUploadDenied()
	// case 51:
	// 	decoded = mr.HandlePlaceInQueueRequest()
	// default:
	// 	log.Println("Unsupported peer message code!", code)
	// }
	return decoded, nil
}

func (mr *PeerMessageReader) HandleGetSharedFileList() {

}

func (mr *PeerMessageReader) HandleFileSearchResponse() {

}
