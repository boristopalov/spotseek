package api

import (
	"encoding/json"
	"fmt"
	"io"
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
	username := r.URL.Query().Get("username")
	connType := r.URL.Query().Get("connType")
	filename := r.URL.Query().Get("filename")
	if username == "" || connType == "" || filename == "" {
		http.Error(w, "Missing username, connType, or filename parameter", http.StatusBadRequest)
		return
	}

	peer := h.client.PeerManager.GetPeer(username)
	if peer == nil {
		http.Error(w, "Not connected to the specified peer", http.StatusBadRequest)
		return
	}

	peer.QueueUpload(filename)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Download queued"})
}

func (h *APIHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		http.Error(w, "Missing query parameter", http.StatusBadRequest)
		return
	}

	h.client.FileSearch(query)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Search initiated"})
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
