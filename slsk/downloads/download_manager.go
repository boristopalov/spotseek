package downloads

import (
	"fmt"
	"log/slog"
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

type Download struct {
	ID            uint32         `json:"id"`
	SearchID      *uint32        `json:"searchId,omitempty"` // Optional link to search
	Username      string         `json:"username"`
	Filename      string         `json:"filename"` // Full virtual path
	Size          uint64         `json:"size"`     // Expected file size
	Status        DownloadStatus `json:"status"`
	BytesReceived uint64         `json:"bytesReceived"` // Progress tracking
	QueuePosition *uint32        `json:"queuePosition,omitempty"`
	Error         string         `json:"error,omitempty"`
	Token         uint32         `json:"token"` // Transfer token for tracking
	CreatedAt     time.Time      `json:"createdAt"`
	CompletedAt   *time.Time     `json:"completedAt,omitempty"`
	mu            sync.RWMutex   `json:"-"`
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
	downloads      map[uint32]*Download // downloadID -> Download
	tokenToID      map[uint32]uint32    // transferToken -> downloadID
	mu             sync.RWMutex
	ttl            time.Duration
	logger         *slog.Logger
	nextDownloadID uint32
}

func NewDownloadManager(ttl time.Duration, logger *slog.Logger) *DownloadManager {
	dm := &DownloadManager{
		downloads:      make(map[uint32]*Download),
		tokenToID:      make(map[uint32]uint32),
		ttl:            ttl,
		logger:         logger,
		nextDownloadID: 1,
	}
	go dm.cleanupCompletedDownloads()
	return dm
}

func (dm *DownloadManager) CreateDownload(searchID *uint32, username, filename string, size uint64, token uint32) *Download {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	downloadID := dm.nextDownloadID
	dm.nextDownloadID++

	download := &Download{
		ID:        downloadID,
		SearchID:  searchID,
		Username:  username,
		Filename:  filename,
		Size:      size,
		Status:    DownloadPending,
		Token:     token,
		CreatedAt: time.Now(),
	}

	dm.downloads[downloadID] = download
	dm.tokenToID[token] = downloadID

	dm.logger.Info("Download created",
		"downloadId", downloadID,
		"username", username,
		"filename", filename,
		"token", token,
	)

	return download
}

func (dm *DownloadManager) GetDownload(id uint32) (*Download, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	download, exists := dm.downloads[id]
	if !exists {
		return nil, fmt.Errorf("download not found: %d", id)
	}
	return download, nil
}

func (dm *DownloadManager) GetDownloadByToken(token uint32) (*Download, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	downloadID, exists := dm.tokenToID[token]
	if !exists {
		return nil, fmt.Errorf("no download found for token: %d", token)
	}

	download, exists := dm.downloads[downloadID]
	if !exists {
		return nil, fmt.Errorf("download not found: %d", downloadID)
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

func (dm *DownloadManager) CancelDownload(id uint32) error {
	download, err := dm.GetDownload(id)
	if err != nil {
		return err
	}

	download.UpdateStatus(DownloadCancelled)
	dm.logger.Info("Download cancelled", "downloadId", id)
	return nil
}

func (dm *DownloadManager) UpdateProgress(token uint32, bytesReceived uint64) error {
	download, err := dm.GetDownloadByToken(token)
	if err != nil {
		return err
	}

	download.UpdateProgress(bytesReceived)
	return nil
}

func (dm *DownloadManager) UpdateStatus(token uint32, status DownloadStatus) error {
	download, err := dm.GetDownloadByToken(token)
	if err != nil {
		return err
	}

	download.UpdateStatus(status)
	dm.logger.Info("Download status updated",
		"token", token,
		"downloadId", download.ID,
		"status", status,
	)
	return nil
}

func (dm *DownloadManager) SetError(token uint32, errorMsg string) error {
	download, err := dm.GetDownloadByToken(token)
	if err != nil {
		return err
	}

	download.SetError(errorMsg)
	dm.logger.Warn("Download error",
		"token", token,
		"downloadId", download.ID,
		"error", errorMsg,
	)
	return nil
}

func (dm *DownloadManager) SetQueuePosition(token uint32, position uint32) error {
	download, err := dm.GetDownloadByToken(token)
	if err != nil {
		return err
	}

	download.SetQueuePosition(position)
	dm.logger.Info("Download queue position updated",
		"token", token,
		"downloadId", download.ID,
		"position", position,
	)
	return nil
}

func (dm *DownloadManager) cleanupCompletedDownloads() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		dm.mu.Lock()
		now := time.Now()
		for id, download := range dm.downloads {
			if download.CompletedAt != nil && now.Sub(*download.CompletedAt) > dm.ttl {
				delete(dm.downloads, id)
				delete(dm.tokenToID, download.Token)
				dm.logger.Debug("Cleaned up completed download",
					"downloadId", id,
					"status", download.Status,
				)
			}
		}
		dm.mu.Unlock()
	}
}
