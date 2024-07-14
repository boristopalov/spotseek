package client

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"spotseek/src/config"
	"spotseek/src/slsk/client/listen"
	"spotseek/src/slsk/messages"
	"spotseek/src/slsk/messages/peerMessages"
	"spotseek/src/slsk/messages/serverMessages"
	"spotseek/src/slsk/peer"
	"spotseek/src/slsk/shared"
	"strconv"
	"sync"
)

type IP struct {
	IP   string
	port uint32
}

type PendingTokenConn struct {
	username string
	connType string
}

type SlskClient struct {
	Host                     string
	Port                     int
	Server                   *listen.Listener
	Listener                 net.Listener
	ConnectedPeers           map[string]peer.Peer // username --> Peer
	mu                       sync.RWMutex
	User                     string               // the user that is logged in
	UsernameIps              map[string]IP        // username -> IP address
	PendingPeerInits         map[string]peer.Peer // username -> Peer
	PendingUsernameConnTypes map[string]string
	PendingTokenConnTypes    map[uint32]PendingTokenConn // token --> connType
	TokenSearches            map[uint32]string
	ConnectionToken          uint32
	SearchToken              uint32
	SearchResults            map[uint32][]shared.SearchResult // token --> search results
	DownloadQueue            map[string]*Transfer
	UploadQueue              map[string]*Transfer
	TransferListeners        []TransferListener
}

type Transfer struct {
	Username string
	Filename string
	Size     int64
	Progress int64
	Status   string
}

type TransferListener func(transfer *Transfer)

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
	}
}

func (c *SlskClient) String() string {
	json, err := c.Json()
	if err != nil {
		return ""
	}
	return string(json)
}

func (c *SlskClient) Json() ([]byte, error) {
	json, err := json.MarshalIndent(c, "", " ")
	if err != nil {
		return nil, err
	}
	return json, nil
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
	c.Server = &listen.Listener{Conn: conn}
	c.Listener = listener
	go c.ListenForServerMessages()
	go c.ListenForIncomingPeers()
	c.Login(config.SOULSEEK_USERNAME, config.SOULSEEK_PASSWORD)
	c.SetWaitPort(2234)
	log.Println("Established connection to Soulseek server")
	log.Println("Listening on port 2234")
	c.User = config.SOULSEEK_USERNAME

	//time.AfterFunc(5*time.Second, func () { c.ConnectToPeer("forthelulz", "P") })
	//time.AfterFunc(10*time.Second, func () { c.UserSearch("amsterdamn", "hamada")})

	return nil
}

func (c *SlskClient) Close() error {
	if c.Server == nil {
		return nil // Connection is already closed
	}

	err := c.Server.Close()
	if err != nil {
		return err
	}
	c.User = ""
	log.Println("Connection closed")
	return nil
}

func (c *SlskClient) ReadMessage() ([]byte, error) {
	sizeBuf := make([]byte, 4)
	_, err := io.ReadFull(c.Server.Conn, sizeBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to read message size: %w", err)
	}
	size := binary.LittleEndian.Uint32(sizeBuf)

	message := make([]byte, size)
	_, err = io.ReadFull(c.Server.Conn, message)
	if err != nil {
		return nil, fmt.Errorf("failed to read message body: %w", err)
	}

	return message, nil
}

// ListenForIncomingPeers listens for new peer connections
func (c *SlskClient) ListenForIncomingPeers() {
	for {
		peerConn, err := c.Listener.Accept()
		if err != nil {
			log.Printf("Error accepting peer connection: %v", err)
			continue
		}

		go c.handlePeerConnection(peerConn)
	}
}

func (c *SlskClient) handlePeerConnection(peerConn net.Conn) (map[string]interface{}, error) {
	// defer peerConn.Close()
	message, err := c.readPeerInitMessage(peerConn)
	if err != nil {
		return nil, fmt.Errorf("error reading peer message: %v", err)
	}

	peerMsgReader := peerMessages.PeerInitMessageReader{MessageReader: messages.NewMessageReader(message)}
	code := peerMsgReader.ReadInt8()

	log.Printf("Peer message: code %d; address %s", code, peerConn.RemoteAddr().String())

	var decoded map[string]interface{}
	switch code {
	case 0:
		decoded, err = c.handlePierceFirewall(peerConn, &peerMsgReader)
	case 1:
		decoded, err = c.handlePeerInit(peerConn, &peerMsgReader)
	default:
		return nil, fmt.Errorf("unknown peer message code: %d", code)
	}
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func (c *SlskClient) readPeerInitMessage(conn net.Conn) ([]byte, error) {
	sizeBuf := make([]byte, 4)
	_, err := io.ReadFull(conn, sizeBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to read message size: %w", err)
	}
	size := binary.LittleEndian.Uint32(sizeBuf)

	if size > 4096 {
		return nil, fmt.Errorf("message size too large: %d bytes", size)
	}

	message := make([]byte, size)
	_, err = io.ReadFull(conn, message)
	if err != nil {
		return nil, fmt.Errorf("failed to read message body: %w", err)
	}

	return message, nil
}

func (c *SlskClient) handlePierceFirewall(conn net.Conn, reader *peerMessages.PeerInitMessageReader) (map[string]interface{}, error) {
	token := reader.ParsePierceFirewall()
	usernameAndConnType, ok := c.PendingTokenConnTypes[token]
	if !ok {
		log.Printf("No pending connection for token %d", token)
		return map[string]interface{}{
			"token": token,
		}, nil
	}

	log.Printf("received PierceFirewall from %s", usernameAndConnType.username)
	// return map[string]interface{}{
	// 	"token": token,
	// }, nil

	// I don't think we need to do anything here?
	peer, err := c.createPeerFromConnection(conn, usernameAndConnType.username, usernameAndConnType.connType, token)
	if err != nil {
		log.Printf("Error establishing connection to peer while handling PierceFirewall: %v", err)
		return nil, fmt.Errorf("HandlePierceFirewall Error: %v", err)
	}

	c.mu.Lock()
	c.ConnectedPeers[peer.Username] = *peer
	delete(c.PendingTokenConnTypes, token)
	c.mu.Unlock()

	// err = peer.PierceFirewall(token)
	// if err != nil {
	// 	log.Printf("Error sending PierceFirewall: %v", err)
	// 	return nil, fmt.Errorf("HandlePierceFirewall Error: %v", err)
	// }

	go c.ListenForPeerMessages(peer)
	return map[string]interface{}{
		"token": token,
	}, nil
}

// Step 3 (User B)
// If User B receives the PeerInit message, a connection is established, and user A is free to send peer messages.
func (c *SlskClient) handlePeerInit(conn net.Conn, reader *peerMessages.PeerInitMessageReader) (map[string]interface{}, error) {
	username, connType, token := reader.ParsePeerInit()

	peer, err := c.createPeerFromConnection(conn, username, connType, token)
	if err != nil {
		log.Printf("Error establishing connection to peer while handling PeerInit: %v", err)
		return nil, err
	}
	c.mu.RLock()
	connectedPeer, ok := c.ConnectedPeers[username]
	c.mu.RUnlock()
	if ok && connType == connectedPeer.ConnType {
		log.Printf("Already connected to %s", username)
		return nil, err
	}

	log.Printf("Connection established with peer %v", peer)
	c.mu.Lock()
	c.ConnectedPeers[username] = *peer
	delete(c.PendingTokenConnTypes, token)
	c.mu.Unlock()

	go c.ListenForPeerMessages(peer)
	return map[string]interface{}{
		"username": username,
		"connType": connType,
		"token":    token,
	}, nil
}

func (c *SlskClient) createPeerFromConnection(conn net.Conn, username, connType string, token uint32) (*peer.Peer, error) {
	ip, portStr, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		return nil, fmt.Errorf("error getting IP and port: %w", err)
	}

	port, err := strconv.ParseUint(portStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("error parsing port: %w", err)
	}

	peer, err := peer.NewPeer(username, c.Listener, connType, token, ip, uint32(port))
	if err != nil {
		return nil, err
	}
	return peer, nil
}

func (c *SlskClient) ListenForPeerMessages(p *peer.Peer) {
	for {
		message, err := p.ReadMessage()
		if err != nil {
			if err == io.EOF {
				log.Printf("Peer %s closed the connection", p.Username)
				p.Conn.Close()
				c.mu.Lock()
				delete(c.ConnectedPeers, p.Username)
				c.mu.Unlock()
				return
			}
			log.Printf("Error reading from peer %s: %v", p.Username, err)
			continue
		}

		mr := peerMessages.PeerMessageReader{MessageReader: messages.NewMessageReader(message)}
		msg, err := mr.HandlePeerMessage()
		if err != nil {
			log.Println("Error reading message from peer:", err)
		} else {
			log.Println("Message from peer:", msg)
			if msg["type"] == "FileSearchResponse" {
				c.SearchResults[msg["token"].(uint32)] = msg["results"].([]shared.SearchResult)
			}
		}
	}
}

// SERVER MESSAGE HANDLING
func (c *SlskClient) ListenForServerMessages() {
	readBuffer := make([]byte, 4096)
	var currentMessage []byte
	var messageLength uint32

	for {
		n, err := c.Server.Read(readBuffer)
		if err != nil {
			log.Printf("Error reading from server connection: %v", err)
			return
		}

		currentMessage = append(currentMessage, readBuffer[:n]...)
		currentMessage = c.processServerMessages(currentMessage, &messageLength)
	}
}

func (c *SlskClient) processServerMessages(data []byte, messageLength *uint32) []byte {
	for {
		if *messageLength == 0 {
			if len(data) < 4 {
				return data // Not enough data to read message length
			}
			*messageLength = binary.LittleEndian.Uint32(data[:4])
			data = data[4:]
		}

		if uint32(len(data)) < *messageLength {
			return data // Not enough data for full message
		}

		c.handleServerMessage(data[:*messageLength])

		data = data[*messageLength:]
		*messageLength = 0
	}
}

func (c *SlskClient) handleServerMessage(messageData []byte) {
	mr := messages.NewMessageReader(messageData)
	serverMsgReader := serverMessages.ServerMessageReader{MessageReader: mr}

	msg, err := c.HandleServerMessage(&serverMsgReader)
	if err != nil {
		log.Printf("Error decoding server message: %v", err)
	} else {
		log.Printf("Server message: message: %v", msg)
	}
	log.Println("--------------- End of message ----------------")
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
