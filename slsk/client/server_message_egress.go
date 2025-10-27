package client

import (
	"math/rand/v2"
	"spotseek/slsk/messages"
)

func (c *SlskClient) Send(msg []byte) {
	c.ServerConnection.SendMessage(msg)
}

func (c *SlskClient) Login(username string, password string) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.Login(username, password)
	c.logger.Info("Sending Login", "username", username)
	c.Send(msg)
}

// 0: Offline
// 1: Away
// 2: Online
func (c *SlskClient) SetStatus(status uint32) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.SetStatus(status)
	c.Send(msg)
}

func (c *SlskClient) SetWaitPort(port uint32) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.SetWaitPort(port)
	c.logger.Info("Sending SetWaitPort", "port", port)
	c.Send(msg)
}

// see HandleGetPeerAddress() for how we attempt direct connection requests
func (c *SlskClient) GetPeerAddress(username string) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.GetPeerAddress(username)
	c.logger.Info("Sending GetPeerAddress", "username", username)
	c.Send(msg)
}

// Server forwards our query to the distributed network
// Returns the search token/ID that can be used to retrieve results
func (c *SlskClient) FileSearch(query string) uint32 {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	t := uint32(rand.Int32()) // rand.Int32() returns non-negative, this is safe

	c.SearchManager.CreateSearch(t, query)

	c.logger.Info("Sending FileSearch", "token", t, "query", query)
	msg := mb.FileSearch(t, query)
	c.Send(msg)
	return t
}

// RequestPeerConnection sends a ConnectToPeer message to the server (does NOT open a TCP connection)
// The server will coordinate with the peer for connection establishment
func (c *SlskClient) RequestPeerConnection(username string, connType string, token uint32, isParent bool) {
	if connType == "P" {
		if peer := c.PeerManager.GetDefaultPeer(username); peer != nil {
			c.logger.Warn("Existing connection to this peer-- skipping connection request",
				"username", username, "type", connType)
			return
		}
	}
	if connType == "D" {
		if peer := c.PeerManager.GetDistributedPeer(username); peer != nil {
			c.logger.Warn("Existing connection to this peer-- skipping connection request",
				"username", username, "type", connType)
			return
		}
	}
	if connType == "F" {
		if peer := c.PeerManager.GetFileTransferPeer(username); peer != nil {
			c.logger.Warn("Existing connection to this peer-- skipping connection request",
				"username", username, "type", connType)
			return
		}
	}

	if connInfo, found := c.PeerManager.GetPendingConnection(username); found {
		if connInfo.ConnType == connType {
			c.logger.Warn("Peer already pending-- skipping connection request")
			return
		}

		// if connection type is different, remove it since we are about to add it again
		c.PeerManager.RemovePendingConnection(username)
	}

	c.logger.Info("Sending ConnectToPeer message to server",
		"token", token,
		"username", username,
		"connType", connType,
		"isParent", isParent,
	)

	c.PeerManager.AddPendingConnection(username, token, connType, 0, isParent)

	// Attempt to get IP of user so that we can send a direct connection request
	c.GetPeerAddress(username)

	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.ConnectToPeer(token, username, connType)
	c.Send(msg)

}

func (c *SlskClient) CantConnectToPeer(token uint32, username string) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.CantConnectToPeer(token, username)
	c.logger.Info("Sending CantConnectToPeer", "token", token, "username", username)
	c.Send(msg)
}

func (c *SlskClient) GetUserStatus(username string) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.GetUserStatus(username)
	c.logger.Info("Sending GetUserStatus", "username", username)
	c.Send(msg)
}

func (c *SlskClient) UserSearch(username string, query string) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	t := uint32(rand.Int32()) // rand.Int32() returns non-negative, this is safe
	msg := mb.UserSearch(username, t, query)
	c.Send(msg)
}

func (c *SlskClient) JoinRoom(room string) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.JoinRoom(room)
	c.Send(msg)
}

func (c *SlskClient) AckMessage(id uint32) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.MessageAcked(id)
	c.Send(msg)
}

func (c *SlskClient) SharedFoldersFiles(folderCount, fileCount uint32) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.SharedFoldersFiles(folderCount, fileCount)
	c.Send(msg)
}

// if haveNoParent == 1, the server eventually sends us PossibleParents msg
// if no possible parents are found, we eventually become a branch root
func (c *SlskClient) HaveNoParent(haveNoParent uint8) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.HaveNoParent(haveNoParent)
	c.logger.Info("Sending HaveNoParent", "hasNoParent", haveNoParent)
	c.Send(msg)
}

func (c *SlskClient) BranchLevel(branchLevel uint32) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.BranchLevel(branchLevel)
	c.logger.Info("Sending BranchLevel", "branchLevel", branchLevel)
	c.Send(msg)
}

func (c *SlskClient) BranchRoot(branchRoot string) {
	mb := messages.ServerMessageBuilder{MessageBuilder: messages.NewMessageBuilder()}
	msg := mb.BranchRoot(branchRoot)
	c.logger.Info("Sending BranchRoot", "branchRoot", branchRoot)
	c.Send(msg)
}
