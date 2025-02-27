package fileshare

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"spotseek/config"

	"go.senan.xyz/taglib"
)

var validExts = map[string]bool{
	".mp3":  true,
	".wav":  true,
	".flac": true,
	".m4a":  true,
	".ogg":  true,
	".wma":  true,
}

// ShareStats holds statistics about shared files and folders
type ShareStats struct {
	TotalFiles   uint32
	TotalFolders uint32
}

// SharedFile represents a file in the shared system
type SharedFile struct {
	Key         string
	Value       FileInfo
	VirtualPath string
}

// FileInfo contains file metadata
type FileInfo struct {
	Path            string // Real path
	Size            int64  // File size in bytes
	BitRate         uint   // BitRate in kbps
	DurationSeconds uint   // Duration in seconds
	SampleRate      uint   // Sample rate in Hz
	BitDepth        uint   // Bit depth
}

// Shared manages the collection of shared files
type Shared struct {
	Files    []SharedFile
	settings *config.Settings
	logger   *slog.Logger
}

// NewShared creates a new Shared instance
func NewShared(settings *config.Settings, logger *slog.Logger) *Shared {
	shared := &Shared{
		Files:    make([]SharedFile, 0),
		settings: settings,
		logger:   logger,
	}

	// Create initial symlinks and scan directories
	err := shared.RefreshShares()
	if err != nil {
		shared.logger.Warn("Warning: Error refreshing shares", "err", err)
	}

	shared.logger.Info("Shared initialized", "numFiles", len(shared.Files))
	return shared
}

// GetShareStats returns statistics about shared files and folders
func (s *Shared) GetShareStats() ShareStats {
	stats := ShareStats{
		TotalFiles:   uint32(len(s.Files)),
		TotalFolders: uint32(len(s.settings.SharePaths)),
	}
	return stats
}

// RefreshShares scans real paths for files, collects metadata, then creates symlinks
func (s *Shared) RefreshShares() error {
	// Clear existing files
	s.Files = make([]SharedFile, 0)

	// Get base directory for symlinks
	shareLinksDir := getShareLinksDir()

	// Scan real paths first
	for _, realPath := range s.settings.SharePaths {
		err := filepath.Walk(realPath, func(path string, info os.FileInfo, err error) error {

			if err != nil {
				return err
			}

			relPath, err := filepath.Rel(realPath, path)

			if err != nil {
				return err
			}

			if info.IsDir() {
				// Create directory structure in shareLinksDir
				targetDir := filepath.Join(shareLinksDir, relPath)
				if err := os.MkdirAll(targetDir, 0755); err != nil {
					s.logger.Error("could not create directory",
						"dir", targetDir,
						"err", err,
					)
					return err
				}
				return nil
			}

			// Get audio metadata
			fileInfo, err := getAudioMetadata(path)
			if err != nil {
				s.logger.Error("could not read audio metadata",
					"path", path,
					"err", err,
				)
				// Set basic file info even if metadata reading fails
				fileInfo = FileInfo{
					Path: path,
					Size: info.Size(),
				}
			}

			// Create symlink
			symlinkPath := filepath.Join(shareLinksDir, relPath)
			// Remove existing symlink if it exists
			if err := os.Remove(symlinkPath); err != nil && !os.IsNotExist(err) {
				s.logger.Error("could not remove existing symlink",
					"path", symlinkPath,
					"err", err,
				)
				return err
			}
			// Create new symlink
			if err := os.Symlink(path, symlinkPath); err != nil {
				s.logger.Error("could not create symlink",
					"from", path,
					"to", symlinkPath,
					"err", err,
				)
				return err
			}
			f := SharedFile{
				Key:         relPath,
				Value:       fileInfo,
				VirtualPath: symlinkPath,
			}

			// log.Info("file info", "sharedFile", f)
			s.Files = append(s.Files, f)
			return nil
		})
		if err != nil {
			s.logger.Error("could not scan folder",
				"realPath", realPath,
				"err", err,
			)
		}
	}

	return nil
}

// AddSharedDirectory adds a new directory to be shared
func (s *Shared) AddSharedDirectory(realPath string) error {
	// Add to settings
	s.settings.SharePaths = append(s.settings.SharePaths, realPath)

	// Save settings
	err := config.SaveSettings(s.settings)
	if err != nil {
		return fmt.Errorf("failed to save settings: %v", err)
	}

	// Refresh symlinks and scan
	return s.RefreshShares()
}

// RemoveSharedDirectory removes a shared directory
func (s *Shared) RemoveSharedDirectory(realPath string) error {
	// Remove from settings
	newPaths := make([]string, 0)
	for _, path := range s.settings.SharePaths {
		if path != realPath {
			newPaths = append(newPaths, path)
		}
	}
	s.settings.SharePaths = newPaths

	// Save settings
	err := config.SaveSettings(s.settings)
	if err != nil {
		return fmt.Errorf("failed to save settings: %v", err)
	}

	// Refresh symlinks and scan
	return s.RefreshShares()
}

// extracts audio file metadata using taglib
func getAudioMetadata(path string) (FileInfo, error) {
	info := FileInfo{}
	info.Path = path

	// Get file size
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return info, fmt.Errorf("file does not exist: %v", err)
		}
		return info, fmt.Errorf("failed to get file stats: %v", err)
	}
	if !stat.Mode().IsRegular() {
		return info, fmt.Errorf("path is not a regular file: %s", path)
	}
	ext := strings.ToLower(filepath.Ext(path))
	if !validExts[ext] {
		return info, fmt.Errorf("file is not a supported audio format: %s", ext)
	}

	info.Size = stat.Size()

	props, err := taglib.ReadProperties(path)
	if err != nil {
		return info, fmt.Errorf("failed to read properties: %v", err)
	}

	info.BitRate = props.Bitrate
	info.SampleRate = props.SampleRate
	info.DurationSeconds = uint(props.Length.Seconds())

	return info, nil
}

// Search returns files matching the query
func (s *Shared) Search(query string) []SharedFile {
	var results []SharedFile
	for _, file := range s.Files {
		if matches(file.Key, query) {
			results = append(results, file)
		}
	}
	return results
}

// matches checks if a string matches the search query
// '-' in the query means that the string should not contain the following criterion
func matches(str, query string) bool {
	str = strings.ToLower(str)
	criteria := strings.Split(strings.ToLower(query), " ")

	for _, criterion := range criteria {
		if strings.HasPrefix(criterion, "-") {
			if strings.Contains(str, criterion[1:]) {
				return false
			}
		} else if !strings.Contains(str, criterion) {
			return false
		}
	}
	return true
}

func getShareLinksDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	shareLinksPath := filepath.Join(home, config.APP_NAME, "shares")
	if _, err := os.Stat(shareLinksPath); os.IsNotExist(err) {
		err = os.MkdirAll(shareLinksPath, 0755)
		if err != nil {
			return ""
		}
	}
	return shareLinksPath
}
