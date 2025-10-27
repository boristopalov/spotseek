package peer

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	nc "spotseek/slsk/net"
	"sync"
	"time"
)

// PeerConnection is the base connection type with common functionality
type PeerConnection struct {
	Username   string         `json:"username"`
	Conn       *nc.Connection `json:"-"`
	Host       string         `json:"host"`
	Port       uint32         `json:"port"`
	Privileged uint8          `json:"privileged"`
	logger     *slog.Logger   `json:"-"`
	mu         sync.RWMutex   `json:"-"`
}

type PeerType int

const (
	PeerTypeDefault PeerType = iota
	PeerTypeDistributed
	PeerTypeFileTransfer
)

func (p PeerType) String() string {
	switch p {
	case PeerTypeDefault:
		return "P"
	case PeerTypeDistributed:
		return "D"
	case PeerTypeFileTransfer:
		return "F"
	default:
		return fmt.Sprintf("Unkwnown Peer Type (%d)", p)
	}
}

// MarshalJSON implements the json.Marshaler interface
func (p PeerType) MarshalJSON() ([]byte, error) {
	var s string
	switch p {
	case PeerTypeDefault:
		s = "default (P)"
	case PeerTypeDistributed:
		s = "distributed (D)"
	case PeerTypeFileTransfer:
		s = "file_transfer (F)"
	default:
		return nil, fmt.Errorf("unknown peer type: %d", p)
	}
	return json.Marshal(s)
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (p *PeerType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	switch s {
	case "default", "P":
		*p = PeerTypeDefault
	case "distributed", "D":
		*p = PeerTypeDistributed
	case "file_transfer", "F":
		*p = PeerTypeFileTransfer
	default:
		return fmt.Errorf("unknown peer type: %s", s)
	}
	return nil
}

// DefaultPeer handles standard peer-to-peer operations (searches, transfers)
type DefaultPeer struct {
	*PeerConnection
	mgrCh            chan<- PeerEvent
	pendingTransfers map[uint32]*FileTransfer
	transfersMutex   sync.RWMutex
}

// DistributedPeer handles distributed network operations
type DistributedPeer struct {
	*PeerConnection
	mgrCh           chan<- PeerEvent
	distribSearchCh chan<- DistribSearchMsg
	BranchLevel     uint32 `json:"branchLevel"`
	BranchRoot      string `json:"branchRoot"`
}

// FileTransferPeer handles file upload/download streaming
type FileTransferPeer struct {
	*PeerConnection
	mgrCh chan<- PeerEvent
	Token uint32
}

// newPeerConnection creates a new peer connection with TCP dial
func newPeerConnection(username string, host string, port uint32, privileged uint8, logger *slog.Logger) (*PeerConnection, error) {
	logger.Info("Connecting to peer", "username", username, "host", host, "port", port)
	c, err := net.DialTimeout("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)), 10*time.Second)
	if err != nil {
		logger.Error("unable to establish connection to peer", "username", username, "error", err)
		return nil, fmt.Errorf("unable to establish connection to peer %s: %v", username, err)
	}
	logger.Info("connected to peer", "username", username, "host", host, "port", port)
	return &PeerConnection{
		Username:   username,
		Conn:       &nc.Connection{Conn: c},
		Host:       host,
		Port:       port,
		Privileged: privileged,
		logger:     logger,
	}, nil
}

// newPeerConnectionWithConn creates a peer connection from an existing net.Conn
func newPeerConnectionWithConn(username string, host string, port uint32, privileged uint8, logger *slog.Logger, existingConn net.Conn) *PeerConnection {
	return &PeerConnection{
		Username:   username,
		Conn:       &nc.Connection{Conn: existingConn},
		Host:       host,
		Port:       port,
		Privileged: privileged,
		logger:     logger,
	}
}

// newDefaultPeer creates a new DefaultPeer
func newDefaultPeer(peerConn *PeerConnection, peerCh chan<- PeerEvent) *DefaultPeer {
	return &DefaultPeer{
		PeerConnection:   peerConn,
		mgrCh:            peerCh,
		pendingTransfers: make(map[uint32]*FileTransfer),
	}
}

// newDistributedPeer creates a new DistributedPeer
func newDistributedPeer(peerConn *PeerConnection, peerCh chan<- PeerEvent, distribSearchCh chan<- DistribSearchMsg) *DistributedPeer {
	return &DistributedPeer{
		PeerConnection:  peerConn,
		mgrCh:           peerCh,
		distribSearchCh: distribSearchCh,
	}
}

// newFileTransferPeer creates a new FileTransferPeer
func newFileTransferPeer(peerConn *PeerConnection, peerCh chan<- PeerEvent, token uint32) *FileTransferPeer {
	return &FileTransferPeer{
		PeerConnection: peerConn,
		mgrCh:          peerCh,
		Token:          token,
	}
}

// TODO: maybe separate this into incoming and outgoing transfer
type FileTransfer struct {
	Filename     string
	Size         uint64
	PeerUsername string
	Token        uint32
	Offset       uint64
}

// Common methods on PeerConnection
func (pc *PeerConnection) SendMessage(msg []byte) error {
	if pc == nil {
		return fmt.Errorf("tried to send message to peer but peer is nil")
	}
	if pc.Conn == nil {
		return fmt.Errorf("cannot send message to peer. no active connection")
	}
	return pc.Conn.SendMessage(msg)
}

func (pc *PeerConnection) Close() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.Conn.Close()
}

func (pc *PeerConnection) Read(buf []byte) (int, error) {
	return pc.Conn.Read(buf)
}

func (pc *PeerConnection) GetUsername() string {
	return pc.Username
}

func (pc *PeerConnection) GetHost() string {
	return pc.Host
}

func (pc *PeerConnection) GetPort() uint32 {
	return pc.Port
}

// DefaultPeer methods
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

// typical order of operations for searching and downloading
// 1. we send FileSearch to server
// 2. peers send FileSearchResponse to us
// 3. we pick a peer and send a QueueUpload to them
// 4. peer sends PlaceInQueueResponse to us?
// 5. peer sends a TransferRequest to us
// 6. we send a TransferResponse to the peer
// 7. peer starts "F" connection with file data
// 8. we download the file
