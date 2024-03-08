package api

import (
	"fmt"
	"net/http"
	"spotseek/src/slskClient"

	"github.com/go-chi/chi/v5"
)

func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("service is up!"))
}

func ConnectToPeer(w http.ResponseWriter, r *http.Request) {
    c, ok := r.Context().Value("slskClient").(slskClient.SlskClient)  
    if !ok {
        fmt.Println(c)
        fmt.Println("no client found in fn ConnectToPeer()")
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
        return
    }
    fmt.Println(c.String())
    username := chi.URLParam(r, "username")
    connType := chi.URLParam(r, "connType")
    c.ConnectToPeer(username, connType)
}


func DownloadFile(w http.ResponseWriter, r *http.Request) {

}
