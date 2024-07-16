package client

import (
	"log"
	"spotseek/src/slsk/messages"
	"spotseek/src/slsk/shared"
)

func (c *SlskClient) NextConnectionToken() uint32 {
	c.ConnectionToken = c.ConnectionToken + 1
	return c.ConnectionToken
}

func (c *SlskClient) NextSearchToken() uint32 {
	c.SearchToken = c.SearchToken + 1
	log.Println("NextSearchToken:", c.SearchToken)
	return c.SearchToken
}

func (c *SlskClient) Login(username string, password string) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.Login(username, password)
	c.ServerConnection.SendMessage(msg)
}

func (c *SlskClient) SetWaitPort(port uint32) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.SetWaitPort(port)
	c.ServerConnection.SendMessage(msg)
}

func (c *SlskClient) GetPeerAddress(username string) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.GetPeerAddress(username)
	c.ServerConnection.SendMessage(msg)
}

func (c *SlskClient) FileSearch(query string) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	t := c.NextSearchToken()
	log.Printf("Got token: %d", t)
	c.TokenSearches[t] = query
	c.SearchResults[t] = make([]shared.SearchResult, 0)
	msg := mb.FileSearch(t, query)
	c.ServerConnection.SendMessage(msg)
}

func (c *SlskClient) ConnectToPeer(username string, connType string) {
	// First, check if we're already connected to this peer
	//
	if peer := c.PeerManager.GetPeer(username); peer != nil {
		return
	}
	// Step 1: indirect connection request via sending ConnectToPeer message to Soulseek
	token := c.NextConnectionToken()
	c.PendingPeerConnections[token] = PendingTokenConn{username: username, connType: connType}

	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.ConnectToPeer(token, username, connType)
	c.ServerConnection.SendMessage(msg)

	// Step 1.5: Initialize direct connection request
	// First we need to get the peer's address
	// see fromServer.HandleGetPeerAddress for rest of implementation
	// we establish a connection to a peer when we receive info about their IP address
	c.PeerManager.AddPendingPeer(username, connType)
	c.GetPeerAddress(username)
}

func (c *SlskClient) CantConnectToPeer(token uint32, username string) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.CantConnectToPeer(token, username)
	c.ServerConnection.SendMessage(msg)
}

func (c *SlskClient) GetUserStatus(username string) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.GetUserStatus(username)
	c.ServerConnection.SendMessage(msg)
}

func (c *SlskClient) UserSearch(username string, query string) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.UserSearch(username, c.NextSearchToken(), query)
	c.ServerConnection.SendMessage(msg)
}

func (c *SlskClient) JoinRoom(room string) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.JoinRoom(room)
	c.ServerConnection.SendMessage(msg)
}
