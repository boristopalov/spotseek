package downloads

import (
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"
)

type DownloadStatus string

const (
	DownloadPending     DownloadStatus = "pending"
	DownloadConnecting  DownloadStatus = "connecting"
	DownloadQueued      DownloadStatus = "queued"
	DownloadDownloading DownloadStatus = "downloading"
	DownloadCompleted   DownloadStatus = "completed"
	DownloadFailed      DownloadStatus = "failed"
	DownloadCancelled   DownloadStatus = "cancelled"
)

type DownloadKey struct {
	Username string
	Filename string
}

type Download struct {
	Username      string         `json:"username"`
	Filename      string         `json:"filename"` // Full virtual path
	Size          uint64         `json:"size"`     // Expected file size
	Status        DownloadStatus `json:"status"`
	BytesReceived uint64         `json:"bytesReceived"` // Progress tracking
	QueuePosition *uint32        `json:"queuePosition,omitempty"`
	Error         string         `json:"error,omitempty"`
	Token         uint32         `json:"token,omitempty"` // Peer's transfer token (for debugging/logs only)
	CreatedAt     time.Time      `json:"createdAt"`
	CompletedAt   *time.Time     `json:"completedAt,omitempty"`
	mu            sync.RWMutex   `json:"-"`
}

func (d *Download) InProgress() bool {
	return d.Status == DownloadPending || d.Status == DownloadDownloading
}

func (d *Download) GetStatus() DownloadStatus {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.Status
}

func (d *Download) GetBytesReceived() uint64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.BytesReceived
}

func (d *Download) UpdateProgress(bytes uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.BytesReceived = bytes
	if d.Status == DownloadPending || d.Status == DownloadConnecting || d.Status == DownloadQueued {
		d.Status = DownloadDownloading
	}
}

func (d *Download) UpdateStatus(status DownloadStatus) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Status = status
	if status == DownloadCompleted || status == DownloadFailed || status == DownloadCancelled {
		now := time.Now()
		d.CompletedAt = &now
	}
}

func (d *Download) SetError(err string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Error = err
	d.Status = DownloadFailed
	now := time.Now()
	d.CompletedAt = &now
}

func (d *Download) SetQueuePosition(position uint32) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.QueuePosition = &position
	d.Status = DownloadQueued
}

type DownloadManager struct {
	downloads     map[DownloadKey]*Download // (username, filename) -> Download
	pendingByPeer map[string][]DownloadKey  // username -> downloads waiting for peer connection
	mu            sync.RWMutex
	ttl           time.Duration
	logger        *slog.Logger
}

func NewDownloadManager(ttl time.Duration, logger *slog.Logger) *DownloadManager {
	dm := &DownloadManager{
		downloads:     make(map[DownloadKey]*Download),
		pendingByPeer: make(map[string][]DownloadKey),
		ttl:           ttl,
		logger:        logger,
	}
	go dm.cleanupCompletedDownloads()
	return dm
}

func (dm *DownloadManager) CreateDownload(username, filename string, token uint32, size uint64) (*Download, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	key := DownloadKey{Username: username, Filename: filename}

	// Check if already exists
	if existing, exists := dm.downloads[key]; exists {
		dm.logger.Info("Download already exists",
			"username", username,
			"filename", filename,
			"status", existing.Status)
		return nil, fmt.Errorf("download already exists")
	}

	download := &Download{
		Username:  username,
		Filename:  filename,
		Size:      size,
		Token:     token,
		Status:    DownloadPending,
		CreatedAt: time.Now(),
	}

	dm.downloads[key] = download

	dm.logger.Info("Download created",
		"username", username,
		"filename", filename,
	)

	return download, nil
}

func (dm *DownloadManager) GetDownload(username, filename string) (*Download, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	key := DownloadKey{Username: username, Filename: filename}
	download, exists := dm.downloads[key]
	if !exists {
		return nil, fmt.Errorf("download not found: %s/%s", username, filename)
	}
	return download, nil
}

func (dm *DownloadManager) ListDownloads() []*Download {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	downloads := make([]*Download, 0, len(dm.downloads))
	for _, download := range dm.downloads {
		downloads = append(downloads, download)
	}
	return downloads
}

func (dm *DownloadManager) CancelDownload(username, filename string) error {
	download, err := dm.GetDownload(username, filename)
	if err != nil {
		return err
	}

	download.UpdateStatus(DownloadCancelled)

	// Remove from active downloads
	dm.mu.Lock()
	key := DownloadKey{Username: username, Filename: filename}
	delete(dm.downloads, key)
	dm.mu.Unlock()

	dm.logger.Info("Download cancelled", "username", username, "filename", filename)
	return nil
}

func (dm *DownloadManager) UpdateProgress(username, filename string, bytesReceived uint64) error {
	download, err := dm.GetDownload(username, filename)
	if err != nil {
		return err
	}

	download.UpdateProgress(bytesReceived)
	return nil
}

func (dm *DownloadManager) UpdateStatus(username, filename string, status DownloadStatus) error {
	download, err := dm.GetDownload(username, filename)
	if err != nil {
		return err
	}

	download.UpdateStatus(status)
	dm.logger.Info("Download status updated",
		"username", username,
		"filename", filename,
		"status", status,
	)

	// Remove from active downloads on terminal state
	// give it a few min tho
	if status == DownloadCompleted || status == DownloadFailed || status == DownloadCancelled {
		time.AfterFunc(10*time.Minute, func() {
			dm.mu.Lock()
			key := DownloadKey{Username: username, Filename: filename}
			delete(dm.downloads, key)
			dm.mu.Unlock()
		})
	}

	return nil
}

func (dm *DownloadManager) SetError(username, filename string, errorMsg string) error {
	download, err := dm.GetDownload(username, filename)
	if err != nil {
		return err
	}

	download.SetError(errorMsg)

	// Remove from active downloads immediately (allows re-download)
	dm.mu.Lock()
	key := DownloadKey{Username: username, Filename: filename}
	delete(dm.downloads, key)
	dm.mu.Unlock()

	dm.logger.Warn("Download error",
		"username", username,
		"filename", filename,
		"error", errorMsg,
	)
	return nil
}

func (dm *DownloadManager) SetQueuePosition(username, filename string, position uint32) error {
	download, err := dm.GetDownload(username, filename)
	if err != nil {
		return err
	}

	download.SetQueuePosition(position)
	dm.logger.Info("Download queue position updated",
		"username", username,
		"filename", filename,
		"position", position,
	)
	return nil
}

func (dm *DownloadManager) SetToken(username, filename string, token uint32) error {
	download, err := dm.GetDownload(username, filename)
	if err != nil {
		return err
	}

	download.mu.Lock()
	download.Token = token
	download.mu.Unlock()

	dm.logger.Info("Set transfer token",
		"username", username,
		"filename", filename,
		"token", token)
	return nil
}

func (dm *DownloadManager) cleanupCompletedDownloads() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		dm.mu.Lock()
		now := time.Now()
		for key, download := range dm.downloads {
			if download.CompletedAt != nil && now.Sub(*download.CompletedAt) > dm.ttl {
				delete(dm.downloads, key)
				dm.logger.Info("Cleaned up completed download",
					"username", download.Username,
					"filename", download.Filename,
					"status", download.Status,
				)
			}
		}
		dm.mu.Unlock()
	}
}

// AddPendingForPeer adds a download to the pending queue for a specific peer
func (dm *DownloadManager) AddPendingForPeer(username, filename string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	key := DownloadKey{Username: username, Filename: filename}
	dm.pendingByPeer[username] = append(dm.pendingByPeer[username], key)
	dm.logger.Info("Added pending download for peer",
		"username", username,
		"filename", filename,
	)
}

// GetPendingForPeer returns all downloads waiting for a peer connection
func (dm *DownloadManager) GetPendingForPeer(username string) []DownloadKey {
	dm.mu.RLock()
	downloadKeys := dm.pendingByPeer[username]
	dm.mu.RUnlock()

	return downloadKeys
}

func (dm *DownloadManager) GetDownloads(downloadKeys []DownloadKey) []*Download {
	downloads := make([]*Download, 0)
	for _, key := range downloadKeys {
		dm.mu.RLock()
		download, exists := dm.downloads[key]
		dm.mu.RUnlock()
		if exists {
			downloads = append(downloads, download)
		}
	}
	return downloads
}

// ClearAllPendingForPeer removes all pending downloads for a peer
func (dm *DownloadManager) ClearAllPendingForPeer(username string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	delete(dm.pendingByPeer, username)
	dm.logger.Info("Cleared pending downloads for peer", "username", username)
}

// ClearPendingForPeer removes all pending downloads for a peer
func (dm *DownloadManager) ClearPendingForPeer(username, filename string) {
	pending := dm.GetPendingForPeer(username)
	pending = slices.DeleteFunc(pending, func(key DownloadKey) bool {
		return key.Filename == filename
	})
	dm.mu.Lock()
	dm.pendingByPeer[username] = pending
	dm.mu.Unlock()
	dm.logger.Info("Cleared pending downloads for peer", "username", username)
}
