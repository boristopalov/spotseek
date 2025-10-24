package peer

import (
	"fmt"
	"log/slog"
	"net"
	"spotseek/slsk/downloads"
	"spotseek/slsk/fileshare"
	"spotseek/slsk/messages"
	nc "spotseek/slsk/net"
	"sync"
	"time"
)

type SearchManager interface {
	AddResult(token uint32, result fileshare.SearchResult) error
}

type DownloadManager interface {
	UpdateProgress(username string, filename string, bytesReceived uint64) error
	UpdateStatus(username string, filename string, status downloads.DownloadStatus) error
	SetError(userame string, filename string, errorMsg string) error
	SetQueuePosition(username string, filename string, position uint32) error
	SetToken(username string, filename string, token uint32) error
}

type PeerManager struct {
	username        string
	peers           map[string]*Peer // username --> Peer info
	mu              sync.RWMutex
	peerCh          chan PeerEvent
	distribSearchCh chan DistribSearchMsg
	children        []DistributedPeer
	searchManager   SearchManager                       // manages search state and results
	downloadManager DownloadManager                     // manages download state and progress
	SearchRequests  map[string][]fileshare.SearchResult // peer username --> incoming distributed search requests
	transferReqCh   chan FileTransfer                   // used when peers request files from us
	logger          *slog.Logger
	shares          *fileshare.Shared
}

func NewPeerManager(username string, distribSearchCh chan DistribSearchMsg, transferReqCh chan FileTransfer, shares *fileshare.Shared, searchManager SearchManager, downloadManager DownloadManager, logger *slog.Logger) *PeerManager {
	m := &PeerManager{
		username:        username,
		peers:           make(map[string]*Peer),
		peerCh:          make(chan PeerEvent),
		distribSearchCh: distribSearchCh,
		transferReqCh:   transferReqCh,
		children:        make([]DistributedPeer, 0),
		searchManager:   searchManager,
		downloadManager: downloadManager,
		logger:          logger,
		shares:          shares,
	}
	go m.listenForEvents()
	go m.listenForDistribMessages()
	return m
}

func (manager *PeerManager) RemovePeer(peer *Peer) {
	manager.mu.Lock()
	delete(manager.peers, peer.Username)
	manager.mu.Unlock()
}

func (manager *PeerManager) GetAllPeers() map[string]*Peer {
	return manager.peers
}

// ConnectToPeer establishes a direct connection to a peer (we initiate with PeerInit)
// This is called after receiving GetPeerAddress from the server
func (manager *PeerManager) ConnectToPeer(host string, port uint32, username string, connType string, token uint32, privileged uint8) error {
	peer := manager.AddPeer(username, connType, host, port, token, privileged, nil)
	if peer == nil {
		return fmt.Errorf("cannot connect to user %s with connType %s", username, connType)
	}

	// For direct connections, we always send PeerInit (code 1)
	peer.PeerInit(manager.username, peer.ConnType, peer.Token)

	if peer.ConnType == "F" {
		go peer.FileListen()
	} else {
		go peer.Listen()
	}

	manager.logger.Info("Connected to peer (direct)", "username", username, "connType", connType)
	return nil
}

// PierceFirewallToPeer establishes an indirect connection to a peer (we respond with PierceFirewall)
// This is called after receiving ConnectToPeer from the server
func (manager *PeerManager) PierceFirewallToPeer(host string, port uint32, username string, connType string, token uint32, privileged uint8) error {
	peer := manager.AddPeer(username, connType, host, port, token, privileged, nil)
	if peer == nil {
		return fmt.Errorf("cannot connect to user %s with connType %s", username, connType)
	}

	// For indirect connections, we always send PierceFirewall (code 0)
	peer.PierceFirewall(peer.Token)

	if peer.ConnType == "F" {
		go peer.FileListen()
	} else {
		go peer.Listen()
	}

	manager.logger.Info("Connected to peer (indirect/pierce firewall)", "username", username, "connType", connType)
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

func (manager *PeerManager) AddPeer(username string, connType string, host string, port uint32, token uint32, privileged uint8, existingConn net.Conn) *Peer {
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
		c, err := net.DialTimeout("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)), 10*time.Second)
		if err != nil {
			return nil
		}
		peer.mu.Lock()
		peer.PeerConnection = &nc.Connection{Conn: c}
		peer.mu.Unlock()
		return peer
	}

	// Use existing connection if provided, otherwise establish a new one
	var err error

	if existingConn != nil {
		peer, err = newPeerWithConnection(username, connType, token, host, port, privileged, manager.peerCh, manager.distribSearchCh, manager.logger, existingConn)
	} else {
		peer, err = newPeer(username, connType, token, host, port, privileged, manager.peerCh, manager.distribSearchCh, manager.logger)
	}

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

func (peer *Peer) PeerInit(username string, connType string, token uint32) error {

	mb := messages.NewMessageBuilder()
	mb.AddString(username)
	mb.AddString(connType)
	mb.AddInt32(token)
	err := peer.SendMessage(mb.BuildPeerInit(1))
	return err
}

func (peer *Peer) PierceFirewall(token uint32) error {
	mb := messages.NewMessageBuilder()
	mb.AddInt32(token)
	err := peer.SendMessage(mb.BuildPeerInit(0))
	return err
}

func (manager *PeerManager) listenForDistribMessages() {
	for msg := range manager.distribSearchCh {
		// send to client
		manager.distribSearchCh <- msg

		// send to children
		manager.SendDistribSearch(msg.Username, msg.Token, msg.Query)
	}
}

// TODO: complete handling of PeerEvents
func (manager *PeerManager) listenForEvents() {
	for event := range manager.peerCh {
		switch event.Type {
		case PeerDisconnected:
			manager.logger.Info("Peer Disconnected from us",
				"username", event.Peer.Username,
				"host", event.Peer.Host,
				"port", event.Peer.Port,
				"connType", event.Peer.ConnType,
			)
			event.Peer.Close()
			manager.RemovePeer(event.Peer)

		// we are ready to start uploading a file
		// client will send a new ConnectToPeer with type "F"
		// see client.listenForTransferRequests()
		case TransferRequest:
			msg := event.Msg.(TransferRequestMsg)

			// Link the download to the peer's transfer token
			// The peer generated this token and we need it for the F connection
			err := manager.downloadManager.SetToken(msg.PeerUsername, msg.Filename, msg.Token)
			if err != nil {
				manager.logger.Warn("Failed to link download to transfer token",
					"username", msg.PeerUsername,
					"filename", msg.Filename,
					"token", msg.Token,
					"error", err)
			}

			manager.transferReqCh <- FileTransfer{
				Filename:     msg.Filename,
				Token:        msg.Token,
				Size:         msg.Size,
				PeerUsername: msg.PeerUsername,
				Offset:       0,
				Buffer:       nil,
			}
			// idk about this
			event.Peer.Close()
			// event.Peer.PeerInit(manager.username, "F", msg.Token)

		case SharedFileListRequest:
			event.Peer.SharedFileListResponse(manager.shares)

		// peer wants to download a file
		case UploadRequest:
			msg := event.Msg.(UploadRequestMsg)
			res := manager.shares.Search(msg.Filename)
			if len(res) > 0 {
				event.Peer.TransferRequest(1, res[0].Value.Path, uint64(res[0].Value.Size))
			} else {
				event.Peer.UploadDenied(msg.Filename, "File not shared.")
			}

		// incoming file search response
		case FileSearchResponse:
			msg := event.Msg.(FileSearchMsg)
			err := manager.searchManager.AddResult(msg.Token, msg.Results)
			if err != nil {
				manager.logger.Warn("Failed to add search result", "token", msg.Token, "err", err)
			}

		case UploadComplete:
			manager.RemovePeer(event.Peer)
			event.Peer.Close()

		case DownloadProgress:
			msg := event.Msg.(DownloadProgressMsg)
			err := manager.downloadManager.UpdateProgress(msg.Username, msg.Filename, msg.BytesReceived)
			if err != nil {
				manager.logger.Warn("Failed to update download progress",
					"username", msg.Username,
					"filename", msg.Filename,
					"err", err)
			}

		case DownloadComplete:
			msg := event.Msg.(DownloadCompleteMsg)
			err := manager.downloadManager.UpdateStatus(msg.Username, msg.Filename, "completed")
			if err != nil {
				manager.logger.Warn("Failed to update download status",
					"username", msg.Username,
					"filename", msg.Filename,
					"err", err)
			}
			event.Peer.Close()
			manager.RemovePeer(event.Peer)

		case DownloadFailed:
			msg := event.Msg.(DownloadFailedMsg)
			err := manager.downloadManager.SetError(msg.Username, msg.Filename, msg.Error)
			if err != nil {
				manager.logger.Warn("Failed to set download error",
					"username", msg.Username,
					"filename", msg.Filename,
					"err", err)
			}
			event.Peer.Close()
			manager.RemovePeer(event.Peer)

		case PlaceInQueueResponse:
			msg := event.Msg.(PlaceInQueueMsg)
			err := manager.downloadManager.SetQueuePosition(event.Peer.Username, msg.Filename, msg.Place)
			if err != nil {
				manager.logger.Debug("Failed to set queue position (may not be a download)",
					"username", event.Peer.Username,
					"filename", msg.Filename,
					"place", msg.Place,
					"err", err)
			}

		}
	}
}

func (mgr *PeerManager) SendDistribMsg(code uint8, data []byte) (map[string]any, error) {
	mb := messages.NewMessageBuilder()
	mb.AddInt8(code)
	mb.Message = append(mb.Message, data...)
	msg := mb.Build(93)
	mgr.logger.Info("Sending DistributedMessage", "code", code)
	for _, child := range mgr.children {
		child.SendMessage(msg)
	}
	return map[string]any{
		"type": "DistributedMessage",
	}, nil
}

func (mgr *PeerManager) SendDistribBranchRoot(branchRoot string) {
	mb := messages.NewMessageBuilder()
	mb.AddString(branchRoot)
	data := mb.Build(5)
	mgr.logger.Info("Sending DistributedBranchRootMessage to children", "branchRoot", branchRoot)
	for _, child := range mgr.children {
		child.SendMessage(data)
	}
}

func (mgr *PeerManager) SendDistribBranchLevel(level uint32) {
	if level == 0 {
		// TODO
	}
	mb := messages.NewMessageBuilder()
	mb.AddInt32(level)
	data := mb.Build(4)
	for _, child := range mgr.children {
		child.SendMessage(data)
	}
}

func (mgr *PeerManager) SendDistribSearch(username string, token uint32, query string) (map[string]any, error) {
	mb := messages.NewMessageBuilder()
	mb.AddInt32(1) // unknown
	mb.AddString(username)
	mb.AddString(query)
	mb.AddInt32(token)
	data := mb.Build(3)
	// noisy log
	// mgr.logger.Info("Sending DistributedSearchMessage to children", "query", query, "token", token, "username", username)
	for _, child := range mgr.children {
		child.SendMessage(data)
	}
	return map[string]any{
		"type":     "Search",
		"username": username,
		"token":    token,
		"query":    query,
	}, nil
}
