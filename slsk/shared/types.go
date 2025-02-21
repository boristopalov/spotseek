package shared

type SearchResult struct {
	Username    string `json:"username"`
	Token       uint32 `json:"token"`
	PublicFiles []File `json:"publicFiles"`
	SlotFree    uint8  `json:"slotFree"`
	AvgSpeed    uint32 `json:"avgSpeed"`
	QueueLength uint32 `json:"queueLength"`
}

type File struct {
	Filename string `json:"filename"`
	Size     uint64 `json:"size"`
	BitRate  uint32 `json:"bitRate"`
	Duration uint32 `json:"duration"`
}
