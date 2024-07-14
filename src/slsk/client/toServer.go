package client

import (
	"log"
	"spotseek/src/slsk/messages"
	"spotseek/src/slsk/messages/serverMessages"
	"spotseek/src/slsk/shared"
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
	msg := mb.GetPeerAddress(username)
	c.Server.SendMessage(msg)
}

func (c *SlskClient) FileSearch(query string) {
	mb := serverMessages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	t := c.NextSearchToken()
	c.TokenSearches[t] = query
	c.SearchResults[t] = make([]shared.SearchResult, 0)
	msg := mb.FileSearch(t, query)
	c.Server.SendMessage(msg)
}

func (c *SlskClient) ConnectToPeer(username string, connType string) {
	// First, check if we're already connected to this peer
	//
	if _, connected := c.ConnectedPeers[username]; connected {
		log.Printf("Already connected to %s", username)
		return
	}
	// Step 1: indirect connection request via sending ConnectToPeer message to Soulseek
	c.RequestPeerConnection(username, connType, c.NextConnectionToken())

	// Step 1.5: Initialize direct connection request
	// First we need to get the peer's address
	// see fromServer.HandleGetPeerAddress for rest of implementation
	// we establish a connection to a peer when we receive info about their IP address
	c.PendingUsernameConnTypes[username] = connType
	c.GetPeerAddress(username)
}

func (c *SlskClient) RequestPeerConnection(username, connType string, token uint32) {
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
