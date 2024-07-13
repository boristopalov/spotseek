package api

import (
	"encoding/json"
	"net/http"
	"spotseek/src/slsk/client"

	"github.com/go-chi/chi/v5"
)

func ConnectToPeer(w http.ResponseWriter, r *http.Request) {
	c, ok := r.Context().Value("slskClient").(client.SlskClient)
	if !ok {
		http.Error(w, "Cannot retrieve slskClient from context", http.StatusInternalServerError)
		return
	}
	username := chi.URLParam(r, "username")
	connType := chi.URLParam(r, "connType")
	if username == "" || connType == "" {
		http.Error(w, "Missing username or connType parameter", http.StatusBadRequest)
		return
	}

	c.ConnectToPeer(username, connType)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Connection to peer initiated"})
}

func Download(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	filename := r.URL.Query().Get("filename")
	if username == "" || filename == "" {
		http.Error(w, "Missing username or filename parameter", http.StatusBadRequest)
		return
	}

	c, ok := r.Context().Value("slskClient").(client.SlskClient)
	if !ok {
		http.Error(w, "Cannot retrieve slskClient from context", http.StatusInternalServerError)
		return
	}

	peer, ok := c.ConnectedPeers[username]
	if !ok {
		http.Error(w, "Not connected to the specified peer", http.StatusBadRequest)
		return
	}

	err := peer.QueueUpload(filename)
	if err != nil {
		http.Error(w, "Failed to queue upload: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Download queued"})
}

func Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		http.Error(w, "Missing query parameter", http.StatusBadRequest)
		return
	}

	c, ok := r.Context().Value("slskClient").(client.SlskClient)
	if !ok {
		http.Error(w, "Cannot retrieve slskClient from context", http.StatusInternalServerError)
		return
	}

	c.FileSearch(query)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Search initiated"})
}
