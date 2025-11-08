package db

import (
	"database/sql"
	"time"
)

type Download struct {
	ID            int64
	Username      string
	Filename      string
	Size          uint64
	Status        string
	BytesReceived uint64
	QueuePosition *uint32
	Error         *string
	Token         *uint32
	CreatedAt     time.Time
	CompletedAt   *time.Time
	UpdatedAt     time.Time
}

type DownloadRepository struct {
	db Executor
}

func NewDownloadRepository(db Executor) *DownloadRepository {
	return &DownloadRepository{db: db}
}

func (r *DownloadRepository) InsertDownload(username, filename string, size uint64, status string, token uint32, createdAt time.Time) error {
	_, err := r.db.Exec(`
		INSERT OR REPLACE INTO downloads (username, filename, size, status, token, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, username, filename, size, status, token, createdAt)
	return err
}

func (r *DownloadRepository) UpdateDownloadStatus(username, filename, status string, completedAt *time.Time) error {
	_, err := r.db.Exec(`
		UPDATE downloads
		SET status = ?, completed_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE username = ? AND filename = ?
	`, status, completedAt, username, filename)
	return err
}

func (r *DownloadRepository) UpdateDownloadProgress(username, filename string, bytesReceived uint64, status string) error {
	_, err := r.db.Exec(`
		UPDATE downloads
		SET bytes_received = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE username = ? AND filename = ?
	`, bytesReceived, status, username, filename)
	return err
}

func (r *DownloadRepository) UpdateDownloadQueuePosition(username, filename string, position uint32, status string) error {
	_, err := r.db.Exec(`
		UPDATE downloads
		SET queue_position = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE username = ? AND filename = ?
	`, position, status, username, filename)
	return err
}

func (r *DownloadRepository) UpdateDownloadError(username, filename, errorMsg string, status string, completedAt *time.Time) error {
	_, err := r.db.Exec(`
		UPDATE downloads
		SET error = ?, status = ?, completed_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE username = ? AND filename = ?
	`, errorMsg, status, completedAt, username, filename)
	return err
}

func (r *DownloadRepository) UpdateDownloadToken(username, filename string, token uint32) error {
	_, err := r.db.Exec(`
		UPDATE downloads
		SET token = ?, updated_at = CURRENT_TIMESTAMP
		WHERE username = ? AND filename = ?
	`, token, username, filename)
	return err
}

func (r *DownloadRepository) GetDownload(username, filename string) (*Download, error) {
	var d Download
	var queuePos sql.NullInt32
	var errorMsg sql.NullString
	var token sql.NullInt32
	var completedAt sql.NullTime

	err := r.db.QueryRow(`
		SELECT id, username, filename, size, status, bytes_received, queue_position,
		       error, token, created_at, completed_at, updated_at
		FROM downloads
		WHERE username = ? AND filename = ?
	`, username, filename).Scan(
		&d.ID, &d.Username, &d.Filename, &d.Size, &d.Status,
		&d.BytesReceived, &queuePos, &errorMsg, &token,
		&d.CreatedAt, &completedAt, &d.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if queuePos.Valid {
		pos := uint32(queuePos.Int32)
		d.QueuePosition = &pos
	}
	if errorMsg.Valid {
		d.Error = &errorMsg.String
	}
	if token.Valid {
		tok := uint32(token.Int32)
		d.Token = &tok
	}
	if completedAt.Valid {
		d.CompletedAt = &completedAt.Time
	}

	return &d, nil
}

func (r *DownloadRepository) GetDownloadsByStatus(status string, limit int) ([]*Download, error) {
	query := `
		SELECT id, username, filename, size, status, bytes_received, queue_position,
		       error, token, created_at, completed_at, updated_at
		FROM downloads
		WHERE status = ?
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := r.db.Query(query, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanDownloads(rows)
}

func (r *DownloadRepository) GetAllDownloads(limit int) ([]*Download, error) {
	query := `
		SELECT id, username, filename, size, status, bytes_received, queue_position,
		       error, token, created_at, completed_at, updated_at
		FROM downloads
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanDownloads(rows)
}

func (r *DownloadRepository) DeleteDownload(username, filename string) error {
	_, err := r.db.Exec(`
		DELETE FROM downloads
		WHERE username = ? AND filename = ?
	`, username, filename)
	return err
}

func (r *DownloadRepository) DeleteCompletedDownloads(olderThan time.Time) error {
	_, err := r.db.Exec(`
		DELETE FROM downloads
		WHERE completed_at IS NOT NULL AND completed_at < ?
	`, olderThan)
	return err
}

func (r *DownloadRepository) scanDownloads(rows *sql.Rows) ([]*Download, error) {
	var downloads []*Download
	for rows.Next() {
		var d Download
		var queuePos sql.NullInt32
		var errorMsg sql.NullString
		var token sql.NullInt32
		var completedAt sql.NullTime

		err := rows.Scan(
			&d.ID, &d.Username, &d.Filename, &d.Size, &d.Status,
			&d.BytesReceived, &queuePos, &errorMsg, &token,
			&d.CreatedAt, &completedAt, &d.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if queuePos.Valid {
			pos := uint32(queuePos.Int32)
			d.QueuePosition = &pos
		}
		if errorMsg.Valid {
			d.Error = &errorMsg.String
		}
		if token.Valid {
			tok := uint32(token.Int32)
			d.Token = &tok
		}
		if completedAt.Valid {
			d.CompletedAt = &completedAt.Time
		}

		downloads = append(downloads, &d)
	}
	return downloads, rows.Err()
}

func (r *DownloadRepository) GetDownloadStats() (map[string]int, error) {
	stats := make(map[string]int)

	rows, err := r.db.Query(`
		SELECT status, COUNT(*) as count
		FROM downloads
		GROUP BY status
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		stats[status] = count
	}

	return stats, rows.Err()
}
