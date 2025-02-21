package peer

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"spotseek/slsk/messages"
	"spotseek/slsk/shared"
)

type Event int

const (
	PeerConnected Event = iota
	PeerDisconnected
	FileSearchResponse
	// Maximum allowed message size (32MB)
	MaxMessageSize = 32 * 1024 * 1024
)

type ConnectionType int

type BasePeer interface {
	ReadMessage() ([]byte, error)
	SendMessage([]byte) error
}

type Peer struct {
	Username       string             `json:"username"`
	PeerConnection *shared.Connection `json:"-"` // Skip in JSON
	ConnType       string             `json:"connType"`
	Token          uint32             `json:"token"`
	Host           string             `json:"host"`
	Port           uint32             `json:"port"`
	Privileged     uint8              `json:"privileged"`
	EventEmitter   chan<- PeerEvent   `json:"-"` // Skip in JSON
}

type PeerEvent struct {
	Type Event
	Peer *Peer
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
		peer.EventEmitter <- PeerEvent{Type: PeerDisconnected, Peer: peer}
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
	// case "F":
	// decoded, err = peer.handleFileTransferMessage(code, reader, messageLength)
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
	case 4:
		return peer.handleGetSharedFileList(reader)
	case 9:
		return peer.handleFileSearchResponse(reader, messageLength)
	default:
		log.Error("Unsupported standard peer message code",
			"code", code,
			"peer", peer.Username)
		return nil, nil
	}
}

func (peer *Peer) handleGetSharedFileList(reader *messages.MessageReader) (map[string]interface{}, error) {
	// TODO: Implement shared file list handling
	return nil, nil
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
		_ = decompressedReader.ReadString() // extension (always blank in Qt)
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
			Filename: filename,
			Size:     size,
			BitRate:  bitrate,
			Duration: duration,
		}
	}

	result.SlotFree = decompressedReader.ReadInt8()
	result.AvgSpeed = decompressedReader.ReadInt32()
	result.QueueLength = decompressedReader.ReadInt32()

	return map[string]interface{}{
		"type":   "FileSearchResponse",
		"result": result,
	}, nil
}
