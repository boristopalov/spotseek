package uploads

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type UploadStatus string

const (
	UploadPending      UploadStatus = "pending"      // Peer requested, searching for file
	UploadQueued       UploadStatus = "queued"       // File found, sent TransferRequest
	UploadConnecting   UploadStatus = "connecting"   // Waiting for F connection
	UploadTransferring UploadStatus = "transferring" // Actively sending file
	UploadCompleted    UploadStatus = "completed"    // Transfer finished successfully
	UploadFailed       UploadStatus = "failed"       // Transfer failed
	UploadCancelled    UploadStatus = "cancelled"    // Transfer cancelled
	UploadDenied       UploadStatus = "denied"       // File not shared or denied
)

type UploadKey struct {
	Username string
	Filename string
}

type PendingUploadKey struct {
	Username string
	Token    uint32
}

type Upload struct {
	Username    string       `json:"username"`
	Filename    string       `json:"filename"` // Full virtual path requested by peer
	RealPath    string       `json:"realPath"` // Actual file path on disk
	Size        uint64       `json:"size"`     // File size
	Status      UploadStatus `json:"status"`
	BytesSent   uint64       `json:"bytesSent"` // Progress tracking
	Token       uint32       `json:"token"`     // Transfer token (for F connection lookup)
	Error       string       `json:"error,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
	CompletedAt *time.Time   `json:"completedAt,omitempty"`
	mu          sync.RWMutex `json:"-"`
}

func (u *Upload) InProgress() bool {
	return u.Status == UploadPending || u.Status == UploadQueued ||
		u.Status == UploadConnecting || u.Status == UploadTransferring
}

func (u *Upload) GetStatus() UploadStatus {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.Status
}

func (u *Upload) GetBytesSent() uint64 {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.BytesSent
}

func (u *Upload) UpdateProgress(bytes uint64) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.BytesSent = bytes
	if u.Status == UploadQueued || u.Status == UploadConnecting {
		u.Status = UploadTransferring
	}
}

func (u *Upload) UpdateStatus(status UploadStatus) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.Status = status
	if status == UploadCompleted || status == UploadFailed || status == UploadCancelled || status == UploadDenied {
		now := time.Now()
		u.CompletedAt = &now
	}
}

func (u *Upload) SetError(err string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.Error = err
	u.Status = UploadFailed
	now := time.Now()
	u.CompletedAt = &now
}

func (u *Upload) SetToken(token uint32) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.Token = token
}

type UploadManager struct {
	uploads                map[UploadKey]*Upload        // (username, filename) -> Upload
	uploadsByToken         map[uint32]*Upload           // token -> Upload
	uploadsByUsernameToken map[PendingUploadKey]*Upload // (username, token) -> Upload
	mu                     sync.RWMutex
	ttl                    time.Duration
	logger                 *slog.Logger
}

func NewUploadManager(ttl time.Duration, logger *slog.Logger) *UploadManager {
	um := &UploadManager{
		uploads:                make(map[UploadKey]*Upload),
		uploadsByToken:         make(map[uint32]*Upload),
		uploadsByUsernameToken: make(map[PendingUploadKey]*Upload),
		ttl:                    ttl,
		logger:                 logger,
	}
	go um.cleanupCompletedUploads()
	return um
}

func (um *UploadManager) CreateUpload(username, filename string) *Upload {
	um.mu.Lock()
	defer um.mu.Unlock()

	key := UploadKey{Username: username, Filename: filename}

	// Check if already exists and in progress
	if existing, exists := um.uploads[key]; exists && existing.InProgress() {
		um.logger.Warn("Upload already in progress",
			"username", username,
			"filename", filename,
			"status", existing.Status)
		return existing
	}

	upload := &Upload{
		Username:  username,
		Filename:  filename,
		Status:    UploadPending,
		CreatedAt: time.Now(),
	}

	um.uploads[key] = upload

	um.logger.Info("Upload created",
		"username", username,
		"filename", filename,
	)

	return upload
}

func (um *UploadManager) GetUpload(username, filename string) (*Upload, error) {
	um.mu.RLock()
	defer um.mu.RUnlock()

	key := UploadKey{Username: username, Filename: filename}
	upload, exists := um.uploads[key]
	if !exists {
		return nil, fmt.Errorf("upload not found: %s/%s", username, filename)
	}
	return upload, nil
}

func (um *UploadManager) GetUploadByUserToken(username string, token uint32) (*Upload, error) {
	um.mu.RLock()
	defer um.mu.RUnlock()

	key := PendingUploadKey{
		Username: username,
		Token:    token,
	}

	upload, exists := um.uploadsByUsernameToken[key]
	if !exists {
		return nil, fmt.Errorf("upload not found for token: %d", token)
	}
	return upload, nil
}

func (um *UploadManager) ListUploads() []*Upload {
	um.mu.RLock()
	defer um.mu.RUnlock()

	uploads := make([]*Upload, 0, len(um.uploads))
	for _, upload := range um.uploads {
		uploads = append(uploads, upload)
	}
	return uploads
}

func (um *UploadManager) CancelUpload(username, filename string) error {
	upload, err := um.GetUpload(username, filename)
	if err != nil {
		return err
	}

	upload.UpdateStatus(UploadCancelled)

	um.logger.Info("Upload cancelled", "username", username, "filename", filename)
	return nil
}

func (um *UploadManager) UpdateProgress(username string, token uint32, bytesSent uint64) error {
	upload, err := um.GetUploadByUserToken(username, token)
	if err != nil {
		return err
	}

	upload.UpdateProgress(bytesSent)
	return nil
}

func (um *UploadManager) UpdateStatus(username, filename string, status UploadStatus) error {
	upload, err := um.GetUpload(username, filename)
	if err != nil {
		return err
	}

	upload.UpdateStatus(status)
	um.logger.Info("Upload status updated",
		"username", username,
		"filename", filename,
		"status", status,
	)

	return nil
}

func (um *UploadManager) SetError(username, filename string, errorMsg string) error {
	upload, err := um.GetUpload(username, filename)
	if err != nil {
		return err
	}

	upload.SetError(errorMsg)

	um.logger.Warn("Upload error",
		"username", username,
		"filename", filename,
		"error", errorMsg,
	)
	return nil
}

func (um *UploadManager) SetFileInfo(username, filename, realPath string, size uint64, token uint32) error {
	upload, err := um.GetUpload(username, filename)
	if err != nil {
		return err
	}

	upload.mu.Lock()
	upload.RealPath = realPath
	upload.Size = size
	upload.Token = token
	upload.mu.Unlock()

	// Add to token lookup map
	um.mu.Lock()
	um.uploadsByToken[token] = upload
	um.mu.Unlock()

	um.logger.Info("Upload file info set",
		"username", username,
		"filename", filename,
		"realPath", realPath,
		"size", size,
		"token", token)
	return nil
}

func (um *UploadManager) cleanupCompletedUploads() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		um.mu.Lock()
		now := time.Now()
		for key, upload := range um.uploads {
			if upload.CompletedAt != nil && now.Sub(*upload.CompletedAt) > um.ttl {
				// Remove from both maps
				delete(um.uploads, key)
				if upload.Token != 0 {
					delete(um.uploadsByToken, upload.Token)
				}
				um.logger.Debug("Cleaned up completed upload",
					"username", upload.Username,
					"filename", upload.Filename,
					"status", upload.Status,
				)
			}
		}
		um.mu.Unlock()
	}
}
