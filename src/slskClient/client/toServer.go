package client

import (
	"log"
	"spotseek/src/slskClient/messages"
	"spotseek/src/slskClient/messages/serverMessages"
)

var CONN_TOKEN uint32 = 2
var SEARCH_TOKEN uint32 = 2

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
	c.TokenSearches[SEARCH_TOKEN] = query
	msg := mb.FileSearch(SEARCH_TOKEN, query)
	SEARCH_TOKEN += 1
	c.Server.SendMessage(msg)
}

func (c *SlskClient) ConnectToPeer(username string, connType string) {
	mb := serverMessages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.ConnectToPeer(CONN_TOKEN, username, connType)
	c.PendingTokenConnTypes[CONN_TOKEN] = PendingTokenConn{username: username, connType: connType}
	c.PendingUsernameConnTypes[username] = connType
	CONN_TOKEN += 1
	log.Println("attempting indirect connection to", username, "with connection type", connType)
	c.Server.SendMessage(msg)
	c.GetPeerAddress(username)
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
	msg := mb.UserSearch(username, SEARCH_TOKEN, query)
	c.Server.SendMessage(msg)
	SEARCH_TOKEN += 1
}
