package peer

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"log/slog"
	"net"
	"spotseek/slsk/fileshare"
	"spotseek/slsk/messages"
	"spotseek/slsk/shared"
	"sync"
	"time"
)

type PeerManager struct {
	peers         map[string]*Peer // username --> Peer info
	mu            sync.RWMutex
	peerCh        chan PeerEvent
	clientCh      chan<- PeerEvent
	children      []*DistributedPeer
	SearchResults map[uint32][]shared.SearchResult // token --> search results
	logger        *slog.Logger
	shares        *fileshare.Shared
}

func NewPeerManager(eventChan chan<- PeerEvent, shares *fileshare.Shared, logger *slog.Logger) *PeerManager {
	m := &PeerManager{
		peers:         make(map[string]*Peer),
		peerCh:        make(chan PeerEvent),
		clientCh:      eventChan,
		children:      make([]*DistributedPeer, 0),
		SearchResults: make(map[uint32][]shared.SearchResult),
		logger:        logger,
		shares:        shares,
	}
	go m.listenForEvents()
	return m
}

func (manager *PeerManager) RemovePeer(peer *Peer) {
	manager.mu.Lock()
	delete(manager.peers, peer.Username)
	manager.mu.Unlock()
}

func (manager *PeerManager) ConnectToPeerDistrib(ownUser string, host string, port uint32, peerUsername string, connType string, token uint32, privileged uint8) error {
	peer := manager.AddPeer(peerUsername, connType, host, port, token, privileged)
	if peer == nil {
		return fmt.Errorf("cannot connect to user %s with connType %s", peerUsername, connType)
	}

	peer.PeerInit(ownUser, peer.ConnType, peer.Token)
	peer.PierceFirewall(peer.Token)
	go peer.Listen()
	return nil
}

func (manager *PeerManager) ConnectToPeer(host string, port uint32, username string, connType string, token uint32, privileged uint8) error {
	peer := manager.AddPeer(username, connType, host, port, token, privileged)
	if peer == nil {
		return fmt.Errorf("cannot connect to user %s with connType %s", username, connType)
	}

	if peer.ConnType == "F" {
		peer.logger.Info("Listening for file transfer", "peer", peer.Username)
		filePeer := NewFileTransferPeer(peer)
		peer.PierceFirewall(peer.Token)
		go filePeer.FileListen()
		return nil
	}

	peer.PierceFirewall(peer.Token)
	go peer.Listen()
	return nil
}

func (manager *PeerManager) GetPeer(username string) *Peer {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	peer, exists := manager.peers[username]
	if exists {
		return peer
	}
	return nil
}

func (manager *PeerManager) AddPeer(username string, connType string, ip string, port uint32, token uint32, privileged uint8) *Peer {
	manager.mu.RLock()
	peer, exists := manager.peers[username]
	manager.mu.RUnlock()

	if exists {
		if peer.ConnType == connType {
			manager.logger.Warn("peer already connected", "peer", peer)
			return nil
		}

		peer.mu.Lock()
		peer.ConnType = connType
		peer.Token = token
		peer.mu.Unlock()

		// close the existing connection
		peer.Close()
		// open a new connection
		c, err := net.DialTimeout("tcp", net.JoinHostPort(ip, fmt.Sprintf("%d", port)), 10*time.Second)
		if err != nil {
			return nil
		}
		peer.mu.Lock()
		peer.PeerConnection = &shared.Connection{Conn: c}
		peer.mu.Unlock()
		return peer
	}

	peer, err := newPeer(username, connType, token, ip, port, privileged, manager.peerCh, manager.logger)
	if err != nil {
		return nil
	}

	// only update the peer if it doesn't exist
	// that way we don't lose track of any pending transfers
	manager.mu.Lock()
	manager.peers[username] = peer
	manager.mu.Unlock()
	return peer
}

func newPeer(username string, connType string, token uint32, host string, port uint32, privileged uint8, peerCh chan<- PeerEvent, logger *slog.Logger) (*Peer, error) {
	c, err := net.DialTimeout("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)), 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("unable to establish connection to peer %s: %v", username, err)
	} else {
		return &Peer{
			Username:       username,
			PeerConnection: &shared.Connection{Conn: c},
			ConnType:       connType,
			Token:          token,
			Host:           host,
			Port:           port,
			Privileged:     privileged,
			mgrCh:          peerCh,
			downloadFileCh: make(chan struct{}),
			logger:         logger,
		}, nil
	}
}

func (peer *Peer) PeerInit(username string, connType string, token uint32) error {

	mb := messages.PeerInitMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.PeerInit(username, connType, token)
	err := peer.SendMessage(msg)
	return err
}

func (peer *Peer) PierceFirewall(token uint32) error {
	mb := messages.PeerInitMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.PierceFirewall(token)
	err := peer.SendMessage(msg)
	return err
}

// TODO: complete handling of PeerEvents
func (manager *PeerManager) listenForEvents() {
	for event := range manager.peerCh {
		switch event.Type {
		case PeerDisconnected:
			event.Peer.Close()
			manager.RemovePeer(event.Peer)

		case SharedFileListRequest:
			// manager.logger.Info("Received shared file list request", "peer", event.Peer.Username)
			// construct a message with our shared file list (SharedFileListResponse)
			mb := messages.NewMessageBuilder()
			mb.AddInt32(manager.shares.GetShareStats().TotalFolders)

			// currently only one directory
			mb.AddString(manager.shares.Files[0].Dir)
			mb.AddInt32(manager.shares.GetShareStats().TotalFiles)

			for _, file := range manager.shares.Files {
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

			// Send SharedFileListResponse
			msg := mb.Build(5)
			event.Peer.SendMessage(msg)

		// incoming file search request
		case UploadRequest:
			msg := event.Data.(UploadRequestMessage)
			// manager.logger.Info("Received upload request", "peer", event.Peer.Username, "token", msg.Token, "filename", msg.Filename)
			res := manager.shares.Search(msg.Filename)
			if len(res) > 0 {
				event.Peer.TransferRequest(1, msg.Token, res[0].Key, uint64(res[0].Value.Size))
			} else {
				event.Peer.UploadDenied(msg.Filename, "File read error.")
			}

		// incoming file search response
		case FileSearchResponse:
			msg := event.Data.(FileSearchData)
			manager.SearchResults[msg.Token] =
				append(manager.SearchResults[msg.Token], msg.Results)
			manager.clientCh <- event

		// Distributed Messages
		case DistribSearch:
			msg := event.Data.(DistribSearchMessage)
			mb := messages.NewMessageBuilder()
			mb.AddInt32(1) // unknown
			mb.AddString(msg.Username)
			mb.AddInt32(msg.Token)
			mb.AddString(msg.Query)
			data := mb.Build(3)
			for _, child := range manager.children {
				child.PeerConnection.SendMessage(data)
			}
			// manager.
		case BranchLevel:
			msg := event.Data.(BranchLevelMessage)
			if msg.BranchLevel == 0 {
				// TODO
			}
			mb := messages.NewMessageBuilder()
			mb.AddInt32(msg.BranchLevel)
			data := mb.Build(4)
			for _, child := range manager.children {
				child.PeerConnection.SendMessage(data)
			}
		case BranchRoot:
			msg := event.Data.(BranchRootMessage)
			mb := messages.NewMessageBuilder()
			mb.AddString(msg.BranchRootUsername)
			data := mb.Build(5)
			for _, child := range manager.children {
				child.PeerConnection.SendMessage(data)
			}
		}
	}
}
