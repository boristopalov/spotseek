package client

import (
	"errors"
	"fmt"
	"log"
	"net"
	"spotseek/src/config"
	"spotseek/src/slsk/listen"
	"spotseek/src/slsk/peer"
	"spotseek/src/slsk/shared"
)

func NewSlskClient(host string, port int) *SlskClient {
	return &SlskClient{
		Host:                     host,
		Port:                     port,
		ConnectionToken:          0,
		SearchToken:              0,
		DownloadQueue:            make(map[string]*Transfer),
		UploadQueue:              make(map[string]*Transfer),
		TransferListeners:        make([]TransferListener, 0),
		SearchResults:            make(map[uint32][]shared.SearchResult),
		TokenSearches:            make(map[uint32]string),
		PendingTokenConnTypes:    make(map[uint32]PendingTokenConn),
		PendingUsernameConnTypes: make(map[string]string),
		UsernameIps:              make(map[string]IP),
		ConnectedPeers:           make(map[string]peer.Peer),
		PendingPeerInits:         make(map[string]peer.Peer),
		JoinedRooms:              make(map[string][]string),
	}
}

func (c *SlskClient) Connect() error {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port))
	if err != nil {
		return errors.New("unable to dial tcp connection; " + err.Error())
	}
	listener, err := net.Listen("tcp", ":2234")
	if err != nil {
		return errors.New("unable to dial tcp connection; " + err.Error())
	}
	c.ServerConnection = &listen.Connection{Conn: conn}
	c.Listener = listener
	go c.ListenForServerMessages()
	go c.ListenForIncomingPeers()
	c.Login(config.SOULSEEK_USERNAME, config.SOULSEEK_PASSWORD)
	c.SetWaitPort(2234)
	log.Println("Established connection to Soulseek server")
	log.Println("Listening on port 2234")
	c.User = config.SOULSEEK_USERNAME
	return nil
}

func (c *SlskClient) Close() error {
	if c.ServerConnection == nil {
		return nil // Connection is already closed
	}

	err := c.ServerConnection.Close()
	if err != nil {
		return err
	}
	c.User = ""
	log.Println("Connection closed")
	return nil
}

// FILE TRANSFER HANDLING
func (c *SlskClient) QueueDownload(username, filename string, size int64) error {
	key := username + "|" + filename
	if _, exists := c.DownloadQueue[key]; exists {
		return errors.New("download already queued")
	}

	c.DownloadQueue[key] = &Transfer{
		Username: username,
		Filename: filename,
		Size:     size,
		Progress: 0,
		Status:   "Queued",
	}

	c.mu.RLock()
	peer, ok := c.ConnectedPeers[username]
	c.mu.RUnlock()
	if !ok {
		return errors.New("not connected to peer")
	}

	return peer.QueueUpload(filename)
}

func (c *SlskClient) UpdateTransferProgress(username, filename string, progress int64, isUpload bool) {
	key := username + "|" + filename
	var transfer *Transfer

	if isUpload {
		if upload, ok := c.UploadQueue[key]; ok {
			upload.Progress = progress
			upload.Status = "Transferring"
			transfer = upload
		}
	} else {
		if download, ok := c.DownloadQueue[key]; ok {
			download.Progress = progress
			download.Status = "Transferring"
			transfer = download
		}
	}

	if transfer != nil {
		for _, listener := range c.TransferListeners {
			listener(transfer)
		}
	}
}

func (c *SlskClient) AddTransferListener(listener TransferListener) {
	c.TransferListeners = append(c.TransferListeners, listener)
}

// func (c *SlskClient) DownloadPeerFile(token uint32, peer *peer.Peer) error {
// 	log.Printf("Downloading file from peer %s", peer.Username)

// 	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", peer.Host, peer.Port))
// 	if err != nil {
// 		return fmt.Errorf("failed to connect to peer: %v", err)
// 	}
// 	defer conn.Close()

// 	c.DownloadQueue
// 	c.UpdateTransferProgress(peer.Username, filename, 0, false)

// 	received := false
// 	requestToken := token
// 	buf := make([]byte, 0, size)
// 	reader := bufio.NewReader(conn)

// 	for {
// 		if !noPierce && !received {
// 			tokenBytes := make([]byte, 4)
// 			_, err := io.ReadFull(reader, tokenBytes)
// 			if err != nil {
// 				return fmt.Errorf("failed to read token: %v", err)
// 			}
// 			requestToken = uint32(tokenBytes[0]) | uint32(tokenBytes[1])<<8 | uint32(tokenBytes[2])<<16 | uint32(tokenBytes[3])<<24
// 			conn.Write([]byte{0, 0, 0, 0, 0, 0, 0, 0})
// 			received = true
// 		} else {
// 			chunk := make([]byte, 4096)
// 			n, err := reader.Read(chunk)
// 			if err == io.EOF {
// 				break
// 			}
// 			if err != nil {
// 				return fmt.Errorf("error reading file data: %v", err)
// 			}
// 			buf = append(buf, chunk[:n]...)

// 			if int64(len(buf)) >= size {
// 				break
// 			}
// 		}
// 	}

// 	filePath := getFilePathName(p.Username, filename)
// 	err = os.MkdirAll(filepath.Dir(filePath), 0755)
// 	if err != nil {
// 		return fmt.Errorf("failed to create directory: %v", err)
// 	}

// 	err = os.WriteFile(filePath, buf, 0644)
// 	if err != nil {
// 		return fmt.Errorf("failed to write file: %v", err)
// 	}

// 	log.Printf("File downloaded successfully: %s", filePath)
// 	return nil
// }

// func getFilePathName(user, file string) string {
// 	return filepath.Join(os.TempDir(), "slsk", fmt.Sprintf("%s_%s", user, filepath.Base(file)))
// }
