package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/joho/godotenv"
)

var (
	CLIENT_ID         string
	CLIENT_SECRET     string
	REDIRECT_URI      string
	REDIS_URI         string
	REDIS_PASSWORD    string
	REDIS_DB          int
	SOULSEEK_USERNAME string
	SOULSEEK_PASSWORD string
)

type Settings struct {
	DownloadPath string            `json:"downloadPath"`
	SharePaths   []string          `json:"sharePaths"`
	shareLinks   map[string]string // internal mapping of virtual to real paths
}

const APP_NAME = ".spotseek"
const SETTINGS_FILE_NAME = "settings.json"

func rootDir() string {
	_, b, _, _ := runtime.Caller(0)
	d := path.Join(path.Dir(b))
	return filepath.Dir(d)
}

func init() {
	// load environment variables from .env file
	err := godotenv.Load(filepath.Join(rootDir(), ".env"))
	if err != nil {
		// handle error
		log.Fatal("cannot load .env file")
	}

	// retrieve the environment variables and store them in package-level variables
	SOULSEEK_USERNAME = os.Getenv("SOULSEEK_USERNAME")
	SOULSEEK_PASSWORD = os.Getenv("SOULSEEK_PASSWORD")
	CLIENT_ID = os.Getenv("CLIENT_ID")
	CLIENT_SECRET = os.Getenv("CLIENT_SECRET")
	REDIRECT_URI = os.Getenv("REDIRECT_URI")
	REDIS_URI = os.Getenv("REDIS_URI")
	REDIS_PASSWORD = os.Getenv("REDIS_PASSWORD")
	redisDbInt, err := strconv.Atoi(os.Getenv("REDIS_DB"))
	if err != nil {
		log.Fatal("cannot read Redis connection params")
	}
	REDIS_DB = redisDbInt
}

func getSettingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("Cannot determine user home directory")
	}
	appPath := filepath.Join(home, APP_NAME)
	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		os.Mkdir(appPath, 0755)
	}
	return filepath.Join(appPath, SETTINGS_FILE_NAME)
}

func getShareLinksDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("Cannot determine user home directory")
	}
	shareLinksPath := filepath.Join(home, APP_NAME, "shares")
	if _, err := os.Stat(shareLinksPath); os.IsNotExist(err) {
		err = os.MkdirAll(shareLinksPath, 0755)
		if err != nil {
			log.Fatal("Cannot create share links directory")
		}
	}
	return shareLinksPath
}

func (s *Settings) CreateShareLinks() error {
	shareLinksDir := getShareLinksDir()
	s.shareLinks = make(map[string]string)

	// Clean up existing symlinks
	entries, _ := os.ReadDir(shareLinksDir)
	for _, entry := range entries {
		os.Remove(filepath.Join(shareLinksDir, entry.Name()))
	}

	// Create new symlinks
	for i, realPath := range s.SharePaths {
		if realPath == "" {
			continue
		}
		// Create a generic name for the symlink
		linkName := fmt.Sprintf("share_%d", i)
		linkPath := filepath.Join(shareLinksDir, linkName)

		err := os.Symlink(realPath, linkPath)
		if err != nil {
			return fmt.Errorf("failed to create symlink for %s: %v", realPath, err)
		}
		s.shareLinks[linkPath] = realPath
	}
	return nil
}

func (s *Settings) GetShareLinks() []string {
	shareLinksDir := getShareLinksDir()
	var links []string
	entries, err := os.ReadDir(shareLinksDir)
	if err != nil {
		return []string{}
	}
	for _, entry := range entries {
		links = append(links, filepath.Join(shareLinksDir, entry.Name()))
	}
	return links
}

func GetSettings() *Settings {
	settingsPath := getSettingsPath()

	// Default settings
	settings := &Settings{
		DownloadPath: filepath.Join(os.Getenv("HOME"), "Downloads"),
		SharePaths:   []string{"/Users/boris/Desktop/Music"},
		shareLinks:   make(map[string]string),
	}

	// Try to read existing settings
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		err = json.Unmarshal(data, settings)
		if err != nil {
			log.Printf("Error parsing settings file: %v", err)
			// Continue with defaults
		}
	}

	// Create symlinks for share paths
	err = settings.CreateShareLinks()
	if err != nil {
		log.Printf("Error creating share links: %v", err)
	}

	return settings
}

func SaveSettings(settings *Settings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	settingsPath := getSettingsPath()
	return os.WriteFile(settingsPath, data, 0600)
}
