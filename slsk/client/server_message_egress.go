package client

import (
	"spotseek/slsk/messages"
	"spotseek/slsk/shared"
)

func (c *SlskClient) NextConnectionToken() uint32 {
	c.ConnectionToken = c.ConnectionToken + 1
	return c.ConnectionToken
}

func (c *SlskClient) NextSearchToken() uint32 {
	c.SearchToken = c.SearchToken + 1
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

// see HandleGetPeerAddress() for how we attempt direct connection requests
func (c *SlskClient) GetPeerAddress(username string) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.GetPeerAddress(username)
	c.ServerConnection.SendMessage(msg)
}

// Server forwards our query to the distributed network
func (c *SlskClient) FileSearch(query string) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	t := c.NextSearchToken()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.SearchResults[t] = make([]shared.SearchResult, 0)
	msg := mb.FileSearch(t, query)
	c.ServerConnection.SendMessage(msg)
}

func (c *SlskClient) ConnectToPeer(username string, connType string) {
	if peer := c.PeerManager.GetPeer(username, connType); peer != nil {
		return
	}
	token := c.NextConnectionToken()

	log.Info("Requesting indirect connection to user",
		"token", token,
		"username", username,
		"connType", connType,
	)

	c.AddPendingPeer(token, username, connType, 0)

	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.ConnectToPeer(token, username, connType)
	c.ServerConnection.SendMessage(msg)

	// Attempt to get IP of user so that we can send a direct connection request
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

func (c *SlskClient) AckMessage(id uint32) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.MessageAcked(id)
	c.ServerConnection.SendMessage(msg)
}

func (c *SlskClient) SharedFoldersFiles(folderCount, fileCount uint32) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.SharedFoldersFiles(folderCount, fileCount)
	c.ServerConnection.SendMessage(msg)
}

// if haveNoParent == 1, the server eventually sends us PossibleParents msg
// if no possible parents are found, we eventually become a branch root
func (c *SlskClient) HaveNoParent(haveNoParent uint8) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.HaveNoParent(haveNoParent)
	c.ServerConnection.SendMessage(msg)
}

func (c *SlskClient) BranchLevel(branchLevel uint32) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.BranchLevel(branchLevel)
	c.ServerConnection.SendMessage(msg)
}

func (c *SlskClient) BranchRoot(branchRoot string) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.BranchRoot(branchRoot)
	c.ServerConnection.SendMessage(msg)
}
