package config

import (
	"encoding/json"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	"github.com/joho/godotenv"
)

var (
	CLIENT_ID          string
	CLIENT_SECRET      string
	REDIRECT_URI       string
	REDIS_URI          string
	REDIS_PASSWORD     string
	REDIS_DB           int
	SOULSEEK_USERNAME  string
	SOULSEEK_PASSWORD  string
	DEFAULT_SHARE_PATH string

	settings *Settings
	once     sync.Once
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
	DEFAULT_SHARE_PATH = os.Getenv("DEFAULT_SHARE_PATH")
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

func GetSettings() *Settings {
	if settings != nil {
		return settings
	}
	settingsPath := getSettingsPath()

	// Default settings
	settings := &Settings{
		DownloadPath: filepath.Join(os.Getenv("HOME"), "Downloads"),
		SharePaths:   []string{DEFAULT_SHARE_PATH},
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

	return settings
}

func SaveSettings(newSettings *Settings) error {
	data, err := json.MarshalIndent(newSettings, "", "  ")
	if err != nil {
		return err
	}

	settings = newSettings
	settingsPath := getSettingsPath()
	return os.WriteFile(settingsPath, data, 0600)
}
