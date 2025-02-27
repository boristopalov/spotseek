package fileshare

import (
	"os"
	"path/filepath"
	"testing"

	"log/slog"
	"spotseek/config"
)

func TestSharedFileHasCorrectDir(t *testing.T) {
	// Setup test environment
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	settings := &config.Settings{
		SharePaths: []string{"./testdata"},
	}

	// Create test directory and file
	testDir := filepath.Join(".", "testdata")
	err := os.MkdirAll(testDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testDir)

	// Create a dummy file
	testFile := filepath.Join(testDir, "test.mp3")
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	f.Close()

	// Initialize shared
	shared := NewShared(settings, logger)

	// Verify that at least one file was found
	if len(shared.Files) == 0 {
		t.Fatal("No files were found in the test directory")
	}

	// Check that the Dir field is correctly set
	for _, file := range shared.Files {
		if file.Dir != testDir {
			t.Errorf("Expected Dir to be %s, got %s", testDir, file.Dir)
		}
	}
}
