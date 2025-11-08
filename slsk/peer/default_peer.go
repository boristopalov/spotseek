package peer

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"spotseek/slsk/fileshare"
	"spotseek/slsk/messages"
)

func (peer *DefaultPeer) handleMessage(messageData []byte, messageLength uint32) error {
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
	_ = decoded // satisfy the compiler for now

	return nil
}

// ---------------------------
// ---------------------------
// ---------------------------
// Incoming Messages
// ---------------------------
// ---------------------------
// ---------------------------
// this is too much jumping around
func (peer *DefaultPeer) handleGetSharedFileList(reader *messages.MessageReader) (map[string]any, error) {
	peer.mgrCh <- PeerEvent{
		Type:     SharedFileListRequest,
		Username: peer.Username,
		Host:     peer.Host,
		Port:     peer.Port,
		Msg:      SharedFileListMsg{},
	}

	peer.logger.Info("Received SharedFileListRequest", "peer", peer.Username)
	return nil, nil
}

func (peer *DefaultPeer) handleSharedFileListResponse(reader *messages.MessageReader, messageLength uint32) (map[string]any, error) {
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

	result := fileshare.SharedFileList{}

	// Read number of directories
	dirCount := decompressedReader.ReadInt32()
	result.Directories = make([]fileshare.Directory, dirCount)

	// Iterate through directories
	for i := 0; i < int(dirCount); i++ {
		dir := fileshare.Directory{}
		dir.Name = decompressedReader.ReadString()

		// Read number of files in this directory
		fileCount := decompressedReader.ReadInt32()
		dir.Files = make([]fileshare.File, fileCount)

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

			dir.Files[j] = fileshare.File{
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

func (peer *DefaultPeer) handleFolderContentsRequest(reader *messages.MessageReader) (map[string]any, error) {
	token := reader.ReadInt32()
	folderName := reader.ReadString()
	peer.mgrCh <- PeerEvent{Type: FolderContentsRequest, Username: peer.Username, Host: peer.Host, Port: peer.Port, Msg: FolderContentsMsg{Token: token, FolderName: folderName}}
	peer.logger.Info("Received FolderContentsRequest", "peer", peer.Username)
	return map[string]any{
		"type":   "FolderContentsRequest",
		"token":  token,
		"folder": folderName,
	}, nil
}

func (peer *DefaultPeer) handleFileSearchResponse(reader *messages.MessageReader, messageLength uint32) (map[string]any, error) {
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

	result := fileshare.SearchResult{}
	result.Username = decompressedReader.ReadString()
	result.Token = decompressedReader.ReadInt32()

	// Handle public files
	fileCount := decompressedReader.ReadInt32()
	if fileCount < 1 {
		return nil, nil
	}
	result.PublicFiles = make([]fileshare.File, fileCount)

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

		result.PublicFiles[i] = fileshare.File{
			Name:        filename,
			VirtualPath: filename, // Full path from protocol
			Size:        size,
			BitRate:     bitrate,
			Duration:    duration,
			SampleRate:  sampleRate,
			Extension:   extension,
		}
	}

	result.SlotFree = decompressedReader.ReadInt8()
	result.AvgSpeed = decompressedReader.ReadInt32()
	result.QueuePosition = decompressedReader.ReadInt32()

	peer.mgrCh <- PeerEvent{Type: FileSearchResponse, Username: peer.Username, Host: peer.Host, Port: peer.Port, Msg: FileSearchMsg{Token: result.Token, Results: result}}

	peer.logger.Info("Received FileSearchResponse", "peer", peer.Username, "results", result)
	return map[string]any{
		"type":   "FileSearchResponse",
		"result": result,
	}, nil
}

func (peer *DefaultPeer) handleTransferRequest(reader *messages.MessageReader) (map[string]any, error) {
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
	peer.logger.Info("Received TransferRequest", "peer", peer.Username, "filename", filename, "filesize", filesize, "token", token)

	peer.mgrCh <- PeerEvent{
		Type:     DownloadRequest,
		ConnType: PeerTypeDefault,
		Username: peer.Username,
		Host:     peer.Host,
		Port:     peer.Port,
		Msg: DownloadRequestMsg{
			Token:        token,
			Filename:     filename,
			Size:         uint64(filesize),
			PeerUsername: peer.Username,
		},
	}

	// Tell the peer we're ready to receive
	// We expect to recieve an "F" connection after this
	// See slsk/client/listener.go for handling "F" connections
	peer.TransferResponse(token, true)
	return map[string]any{
		"type":   "TransferRequest",
		"result": result,
	}, nil
}

// we receive this from a peer that wants to download a file from us
// if allowed == true, we should attempt to start uploading the file to the peer
func (peer *DefaultPeer) handleTransferResponse(reader *messages.MessageReader) (map[string]any, error) {
	token := reader.ReadInt32()
	allowed := reader.ReadInt8() == 1

	result := map[string]any{
		"token":   token,
		"allowed": allowed,
	}

	if !allowed {
		result["reason"] = reader.ReadString()
	}

	peer.logger.Info("Received TransferResponse", "peer", peer.Username, "token", token, "allowed", allowed)
	if allowed {
		peer.mgrCh <- PeerEvent{
			Type:     UploadStart,
			Username: peer.Username,
			Host:     peer.Host,
			Port:     peer.Port,
			Msg: UploadStartMsg{
				Token:    token,
				Username: peer.Username,
			},
		}

	}
	return map[string]any{
		"type":   "TransferResponse",
		"result": result,
	}, nil
}

// Peer tells us that we should queue a file for transfer. Send transfer request here
func (peer *DefaultPeer) handleQueueUpload(reader *messages.MessageReader) (map[string]any, error) {
	filename := reader.ReadString()

	// Send UploadRequest event to PeerManager
	// PeerManager will create the upload in UploadManager and search for the file
	peer.mgrCh <- PeerEvent{
		Type:     UploadRequest,
		Username: peer.Username,
		Host:     peer.Host,
		Port:     peer.Port,
		Msg: UploadRequestMsg{
			Filename: filename,
		},
	}

	peer.logger.Info("Received QueueUpload", "peer", peer.Username, "filename", filename)
	return map[string]any{
		"type":   "QueueUpload",
		"result": filename,
	}, nil
}

func (peer *DefaultPeer) handlePlaceInQueueResponse(reader *messages.MessageReader) (map[string]any, error) {
	filename := reader.ReadString()
	place := reader.ReadInt32()

	peer.mgrCh <- PeerEvent{Type: PlaceInQueueResponse, Username: peer.Username, Host: peer.Host, Port: peer.Port, Msg: PlaceInQueueMsg{Filename: filename, Place: place}}

	return map[string]any{
		"type":     "PlaceInQueueResponse",
		"filename": filename,
		"place":    place,
	}, nil
}

func (peer *DefaultPeer) handleUploadFailed(reader *messages.MessageReader) (map[string]any, error) {
	filename := reader.ReadString()

	peer.logger.Info("Received UploadFailed", "peer", peer.Username, "filename", filename)
	peer.mgrCh <- PeerEvent{Type: DownloadFailed, Username: peer.Username, Host: peer.Host, Port: peer.Port, Msg: DownloadFailedMsg{Username: peer.Username, Filename: filename, Error: "received UploadFailed from peer"}}

	return map[string]any{
		"type":     "UploadFailed",
		"filename": filename,
	}, nil

}

func (peer *DefaultPeer) handleUploadDenied(reader *messages.MessageReader) (map[string]any, error) {
	filename := reader.ReadString()
	reason := reader.ReadString()

	peer.logger.Info("Received UploadDenied", "peer", peer.Username, "filename", filename, "reason", reason)
	return map[string]any{
		"type":     "UploadDenied",
		"filename": filename,
		"reason":   reason,
	}, nil
}

func (peer *DefaultPeer) handlePlaceInQueueRequest(reader *messages.MessageReader) (map[string]any, error) {
	filename := reader.ReadString()
	return map[string]any{
		"type":     "PlaceInQueueRequest",
		"filename": filename,
	}, nil

	// peer.mgrCh <- PeerEvent{Type: PlaceInQueueRequest, Peer: peer, Data: PlaceInQueueData{Filename: filename}}
}

// ---------------------------
// ---------------------------
// ---------------------------
// Outgoing Messages
// ---------------------------
// ---------------------------
// ---------------------------
func (peer *DefaultPeer) SharedFileListResponse(shares *fileshare.Shared) {
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

func (peer *DefaultPeer) FileSearchResponse(username string, token uint32, results []fileshare.SharedFile) {
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

	mb.AddInt32(10) // TODO: Slotfree

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

// We send this when we want to upload a file to a peer (peer wants to download from us)
func (peer *DefaultPeer) TransferRequest(direction uint32, filename string, filesize uint64) {
	mb := messages.NewMessageBuilder()
	token := uint32(rand.Int32())
	mb.AddInt32(direction)
	mb.AddInt32(token)
	mb.AddString(filename)
	if direction == 1 {
		mb.AddInt64(filesize)
	}

	peer.logger.Info("Sending TransferRequest", "filename", filename, "peer", peer.Username)
	peer.SendMessage(mb.Build(40))
}

func (peer *DefaultPeer) TransferResponse(token uint32, allowed bool) {
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

// Tell the peer that we want to download a file, i.e. they should queue an upload on their end
func (peer *DefaultPeer) QueueUpload(filename string) {
	mb := messages.NewMessageBuilder()
	mb.AddString(filename)

	peer.logger.Info("Sending QueueUpload (requesting file from peer)", "peer", peer.Username, "filename", filename)
	peer.SendMessage(mb.Build(43))
}

func (peer *DefaultPeer) PlaceInQueueResponse(filename string, place uint32) {
	mb := messages.NewMessageBuilder()
	mb.AddString(filename)
	mb.AddInt32(place)

	peer.SendMessage(mb.Build(44))
}

func (peer *DefaultPeer) UploadFailed(filename string) {
	mb := messages.NewMessageBuilder()
	mb.AddString(filename)

	peer.logger.Info("Sending UploadFailed", "peer", peer.Username, "filename", filename)
	peer.SendMessage(mb.Build(46))
}

func (peer *DefaultPeer) UploadDenied(filename string, reason string) {
	mb := messages.NewMessageBuilder()
	mb.AddString(filename)
	mb.AddString(reason)

	peer.logger.Info("Sending UploadDenied", "peer", peer.Username, "filename", filename, "reason", reason)
	peer.SendMessage(mb.Build(50))
}

func (peer *DefaultPeer) PlaceInQueueRequest(filename string) {
	mb := messages.NewMessageBuilder()
	mb.AddString(filename)

	peer.SendMessage(mb.Build(51))
}

func (peer *DefaultPeer) Listen() {
	readBuffer := make([]byte, 4096)
	var currentMessage []byte
	var messageLength uint32

	defer func() {
		peer.mgrCh <- PeerEvent{Type: PeerDisconnected, Username: peer.Username, Host: peer.Host, Port: peer.Port}
	}()

	for {
		n, err := peer.Conn.Read(readBuffer)
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

func (peer *DefaultPeer) processMessage(data []byte, messageLength uint32) ([]byte, uint32) {
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

		// sometimes the message length in the msg is different than actual buffer length
		// this seems to only happen for file search responses
		// maybe a different protocol version
		defer func() {
			if r := recover(); r != nil {
				peer.logger.Error("recovered from panic",
					"error", r,
				)
			}
		}()

		if err := peer.handleMessage(data[:messageLength], messageLength); err != nil {
			peer.logger.Error("Error handling peer message",
				"err", err,
				"length", messageLength,
				"peer", peer.Username)
		}

		data = data[messageLength:]
		messageLength = 0

		if len(data) == 0 {
			return data, messageLength
		}
	}
}
