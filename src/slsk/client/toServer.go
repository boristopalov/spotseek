package client

import (
	"log"
	"spotseek/src/slsk/messages"
	"spotseek/src/slsk/messages/serverMessages"
	"spotseek/src/slsk/peer"
	"time"
)

func (c *SlskClient) NextConnectionToken() uint32 {
	t := c.ConnectionToken
	c.ConnectionToken++
	return t
}

func (c *SlskClient) NextSearchToken() uint32 {
	t := c.SearchToken
	c.SearchToken++
	return t
}

func (c *SlskClient) Login(username string, password string) {
	mb := serverMessages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.Login(username, password)
	c.Server.SendMessage(msg)
}

func (c *SlskClient) SetWaitPort(port uint32) {
	mb := serverMessages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.SetWaitPort(port)
	c.Server.SendMessage(msg)
}

func (c *SlskClient) GetPeerAddress(username string) {
	mb := serverMessages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	c.PendingUsernameIps[username] = true
	msg := mb.GetPeerAddress(username)
	c.Server.SendMessage(msg)
}

func (c *SlskClient) FileSearch(query string) {
	mb := serverMessages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	t := c.NextSearchToken()
	c.TokenSearches[t] = query
	msg := mb.FileSearch(t, query)
	c.Server.SendMessage(msg)
}

func (c *SlskClient) ConnectToPeer(username string, connType string) {
	// First, check if we're already connected to this peer
	if _, connected := c.ConnectedPeers[username]; connected {
		log.Printf("Already connected to %s", username)
		return
	}
	c.GetPeerAddress(username)

	// Wait for a lil instead of listening to address event lol
	time.Sleep(3 * time.Second)
	ip, ok := c.UsernameIps[username]
	if ok {
		token := c.NextConnectionToken()
		peer := peer.NewPeer(username, c.Listener, connType, token, ip.IP, ip.port)
		if peer != nil {
			err := peer.PierceFirewall(token)
			if err == nil {
				err = peer.PeerInit(c.User, connType, token)
				if err == nil {
					c.ConnectedPeers[username] = *peer
					go c.ListenForPeerMessages(peer)
					log.Printf("Direct connection established with %s", username)
					return
				}
			}
			log.Printf("Direct connection failed with %s: %v", username, err)
		}
	}

	token := c.NextConnectionToken()
	c.requestPeerConnection(username, connType, token)
}

func (c *SlskClient) requestPeerConnection(username, connType string, token uint32) {
	mb := serverMessages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.ConnectToPeer(token, username, connType)
	c.PendingTokenConnTypes[token] = PendingTokenConn{username: username, connType: connType}
	log.Println("attempting indirect connection to", username, "with connection type", connType)
	c.Server.SendMessage(msg)
}

func (c *SlskClient) CantConnectToPeer(token uint32, username string) {
	mb := serverMessages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.CantConnectToPeer(token, username)
	c.Server.SendMessage(msg)
}

func (c *SlskClient) GetUserStatus(username string) {
	mb := serverMessages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.GetUserStatus(username)
	c.Server.SendMessage(msg)
}

func (c *SlskClient) UserSearch(username string, query string) {
	mb := serverMessages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.UserSearch(username, c.NextSearchToken(), query)
	c.Server.SendMessage(msg)
}
