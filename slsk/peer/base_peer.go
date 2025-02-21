package peer

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"spotseek/slsk/messages"
	"spotseek/slsk/shared"
	"sync"
)

type Event int

const (
	PeerConnected Event = iota
	PeerDisconnected
	FileSearchResponse
	FolderContentsRequest
	PlaceInQueueResponse
	// Maximum allowed message size (32MB)
	MaxMessageSize = 32 * 1024 * 1024
)

type ConnectionType int

type BasePeer interface {
	ReadMessage() ([]byte, error)
	SendMessage([]byte) error
}

type Peer struct {
	Username         string                   `json:"username"`
	PeerConnection   *shared.Connection       `json:"-"` // Skip in JSON
	ConnType         string                   `json:"connType"`
	Token            uint32                   `json:"token"`
	Host             string                   `json:"host"`
	Port             uint32                   `json:"port"`
	Privileged       uint8                    `json:"privileged"`
	mgrCh            chan<- PeerEvent         `json:"-"` // Skip in JSON
	pendingTransfers map[uint32]*FileTransfer `json:"-"`
	transfersMutex   sync.RWMutex             `json:"-"`
}

type PeerEvent struct {
	Type Event
	Peer *Peer
	Data interface{}
}

type FolderContentsData struct {
	FolderName string
	Token      uint32
}

type FileSearchData struct {
	Token   uint32
	Results shared.SearchResult
}

type PlaceInQueueData struct {
	Filename string
	Place    uint32
}

type FileTransfer struct {
	Filename string
	Size     uint64
	Token    uint32
	Buffer   *bytes.Buffer
	Offset   uint64
}

func (p *Peer) SendMessage(msg []byte) error {
	if p == nil {
		return fmt.Errorf("tried to send message to peer but peer is nil")
	}
	if p.PeerConnection == nil {
		return fmt.Errorf("cannot send message to peer. no active connection")
	}
	return p.PeerConnection.SendMessage(msg)
}

func (peer *Peer) Listen() {
	readBuffer := make([]byte, 4096)
	var currentMessage []byte
	var messageLength uint32

	defer func() {
		peer.mgrCh <- PeerEvent{Type: PeerDisconnected, Peer: peer}
	}()

	for {
		n, err := peer.PeerConnection.Read(readBuffer)
		if err != nil {
			if err == io.EOF {
				log.Error("Peer closed the connection",
					"peer", peer.Username)
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				log.Error("Timeout reading from peer, retrying...",
					"peer", peer.Username)
				continue
			}
			log.Error("Error reading from peer",
				"peer", peer.Username,
				"err", err)
			return
		}

		currentMessage = append(currentMessage, readBuffer[:n]...)
		currentMessage, messageLength = peer.processMessage(currentMessage, messageLength)
	}
}

func (peer *Peer) ClosePeer() {
	peer.PeerConnection.Close()
}

func (peer *Peer) processMessage(data []byte, messageLength uint32) ([]byte, uint32) {
	if len(data) == 0 {
		return data, messageLength
	}
	for {
		if messageLength == 0 {
			if len(data) < 4 {
				return data, messageLength // Not enough data to read message length
			}
			messageLength = binary.LittleEndian.Uint32(data[:4])
			data = data[4:]
		}

		if uint32(len(data)) < messageLength {
			return data, messageLength // Not enough data for full message
		}

		// Handle valid message
		if err := peer.handlePeerMessage(data[:messageLength], messageLength); err != nil {
			log.Error("Error handling peer message",
				"err", err,
				"length", messageLength,
				"peer", peer.Username)
		}

		data = data[messageLength:]
		messageLength = 0
	}
}

func (peer *Peer) handlePeerMessage(messageData []byte, messageLength uint32) error {
	reader := messages.NewMessageReader(messageData)
	code := reader.ReadInt32()

	var decoded map[string]interface{}
	var err error

	switch peer.ConnType {
	case "P":
		decoded, err = peer.handleStandardMessage(code, reader, messageLength)
	// case "D":
	// decoded, err = peer.handleDistributedMessage(code, reader, messageLength)
	default:
		return fmt.Errorf("unknown connection type: %v", peer.ConnType)
	}

	if err != nil {
		return fmt.Errorf("error processing peer msg: %w", err)
	}

	log.Info("received message from peer",
		"code", code,
		"message", decoded,
		"peer", peer.Username)
	return nil
}

func (peer *Peer) handleStandardMessage(code uint32, reader *messages.MessageReader, messageLength uint32) (map[string]interface{}, error) {
	switch code {
	case 5:
		return peer.handleSharedFileListResponse(reader, messageLength)
	case 9:
		return peer.handleFileSearchResponse(reader, messageLength)
	case 36:
		return peer.handleFolderContentsRequest(reader)
	case 40:
		return peer.handleTransferRequest(reader)
	case 41:
		return peer.handleTransferResponse(reader)
	case 43:
		return peer.handleQueueUpload(reader)
	case 44:
		return peer.handlePlaceInQueueResponse(reader)
	case 46:
		return peer.handleUploadFailed(reader)
	case 50:
		return peer.handleUploadDenied(reader)
	case 51:
		return peer.handlePlaceInQueueRequest(reader)
	default:
		log.Error("Unsupported standard peer message code",
			"code", code,
			"peer", peer.Username)
		return nil, nil
	}
}

func (peer *Peer) handleSharedFileListResponse(reader *messages.MessageReader, messageLength uint32) (map[string]interface{}, error) {
	// Get the compressed content
	compressedData := reader.Message[reader.Pointer:messageLength]

	r, err := zlib.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		log.Error("Error creating zlib reader",
			"err", err,
			"compressed_length", len(compressedData))
		return nil, err
	}
	defer r.Close()

	decompressed, err := io.ReadAll(r)
	if err != nil {
		log.Error("Error decompressing message", "err", err)
		return nil, err
	}

	decompressedReader := messages.NewMessageReader(decompressed)

	result := shared.SharedFileList{}

	// Read number of directories
	dirCount := decompressedReader.ReadInt32()
	result.Directories = make([]shared.Directory, dirCount)

	// Iterate through directories
	for i := 0; i < int(dirCount); i++ {
		dir := shared.Directory{}
		dir.Name = decompressedReader.ReadString()

		// Read number of files in this directory
		fileCount := decompressedReader.ReadInt32()
		dir.Files = make([]shared.File, fileCount)

		// Iterate through files
		for j := 0; j < int(fileCount); j++ {
			_ = decompressedReader.ReadInt8() // code is always 1
			filename := decompressedReader.ReadString()
			size := decompressedReader.ReadInt64()
			extension := decompressedReader.ReadString() // extension

			// Read file attributes
			attrCount := decompressedReader.ReadInt32()
			var bitrate, duration uint32

			for k := 0; k < int(attrCount); k++ {
				attrType := decompressedReader.ReadInt32()
				if attrType == 0 {
					bitrate = decompressedReader.ReadInt32()
				} else if attrType == 1 {
					duration = decompressedReader.ReadInt32()
				}
			}

			dir.Files[j] = shared.File{
				Name:      filename,
				Size:      size,
				BitRate:   bitrate,
				Duration:  duration,
				Extension: extension,
			}
		}

		result.Directories[i] = dir
	}

	return map[string]interface{}{
		"type":   "SharedFileListResponse",
		"result": result,
	}, nil
}

func (peer *Peer) handleFolderContentsRequest(reader *messages.MessageReader) (map[string]interface{}, error) {
	token := reader.ReadInt32()
	folderName := reader.ReadString()
	peer.mgrCh <- PeerEvent{Type: FolderContentsRequest, Peer: peer, Data: FolderContentsData{Token: token, FolderName: folderName}}
	return map[string]interface{}{
		"type":   "FolderContentsRequest",
		"token":  token,
		"folder": folderName,
	}, nil
}

func (peer *Peer) handleFileSearchResponse(reader *messages.MessageReader, messageLength uint32) (map[string]interface{}, error) {
	// Get the compressed content using the message length
	compressedData := reader.Message[reader.Pointer:messageLength]

	log.Debug("Attempting to decompress data",
		"compressed_length", len(compressedData),
		"first_bytes", compressedData)

	r, err := zlib.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		log.Error("Error creating zlib reader",
			"err", err,
			"compressed_length", len(compressedData))
		return nil, err
	}

	defer r.Close()
	decompressed, err := io.ReadAll(r)
	if err != nil {
		log.Error("Error decompressing message", "err", err)
		return nil, err
	}

	decompressedReader := messages.NewMessageReader(decompressed)

	result := shared.SearchResult{}
	result.Username = decompressedReader.ReadString()
	result.Token = decompressedReader.ReadInt32()

	// Handle public files
	fileCount := decompressedReader.ReadInt32()
	result.PublicFiles = make([]shared.File, fileCount)

	for i := 0; i < int(fileCount); i++ {
		_ = decompressedReader.ReadInt8() // code is always 1
		filename := decompressedReader.ReadString()
		if filename == "" {
			continue
		}
		size := decompressedReader.ReadInt64()
		extension := decompressedReader.ReadString() // extension (always blank in Qt)
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

		result.PublicFiles[i] = shared.File{
			Name:      filename,
			Size:      size,
			BitRate:   bitrate,
			Duration:  duration,
			Extension: extension,
		}
	}

	result.SlotFree = decompressedReader.ReadInt8()
	result.AvgSpeed = decompressedReader.ReadInt32()
	result.QueueLength = decompressedReader.ReadInt32()

	peer.mgrCh <- PeerEvent{Type: FileSearchResponse, Peer: peer, Data: FileSearchData{Token: result.Token, Results: result}}

	return map[string]interface{}{
		"type":   "FileSearchResponse",
		"result": result,
	}, nil
}

func (peer *Peer) TransferRequest(direction uint32, token uint32, filename string, filesize uint64) []byte {
	mb := messages.NewMessageBuilder()
	mb.AddInt32(direction)
	mb.AddInt32(token)
	mb.AddString(filename)
	if direction == 1 {
		mb.AddInt64(filesize)
	}

	return mb.Build(40)
}

func (peer *Peer) handleTransferRequest(reader *messages.MessageReader) (map[string]interface{}, error) {
	direction := reader.ReadInt32()
	if direction != 0 && direction != 1 {
		return nil, fmt.Errorf("invalid transfer direction: %d", direction)
	}

	token := reader.ReadInt32()
	filename := reader.ReadString()

	result := map[string]interface{}{
		"direction": direction,
		"token":     token,
		"filename":  filename,
	}

	// For upload requests (direction == 1), read the file size
	// at this point we should have already sent a QueueUpload to the peer
	// This message is sent in response to our QueueUpload message
	// We send a TransferResponse to the peer to let them know we are ready to receive the file
	if direction == 1 {
		filesize := reader.ReadInt64()
		result["filesize"] = filesize

		// Initialize transfer tracking
		peer.transfersMutex.Lock()
		if peer.pendingTransfers == nil {
			peer.pendingTransfers = make(map[uint32]*FileTransfer)
		}
		peer.pendingTransfers[token] = &FileTransfer{
			Filename: filename,
			Size:     uint64(filesize),
			Token:    token,
			Buffer:   bytes.NewBuffer(make([]byte, 0, filesize)),
		}
		peer.transfersMutex.Unlock()

		// Tell the peer we're ready to receive
		// We expect to recieve an "F" connection after this
		// See slsk/client/listener.go for handling "F" connections
		peer.TransferResponse(token, true, uint64(filesize))
	}

	return map[string]interface{}{
		"type":   "TransferRequest",
		"result": result,
	}, nil
}

func (peer *Peer) TransferResponse(token uint32, allowed bool, filesize uint64) {
	mb := messages.NewMessageBuilder()

	mb.AddInt32(token)
	if allowed {
		mb.AddInt8(1)
		mb.AddInt64(filesize)
	} else {
		mb.AddInt8(0)
		mb.AddString("Cancelled") // placeholder for now
	}

	peer.SendMessage(mb.Build(41))
}

func (peer *Peer) handleTransferResponse(reader *messages.MessageReader) (map[string]interface{}, error) {
	token := reader.ReadInt32()
	allowed := reader.ReadInt8() == 1

	result := map[string]interface{}{
		"token":   token,
		"allowed": allowed,
	}

	if !allowed {
		result["reason"] = reader.ReadString()
	}

	return map[string]interface{}{
		"type":   "TransferResponse",
		"result": result,
	}, nil
}

// Tell the peer that we want to download a file, i.e. they should queue an upload on their end
func (peer *Peer) QueueUpload(filename string) {
	mb := messages.NewMessageBuilder()
	mb.AddString(filename)

	peer.SendMessage(mb.Build(43))
}

// Tells us that we should queue a file for transfer. Send transfer request here
func (peer *Peer) handleQueueUpload(reader *messages.MessageReader) (map[string]interface{}, error) {
	filename := reader.ReadString()
	token := rand.Uint32()

	peer.TransferRequest(1, token, filename, 0)

	return map[string]interface{}{
		"type":   "QueueUpload",
		"result": filename,
	}, nil
}

func (peer *Peer) handlePlaceInQueueResponse(reader *messages.MessageReader) (map[string]interface{}, error) {
	filename := reader.ReadString()
	place := reader.ReadInt32()

	peer.mgrCh <- PeerEvent{Type: PlaceInQueueResponse, Peer: peer, Data: PlaceInQueueData{Filename: filename, Place: place}}

	return map[string]interface{}{
		"type":     "PlaceInQueueResponse",
		"filename": filename,
		"place":    place,
	}, nil
}

func (peer *Peer) PlaceInQueueResponse(filename string, place uint32) {
	mb := messages.NewMessageBuilder()
	mb.AddString(filename)
	mb.AddInt32(place)

	peer.SendMessage(mb.Build(44))
}

// UploadFailed handling
func (peer *Peer) handleUploadFailed(reader *messages.MessageReader) (map[string]interface{}, error) {
	filename := reader.ReadString()
	return map[string]interface{}{
		"type":     "UploadFailed",
		"filename": filename,
	}, nil
}

func (peer *Peer) UploadFailed(filename string) {
	mb := messages.NewMessageBuilder()
	mb.AddString(filename)

	peer.SendMessage(mb.Build(46))
}

// UploadDenied handling
func (peer *Peer) handleUploadDenied(reader *messages.MessageReader) (map[string]interface{}, error) {
	filename := reader.ReadString()
	reason := reader.ReadString()

	return map[string]interface{}{
		"type":     "UploadDenied",
		"filename": filename,
		"reason":   reason,
	}, nil
}

func (peer *Peer) UploadDenied(filename string, reason string) {
	mb := messages.NewMessageBuilder()
	mb.AddString(filename)
	mb.AddString(reason)
	peer.SendMessage(mb.Build(50))
}

func (peer *Peer) handlePlaceInQueueRequest(reader *messages.MessageReader) (map[string]interface{}, error) {
	filename := reader.ReadString()
	return map[string]interface{}{
		"type":     "PlaceInQueueRequest",
		"filename": filename,
	}, nil
}

func (peer *Peer) PlaceInQueueRequest(filename string, place uint32) {
	mb := messages.NewMessageBuilder()
	mb.AddString(filename)

	peer.SendMessage(mb.Build(51))
}

// typical order of operations for searching and downloading
// 1. we send FileSearch to server
// 2. peers send FileSearchResponse to us
// 3. we pick a peer and send a QueueUpload to them
// 4. peer sends PlaceInQueueResponse to us?
// 5. peer sends a TransferRequest to us
// 6. we send a TransferResponse to the peer
// 7. ???
