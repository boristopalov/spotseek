package shared

type SearchResult struct {
	Username    string `json:"username"`
	Token       uint32 `json:"token"`
	PublicFiles []File `json:"publicFiles"`
	SlotFree    uint8  `json:"slotFree"`
	AvgSpeed    uint32 `json:"avgSpeed"`
	QueueLength uint32 `json:"queueLength"`
}

type SharedFileList struct {
	Directories []Directory `json:"directories"`
}

type Directory struct {
	Name  string `json:"name"`
	Files []File `json:"files"`
}

type File struct {
	Name       string `json:"name"`
	Size       uint64 `json:"size"`
	BitRate    uint32 `json:"bitRate"`
	Extension  string `json:"extension"`
	Duration   uint32 `json:"duration"`
	SampleRate uint32 `json:"sampleRate"`
}
