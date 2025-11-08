package db

import (
	"database/sql"
	"time"
)

type Track struct {
	ID               int64
	SpotifyID        string
	TrackName        string
	Artist           string
	Album            string
	AddedAt          time.Time
	Status           string // pending, searching, downloading, completed, failed, cancelled
	SearchID         *uint32
	SearchTime       *time.Time
	DownloadUsername *string
	DownloadFilename *string
	DownloadPath     *string
	ErrorMsg         *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type TrackRepository struct {
	db Executor
}

func NewTrackRepository(db Executor) *TrackRepository {
	return &TrackRepository{db: db}
}

func (r *TrackRepository) InsertTrack(spotifyID, trackName, artist, album string, addedAt time.Time) error {
	_, err := r.db.Exec(`
		INSERT OR IGNORE INTO tracks (spotify_id, track_name, artist, album, added_at, status)
		VALUES (?, ?, ?, ?, ?, 'pending')
	`, spotifyID, trackName, artist, album, addedAt)
	return err
}

func (r *TrackRepository) TrackExists(spotifyID string) (bool, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM tracks WHERE spotify_id = ?", spotifyID).Scan(&count)
	return count > 0, err
}

func (r *TrackRepository) GetPendingTracks(limit int) ([]Track, error) {
	rows, err := r.db.Query(`
		SELECT id, spotify_id, track_name, artist, album, added_at, status, search_id, search_time, download_username, download_filename, download_path, error_msg, created_at, updated_at
		FROM tracks
		WHERE status = 'pending'
		ORDER BY added_at ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTracks(rows)
}

func (r *TrackRepository) GetTracksByStatus(status string, limit int) ([]Track, error) {
	rows, err := r.db.Query(`
		SELECT id, spotify_id, track_name, artist, album, added_at, status, search_id, search_time, download_username, download_filename, download_path, error_msg, created_at, updated_at
		FROM tracks
		WHERE status = ?
		ORDER BY updated_at DESC
		LIMIT ?
	`, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTracks(rows)
}

func (r *TrackRepository) UpdateTrackStatus(id int64, status string, errorMsg *string) error {
	_, err := r.db.Exec(`
		UPDATE tracks
		SET status = ?, error_msg = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, errorMsg, id)
	return err
}

func (r *TrackRepository) UpdateTrackSearchID(id int64, searchID uint32) error {
	now := time.Now()
	_, err := r.db.Exec(`
		UPDATE tracks
		SET search_id = ?, search_time = ?, status = 'searching', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, searchID, now, id)
	return err
}

func (r *TrackRepository) UpdateTrackDownloadPath(id int64, downloadPath string) error {
	_, err := r.db.Exec(`
		UPDATE tracks
		SET download_path = ?, status = 'completed', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, downloadPath, id)
	return err
}

func (r *TrackRepository) GetSearchingTracks() ([]Track, error) {
	rows, err := r.db.Query(`
		SELECT id, spotify_id, track_name, artist, album, added_at, status, search_id, search_time, download_username, download_filename, download_path, error_msg, created_at, updated_at
		FROM tracks
		WHERE status = 'searching' AND search_time IS NOT NULL
		ORDER BY search_time ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTracks(rows)
}

func (r *TrackRepository) GetDownloadingTracks() ([]Track, error) {
	rows, err := r.db.Query(`
		SELECT id, spotify_id, track_name, artist, album, added_at, status, search_id, search_time, download_username, download_filename, download_path, error_msg, created_at, updated_at
		FROM tracks
		WHERE status = 'downloading' AND download_username IS NOT NULL AND download_filename IS NOT NULL
		ORDER BY updated_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTracks(rows)
}

func (r *TrackRepository) scanTracks(rows *sql.Rows) ([]Track, error) {
	var tracks []Track
	for rows.Next() {
		var t Track
		err := rows.Scan(
			&t.ID,
			&t.SpotifyID,
			&t.TrackName,
			&t.Artist,
			&t.Album,
			&t.AddedAt,
			&t.Status,
			&t.SearchID,
			&t.SearchTime,
			&t.DownloadUsername,
			&t.DownloadFilename,
			&t.DownloadPath,
			&t.ErrorMsg,
			&t.CreatedAt,
			&t.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

func (r *TrackRepository) UpdateTrackDownloadInfo(id int64, username, filename string) error {
	_, err := r.db.Exec(`
		UPDATE tracks
		SET download_username = ?, download_filename = ?, status = 'downloading', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, username, filename, id)
	return err
}

func (r *TrackRepository) UpdateTrackDownloadStatus(username, filename, status string, errorMsg *string) error {
	query := `
		UPDATE tracks
		SET status = ?, error_msg = ?, updated_at = CURRENT_TIMESTAMP
		WHERE download_username = ? AND download_filename = ?
	`
	_, err := r.db.Exec(query, status, errorMsg, username, filename)
	return err
}

func (r *TrackRepository) GetStats() (map[string]int, error) {
	stats := make(map[string]int)

	rows, err := r.db.Query(`
		SELECT status, COUNT(*) as count
		FROM tracks
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
