package peerMessages

import (
	// "spotseek/src/slskClient/peer"
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"log"
	"spotseek/src/slsk/messages"
	"spotseek/src/slsk/shared"
)

type PeerMessageReader struct {
	*messages.MessageReader
}

func (mr *PeerMessageReader) HandlePeerMessage() (map[string]interface{}, error) {
	// messageLength := mr.ReadInt32()
	// log.Println("Message length frmo peer", messageLength)
	code := mr.ReadInt32()
	if code < 1 {
		return nil, fmt.Errorf("invalid peer code. Received code %d", code)
	}
	var decoded map[string]interface{}
	var err error
	switch code {
	case 1: // PeerInit
		username, connType, token := mr.ParsePeerInit()
		log.Printf("Received PeerInit from %s with connType %s and token %d", username, connType, token)
		// Handle PeerInit
	case 4: // SharedFileList
		// Handle SharedFileList
	// Add more cases for other peer message types
	// case 4:
	// 	decoded = mr.HandleGetSharedFileList()
	// case 5:
	// 	decoded = mr.HandleSharedFileListResponse()
	case 9:
		decoded, err = mr.HandleFileSearchResponse()
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
	default:
		log.Println("Unsupported peer message code!", code)
	}
	return decoded, err
}

func (mr *PeerMessageReader) ParsePeerInit() (string, string, uint32) {
	return mr.ReadString(), mr.ReadString(), mr.ReadInt32()
}

func (mr *PeerMessageReader) HandleGetSharedFileList() {

}

// peers will send us this after we call FileSearch with their matches
func (mr *PeerMessageReader) HandleFileSearchResponse() (map[string]interface{}, error) {
	r, err := zlib.NewReader(bytes.NewReader(mr.Message))
	if err != nil {
		log.Printf("Error decompressing message: %s", err)
		return nil, err
	}

	defer r.Close()
	decompressed, err := io.ReadAll(r)
	if err != nil {
		log.Printf("Error decompressing message: %s", err)
		return nil, err
	}

	// Create a new MessageReader for the decompressed data
	decompressedReader := &PeerMessageReader{
		MessageReader: messages.NewMessageReader(decompressed),
	}

	username := decompressedReader.ReadString()
	token := decompressedReader.ReadInt32()
	fileCount := decompressedReader.ReadInt32()
	results := make([]shared.SearchResult, fileCount)
	for i := 0; i < int(fileCount); i++ {
		_ = decompressedReader.ReadInt8() // code is always 1
		filename := decompressedReader.ReadString()
		size := decompressedReader.ReadInt64()
		_ = decompressedReader.ReadString()
		attributeCount := decompressedReader.ReadInt32()

		var bitrate, duration uint32
		for j := 0; j < int(attributeCount); j++ {
			attrType := decompressedReader.ReadInt32()
			if attrType == 0 {
				bitrate = decompressedReader.ReadInt32()
			} else if attrType == 1 {
				duration = decompressedReader.ReadInt32()
			}
		}

		results[i] = shared.SearchResult{
			Username: username,
			Filename: filename,
			Size:     size,
			BitRate:  bitrate,
			Duration: duration,
		}
	}

	return map[string]interface{}{
		"type":    "FileSearchResponse",
		"token":   token,
		"results": results,
	}, nil
}
