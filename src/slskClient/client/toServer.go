package client

import (
	"errors"
	"fmt"
	"log"
	"spotseek/src/messages"
)

var CONN_TOKEN uint32 = 2
var SEARCH_TOKEN uint32 = 2 

func NewServer() *Server { 
	return &Server{}
}

func (server *Server) SendMessage(message []byte) error {
	if server == nil {
		return errors.New("connection is not established")
	}
	_, err := server.Write(message)
	if err != nil {
		fmt.Println(err)
		return err
	}
    log.Println("sent message to Soulseek server....", string(message))
	return nil
}

func (c *SlskClient) Login(username string, password string) { 
	msg := messages.NewMessageBuilder().Login(username, password)
	c.Server.SendMessage(msg)
}

func (c *SlskClient) SetWaitPort(port uint32) { 
	msg := messages.NewMessageBuilder().SetWaitPort(port)
	c.Server.SendMessage(msg)
}

func (c *SlskClient) GetPeerAddress(username string) { 
	c.PendingUsernameIps[username] = true
	msg := messages.NewMessageBuilder().GetPeerAddress(username)
	c.Server.SendMessage(msg)
}

func (c *SlskClient) FileSearch(query string) { 
	c.TokenSearches[SEARCH_TOKEN] = query
	msg := messages.NewMessageBuilder().FileSearch(SEARCH_TOKEN, query)
	SEARCH_TOKEN += 1
	c.Server.SendMessage(msg)
}

func (c *SlskClient) ConnectToPeer(username string, connType string) { 
	msg := messages.NewMessageBuilder().ConnectToPeer(CONN_TOKEN, username, connType)
	c.PendingTokenConnTypes[CONN_TOKEN] = PendingTokenConn{username: username, connType: connType}
	c.PendingUsernameConnTypes[username] = connType
	CONN_TOKEN += 1
	log.Println("attempting indirect connection to", username, "with connection type", connType)
	c.Server.SendMessage(msg)
	c.GetPeerAddress(username)
}

func (c *SlskClient) CantConnectToPeer(token uint32, username string) { 
	msg := messages.NewMessageBuilder().CantConnectToPeer(token, username)
	c.Server.SendMessage(msg)
}

func (c *SlskClient) GetUserStatus(username string) {
	msg := messages.NewMessageBuilder().GetUserStatus(username)
	c.Server.SendMessage(msg)
}

func (c *SlskClient) UserSearch(username string, query string) { 
	msg := messages.NewMessageBuilder().UserSearch(username, SEARCH_TOKEN, query)
	c.Server.SendMessage(msg)
	SEARCH_TOKEN += 1
}
