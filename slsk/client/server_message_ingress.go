package client

import (
	"encoding/binary"
	"errors"
	"fmt"
	"runtime/debug"
	"spotseek/slsk/messages"
	"time"
)

// SERVER MESSAGE HANDLING
func (c *SlskClient) ListenForServerMessages() {
	readBuffer := make([]byte, 4096)
	var currentMessage []byte
	var messageLength uint32

	for {
		n, err := c.ServerConnection.Read(readBuffer)
		if err != nil {
			c.logger.Error("failed to read server message", "err", err)
			return
		}

		currentMessage = append(currentMessage, readBuffer[:n]...)
		currentMessage, messageLength = c.processServerMessages(currentMessage, messageLength)
	}
}

func (c *SlskClient) processServerMessages(data []byte, messageLength uint32) ([]byte, uint32) {
	if len(data) == 0 {
		return data, messageLength
	}
	for {
		if messageLength == 0 {
			if len(data) < 4 {
				return data, messageLength // Not enough data to read message length
			}
			messageLength = binary.LittleEndian.Uint32(data[:4])
			data = data[4:]
		}

		if uint32(len(data)) < messageLength {
			return data, messageLength // Not enough data for full message
		}

		c.handleServerMessage(data[:messageLength])

		data = data[messageLength:]
		messageLength = 0
	}
}

func (c *SlskClient) handleServerMessage(messageData []byte) {
	mr := messages.NewMessageReader(messageData)

	defer func() {
		if r := recover(); r != nil {
			c.logger.Error("recovered from panic",
				"error", r,
			)
			// Optionally log the stack trace
			debug.PrintStack()
		}
	}()

	// var decoded map[string]interface{}
	// var err error
	code := mr.ReadInt32()
	switch code {
	case 1:
		c.HandleLogin(mr)
	case 3:
		c.HandleGetPeerAddress(mr)
	case 5:
		c.HandleAddUser(mr)
	case 7:
		c.HandleGetUserStatus(mr)
	case 13:
		c.HandleSayChatroom(mr)
	case 14:
		c.HandleJoinRoom(mr)
	case 15:
		c.HandleLeaveRoom(mr)
	case 16:
		c.HandleUserJoinedRoom(mr)
	case 17:
		c.HandleUserLeftRoom(mr)
	case 18:
		c.HandleConnectToPeer(mr)
	case 22:
		c.HandleMessageUser(mr)
	case 26:
		c.HandleFileSearch(mr)
	case 36:
		c.HandleGetUserStats(mr)
	case 41:
		c.HandleRelog(mr)
	case 64:
		c.HandleRoomList(mr)
	case 69:
		c.HandlePrivilegedUsers(mr)
	case 83:
		c.HandleParentMinSpeed(mr)
	case 84:
		c.HandleParentSpeedRatio(mr)
	case 92:
		c.HandleCheckPrivileges(mr)
	case 93:
		c.HandleSearchRequest(mr)
	case 102:
		c.HandlePossibleParents(mr)
	case 104:
		c.HandleWishlistInterval(mr)
	case 113:
		c.HandleRoomTickerState(mr)
	case 114:
		c.HandleRoomTickerAdd(mr)
	case 115:
		c.HandleRoomTickerRemove(mr)
	case 133:
		c.HandlePrivateRoomUsers(mr)
	case 134:
		c.HandlePrivateRoomAddUser(mr)
	case 135:
		c.HandlePrivateRoomRemoveUser(mr)
	case 139:
		c.HandlePrivateRoomAdded(mr)
	case 140:
		c.HandlePrivateRoomRemoved(mr)
	case 141:
		c.HandlePrivateRoomToggle(mr)
	case 142:
		c.HandleChangePassword(mr)
	case 143:
		c.HandlePrivateRoomAddOperator(mr)
	case 144:
		c.HandlePrivateRoomRemoveOperator(mr)
	case 145:
		c.HandlePrivateRoomOperatorAdded(mr)
	case 146:
		c.HandlePrivateRoomOperatorRemoved(mr)
	case 148:
		c.HandlePrivateRoomOwned(mr)
	case 160:
		c.HandleExcludedSearchPhrases(mr)
	case 1001:
		c.HandleCantConnectToPeer(mr)
	default:
		c.logger.Error("invalid code", "code", code)
	}
	// log.Info("received message from server", "code", code, "message", decoded)
}

func (c *SlskClient) HandleLogin(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	success := mr.ReadBool()
	c.logger.Info("Login success status", "success", success)
	if !success {
		reason := mr.ReadString()
		return nil, errors.New(reason)
	}
	greetingMessage := mr.ReadString()
	ip := mr.ReadInt32()
	c.logger.Info("Greeting message", "message", greetingMessage)
	c.logger.Info("IP from server", "message", ip)
	return decoded, nil
}

// Typically, to connect to a peer, we send a ConnectToPeer and GetPeerAddress
// When we get ip and port info of peer here, we attempt a direct connection request
func (c *SlskClient) HandleGetPeerAddress(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	username := mr.ReadString()
	host := mr.ReadIp()
	port := mr.ReadInt32()
	decoded["username"] = username
	decoded["ip"] = host
	decoded["port"] = port
	if host == "0.0.0.0" {
		return decoded, fmt.Errorf("User is offline")
	}
	peerInfo, found := c.PendingOutgoingPeerConnections[username]
	if !found {
		return decoded, fmt.Errorf("Np pending outgoing peer connection for user %s", username)
	}
	go func() {
		c.PeerManager.ConnectToPeer(host, port, username, peerInfo.connType, peerInfo.token, peerInfo.privileged)
		c.RemovePendingPeer(username)
		delete(c.PendingOutgoingPeerConnectionTokens, peerInfo.token)
	}()
	return decoded, nil
}

func (c *SlskClient) HandleAddUser(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	username := mr.ReadString()
	exists := mr.ReadInt8()
	decoded["type"] = "AddUser"
	decoded["username"] = username
	decoded["exists"] = exists
	if exists == 0 {
		return decoded, nil
	}
	decoded["status"] = mr.ReadInt32()
	decoded["speed"] = mr.ReadInt32()
	decoded["downloadNum"] = mr.ReadInt32()
	decoded["files"] = mr.ReadInt32()
	decoded["folders"] = mr.ReadInt32()
	return decoded, nil
}

func (c *SlskClient) HandleGetUserStatus(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "getUserStatus"
	decoded["username"] = mr.ReadString()
	decoded["status"] = mr.ReadInt32()
	decoded["privileged"] = mr.ReadInt8()
	return decoded, nil
}

func (c *SlskClient) HandleSayChatroom(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "sayChatroom"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	decoded["message"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandleJoinRoom(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "joinRoom"
	name := mr.ReadString()
	decoded["room"] = name
	numUsers := mr.ReadInt32()
	room := &Room{
		users:    make([]*User, numUsers),
		messages: make([]string, 0),
	}

	for i := uint32(0); i < numUsers; i++ {
		user := NewUser()
		username := mr.ReadString()
		user.username = username
		room.users[i] = user
	}

	_ = mr.ReadInt32() // number of statuses

	for i := uint32(0); i < numUsers; i++ {
		status := mr.ReadInt32()
		room.users[i].status = status
	}

	_ = mr.ReadInt32() // number of user stats

	for i := uint32(0); i < numUsers; i++ {
		room.users[i].avgSpeed = mr.ReadInt32()
		room.users[i].uploadNum = mr.ReadInt32()
		mr.ReadInt32() // unknown
		room.users[i].files = mr.ReadInt32()
		room.users[i].dirs = mr.ReadInt32()
	}

	_ = mr.ReadInt32() // number of slotsfree

	for i := uint32(0); i < numUsers; i++ {
		room.users[i].slotsFree = mr.ReadInt32()
	}

	_ = mr.ReadInt32() // number of user countries

	for i := uint32(0); i < numUsers; i++ {
		room.users[i].country = mr.ReadString()
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.JoinedRooms[name] = room
	return decoded, nil
}

func (c *SlskClient) HandleRoom(mr *messages.MessageReader) ([]map[string]interface{}, error) {
	var rooms []map[string]interface{}
	roomCount := mr.ReadInt32()
	for i := uint32(0); i < roomCount; i++ {
		room := make(map[string]interface{})
		roomName := mr.ReadString()
		room["name"] = roomName
		rooms = append(rooms, room)
	}
	mr.ReadInt32()
	for i := uint32(0); i < roomCount; i++ {
		rooms[i]["users"] = mr.ReadInt32()
	}
	return rooms, nil
}

func (c *SlskClient) HandleRoomList(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "RoomList"
	publicRooms, _ := c.HandleRoom(mr)
	ownedPrivate, _ := c.HandleRoom(mr)
	private, _ := c.HandleRoom(mr)
	decoded["public"] = publicRooms
	decoded["ownedPrivate"] = ownedPrivate
	decoded["private"] = private
	return decoded, nil
}

func (c *SlskClient) HandleLeaveRoom(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "leaveRoom"
	decoded["room"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandleUserJoinedRoom(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "userJoinedRoom"

	roomName := mr.ReadString()
	username := mr.ReadString()
	status := mr.ReadInt32()
	avgSpeed := mr.ReadInt32()
	uploadNum := mr.ReadInt32()
	unknown := mr.ReadInt32()
	files := mr.ReadInt32()
	folders := mr.ReadInt32()
	slotsFree := mr.ReadInt32()
	country := mr.ReadString()

	decoded["room"] = roomName
	decoded["username"] = username
	decoded["status"] = status
	decoded["avgSpeed"] = avgSpeed
	decoded["uploadNum"] = uploadNum
	decoded["unknown"] = unknown
	decoded["files"] = files
	decoded["folders"] = folders
	decoded["slotsFree"] = slotsFree
	decoded["country"] = country

	room, ok := c.JoinedRooms[roomName]
	if !ok {
		return decoded, fmt.Errorf("Room not in JoinedRooms map")
	}

	newUser := &User{
		username:   username,
		status:     uint32(status),
		privileged: false,
		avgSpeed:   uint32(avgSpeed),
		uploadNum:  uint32(uploadNum),
		files:      uint32(files),
		dirs:       uint32(folders),
		slotsFree:  uint32(slotsFree),
		country:    country,
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	room.users = append(room.users, newUser)
	return decoded, nil
}

func (c *SlskClient) HandleUserLeftRoom(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	roomName := mr.ReadString()
	username := mr.ReadString()
	decoded["type"] = "userLeftRoom"
	decoded["room"] = roomName
	decoded["username"] = username
	_, ok := c.JoinedRooms[roomName]
	if !ok {
		return decoded, fmt.Errorf("Room not in JoinedRooms map")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	users := c.JoinedRooms[roomName].users

	// delete the user
	j := 0
	for i, v := range users {
		if v.username != username {
			users[j] = users[i]
			j++
		}
	}
	users = users[:j]
	c.JoinedRooms[roomName].users = users
	return decoded, nil
}

func (c *SlskClient) HandleConnectToPeer(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "connectToPeer"
	username := mr.ReadString()
	connType := mr.ReadString()
	ip := mr.ReadIp()
	port := mr.ReadInt32()
	token := mr.ReadInt32()
	privileged := mr.ReadInt8()
	decoded["username"] = username
	decoded["connType"] = connType
	decoded["ip"] = ip
	decoded["port"] = port
	decoded["token"] = token
	decoded["privileged"] = privileged

	go func() {
		err := c.PeerManager.ConnectToPeer(ip, port, username, connType, token, privileged)
		if err != nil {
			c.CantConnectToPeer(token, username)
		}
	}()
	return decoded, nil
}

func (c *SlskClient) HandleMessageUser(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	id := mr.ReadInt32()
	decoded["type"] = "messageUser"
	decoded["id"] = id
	decoded["timestamp"] = time.Unix(int64(mr.ReadInt32()), 0)
	decoded["username"] = mr.ReadString()
	decoded["message"] = mr.ReadString()

	c.AckMessage(id)
	return decoded, nil
}

// this is sent to us if a user is specifically searching our files
func (c *SlskClient) HandleFileSearch(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "fileSearch"
	decoded["username"] = mr.ReadString()
	decoded["token"] = mr.ReadInt32()
	decoded["query"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePing(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "ping"
	return decoded, nil
}

func (c *SlskClient) HandleGetUserStats(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "getUserStats"
	decoded["username"] = mr.ReadString()
	decoded["speed"] = mr.ReadInt32()
	decoded["downloadNum"] = mr.ReadInt64()
	decoded["files"] = mr.ReadInt32()
	decoded["directories"] = mr.ReadInt32()
	return decoded, nil
}

func (c *SlskClient) HandleRelog(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "relog"
	return decoded, nil
}

func (c *SlskClient) HandlePrivilegedUsers(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	var users []string
	decoded["type"] = "privilegedUsers"

	userCount := mr.ReadInt32()
	for i := uint32(0); i < userCount; i++ {
		user := mr.ReadString()
		users = append(users, user)
		// c.ConnectToPeer(user, "P")
	}
	decoded["users"] = users
	return decoded, nil
}

func (c *SlskClient) HandleParentMinSpeed(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "parentMinSpeed"
	decoded["minSpeed"] = mr.ReadInt32()
	return decoded, nil
}

func (c *SlskClient) HandleParentSpeedRatio(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "parentSpeedRatio"
	decoded["ratio"] = mr.ReadInt32()
	return decoded, nil
}

func (c *SlskClient) HandleCheckPrivileges(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "checkPrivileges"
	decoded["timeLeft"] = mr.ReadInt32()
	return decoded, nil
}

func (c *SlskClient) HandleSearchRequest(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "searchRequest"
	decoded["distributedCode"] = mr.ReadInt8()
	decoded["unknown"] = mr.ReadInt32()
	decoded["username"] = mr.ReadString()
	decoded["token"] = mr.ReadInt32()
	decoded["query"] = mr.ReadString()
	return decoded, nil
}

// parents are all users with higher upload speed than us
// max 10 parents at a time (why is it 4byte int?)
func (c *SlskClient) HandlePossibleParents(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	var parents []map[string]interface{}
	decoded["type"] = "possibleParents"

	parentCount := mr.ReadInt32()
	for i := uint32(0); i < parentCount; i++ {
		parent := make(map[string]interface{})
		username := mr.ReadString()
		host := mr.ReadIp()
		port := mr.ReadInt32()
		parent["username"] = username
		parent["ip"] = host
		parent["port"] = port
		parents = append(parents, parent)
	}
	decoded["parents"] = parents
	go func() {
		// Loop through parents and attempt connection
		for _, parent := range parents {
			username := parent["username"].(string)
			host := parent["ip"].(string)
			port := parent["port"].(uint32)
			err := c.PeerManager.ConnectToPeer(host, port, username, "D", 0, 0)
			if err == nil {
				// tell the server that we found a parent
				c.HaveNoParent(0)
				c.BranchRoot(username)
				return
			}
		}
	}()
	return decoded, nil
}

func (c *SlskClient) HandleWishlistInterval(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "wishlistInterval"
	decoded["interval"] = mr.ReadInt32()
	return decoded, nil
}

func (c *SlskClient) HandleRoomTickerState(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "roomTickerState"
	decoded["room"] = mr.ReadString()

	userCount := mr.ReadInt32()
	users := make(map[string]interface{})
	for i := uint32(0); i < userCount; i++ {
		username := mr.ReadString()
		ticker := mr.ReadString()
		users[username] = ticker
	}
	decoded["users"] = users

	return decoded, nil
}

func (c *SlskClient) HandleRoomTickerAdd(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "roomTickerAdd"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	decoded["ticker"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandleRoomTickerRemove(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "roomTickerRemove"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomUsers(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomUsers"
	var users []string

	userCount := mr.ReadInt32()
	for i := uint32(0); i < userCount; i++ {
		user := mr.ReadString()
		users = append(users, user)
	}
	decoded["users"] = users
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomAddUser(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomAddUser"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomRemoveUser(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomRemoveUser"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomAdded(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomAdded"
	decoded["room"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomRemoved(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomRemoved"
	decoded["room"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomToggle(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomToggle"
	decoded["enable"] = mr.ReadInt8()
	return decoded, nil
}

func (c *SlskClient) HandleChangePassword(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "changePassword"
	decoded["password"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomAddOperator(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomAddOperator"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomRemoveOperator(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomRemoveOperator"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomOperatorAdded(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomOperatorAdded"
	decoded["room"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomOperatorRemoved(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomOperatorRemoved"
	decoded["room"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomOwned(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomOwned"
	decoded["room"] = mr.ReadString()
	var operators []string

	opCount := mr.ReadInt32()
	for i := uint32(0); i < opCount; i++ {
		op := mr.ReadString()
		operators = append(operators, op)
	}
	decoded["operators"] = operators
	return decoded, nil
}

func (c *SlskClient) HandleCantConnectToPeer(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "cantConnectToPeer"
	token := mr.ReadInt32()
	username := mr.ReadString()
	decoded["token"] = token
	decoded["username"] = username
	c.RemovePendingPeer(username)
	return decoded, nil
}

func (c *SlskClient) HandleExcludedSearchPhrases(mr *messages.MessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "excludedSearchPhrases"
	numPhrases := mr.ReadInt32()
	var excludedSearchPhrases []string
	decoded["numPhrases"] = numPhrases
	for i := uint32(0); i < numPhrases; i++ {
		phrase := mr.ReadString()
		excludedSearchPhrases = append(excludedSearchPhrases, phrase)
	}
	decoded["excludedSearchPhrases"] = excludedSearchPhrases
	return decoded, nil
}
