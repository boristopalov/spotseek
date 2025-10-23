package api

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"spotseek/slsk/client"
	"strings"

	"github.com/go-chi/chi/v5"
)

type APIHandler struct {
	client *client.SlskClient
}

func NewAPIHandler(client *client.SlskClient) *APIHandler {
	return &APIHandler{client: client}
}

func (h *APIHandler) GetSlskClient(w http.ResponseWriter, r *http.Request) {
	json, err := h.client.Json()
	if err != nil {
		http.Error(w, "Cannot marshal slskClient to json", http.StatusInternalServerError)
	}
	w.Write(json)
}

// check if our port is open to receive incoming connections
func (h *APIHandler) CheckPort(w http.ResponseWriter, r *http.Request) {
	port := r.URL.Query().Get("port")
	if port == "" {
		http.Error(w, "empty port param", http.StatusBadRequest)
		return
	}
	url := fmt.Sprintf("http://tools.slsknet.org/porttest.php?port=%s", port)
	res, err := http.Get(url)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		http.Error(w, "Error reading response body", http.StatusInternalServerError)
		return
	}
	isOpen := (strings.Contains(string(body), fmt.Sprintf("Port: %s/tcp open", port)))
	if isOpen {
		w.Write(fmt.Appendf(nil, "Port %s is open", port))
	} else {
		w.Write(fmt.Appendf(nil, "Port %s is closed", port))
	}
}

func (h *APIHandler) ConnectToPeer(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	connType := chi.URLParam(r, "connType")
	if username == "" || connType == "" {
		http.Error(w, "Missing username or connType parameter", http.StatusBadRequest)
		return
	}

	h.client.ConnectToPeer(username, connType, 0)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Connection to peer initiated"})
}

func (h *APIHandler) Download(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SearchID *uint32 `json:"searchId"` // Optional
		Username string  `json:"username"`
		Filename string  `json:"filename"` // Full virtual path
		Size     uint64  `json:"size"`     // Expected file size
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Filename == "" || req.Size <= 0 {
		http.Error(w, "Missing username, filename, or size", http.StatusBadRequest)
		return
	}

	// Generate transfer token
	token := uint32(rand.Int32())

	// Create download record
	download := h.client.DownloadManager.CreateDownload(req.SearchID, req.Username, req.Filename, req.Size, token)

	// Check if peer connection exists, if not connect
	peer := h.client.PeerManager.GetPeer(req.Username)
	if peer == nil {
		// Auto-connect to peer
		h.client.ConnectToPeer(req.Username, "P", 0)
		download.UpdateStatus("connecting")
	}

	// Queue the upload (initiate download from our perspective)
	peer = h.client.PeerManager.GetPeer(req.Username)
	if peer != nil {
		peer.QueueUpload(req.Filename)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"downloadId": download.ID,
		"status":     download.Status,
		"username":   download.Username,
		"filename":   download.Filename,
	})
}

func (h *APIHandler) GetDownload(w http.ResponseWriter, r *http.Request) {
	downloadIDStr := chi.URLParam(r, "id")
	if downloadIDStr == "" {
		http.Error(w, "Missing download ID", http.StatusBadRequest)
		return
	}

	var downloadID uint32
	if _, err := fmt.Sscanf(downloadIDStr, "%d", &downloadID); err != nil {
		http.Error(w, "Invalid download ID", http.StatusBadRequest)
		return
	}

	download, err := h.client.DownloadManager.GetDownload(downloadID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(download)
}

func (h *APIHandler) ListDownloads(w http.ResponseWriter, r *http.Request) {
	downloads := h.client.DownloadManager.ListDownloads()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"downloads": downloads,
	})
}

func (h *APIHandler) CancelDownload(w http.ResponseWriter, r *http.Request) {
	downloadIDStr := chi.URLParam(r, "id")
	if downloadIDStr == "" {
		http.Error(w, "Missing download ID", http.StatusBadRequest)
		return
	}

	var downloadID uint32
	if _, err := fmt.Sscanf(downloadIDStr, "%d", &downloadID); err != nil {
		http.Error(w, "Invalid download ID", http.StatusBadRequest)
		return
	}

	err := h.client.DownloadManager.CancelDownload(downloadID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Download cancelled"})
}

func (h *APIHandler) Search(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		http.Error(w, "Missing query field", http.StatusBadRequest)
		return
	}

	searchID := h.client.FileSearch(req.Query)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"searchId": searchID,
		"query":    req.Query,
	})
}

func (h *APIHandler) GetSearch(w http.ResponseWriter, r *http.Request) {
	searchIDStr := chi.URLParam(r, "id")
	if searchIDStr == "" {
		http.Error(w, "Missing search ID", http.StatusBadRequest)
		return
	}

	var searchID uint32
	if _, err := fmt.Sscanf(searchIDStr, "%d", &searchID); err != nil {
		http.Error(w, "Invalid search ID", http.StatusBadRequest)
		return
	}

	search, err := h.client.SearchManager.GetSearch(searchID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":          search.ID,
		"query":       search.Query,
		"status":      search.GetStatus(),
		"createdAt":   search.CreatedAt,
		"resultCount": search.GetResultCount(),
	})
}

func (h *APIHandler) GetSearchResults(w http.ResponseWriter, r *http.Request) {
	searchIDStr := chi.URLParam(r, "id")
	if searchIDStr == "" {
		http.Error(w, "Missing search ID", http.StatusBadRequest)
		return
	}

	var searchID uint32
	if _, err := fmt.Sscanf(searchIDStr, "%d", &searchID); err != nil {
		http.Error(w, "Invalid search ID", http.StatusBadRequest)
		return
	}

	search, err := h.client.SearchManager.GetSearch(searchID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	results := search.GetResults()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"searchId": searchID,
		"query":    search.Query,
		"status":   search.GetStatus(),
		"results":  results,
	})
}

func (h *APIHandler) JoinRoom(w http.ResponseWriter, r *http.Request) {
	room := r.URL.Query().Get("room")
	if room == "" {
		http.Error(w, "Missing room parameter", http.StatusBadRequest)
		return
	}
	h.client.JoinRoom(room)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Joined room"})
}
