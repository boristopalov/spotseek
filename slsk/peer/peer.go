package peer

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"os"
	"spotseek/slsk/fileshare"
	"spotseek/slsk/messages"
	"spotseek/slsk/shared"
	"sync"
)

type Event int

const (
	PeerDisconnected Event = iota
	FileSearchResponse
	SharedFileListRequest
	FolderContentsRequest
	PlaceInQueueResponse
	UploadRequest
	BranchLevel
	BranchRoot
	TransferRequest
	UploadComplete
	// Maximum allowed message size (32MB)
	MaxMessageSize = 32 * 1024 * 1024
)

type ConnectionType int

type BasePeer interface {
	ReadMessage() ([]byte, error)
	SendMessage([]byte) error
}

type Peer struct {
	Username         string                      `json:"username"`
	PeerConnection   *shared.Connection          `json:"-"` // Skip in JSON
	ConnType         string                      `json:"connType"`
	Token            uint32                      `json:"token"`
	Host             string                      `json:"host"`
	Port             uint32                      `json:"port"`
	Privileged       uint8                       `json:"privileged"`
	mgrCh            chan<- PeerEvent            `json:"-"` // Skip in JSON
	distribSearchCh  chan<- DistribSearchMessage `json:"-"`
	fileTransferCh   chan<- struct{}             `json:"-"`
	pendingTransfers map[uint32]*FileTransfer    `json:"-"` // transfers that we request
	transfersMutex   sync.RWMutex                `json:"-"`
	logger           *slog.Logger                `json:"-"`
	mu               sync.RWMutex                `json:"-"`
}

type PeerEvent struct {
	Type Event
	Peer *Peer
	Data any
}

type FolderContentsData struct {
	FolderName string
	Token      uint32
}

type SharedFileListMessage struct{}

type FileSearchData struct {
	Token   uint32
	Results shared.SearchResult
}

type PlaceInQueueData struct {
	Filename string
	Place    uint32
}

type TransferRequestMessage struct {
	Token        uint32
	Filename     string
	PeerUsername string
	Size         uint64
}

type UploadCompleteData struct {
	Token    uint32
	Filename string
}

// type PlaceInQueueRequestData struct {
// 	Filename string
// }

type FileTransfer struct {
	Filename     string
	Size         uint64
	PeerUsername string
	Token        uint32
	Buffer       *bytes.Buffer
	Offset       uint64
}

type DistribSearchMessage struct {
	Username string
	Token    uint32
	Query    string
}

type BranchLevelMessage struct {
	BranchLevel uint32
}

type BranchRootMessage struct {
	BranchRootUsername string
}

type UploadRequestMessage struct {
	Filename string
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

func (peer *Peer) Close() {
	peer.mu.Lock()
	defer peer.mu.Unlock()
	peer.PeerConnection.Close()
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
				peer.logger.Warn("Peer closed the connection",
					"peer", peer.Username)
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				peer.logger.Error("Timeout reading from peer, retrying...",
					"peer", peer.Username)
				continue
			}
			peer.logger.Error("Error reading from peer",
				"peer", peer.Username,
				"err", err)
			return
		}
		peer.logger.Debug("received message from peer",
			"length", n,
			"peer", peer.Username)
		currentMessage = append(currentMessage, readBuffer[:n]...)
		currentMessage, messageLength = peer.processMessage(currentMessage, messageLength)
	}
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

		defer func() {
			if r := recover(); r != nil {
				peer.logger.Error("recovered from panic",
					"error", r,
				)
				// Optionally log the stack trace
				// debug.PrintStack()
			}
		}()

		if peer.ConnType == "P" {
			// Handle valid message
			if err := peer.handleMessage(data[:messageLength], messageLength); err != nil {
				peer.logger.Error("Error handling peer message",
					"err", err,
					"length", messageLength,
					"peer", peer.Username)
			}

		} else if peer.ConnType == "D" {
			p := NewDistributedPeer(peer)
			if err := p.handleMessage(data[:messageLength]); err != nil {
				peer.logger.Error("Error handling peer message",
					"err", err,
					"length", messageLength,
					"peer", peer.Username)
			}
		} else {
			peer.logger.Error("Unsupported connection type when handling message", "connType", peer.ConnType, "peer", peer)
			return nil, 0
		}

		data = data[messageLength:]
		messageLength = 0

		// Todo: This fixes an infinite loop issue, but why?
		if len(data) == 0 {
			return data, messageLength
		}
	}
}

func (peer *Peer) handleMessage(messageData []byte, messageLength uint32) error {
	reader := messages.NewMessageReader(messageData)
	code := reader.ReadInt32()
	peer.logger.Debug("handling message",
		"code", code,
		"peer", peer.Username)

	var decoded map[string]any
	var err error
	switch code {
	case 4:
		decoded, err = peer.handleGetSharedFileList(reader)
	case 5:
		decoded, err = peer.handleSharedFileListResponse(reader, messageLength)
	case 9:
		decoded, err = peer.handleFileSearchResponse(reader, messageLength)
	case 36:
		decoded, err = peer.handleFolderContentsRequest(reader)
	case 40:
		decoded, err = peer.handleTransferRequest(reader)
	case 41:
		decoded, err = peer.handleTransferResponse(reader)
	case 43:
		decoded, err = peer.handleQueueUpload(reader)
	case 44:
		decoded, err = peer.handlePlaceInQueueResponse(reader)
	case 46:
		decoded, err = peer.handleUploadFailed(reader)
	case 50:
		decoded, err = peer.handleUploadDenied(reader)
	case 51:
		decoded, err = peer.handlePlaceInQueueRequest(reader)
	default:
		peer.logger.Error("Unsupported standard peer message code",
			"code", code,
			"peer", peer.Username)
		return fmt.Errorf("unsupported message code %d", code)
	}

	if err != nil {
		return fmt.Errorf("error processing peer msg: %w", err)
	}

	peer.logger.Info("received message from peer",
		"code", code,
		"message", decoded,
		"peer", peer.Username)
	return nil
}

// this is too much jumping around
func (peer *Peer) handleGetSharedFileList(reader *messages.MessageReader) (map[string]any, error) {
	peer.mgrCh <- PeerEvent{
		Type: SharedFileListRequest,
		Peer: peer,
		Data: SharedFileListMessage{},
	}

	peer.logger.Info("Received SharedFileListRequest", "peer", peer.Username)
	return nil, nil
}

func (peer *Peer) SharedFileListResponse(shares *fileshare.Shared) {
	mb := messages.NewMessageBuilder()
	mb.AddInt32(shares.GetShareStats().TotalFolders)

	// currently only one directory
	mb.AddString(shares.Files[0].Dir)
	mb.AddInt32(shares.GetShareStats().TotalFiles)

	for _, file := range shares.Files {
		mb.AddInt8(1) // Code - value is always 1
		mb.AddString(file.Key)
		mb.AddInt64(uint64(file.Value.Size))
		mb.AddString(file.Value.Extension)

		mb.AddInt32(3) // 3 file attributes

		// Bitrate
		mb.AddInt32(0) // code 0
		mb.AddInt32(uint32(file.Value.BitRate))

		// Duration
		mb.AddInt32(1) // code 1
		mb.AddInt32(uint32(file.Value.DurationSeconds))

		// Sample rate
		mb.AddInt32(4) // code 4
		mb.AddInt32(uint32(file.Value.SampleRate))
	}
	mb.AddInt32(0) // unknown
	mb.AddInt32(0) // private diectories

	// zlib compress
	var compressedData bytes.Buffer
	zlibWriter := zlib.NewWriter(&compressedData)
	zlibWriter.Write(mb.Message)
	zlibWriter.Close()
	mb.Message = compressedData.Bytes()

	peer.logger.Info("Sending SharedFileListResponse", "peer", peer.Username)
	peer.SendMessage(mb.Build(5))
}

func (peer *Peer) handleSharedFileListResponse(reader *messages.MessageReader, messageLength uint32) (map[string]any, error) {
	// Get the compressed content
	compressedData := reader.Message[reader.Pointer:messageLength]

	r, err := zlib.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		peer.logger.Error("Error creating zlib reader",
			"err", err,
			"compressed_length", len(compressedData))
		return nil, err
	}
	defer r.Close()

	decompressed, err := io.ReadAll(r)
	if err != nil {
		peer.logger.Error("Error decompressing message", "err", err)
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

	peer.logger.Info("Received SharedFileListResponse", "peer", peer.Username)
	return map[string]any{
		"type":   "SharedFileListResponse",
		"result": result,
	}, nil
}

func (peer *Peer) handleFolderContentsRequest(reader *messages.MessageReader) (map[string]any, error) {
	token := reader.ReadInt32()
	folderName := reader.ReadString()
	peer.mgrCh <- PeerEvent{Type: FolderContentsRequest, Peer: peer, Data: FolderContentsData{Token: token, FolderName: folderName}}
	peer.logger.Info("Received FolderContentsRequest", "peer", peer.Username)
	return map[string]any{
		"type":   "FolderContentsRequest",
		"token":  token,
		"folder": folderName,
	}, nil
}

func (peer *Peer) handleFileSearchResponse(reader *messages.MessageReader, messageLength uint32) (map[string]any, error) {
	// Get the compressed content using the message length
	compressedData := reader.Message[reader.Pointer:messageLength]

	peer.logger.Debug("Attempting to decompress data",
		"compressed_length", len(compressedData),
		"first_bytes", compressedData)

	r, err := zlib.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		peer.logger.Error("Error creating zlib reader",
			"err", err,
			"compressed_length", len(compressedData))
		return nil, err
	}

	defer r.Close()
	decompressed, err := io.ReadAll(r)
	if err != nil {
		peer.logger.Error("Error decompressing message", "err", err)
		return nil, err
	}

	decompressedReader := messages.NewMessageReader(decompressed)

	result := shared.SearchResult{}
	result.Username = decompressedReader.ReadString()
	result.Token = decompressedReader.ReadInt32()

	// Handle public files
	fileCount := decompressedReader.ReadInt32()
	if fileCount < 1 {
		return nil, nil
	}
	result.PublicFiles = make([]shared.File, fileCount)

	for i := range int(fileCount) {
		_ = decompressedReader.ReadInt8() // code is always 1
		filename := decompressedReader.ReadString()
		if filename == "" {
			continue
		}
		size := decompressedReader.ReadInt64()
		extension := decompressedReader.ReadString() // extension (always blank in Qt)
		attributeCount := decompressedReader.ReadInt32()

		var bitrate, duration, sampleRate uint32
		for j := range int(attributeCount) {
			var _ = j
			attrType := decompressedReader.ReadInt32()
			if attrType == 0 {
				bitrate = decompressedReader.ReadInt32()
			} else if attrType == 1 {
				duration = decompressedReader.ReadInt32()
			} else if attrType == 4 {
				sampleRate = decompressedReader.ReadInt32()
			}
		}

		result.PublicFiles[i] = shared.File{
			Name:       filename,
			Size:       size,
			BitRate:    bitrate,
			Duration:   duration,
			SampleRate: sampleRate,
			Extension:  extension,
		}
	}

	result.SlotFree = decompressedReader.ReadInt8()
	result.AvgSpeed = decompressedReader.ReadInt32()
	result.QueuePosition = decompressedReader.ReadInt32()

	peer.mgrCh <- PeerEvent{Type: FileSearchResponse, Peer: peer, Data: FileSearchData{Token: result.Token, Results: result}}

	peer.logger.Info("Received FileSearchResponse", "peer", peer.Username, "results", result)
	return map[string]any{
		"type":   "FileSearchResponse",
		"result": result,
	}, nil
}

func (peer *Peer) FileSearchResponse(username string, token uint32, results []fileshare.SharedFile) {
	// Create a message builder for the uncompressed data
	mb := messages.NewMessageBuilder()

	// Add username and token
	mb.AddString(username)
	mb.AddInt32(token)

	// Add number of public results
	mb.AddInt32(uint32(len(results)))

	// Add each public file
	for _, file := range results {
		mb.AddInt8(1) // code is always 1
		mb.AddString(file.VirtualPath)
		mb.AddInt64(uint64(file.Value.Size))
		mb.AddString(file.Value.Extension)

		// Add attributes
		mb.AddInt32(3) // 3 attributes for now (Bitrate, Duration, SampleRate)
		// Bitrate
		mb.AddInt32(0)
		mb.AddInt32(uint32(file.Value.BitRate))
		// Duration
		mb.AddInt32(1)
		mb.AddInt32(uint32(file.Value.DurationSeconds))
		// Sample rate
		mb.AddInt32(4)
		mb.AddInt32(uint32(file.Value.SampleRate))
	}

	mb.AddInt32(1) // TODO: Slotfree

	mb.AddInt32(1024 * 1024) // avgspeed in bytes -- hardcode 1mb/s for now

	mb.AddInt32(0) // unknown
	mb.AddInt32(0) // private directories

	// zlib compress
	var compressedData bytes.Buffer
	zlibWriter := zlib.NewWriter(&compressedData)
	zlibWriter.Write(mb.Message)
	zlibWriter.Close()
	mb.Message = compressedData.Bytes()

	peer.logger.Info("Sending FileSearchResponse", "peer", peer.Username, "token", token, "results", results)
	peer.SendMessage(mb.Build(9))
}

func (peer *Peer) TransferRequest(direction uint32, filename string, filesize uint64) {
	mb := messages.NewMessageBuilder()
	token := uint32(rand.Int32())
	mb.AddInt32(direction)
	mb.AddInt32(token)
	mb.AddString(filename)
	if direction == 1 {
		mb.AddInt64(filesize)
	}

	peer.mgrCh <- PeerEvent{
		Type: TransferRequest,
		Peer: peer,
		Data: TransferRequestMessage{
			Token:        token,
			Filename:     filename,
			PeerUsername: peer.Username,
			Size:         filesize,
		},
	}

	peer.logger.Info("Sending TransferRequest", "filename", filename, "peer", peer.Username)
	peer.SendMessage(mb.Build(40))
}

func (peer *Peer) handleTransferRequest(reader *messages.MessageReader) (map[string]any, error) {
	direction := reader.ReadInt32()
	if direction != 1 {
		return nil, fmt.Errorf("invalid transfer direction: %d", direction)
	}

	token := reader.ReadInt32()
	filename := reader.ReadString()
	result := map[string]any{
		"direction": direction,
		"token":     token,
		"filename":  filename,
	}

	// at this point we should have already sent a QueueUpload to the peer
	// This message is sent in response to our QueueUpload message
	// We send a TransferResponse to the peer to let them know we are ready to receive the file
	filesize := reader.ReadInt64()
	result["filesize"] = filesize

	// Initialize transfer tracking
	peer.transfersMutex.Lock()
	if peer.pendingTransfers == nil {
		peer.pendingTransfers = make(map[uint32]*FileTransfer)
	}
	peer.pendingTransfers[token] = &FileTransfer{
		Filename:     filename,
		Size:         uint64(filesize),
		PeerUsername: peer.Username,
		Token:        token,
		Buffer:       bytes.NewBuffer(make([]byte, 0, filesize)),
	}
	peer.transfersMutex.Unlock()

	// Tell the peer we're ready to receive
	// We expect to recieve an "F" connection after this
	// See slsk/client/listener.go for handling "F" connections
	peer.TransferResponse(token, true)

	peer.logger.Info("Received TransferRequest", "peer", peer.Username, "filename", filename, "filesize", filesize, "token", token)
	return map[string]any{
		"type":   "TransferRequest",
		"result": result,
	}, nil
}

func (peer *Peer) TransferResponse(token uint32, allowed bool) {
	mb := messages.NewMessageBuilder()

	mb.AddInt32(token)
	if allowed {
		mb.AddInt8(1)
	} else {
		mb.AddInt8(0)
		mb.AddString("Cancelled") // placeholder for now
	}

	peer.logger.Info("Sending TransferResponse", "peer", peer.Username, "token", token, "allowed", allowed)
	peer.SendMessage(mb.Build(41))
}

func (peer *Peer) handleTransferResponse(reader *messages.MessageReader) (map[string]any, error) {
	token := reader.ReadInt32()
	allowed := reader.ReadInt8() == 1

	result := map[string]any{
		"token":   token,
		"allowed": allowed,
	}

	if !allowed {
		result["reason"] = reader.ReadString()
	}

	// TODO: start uploading file here
	// we should already have tracked the

	peer.logger.Info("Received TransferResponse", "peer", peer.Username, "token", token, "allowed", allowed)
	return map[string]any{
		"type":   "TransferResponse",
		"result": result,
	}, nil
}

// Tell the peer that we want to download a file, i.e. they should queue an upload on their end
func (peer *Peer) QueueUpload(filename string) {
	peer.logger.Info("Requesting file from peer",
		"filename", filename,
		"peer", peer.Username,
	)
	mb := messages.NewMessageBuilder()
	mb.AddString(filename)

	peer.logger.Info("Sending QueueUpload", "peer", peer.Username, "filename", filename)
	peer.SendMessage(mb.Build(43))
}

// Peer tells us that we should queue a file for transfer. Send transfer request here
func (peer *Peer) handleQueueUpload(reader *messages.MessageReader) (map[string]any, error) {
	filename := reader.ReadString()

	peer.mgrCh <- PeerEvent{
		Type: UploadRequest,
		Peer: peer,
		Data: UploadRequestMessage{
			Filename: filename,
		},
	}

	peer.TransferRequest(1, filename, 0)

	peer.logger.Info("Received QueueUpload", "peer", peer.Username, "filename", filename)
	return map[string]any{
		"type":   "QueueUpload",
		"result": filename,
	}, nil
}

func (peer *Peer) handlePlaceInQueueResponse(reader *messages.MessageReader) (map[string]any, error) {
	filename := reader.ReadString()
	place := reader.ReadInt32()

	peer.mgrCh <- PeerEvent{Type: PlaceInQueueResponse, Peer: peer, Data: PlaceInQueueData{Filename: filename, Place: place}}

	return map[string]any{
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
func (peer *Peer) handleUploadFailed(reader *messages.MessageReader) (map[string]any, error) {
	filename := reader.ReadString()

	peer.logger.Info("Received UploadFailed", "peer", peer.Username, "filename", filename)
	return map[string]any{
		"type":     "UploadFailed",
		"filename": filename,
	}, nil
}

func (peer *Peer) UploadFailed(filename string) {
	mb := messages.NewMessageBuilder()
	mb.AddString(filename)

	peer.logger.Info("Sending UploadFailed", "peer", peer.Username, "filename", filename)
	peer.SendMessage(mb.Build(46))
}

// UploadDenied handling
func (peer *Peer) handleUploadDenied(reader *messages.MessageReader) (map[string]any, error) {
	filename := reader.ReadString()
	reason := reader.ReadString()

	peer.logger.Info("Received UploadDenied", "peer", peer.Username, "filename", filename, "reason", reason)
	return map[string]any{
		"type":     "UploadDenied",
		"filename": filename,
		"reason":   reason,
	}, nil
}

func (peer *Peer) UploadDenied(filename string, reason string) {
	mb := messages.NewMessageBuilder()
	mb.AddString(filename)
	mb.AddString(reason)

	peer.logger.Info("Sending UploadDenied", "peer", peer.Username, "filename", filename, "reason", reason)
	peer.SendMessage(mb.Build(50))
}

func (peer *Peer) handlePlaceInQueueRequest(reader *messages.MessageReader) (map[string]any, error) {
	filename := reader.ReadString()
	return map[string]any{
		"type":     "PlaceInQueueRequest",
		"filename": filename,
	}, nil

	// peer.mgrCh <- PeerEvent{Type: PlaceInQueueRequest, Peer: peer, Data: PlaceInQueueData{Filename: filename}}

}

func (peer *Peer) PlaceInQueueRequest(filename string) {
	mb := messages.NewMessageBuilder()
	mb.AddString(filename)

	peer.SendMessage(mb.Build(51))
}

func (peer *Peer) UploadFile(ft FileTransfer) {
	// Open the file - OS will automatically follow the symlink to the actual file
	peer.logger.Info("Uploading file", "peer", peer.Username, "filename", ft.Filename)
	file, err := os.Open(ft.Filename)
	if err != nil {
		peer.logger.Error("failed to open file for upload (virtual path: %s)", "error", err)
		return
	}
	defer file.Close()

	// If there's an offset, seek to that position
	if ft.Offset > 0 {
		_, err = file.Seek(int64(ft.Offset), io.SeekStart)
		if err != nil {
			peer.logger.Error("failed to seek to offset %d", "error", err)
		}
	}

	// Define chunk size (e.g., 8KB)
	const chunkSize = 8 * 1024
	buffer := make([]byte, chunkSize)

	// Calculate remaining bytes to send
	remainingBytes := ft.Size - ft.Offset
	bytesTransferred := uint64(0)

	// Send the file in chunks
	for remainingBytes > 0 {
		// Determine the size of the current chunk
		currentChunkSize := chunkSize
		if remainingBytes < uint64(chunkSize) {
			currentChunkSize = int(remainingBytes)
		}

		// Read a chunk from the file
		n, err := file.Read(buffer[:currentChunkSize])
		if err != nil && err != io.EOF {
			peer.logger.Error("error reading file chunk", "error", err)
		}
		if n == 0 {
			break // End of file
		}

		// Send the chunk
		_, err = peer.PeerConnection.Conn.Write(buffer[:n])
		if err != nil {
			peer.logger.Error("error sending file chunk", "error", err)
		}

		// Update tracking variables
		remainingBytes -= uint64(n)
		bytesTransferred += uint64(n)

	}
	peer.logger.Info("Upload complete", "filename", ft.Filename)
	peer.mgrCh <- PeerEvent{Type: UploadComplete, Peer: peer, Data: UploadCompleteData{Filename: ft.Filename, Token: ft.Token}}
}

// typical order of operations for searching and downloading
// 1. we send FileSearch to server
// 2. peers send FileSearchResponse to us
// 3. we pick a peer and send a QueueUpload to them
// 4. peer sends PlaceInQueueResponse to us?
// 5. peer sends a TransferRequest to us
// 6. we send a TransferResponse to the peer
// 7. peer starts "F" connection with file data
// 8. we download the file
