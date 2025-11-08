package db

import (
	"database/sql"
	"time"
)

type Upload struct {
	ID          int64
	Username    string
	Filename    string
	RealPath    *string
	Size        uint64
	Status      string
	BytesSent   uint64
	Token       *uint32
	Error       *string
	CreatedAt   time.Time
	CompletedAt *time.Time
	UpdatedAt   time.Time
}

type UploadRepository struct {
	db Executor
}

func NewUploadRepository(db Executor) *UploadRepository {
	return &UploadRepository{db: db}
}

func (r *UploadRepository) InsertUpload(username, filename string, status string, createdAt time.Time) error {
	_, err := r.db.Exec(`
		INSERT OR REPLACE INTO uploads (username, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, username, filename, status, createdAt)
	return err
}

func (r *UploadRepository) UpdateUploadStatus(username, filename, status string, completedAt *time.Time) error {
	_, err := r.db.Exec(`
		UPDATE uploads
		SET status = ?, completed_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE username = ? AND filename = ?
	`, status, completedAt, username, filename)
	return err
}

func (r *UploadRepository) UpdateUploadProgress(username, filename string, bytesSent uint64, status string) error {
	_, err := r.db.Exec(`
		UPDATE uploads
		SET bytes_sent = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE username = ? AND filename = ?
	`, bytesSent, status, username, filename)
	return err
}

func (r *UploadRepository) UpdateUploadError(username, filename, errorMsg string, status string, completedAt *time.Time) error {
	_, err := r.db.Exec(`
		UPDATE uploads
		SET error = ?, status = ?, completed_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE username = ? AND filename = ?
	`, errorMsg, status, completedAt, username, filename)
	return err
}

func (r *UploadRepository) UpdateUploadFileInfo(username, filename, realPath string, size uint64, token uint32) error {
	_, err := r.db.Exec(`
		UPDATE uploads
		SET real_path = ?, size = ?, token = ?, updated_at = CURRENT_TIMESTAMP
		WHERE username = ? AND filename = ?
	`, realPath, size, token, username, filename)
	return err
}

func (r *UploadRepository) GetUpload(username, filename string) (*Upload, error) {
	var u Upload
	var realPath sql.NullString
	var token sql.NullInt32
	var errorMsg sql.NullString
	var completedAt sql.NullTime

	err := r.db.QueryRow(`
		SELECT id, username, filename, real_path, size, status, bytes_sent,
		       token, error, created_at, completed_at, updated_at
		FROM uploads
		WHERE username = ? AND filename = ?
	`, username, filename).Scan(
		&u.ID, &u.Username, &u.Filename, &realPath, &u.Size, &u.Status,
		&u.BytesSent, &token, &errorMsg,
		&u.CreatedAt, &completedAt, &u.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if realPath.Valid {
		u.RealPath = &realPath.String
	}
	if token.Valid {
		tok := uint32(token.Int32)
		u.Token = &tok
	}
	if errorMsg.Valid {
		u.Error = &errorMsg.String
	}
	if completedAt.Valid {
		u.CompletedAt = &completedAt.Time
	}

	return &u, nil
}

func (r *UploadRepository) GetUploadByToken(token uint32) (*Upload, error) {
	var u Upload
	var realPath sql.NullString
	var tok sql.NullInt32
	var errorMsg sql.NullString
	var completedAt sql.NullTime

	err := r.db.QueryRow(`
		SELECT id, username, filename, real_path, size, status, bytes_sent,
		       token, error, created_at, completed_at, updated_at
		FROM uploads
		WHERE token = ?
	`, token).Scan(
		&u.ID, &u.Username, &u.Filename, &realPath, &u.Size, &u.Status,
		&u.BytesSent, &tok, &errorMsg,
		&u.CreatedAt, &completedAt, &u.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if realPath.Valid {
		u.RealPath = &realPath.String
	}
	if tok.Valid {
		t := uint32(tok.Int32)
		u.Token = &t
	}
	if errorMsg.Valid {
		u.Error = &errorMsg.String
	}
	if completedAt.Valid {
		u.CompletedAt = &completedAt.Time
	}

	return &u, nil
}

func (r *UploadRepository) GetUploadsByStatus(status string, limit int) ([]*Upload, error) {
	query := `
		SELECT id, username, filename, real_path, size, status, bytes_sent,
		       token, error, created_at, completed_at, updated_at
		FROM uploads
		WHERE status = ?
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := r.db.Query(query, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanUploads(rows)
}

func (r *UploadRepository) GetAllUploads(limit int) ([]*Upload, error) {
	query := `
		SELECT id, username, filename, real_path, size, status, bytes_sent,
		       token, error, created_at, completed_at, updated_at
		FROM uploads
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanUploads(rows)
}

func (r *UploadRepository) DeleteUpload(username, filename string) error {
	_, err := r.db.Exec(`
		DELETE FROM uploads
		WHERE username = ? AND filename = ?
	`, username, filename)
	return err
}

func (r *UploadRepository) DeleteCompletedUploads(olderThan time.Time) error {
	_, err := r.db.Exec(`
		DELETE FROM uploads
		WHERE completed_at IS NOT NULL AND completed_at < ?
	`, olderThan)
	return err
}

func (r *UploadRepository) scanUploads(rows *sql.Rows) ([]*Upload, error) {
	var uploads []*Upload
	for rows.Next() {
		var u Upload
		var realPath sql.NullString
		var token sql.NullInt32
		var errorMsg sql.NullString
		var completedAt sql.NullTime

		err := rows.Scan(
			&u.ID, &u.Username, &u.Filename, &realPath, &u.Size, &u.Status,
			&u.BytesSent, &token, &errorMsg,
			&u.CreatedAt, &completedAt, &u.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if realPath.Valid {
			u.RealPath = &realPath.String
		}
		if token.Valid {
			tok := uint32(token.Int32)
			u.Token = &tok
		}
		if errorMsg.Valid {
			u.Error = &errorMsg.String
		}
		if completedAt.Valid {
			u.CompletedAt = &completedAt.Time
		}

		uploads = append(uploads, &u)
	}
	return uploads, rows.Err()
}

func (r *UploadRepository) GetUploadStats() (map[string]int, error) {
	stats := make(map[string]int)

	rows, err := r.db.Query(`
		SELECT status, COUNT(*) as count
		FROM uploads
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
