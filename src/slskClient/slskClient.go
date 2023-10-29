package slskClient

import (
	"spotseek/src/messages"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

type Server struct {
	net.Conn
}

type IP struct { 
	IP string 
	port uint32
}

type PendingTokenConn struct { 
	username string 
	connType string
}

type SlskClient struct { 
	Host string
	Port int
	Server *Server
	Listener net.Listener
	Peers map[string]Peer // username --> peer info
	User string // the user that is logged in
	UsernameIps map[string]IP // username -> IP address

	PendingUsernameIps map[string]bool // if we request a user's IP we add it here
	PendingUsernameConnTypes map[string]string 
	PendingTokenConnTypes map[uint32]PendingTokenConn // token --> connType

	TokenSearches map[uint32]string
}

func NewSlskClient(host string, port int) *SlskClient {
	return &SlskClient{
		Host: host,
		Port: port,
	}
}

func (c *SlskClient) Connect() error {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port))
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", ":2234")
	if err != nil { 
		return err
	}
	c.Server = &Server{Conn: conn}
	c.Listener = listener
	c.ListenForServerMessages() 
	c.ListenForPeers() // Listen for incoming connections from other clients
	fmt.Println("Established connection to Soulseek server")
	c.Login("***REMOVED***", "***REMOVED***")
	c.SetWaitPort(2234)
	c.User = "***REMOVED***"
	c.Peers = make(map[string]Peer)
	c.UsernameIps = make(map[string]IP)

	c.PendingUsernameIps = make(map[string]bool)
	c.PendingTokenConnTypes = make(map[uint32]PendingTokenConn)
	c.PendingUsernameConnTypes = make(map[string]string)

	c.TokenSearches = make(map[uint32]string)

	time.AfterFunc(5*time.Second, func () { c.ConnectToPeer("forthelulz", "P") })
	time.AfterFunc(10*time.Second, func () { c.UserSearch("amsterdamn", "hamada")})

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
	fmt.Println("Connection closed")
	return nil
}

// only listens for new peers. this does not listen for messages coming from connected peers.
func (c *SlskClient) ListenForPeers()  {
	 go func() {
		for {
			peerConn, err := c.Listener.Accept()
			if err != nil {
				continue 
			}
			buffer := make([]byte, 4096)
			n, readErr := peerConn.Read(buffer)
			if readErr != nil {
				fmt.Println("Error reading from connection:", readErr)
				continue
			}

	    size := binary.LittleEndian.Uint32(buffer[0:4])
			if size > 4096 { 
				fmt.Println("size of incoming peer message is g.t.e 4096 bytes")
				continue
			}
			mr := messages.NewMessageReader(buffer[:n])
			code := mr.ReadInt8()
			fmt.Println("LISTENING FOR PEER. RECEIVED CODE", code)
			if code == 0 { 
				token := mr.ParsePierceFirewall()
				usernameAndConnType, ok := c.PendingTokenConnTypes[token]
				if !ok { 
					fmt.Println("trying to connect to peer but cannot find a pending connection for token with value", token)
					continue
				}

				ip, port, err := net.SplitHostPort(peerConn.RemoteAddr().String())
				if err != nil { 
					fmt.Println("error getting ip and port from", peerConn.RemoteAddr().String())
					continue
				}

				// establish a connection with peer
				// first parse the ip and port from the incoming IP
				portUint64, _ := strconv.ParseUint(port, 10, 64)
				portUint32 := uint32(portUint64)
				username := usernameAndConnType.username

				// then attempt to establish a connection over TCP with the peer
				peer := NewPeer(username, c.Listener, usernameAndConnType.connType, token, ip, portUint32)

				// add the new peer to our map of peers
				c.Peers[username] = *peer

				// connection to peer is no longer pending
				delete(c.PendingTokenConnTypes, token)

				// Send it to the other user to confirm
				peer.PierceFirewall(token) 
				c.ListenForPeerMessages(peer)

		} else if code == 1 { 
				username, connType, token := mr.ParsePeerInit()

				ip, port, err := net.SplitHostPort(peerConn.RemoteAddr().String())
				if err != nil { 
					fmt.Println("error getting ip and port from", peerConn.RemoteAddr().String())
					continue
				}

				// establish a connection with peer
				// first parse the ip and port from the incoming IP
				portUint64, _ := strconv.ParseUint(port, 10, 64)
				portUint32 := uint32(portUint64)

				// then attempt to establish a connection over TCP with the peer
				peer := NewPeer(username, c.Listener, connType, token, ip, portUint32)
				if peer == nil {
					fmt.Println("error establishing PeerInit connection to", username)
					continue
				}

				// add the new peer to our map of peers
				c.Peers[username] = *peer

				// connection to peer is no longer pending
				delete(c.PendingTokenConnTypes, token) 
				c.ListenForPeerMessages(peer)
			}
		}
	}()
}


func (c *SlskClient) ListenForPeerMessages(p *Peer) {
  // This goroutine reads data from the peer
  go func() {
    for {
      readBuffer := make([]byte, 4096)
		  var currentMessage []byte
		  var messageLength uint32 = 0

      n, err := p.Conn.Read(readBuffer)
      if err != nil {
				if err == io.EOF {
					fmt.Println("peer closed the connection:", err)
					p.Conn.Close()
					delete(c.Peers, p.Username)
					return
				}
        fmt.Println("error reading from peer connection:", err)
        continue
      }
			fmt.Println("reading peer message")
			
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
				msg, err := p.HandlePeerMessage(mr)
				if err != nil {
					fmt.Println("Error reading message from Soulseek:", err)
				} else {
					fmt.Println("Message from server:", msg)
				}

				// Remove the processed message from the buffer and reset the message length
				currentMessage = currentMessage[messageLength:]
				messageLength = 0
			}
    }
  }()
}

func (c *SlskClient) ListenForServerMessages() {
	go func() {
		readBuffer := make([]byte, 4096)
		var currentMessage []byte
		var messageLength uint32 = 0

		for {
			n, err := c.Server.Read(readBuffer)
			if err != nil {
				fmt.Println("Error reading from connection:", err)
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
				msg, err := c.HandleServerMessage(mr)
				if err != nil {
					fmt.Println("Error reading message from Soulseek:", err)
				} else {
					fmt.Println("Message from server:", msg)
				}

				// Remove the processed message from the buffer and reset the message length
				currentMessage = currentMessage[messageLength:]
				messageLength = 0
			}
		}
	}()
}
