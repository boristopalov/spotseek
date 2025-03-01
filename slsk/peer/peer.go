package peer

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	nc "spotseek/slsk/net"
	"sync"
	"time"
)

type Peer struct {
	Username         string                   `json:"username"`
	PeerConnection   *nc.Connection           `json:"-"` // Skip in JSON
	ConnType         string                   `json:"connType"`
	Token            uint32                   `json:"token"`
	Host             string                   `json:"host"`
	Port             uint32                   `json:"port"`
	Privileged       uint8                    `json:"privileged"`
	mgrCh            chan<- PeerEvent         `json:"-"` // Skip in JSON
	distribSearchCh  chan<- DistribSearchMsg  `json:"-"`
	fileTransferCh   chan<- struct{}          `json:"-"`
	pendingTransfers map[uint32]*FileTransfer `json:"-"` // transfers that we request
	transfersMutex   sync.RWMutex             `json:"-"`
	logger           *slog.Logger             `json:"-"`
	mu               sync.RWMutex             `json:"-"`
	BranchLevel      uint32                   `json:"branchLevel"`
	BranchRoot       string                   `json:"branchRoot"`
}

func newPeer(username string, connType string, token uint32, host string, port uint32, privileged uint8, peerCh chan<- PeerEvent, distribSearchCh chan<- DistribSearchMsg, logger *slog.Logger) (*Peer, error) {
	logger.Info("Connecting to peer via TCP", "username", username, "host", host, "port", port)
	c, err := net.DialTimeout("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)), 10*time.Second)
	if err != nil {
		logger.Error("unable to establish connection to peer", "username", username, "error", err)
		return nil, fmt.Errorf("unable to establish connection to peer %s: %v", username, err)
	} else {
		return &Peer{
			Username:        username,
			PeerConnection:  &nc.Connection{Conn: c},
			ConnType:        connType,
			Token:           token,
			Host:            host,
			Port:            port,
			Privileged:      privileged,
			mgrCh:           peerCh,
			distribSearchCh: distribSearchCh,
			fileTransferCh:  make(chan struct{}),
			logger:          logger,
		}, nil
	}
}

// Add a new constructor function that accepts an existing connection
func newPeerWithConnection(username string, connType string, token uint32, host string, port uint32, privileged uint8,
	peerCh chan<- PeerEvent, distribSearchCh chan<- DistribSearchMsg,
	logger *slog.Logger, existingConn net.Conn) (*Peer, error) {

	return &Peer{
		Username:         username,
		PeerConnection:   &nc.Connection{Conn: existingConn},
		ConnType:         connType,
		Token:            token,
		Host:             host,
		Port:             port,
		Privileged:       privileged,
		mgrCh:            peerCh,
		distribSearchCh:  distribSearchCh,
		fileTransferCh:   make(chan struct{}),
		pendingTransfers: make(map[uint32]*FileTransfer),
		logger:           logger,
	}, nil
}

// TODO: maybe separate this into incoming and outgoing transfer
type FileTransfer struct {
	Filename     string
	Size         uint64
	PeerUsername string
	Token        uint32
	Buffer       *bytes.Buffer
	Offset       uint64
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

		// sometimes the message length in the msg is different than actual buffer length
		// this seems to only happen for file search responses
		// maybe a different protocol version
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
			if err := peer.handleMessage(data[:messageLength], messageLength); err != nil {
				peer.logger.Error("Error handling peer message",
					"err", err,
					"length", messageLength,
					"peer", peer.Username)
			}

		} else if peer.ConnType == "D" {
			peer.handleDistribMessage(data[:messageLength])
		} else {
			peer.logger.Error("Unsupported connection type when handling message", "connType", peer.ConnType, "peer", peer)
			return nil, 0
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
