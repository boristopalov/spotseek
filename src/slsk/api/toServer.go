package api

import (
	"log"
	"net/http"
	"spotseek/src/slsk/client"

	"github.com/go-chi/chi/v5"
)

func ConnectToPeer(w http.ResponseWriter, r *http.Request) {
	c, ok := r.Context().Value("slskClient").(client.SlskClient)
	if !ok {
		log.Println(c)
		log.Println("no client found in api.ConnectToPeer()")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	username := chi.URLParam(r, "username")
	connType := chi.URLParam(r, "connType")
	c.ConnectToPeer(username, connType)
}

func DownloadFile(w http.ResponseWriter, r *http.Request) {

}
