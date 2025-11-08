package peer

import (
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"spotseek/slsk/downloads"
	"spotseek/slsk/fileshare"
	"spotseek/slsk/messages"
	"spotseek/slsk/uploads"
	"sync"
)

type SearchManager interface {
	AddResult(token uint32, result fileshare.SearchResult) error
}

type PendingTokenConn struct {
	Username   string
	ConnType   string
	Privileged uint8
	Token      uint32
	IsParent   bool
}

type PeerLifecycleEventName int

const (
	EventDisconnected PeerLifecycleEventName = iota
	EventConnected
)

func (e PeerLifecycleEventName) String() string {
	switch e {
	case EventConnected:
		return "CONNECTED"
	case EventDisconnected:
		return "DISCONNECTED"
	default:
		return fmt.Sprintf("Unkwnown Event (%d)", e)
	}
}

// PeerLifecycleEvent represents a peer lifecycle event (connect/disconnect)
type PeerLifecycleEvent struct {
	Username string
	Host     string
	Port     uint32
	Event    PeerLifecycleEventName
}

// PeerInfo represents peer information for API/external consumption
type PeerInfo struct {
	Username string   `json:"username"`
	Host     string   `json:"host"`
	Port     uint32   `json:"port"`
	PeerType PeerType `json:"connType"` // "P", "D", or "F"
}

type PeerManager struct {
	username          string
	defaultPeers      map[string]*DefaultPeer      // username --> DefaultPeer
	distributedPeers  map[string]*DistributedPeer  // username --> DistributedPeer
	fileTransferPeers map[string]*FileTransferPeer // username --> FileTransferPeer
	mu                sync.RWMutex
	peerCh            chan PeerEvent
	distribSearchCh   chan DistribSearchMsg
	children          []*DistributedPeer
	searchManager     SearchManager                       // manages search state and results
	downloadManager   *downloads.DownloadManager          // manages download state and progress
	uploadManager     *uploads.UploadManager              // manages upload state and progress
	SearchRequests    map[string][]fileshare.SearchResult // peer username --> incoming distributed search requests
	transferReqCh     chan UploadStartMsg                 // used when peers request files from us
	logger            *slog.Logger
	shares            *fileshare.Shared
	// Pending connections (connections being established but not yet complete)
	pendingUsers  map[string]PendingTokenConn // username --> connection info
	pendingTokens map[uint32]PendingTokenConn // token --> connection info
	pendingMu     sync.RWMutex
	// Channel for notifying client of peer lifecycle events
	lifecycleEventCh chan<- PeerLifecycleEvent
}

func NewPeerManager(username string, distribSearchCh chan DistribSearchMsg, transferReqCh chan UploadStartMsg, lifecycleEventCh chan PeerLifecycleEvent, shares *fileshare.Shared, searchManager SearchManager, downloadManager *downloads.DownloadManager, uploadManager *uploads.UploadManager, logger *slog.Logger) *PeerManager {
	m := &PeerManager{
		username:          username,
		defaultPeers:      make(map[string]*DefaultPeer),
		distributedPeers:  make(map[string]*DistributedPeer),
		fileTransferPeers: make(map[string]*FileTransferPeer),
		peerCh:            make(chan PeerEvent),
		distribSearchCh:   distribSearchCh,
		transferReqCh:     transferReqCh,
		lifecycleEventCh:  lifecycleEventCh,
		children:          make([]*DistributedPeer, 0),
		searchManager:     searchManager,
		downloadManager:   downloadManager,
		uploadManager:     uploadManager,
		logger:            logger,
		shares:            shares,
		pendingUsers:      make(map[string]PendingTokenConn),
		pendingTokens:     make(map[uint32]PendingTokenConn),
	}
	go m.listenForEvents()
	return m
}

func (manager *PeerManager) RemovePeer(username string, peerType PeerType) {
	switch peerType {
	case PeerTypeDefault:
		manager.removeDefaultPeer(username)
	case PeerTypeDistributed:
		manager.removeDistributedPeer(username)
	case PeerTypeFileTransfer:
		manager.removeFileTransferPeer(username)
	}
}

func (manager *PeerManager) removeDefaultPeer(username string) {
	manager.mu.Lock()
	delete(manager.defaultPeers, username)
	manager.mu.Unlock()
}

func (manager *PeerManager) removeDistributedPeer(username string) {
	manager.mu.Lock()
	delete(manager.distributedPeers, username)
	manager.mu.Unlock()
}

func (manager *PeerManager) removeFileTransferPeer(username string) {
	manager.mu.Lock()
	delete(manager.fileTransferPeers, username)
	manager.mu.Unlock()
}

// GetAllPeers returns information about all connected peers
func (manager *PeerManager) GetAllPeers() []PeerInfo {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	var peers []PeerInfo

	for username, peer := range manager.defaultPeers {
		peers = append(peers, PeerInfo{
			Username: username,
			Host:     peer.Host,
			Port:     peer.Port,
			PeerType: PeerTypeDefault,
		})
	}

	for username, peer := range manager.distributedPeers {
		peers = append(peers, PeerInfo{
			Username: username,
			Host:     peer.Host,
			Port:     peer.Port,
			PeerType: PeerTypeDistributed,
		})
	}

	for username, peer := range manager.fileTransferPeers {
		peers = append(peers, PeerInfo{
			Username: username,
			Host:     peer.Host,
			Port:     peer.Port,
			PeerType: PeerTypeFileTransfer,
		})
	}

	return peers
}

// AddPendingConnection registers a connection that is being established
func (manager *PeerManager) AddPendingConnection(username string, token uint32, connType string, privileged uint8, isParent bool) {
	manager.pendingMu.Lock()
	defer manager.pendingMu.Unlock()
	_, found := manager.pendingUsers[username]
	if found {
		manager.logger.Warn("Peer connection already pending",
			"token", token,
			"username", username,
		)
		return
	}
	connInfo := PendingTokenConn{Username: username, ConnType: connType, Token: token, Privileged: privileged, IsParent: isParent}
	manager.pendingUsers[username] = connInfo
	manager.pendingTokens[token] = connInfo
}

// GetPendingConnection retrieves a pending connection by username
func (manager *PeerManager) GetPendingConnection(username string) (PendingTokenConn, bool) {
	manager.pendingMu.RLock()
	defer manager.pendingMu.RUnlock()
	pendingToken, found := manager.pendingUsers[username]
	if !found {
		return PendingTokenConn{}, false
	}
	return pendingToken, true
}

// GetPendingConnectionByToken retrieves a pending connection by token
func (manager *PeerManager) GetPendingConnectionByToken(token uint32) (PendingTokenConn, bool) {
	manager.pendingMu.RLock()
	defer manager.pendingMu.RUnlock()
	pendingConn, found := manager.pendingTokens[token]
	if !found {
		return PendingTokenConn{}, false
	}
	return pendingConn, true
}

// RemovePendingConnection removes a pending connection
func (manager *PeerManager) RemovePendingConnection(username string) {
	manager.pendingMu.Lock()
	defer manager.pendingMu.Unlock()
	connInfo := manager.pendingUsers[username]
	delete(manager.pendingUsers, username)
	delete(manager.pendingTokens, connInfo.Token)
}

// ConnectToPeer attemps to establish a direct connection to a peer
// This is called after receiving GetPeerAddress from the server
func (manager *PeerManager) ConnectToPeer(host string, port uint32, username string, connType string, token uint32, privileged uint8) error {
	peerConn, err := newPeerConnection(username, host, port, privileged, manager.logger)
	if err != nil {
		return err
	}

	// Send PeerInit for direct connection
	mb := messages.NewMessageBuilder()
	mb.AddString(manager.username)
	mb.AddString(connType)
	mb.AddInt32(0) // token is 0 for direct connections
	if err := peerConn.SendMessage(mb.BuildPeerInit(1)); err != nil {
		return err
	}

	// Create and store the appropriate peer type
	switch connType {
	case "P":
		peer := newDefaultPeer(peerConn, manager.peerCh)
		manager.mu.Lock()
		manager.defaultPeers[username] = peer
		manager.mu.Unlock()
		go peer.Listen()
	case "D":
		peer := newDistributedPeer(peerConn, manager.peerCh, manager.distribSearchCh)
		manager.mu.Lock()
		manager.distributedPeers[username] = peer
		manager.mu.Unlock()
		go peer.Listen()
	default:
		return fmt.Errorf("unsupported connection type for ConnectToPeer: %s", connType)
	}

	manager.logger.Info("Connected to peer (direct)", "username", username, "connType", connType)
	return nil
}

// PierceFirewallToPeer establishes an indirect connection to a peer (we respond with PierceFirewall)
// This is called after receiving ConnectToPeer from the server
func (manager *PeerManager) PierceFirewallToPeer(host string, port uint32, username string, connType string, token uint32, privileged uint8) error {
	// Create base connection
	peerConn, err := newPeerConnection(username, host, port, privileged, manager.logger)
	if err != nil {
		return err
	}

	// Send PierceFirewall for indirect connection
	mb := messages.NewMessageBuilder()
	mb.AddInt32(token)
	if err := peerConn.SendMessage(mb.BuildPeerInit(0)); err != nil {
		return err
	}

	err = manager.AddPeerFromExistingConn(username, connType, peerConn.Host, peerConn.Port, token, privileged, peerConn.Conn)
	if err != nil {
		return err
	}
	switch connType {
	case "P":
		peer := manager.GetDefaultPeer(username)
		if peer == nil {
			return fmt.Errorf("failed to find existing connection for peer %s. Not listening to peer", username)
		}
		go peer.Listen()
	case "D":
		peer := manager.GetDistributedPeer(username)
		if peer == nil {
			return fmt.Errorf("failed to find existing connection for peer %s. Not listening to peer", username)
		}
		go peer.Listen()
	case "F":
		peer := manager.GetFileTransferPeer(username)
		if peer == nil {
			return fmt.Errorf("failed to find existing connection for peer %s. Not listening to peer", username)
		}
		pendingDownloads := manager.downloadManager.GetPendingForPeer(username)
		if len(pendingDownloads) == 0 {
			manager.logger.Error("could not find any pending files to download from user", "username", username)
			return fmt.Errorf("could not find pending file to download from user %s", username)
		}
		downloads := manager.downloadManager.GetDownloads(pendingDownloads)
		// start listening for the file that the peer is going to upload to us
		// IDK if we can listen for multiple at a time.. i would assume not
		if len(downloads) == 0 {
			manager.logger.Error("Could not find Downloads from user", "username", username)
			return fmt.Errorf("could not find Downloads user %s", username)
		}
		download := downloads[0]
		transfer := FileTransfer{
			Filename:     download.Filename,
			Size:         download.Size,
			PeerUsername: username,
			Token:        download.Token,
			Offset:       download.BytesReceived,
		}
		// remove pending download
		manager.downloadManager.ClearPendingForPeer(username, download.Filename)
		go peer.FileListen(&transfer)
	default:
		return fmt.Errorf("unsupported connection type for PierceFirewallToPeer: %s", connType)
	}

	manager.logger.Info("Connected to peer (indirect/pierce firewall)", "username", username, "connType", connType)
	return nil
}

func (manager *PeerManager) GetDefaultPeer(username string) *DefaultPeer {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	peer, exists := manager.defaultPeers[username]
	if exists {
		return peer
	}
	return nil
}

func (manager *PeerManager) GetDistributedPeer(username string) *DistributedPeer {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	peer, exists := manager.distributedPeers[username]
	if exists {
		return peer
	}
	return nil
}

func (manager *PeerManager) GetFileTransferPeer(username string) *FileTransferPeer {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	peer, exists := manager.fileTransferPeers[username]
	if exists {
		return peer
	}
	return nil
}

// AddPeerFromExistingConn adds a peer from an existing connection (used by listener)
// TODO: this is confusing
// We call this for both incoming and outgoing connections. wrong!
// TODO: call peer.Listen() elsewhere
func (manager *PeerManager) AddPeerFromExistingConn(username string, connType string, host string, port uint32, token uint32, privileged uint8, existingConn net.Conn) error {
	peerConn := newPeerConnectionWithConn(username, host, port, privileged, manager.logger, existingConn)

	switch connType {
	case "P":
		peer := newDefaultPeer(peerConn, manager.peerCh)
		manager.mu.Lock()
		manager.defaultPeers[username] = peer
		manager.mu.Unlock()
		// go peer.Listen()
	case "D":
		peer := newDistributedPeer(peerConn, manager.peerCh, manager.distribSearchCh)
		manager.mu.Lock()
		manager.distributedPeers[username] = peer
		manager.mu.Unlock()
		// go peer.Listen()
	case "F":
		//
		peer := newFileTransferPeer(peerConn, manager.peerCh, token)
		manager.mu.Lock()
		manager.fileTransferPeers[username] = peer
		manager.mu.Unlock()
	default:
		return fmt.Errorf("unsupported connection type: %s", connType)
	}

	return nil
}

// TODO: complete handling of PeerEvents
func (manager *PeerManager) listenForEvents() {
	for event := range manager.peerCh {
		switch event.Type {
		case PeerDisconnected:
			manager.logger.Info("Peer Disconnected from us",
				"username", event.Username,
				"host", event.Host,
				"port", event.Port,
			)
			// Notify client of disconnection via channel (for parent handling, etc.)
			if manager.lifecycleEventCh != nil {
				manager.lifecycleEventCh <- PeerLifecycleEvent{
					Username: event.Username,
					Host:     event.Host,
					Port:     event.Port,
					Event:    EventDisconnected,
				}
			}

			manager.RemovePeer(event.Username, event.ConnType)

		// Download event is sent when we request a download fomr a peer
		case DownloadRequest:
			msg := event.Msg.(DownloadRequestMsg)

			_, err := manager.downloadManager.CreateDownload(
				msg.PeerUsername,
				msg.Filename,
				msg.Token,
				msg.Size,
			)
			if err != nil {
				manager.logger.Warn("Failed to create download",
					"username", msg.PeerUsername,
					"filename", msg.Filename,
					"token", msg.Token,
					"error", err)
				return
			} else {
				// Update status to connecting (waiting for peer's TransferResponse)
				manager.downloadManager.UpdateStatus(msg.PeerUsername, msg.Filename, downloads.DownloadConnecting)
			}

			// Close P connection - we'll get a new F connection for the actual transfer
			if peer := manager.GetDefaultPeer(event.Username); peer != nil {
				peer.Close()
			} else {
				manager.logger.Warn("peer already disconnected", "peer", peer.Username)
			}

		case SharedFileListRequest:
			if peer := manager.GetDefaultPeer(event.Username); peer != nil {
				peer.SharedFileListResponse(manager.shares)
			}

		case UploadStart:
			msg := event.Msg.(UploadStartMsg)
			manager.transferReqCh <- msg

		// peer wants to download a file
		// This happens when a peer requests an upload from us (via QueueUpload)
		// We search for the file, generate a token, and send TransferRequest back
		// The peer will respond with TransferResponse, then we'll establish F connection
		case UploadRequest:
			msg := event.Msg.(UploadRequestMsg)

			// Create upload in UploadManager
			upload := manager.uploadManager.CreateUpload(event.Username, msg.Filename)

			// Get the peer
			peer := manager.GetDefaultPeer(event.Username)
			if peer == nil {
				manager.logger.Warn("Cannot find peer for upload request", "username", event.Username)
				upload.UpdateStatus(uploads.UploadDenied)
				upload.SetError("Peer not found")
				return
			}

			// Search for the file in our shares
			res := manager.shares.Search(msg.Filename)
			if len(res) > 0 {
				token := rand.Uint32()
				filename := res[0].Value.Path
				filesize := uint64(res[0].Value.Size)
				manager.uploadManager.SetFileInfo(event.Username, msg.Filename, filename, filesize, token)
				// File found - send TransferRequest to peer
				// The peer will respond with TransferResponse, which triggers the actual upload
				peer.TransferRequest(1, filename, filesize)

				// Update upload status to queued
				upload.UpdateStatus(uploads.UploadQueued)
			} else {
				// File not found - deny the upload
				peer.UploadDenied(msg.Filename, "File not shared.")
				upload.UpdateStatus(uploads.UploadDenied)
				upload.SetError("File not shared")
			}

		// incoming file search response
		case FileSearchResponse:
			msg := event.Msg.(FileSearchMsg)
			err := manager.searchManager.AddResult(msg.Token, msg.Results)
			if err != nil {
				manager.logger.Warn("Failed to add search result", "token", msg.Token, "err", err)
			}

		case UploadProgress:
			msg := event.Msg.(UploadProgressMsg)

			// Update upload progress via token lookup
			err := manager.uploadManager.UpdateProgress(msg.Username, msg.Token, msg.BytesSent)
			if err != nil {
				manager.logger.Warn("Failed to update upload progress",
					"token", msg.Token,
					"error", err)
			}

		case UploadComplete:
			msg := event.Msg.(UploadCompleteMsg)

			// Update upload status to completed
			err := manager.uploadManager.UpdateStatus(event.Username, msg.Filename, uploads.UploadCompleted)
			if err != nil {
				manager.logger.Warn("Failed to update upload status",
					"username", event.Username,
					"filename", msg.Filename,
					"error", err)
			}

			manager.RemovePeer(event.Username, event.ConnType)

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
			manager.RemovePeer(event.Username, event.ConnType)

		case DownloadFailed:
			msg := event.Msg.(DownloadFailedMsg)
			dl, err := manager.downloadManager.GetDownload(msg.Username, msg.Filename)
			if err != nil {
				manager.logger.Error("failed to get download", "peer", msg.Username, "file", msg.Filename)
			} else {
				// We get this message even if a download completed successfully
				// Only set an error if the download status is not Completed, otherwise we can ignore
				if dl.Status != downloads.DownloadCompleted {
					err = manager.downloadManager.SetError(msg.Username, msg.Filename, msg.Error)
					if err != nil {
						manager.logger.Error("Failed to set download error",
							"username", msg.Username,
							"filename", msg.Filename,
							"err", err)
					}
				}
			}
			manager.RemovePeer(event.Username, event.ConnType)

		case PlaceInQueueResponse:
			msg := event.Msg.(PlaceInQueueMsg)
			err := manager.downloadManager.SetQueuePosition(event.Username, msg.Filename, msg.Place)
			if err != nil {
				manager.logger.Error("Failed to set queue position (may not be a download)",
					"username", event.Username,
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
