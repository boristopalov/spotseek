package peer

import "spotseek/slsk/fileshare"

type Event int

const (
	PeerDisconnected Event = iota
	FileSearchResponse
	SharedFileListRequest
	FolderContentsRequest
	PlaceInQueueResponse
	UploadRequest
	TransferRequest
	UploadComplete
	// Maximum allowed message size (32MB)
	MaxMessageSize = 32 * 1024 * 1024
)

type PeerEvent struct {
	Type Event
	Peer *Peer
	Msg  any
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

type TransferRequestMsg struct {
	Token        uint32
	Filename     string
	PeerUsername string
	Size         uint64
}

type UploadCompleteMsg struct {
	Token    uint32
	Filename string
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
