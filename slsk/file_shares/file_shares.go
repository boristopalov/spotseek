package file_shares

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"spotseek/config"

	"go.senan.xyz/taglib"
)

// SharedFile represents a file in the shared system
type SharedFile struct {
	Key      string
	Value    FileInfo
	RealPath string // Added to store the real path
}

// FileInfo contains file metadata
type FileInfo struct {
	Path            string // Virtual path
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
}

// NewShared creates a new Shared instance
func NewShared(settings *config.Settings) *Shared {
	shared := &Shared{
		Files:    make([]SharedFile, 0),
		settings: settings,
	}

	// Create initial symlinks and scan directories
	err := shared.RefreshShares()
	if err != nil {
		log.Printf("Warning: Error refreshing shares: %v", err)
	}

	return shared
}

// RefreshShares updates symlinks and rescans all shared directories
func (s *Shared) RefreshShares() error {
	// This will recreate all symlinks based on SharePaths in settings
	err := s.settings.CreateShareLinks()
	if err != nil {
		return fmt.Errorf("failed to create share links: %v", err)
	}

	// Clear existing files
	s.Files = make([]SharedFile, 0)

	// Scan all virtual paths
	virtualPaths := s.settings.GetShareLinks()
	for _, virtualPath := range virtualPaths {
		err := s.ScanFolder(virtualPath)
		if err != nil {
			log.Printf("Warning: Could not scan folder %s: %v", virtualPath, err)
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

// getAudioMetadata extracts audio file metadata using taglib
func getAudioMetadata(path string) (FileInfo, error) {
	info := FileInfo{}
	info.Path = path

	// Get file size
	stat, err := os.Stat(path)
	if err != nil {
		return info, fmt.Errorf("failed to get file stats: %v", err)
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

// ScanFolder scans a folder and all its subfolders for files
func (s *Shared) ScanFolder(virtualPath string) error {
	return filepath.Walk(virtualPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, err := filepath.Rel(virtualPath, path)
			if err != nil {
				return err
			}

			// Get real path by resolving symlink
			realBase, err := os.Readlink(virtualPath)
			if err != nil {
				return fmt.Errorf("failed to resolve symlink %s: %v", virtualPath, err)
			}
			realPath := filepath.Join(realBase, relPath)

			// Get audio metadata
			fileInfo, err := getAudioMetadata(realPath)
			if err != nil {
				log.Printf("Warning: Could not read audio metadata for %s: %v", realPath, err)
				// Set basic file info even if metadata reading fails
				fileInfo = FileInfo{
					Path: path,
					Size: info.Size(),
				}
			}
			fileInfo.Path = path // Ensure virtual path is set

			s.Files = append(s.Files, SharedFile{
				Key:      relPath,
				Value:    fileInfo,
				RealPath: realPath,
			})
		}
		return nil
	})
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
