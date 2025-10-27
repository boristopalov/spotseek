package peer

import (
	"spotseek/slsk/fileshare"
	"time"
)

type Event int

const (
	PeerDisconnected Event = iota
	FileSearchResponse
	SharedFileListRequest
	FolderContentsRequest
	PlaceInQueueResponse
	UploadStart
	UploadRequest
	UploadProgress
	UploadComplete
	DownloadRequest
	DownloadProgress
	DownloadComplete
	DownloadFailed
	// Maximum allowed message size (32MB)
	MaxMessageSize = 32 * 1024 * 1024
)

type PeerEvent struct {
	Type     Event
	ConnType PeerType
	Username string
	Host     string
	Port     uint32
	Msg      any
}

type FolderContentsMsg struct {
	FolderName string
	Token      uint32
}

type SharedFileListMsg struct{}

type FileSearchMsg struct {
	Token   uint32
	Results fileshare.SearchResult
}

type PlaceInQueueMsg struct {
	Filename string
	Place    uint32
}

type DownloadRequestMsg struct {
	Token        uint32
	Filename     string
	PeerUsername string
	Size         uint64
}

type UploadStartMsg struct {
	Token    uint32
	Username string
}

type UploadProgressMsg struct {
	Username  string
	Filename  string
	BytesSent uint64
	Token     uint32
}

type UploadCompleteMsg struct {
	Token       uint32
	Filename    string
	TimeElapsed time.Duration
}

type DistribSearchMsg struct {
	Username string
	Token    uint32
	Query    string
}

type BranchLevelMsg struct {
	BranchLevel uint32
}

type BranchRootMsg struct {
	BranchRootUsername string
}

type UploadRequestMsg struct {
	Filename string
}

type DownloadProgressMsg struct {
	Username      string
	Filename      string
	BytesReceived uint64
}

type DownloadCompleteMsg struct {
	Username string
	Filename string
}

type DownloadFailedMsg struct {
	Username string
	Filename string
	Error    string
}
