package api

import (
	"fmt"
	"io"
	"net/http"
	"spotseek/src/slskClient"
	"strings"
)


func GetSlskClient(w http.ResponseWriter, r *http.Request) {
    c, ok := r.Context().Value("slskClient").(slskClient.SlskClient)  
    if !ok {
        http.Error(w, "Cannot retrieve slskClient from context", http.StatusInternalServerError)
        return
    }
    json, err := c.Json()
    if err != nil {
        http.Error(w, "Cannot marshal slskClient to json", http.StatusInternalServerError)
    }
    w.Write(json)
}

// check if our port is open to receive incoming connections
func CheckPort(w http.ResponseWriter, r *http.Request) { 
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
    isOpen := (strings.Index(string(body), fmt.Sprintf("Port: %s/tcp open", port)) != -1)
    if isOpen {
        w.Write([]byte(fmt.Sprintf("Port %s is open", port)))
    } else {
        w.Write([]byte(fmt.Sprintf("Port %s is closed", port)))
    }
    return
}
    

