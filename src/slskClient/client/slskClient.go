package client

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"spotseek/src/slskClient/client/serverListener"
	"spotseek/src/slskClient/messages"
	"spotseek/src/slskClient/messages/peerMessages"
	"spotseek/src/slskClient/messages/serverMessages"
	"spotseek/src/slskClient/peer"
	"strconv"
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
	Server                   *serverListener.ServerListener
	Listener                 net.Listener
	ConnectedPeers           map[string]peer.Peer // username --> peer info
	User                     string               // the user that is logged in
	UsernameIps              map[string]IP        // username -> IP address
	PendingPeerInits         map[string]peer.Peer // username -> Peer
	PendingUsernameIps       map[string]bool      // if we request a user's IP we add it here
	PendingUsernameConnTypes map[string]string
	PendingTokenConnTypes    map[uint32]PendingTokenConn // token --> connType
	TokenSearches            map[uint32]string
}

func NewSlskClient(host string, port int) *SlskClient {
	return &SlskClient{
		Host: host,
		Port: port,
	}
}

func (c *SlskClient) String() string {
	json, err := c.Json()
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s", string(json))
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
	c.Server = &serverListener.ServerListener{Conn: conn}
	c.Listener = listener
	go c.ListenForServerMessages()
	go c.ListenForPeers() // Listen for incoming connections from other clients
	c.Login("***REMOVED***", "***REMOVED***")
	c.SetWaitPort(2234)
	log.Println("Established connection to Soulseek server")
	log.Println("Listening on port 2234")
	c.User = "***REMOVED***"
	c.ConnectedPeers = make(map[string]peer.Peer)
	c.UsernameIps = make(map[string]IP)

	c.PendingUsernameIps = make(map[string]bool)
	c.PendingTokenConnTypes = make(map[uint32]PendingTokenConn)
	c.PendingUsernameConnTypes = make(map[string]string)

	c.TokenSearches = make(map[uint32]string)

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

// only listens for new peers. this does not listen for messages coming from connected peers.
func (c *SlskClient) ListenForPeers() {
	for {
		peerConn, err := c.Listener.Accept()
		if err != nil {
			log.Println("Error establishing connection with peer:", err)
			continue
		}

		// at this point we should be able to send and receive messages (we being our client and the peerConn client)
		log.Println("REMOTE ADDRESS OF PEER:", peerConn.RemoteAddr().String())
		buffer := make([]byte, 4096)
		n, readErr := peerConn.Read(buffer) // read the incoming message from peer
		if readErr != nil {
			log.Println("Cannot read into the buffer:", readErr)
			continue
		}

		size := binary.LittleEndian.Uint32(buffer[0:4])
		if size > 4096 { // buffer allocation is 4096 bytes so we cant read more than that
			log.Println("size of incoming peer message is g.t.e 4096 bytes")
			log.Println("msg: ", buffer)
			continue
		}
		peerMsgReader := peerMessages.PeerInitMessageReader{MessageReader: messages.NewMessageReader(buffer[:n])}
		code := peerMsgReader.ReadInt8()
		// this fires sometimes...? have received code 23 in the past, which is not a valid code. something is likely wrong in my code
		log.Println("RECEIVED CODE", code, "FROM PEER")
		// i dont think these are the right codes to check for here?
		// also this should be in a select statement
		// accept connection --> start a new goroutine with select statement of codes
		// send
		if code == 0 {
			token := peerMsgReader.ParsePierceFirewall()
			usernameAndConnType, ok := c.PendingTokenConnTypes[token]
			if !ok {
				log.Println("trying to connect to peer but cannot find a pending connection for token with value", token)
				continue
			}

			ip, port, err := net.SplitHostPort(peerConn.RemoteAddr().String())
			if err != nil {
				log.Println("error getting ip and port from", peerConn.RemoteAddr().String())
				continue
			}

			// establish a connection with peer...
			// first parse the ip and port from the incoming IP
			portUint64, _ := strconv.ParseUint(port, 10, 64)
			portUint32 := uint32(portUint64)
			username := usernameAndConnType.username

			// then attempt to establish a connection over TCP with the peer
			peer := peer.NewPeer(username, c.Listener, usernameAndConnType.connType, token, ip, portUint32)

			// add the new peer to our map of peers
			c.ConnectedPeers[username] = *peer

			// connection to peer is no longer pending
			delete(c.PendingTokenConnTypes, token)

			// Send it to the other user to confirm
			peer.PierceFirewall(token)
			c.ListenForPeerMessages(peer)

		} else if code == 1 {
			username, connType, token := peerMsgReader.ParsePeerInit()

			ip, port, err := net.SplitHostPort(peerConn.RemoteAddr().String())
			if err != nil {
				log.Println("error getting ip and port from", peerConn.RemoteAddr().String())
				continue
			}

			// establish a connection with peer
			// first parse the ip and port from the incoming IP
			portUint64, _ := strconv.ParseUint(port, 10, 64)
			portUint32 := uint32(portUint64)

			// then attempt to establish a connection over TCP with the peer
			peer := peer.NewPeer(username, c.Listener, connType, token, ip, portUint32)
			if peer == nil {
				log.Println("error establishing PeerInit connection to", username)
				continue
			}

			// add the new peer to our map of peers
			c.ConnectedPeers[username] = *peer

			// connection to peer is no longer pending
			delete(c.PendingTokenConnTypes, token)
			c.ListenForPeerMessages(peer)
		}
	}
}

func (c *SlskClient) ListenForPeerMessages(p *peer.Peer) {
	// This goroutine reads data from the peer
	for {
		readBuffer := make([]byte, 4096)
		var currentMessage []byte
		var messageLength uint32 = 0

		n, err := p.Conn.Read(readBuffer)
		if err != nil {
			if err == io.EOF {
				log.Println("peer closed the connection:", err)
				p.Conn.Close()
				delete(c.ConnectedPeers, p.Username)
				return
			}
			log.Println("error reading from peer connection:", err)
			continue
		}
		log.Println("reading peer message")

		currentMessage = append(currentMessage, readBuffer[:n]...)
		for {
			if messageLength == 0 {
				// Check if we have enough data to read the message length
				if len(currentMessage) < 4 {
					break // Not enough data, wait for more
				}

				// Read message length
				messageLength = binary.LittleEndian.Uint32(currentMessage[0:4])
				currentMessage = currentMessage[4:]
			}

			// Check if we have received the full message
			if uint32(len(currentMessage)) < messageLength {
				break // Not enough data, wait for more
			}

			// Process the full message
			mr := messages.NewMessageReader(currentMessage[:messageLength])
			peerMsgReader := peerMessages.PeerMessageReader{MessageReader: mr}
			msg, err := p.HandlePeerMessage(&peerMsgReader)
			if err != nil {
				log.Println("Error reading message from peer:", err)
			} else {
				log.Println("Successfully received message from peer:", msg)
			}

			// Remove the processed message from the buffer and reset the message length
			currentMessage = currentMessage[messageLength:]
			messageLength = 0
		}
	}
}

func (c *SlskClient) ListenForServerMessages() {
	go func() {
		readBuffer := make([]byte, 4096)
		var currentMessage []byte
		var messageLength uint32 = 0

		for {
			n, err := c.Server.Read(readBuffer)
			if err != nil {
				log.Println("Error reading from connection:", err)
				return
			}

			// Add new data to current message
			currentMessage = append(currentMessage, readBuffer[:n]...)

			for {
				if messageLength == 0 {
					// Check if we have enough data to read the message length
					if len(currentMessage) < 4 {
						break // Not enough data, wait for more
					}

					// Read message length
					messageLength = binary.LittleEndian.Uint32(currentMessage[0:4])
					currentMessage = currentMessage[4:]
				}

				// Check if we have received the full message
				if uint32(len(currentMessage)) < messageLength {
					break // Not enough data, wait for more
				}

				// Process the full message
				mr := messages.NewMessageReader(currentMessage[:messageLength])
				serverMsgReader := serverMessages.ServerMessageReader{MessageReader: mr}
				msg, err := c.HandleServerMessage(&serverMsgReader)
				if err != nil {
					log.Println("Error reading message from Soulseek:", err)
				} else {
					log.Println("Message from server:", msg)
				}

				// Remove the processed message from the buffer and reset the message length
				currentMessage = currentMessage[messageLength:]
				messageLength = 0
			}
		}
	}()
}
