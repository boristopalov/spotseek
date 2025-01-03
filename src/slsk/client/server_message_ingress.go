package client

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"spotseek/src/slsk/messages"
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
			log.Printf("Error reading from server connection: %v", err)
			return
		}

		currentMessage = append(currentMessage, readBuffer[:n]...)
		currentMessage = c.processServerMessages(currentMessage, &messageLength)
	}
}

func (c *SlskClient) processServerMessages(data []byte, messageLength *uint32) []byte {
	for {
		if *messageLength == 0 {
			if len(data) < 4 {
				return data // Not enough data to read message length
			}
			*messageLength = binary.LittleEndian.Uint32(data[:4])
			data = data[4:]
		}

		if uint32(len(data)) < *messageLength {
			return data // Not enough data for full message
		}

		c.handleServerMessage(data[:*messageLength])

		data = data[*messageLength:]
		*messageLength = 0
	}
}

func (c *SlskClient) handleServerMessage(messageData []byte) {
	mr := messages.NewMessageReader(messageData)
	serverMsgReader := messages.ServerMessageReader{MessageReader: mr}

	msg, err := c.HandleServerMessage(&serverMsgReader)
	if err != nil {
		log.Printf("Error decoding server message: %v", err)
	} else {
		log.Printf("Server message: message: %v", msg)
	}
	log.Println("--------------- End of message ----------------")
}

// the SlskClient type is listening for messages
// probably simplest to just move this into slskClient package
// peer package is separate since it creates a Peer type for messages ingress/egress
// these methods are being directly listened to by the client
func (c *SlskClient) HandleServerMessage(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	var decoded map[string]interface{}
	var err error
	code := mr.ReadInt32()
	log.Println("--------------- Decoding server message ----------------")
	log.Printf("Server message: code %d", code)
	switch code {
	case 1:
		decoded, err = c.HandleLogin(mr)
	case 3:
		decoded, err = c.HandleGetPeerAddress(mr)
	case 5:
		decoded, err = c.HandleAddUser(mr)
	case 7:
		decoded, err = c.HandleGetUserStatus(mr)
	case 13:
		decoded, err = c.HandleSayChatroom(mr)
	case 14:
		decoded, err = c.HandleJoinRoom(mr)
	case 15:
		decoded, err = c.HandleLeaveRoom(mr)
	case 16:
		decoded, err = c.HandleUserJoinedRoom(mr)
	case 17:
		decoded, err = c.HandleUserLeftRoom(mr)
	case 18:
		decoded, err = c.HandleConnectToPeer(mr)
	case 22:
		decoded, err = c.HandleMessageUser(mr)
	case 26:
		decoded, err = c.HandleFileSearch(mr)
	case 36:
		decoded, err = c.HandleGetUserStats(mr)
	case 41:
		decoded, err = c.HandleRelog(mr)
	case 64:
		decoded, err = c.HandleRoomList(mr)
	case 69:
		decoded, err = c.HandlePrivilegedUsers(mr)
	case 83:
		decoded, err = c.HandleParentMinSpeed(mr)
	case 84:
		decoded, err = c.HandleParentSpeedRatio(mr)
	case 92:
		decoded, err = c.HandleCheckPrivileges(mr)
	case 93:
		decoded, err = c.HandleSearchRequest(mr)
	case 102:
		decoded, err = c.HandlePossibleParents(mr)
	case 104:
		decoded, err = c.HandleWishlistInterval(mr)
	case 113:
		decoded, err = c.HandleRoomTickerState(mr)
	case 114:
		decoded, err = c.HandleRoomTickerAdd(mr)
	case 115:
		decoded, err = c.HandleRoomTickerRemove(mr)
	case 133:
		decoded, err = c.HandlePrivateRoomUsers(mr)
	case 134:
		decoded, err = c.HandlePrivateRoomAddUser(mr)
	case 135:
		decoded, err = c.HandlePrivateRoomRemoveUser(mr)
	case 139:
		decoded, err = c.HandlePrivateRoomAdded(mr)
	case 140:
		decoded, err = c.HandlePrivateRoomRemoved(mr)
	case 141:
		decoded, err = c.HandlePrivateRoomToggle(mr)
	case 142:
		decoded, err = c.HandleChangePassword(mr)
	case 143:
		decoded, err = c.HandlePrivateRoomAddOperator(mr)
	case 144:
		decoded, err = c.HandlePrivateRoomRemoveOperator(mr)
	case 145:
		decoded, err = c.HandlePrivateRoomOperatorAdded(mr)
	case 146:
		decoded, err = c.HandlePrivateRoomOperatorRemoved(mr)
	case 148:
		decoded, err = c.HandlePrivateRoomOwned(mr)
	case 160:
		decoded, err = c.HandleExcludedSearchPhrases(mr)
	case 1001:
		decoded, err = c.HandleCantConnectToPeer(mr)
	default:
		return nil, fmt.Errorf("invalid code. Received code %d", code)
	}
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func (c *SlskClient) HandleLogin(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	success := mr.ReadBool()
	log.Println("Login Success:", success)
	if !success {
		reason := mr.ReadString()
		return nil, errors.New(reason)
	}
	greetingMessage := mr.ReadString()
	ip := mr.ReadInt32()
	log.Println("greeting message:", greetingMessage)
	log.Println(ip)
	return decoded, nil
}

// we are gonna receive this when we try connecting to a peer
func (c *SlskClient) HandleGetPeerAddress(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	username := mr.ReadString()
	ip := mr.ReadIp()
	port := mr.ReadInt32()
	decoded["username"] = username
	decoded["ip"] = ip
	decoded["port"] = port
	if ip == "0.0.0.0" {
		log.Println("HandleGetPeerAddress: Can't get IP because user", username, "is offline")
		return decoded, nil
	}
	uIP := IP{IP: ip, port: port}
	connType := c.PeerManager.GetPendingPeer(username)
	if connType == "" {
		log.Println("HandleGetPeerAddress: no pending connection for", username)
		return decoded, nil
	}

	// Step 2 of Peer Connection: direct connection request
	err := c.PeerManager.ConnectToPeer(uIP.IP, uIP.port, username, connType, c.ConnectionToken)
	return decoded, err
}

func (c *SlskClient) HandleAddUser(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
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

func (c *SlskClient) HandleGetUserStatus(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "getUserStatus"
	decoded["username"] = mr.ReadString()
	decoded["status"] = mr.ReadInt32()
	decoded["privileged"] = mr.ReadInt8()
	return decoded, nil
}

func (c *SlskClient) HandleSayChatroom(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "sayChatroom"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	decoded["message"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandleJoinRoom(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "joinRoom"
	room := mr.ReadString()
	decoded["room"] = room
	numUsers := mr.ReadInt32()
	usernames := make([]string, numUsers)
	// var users []map[string]interface{}
	for i := uint32(0); i < numUsers; i++ {
		user := make(map[string]interface{})
		username := mr.ReadString()
		user["username"] = username
		// users = append(users, user)
		usernames[i] = username
	}
	c.JoinedRooms[room] = usernames

	// _ = mr.ReadInt32() // number of statuses

	// for i := uint32(0); i < numUsers; i++ {
	// 	status := mr.ReadInt32()
	// 	users[i]["status"] = status
	// }

	// _ = mr.ReadInt32() // number of user stats

	// for i := uint32(0); i < numUsers; i++ {
	// 	users[i]["avgspeed"] = mr.ReadInt32()
	// 	users[i]["uploadnum"] = mr.ReadInt32()
	// 	users[i]["unknown"] = mr.ReadInt32()
	// 	users[i]["files"] = mr.ReadInt32()
	// 	users[i]["folders"] = mr.ReadInt32()
	// }

	// _ = mr.ReadInt32() // number of slotsfree

	// for i := uint32(0); i < numUsers; i++ {
	// 	users[i]["slotsFree"] = mr.ReadInt32()
	// }

	// _ = mr.ReadInt32() // number of user countries

	// for i := uint32(0); i < numUsers; i++ {
	// 	users[i]["country"] = mr.ReadString()
	// }

	return decoded, nil
}

func (c *SlskClient) HandleRoom(mr *messages.ServerMessageReader) ([]map[string]interface{}, error) {
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

func (c *SlskClient) HandleRoomList(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
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

func (c *SlskClient) HandleLeaveRoom(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "leaveRoom"
	decoded["room"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandleUserJoinedRoom(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "userJoinedRoom"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	decoded["status"] = mr.ReadInt32()
	decoded["avgSpeed"] = mr.ReadInt32()
	decoded["uploadNum"] = mr.ReadInt32()
	decoded["unknown"] = mr.ReadInt32()
	decoded["files"] = mr.ReadInt32()
	decoded["folders"] = mr.ReadInt32()
	decoded["slotsFree"] = mr.ReadInt32()
	decoded["country"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandleUserLeftRoom(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "userLeftRoom"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandleConnectToPeer(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "connectToPeer"
	username := mr.ReadString()
	cType := mr.ReadString()
	ip := mr.ReadIp()
	port := mr.ReadInt32()
	token := mr.ReadInt32()
	privileged := mr.ReadInt8()
	decoded["username"] = username
	decoded["cType"] = cType
	decoded["ip"] = ip
	decoded["port"] = port
	decoded["token"] = token
	decoded["privileged"] = privileged
	log.Printf("HandleConnectToPeer: Attempting connection to %s; type %s", username, cType)

	c.PeerManager.AddPendingPeer(username, cType)
	go func() {
		// try direct connection first
		err := c.PeerManager.ConnectToPeer(ip, port, username, cType, token)
		if err != nil {
			log.Printf("HandleConnectToPeer: Failed to connect to %s: %v", username, err)
			return
		}
		peer := c.PeerManager.GetPeer(username)
		if peer == nil {
			log.Printf("HandleConnectToPeer: Failed to connect to %s: %v", username, err)
			return
		}
		// try to pierce firewall if sending peer init fails
		for i := 0; i < 6; i++ {
			err := peer.PierceFirewall(token)
			if err == nil {
				c.PeerManager.AddPeer(peer)
				c.PeerManager.RemovePendingPeer(username)
				return
			}
			log.Printf("HandleConnectToPeer: Failed to pierce firewall for %s: %v... Attempt %d of 6", username, err, i+1)
			time.Sleep(10 * time.Second)
		}
		log.Printf("HandleConnectToPeer: Failed to connect to %s: %v", username, err)
	}()
	return decoded, nil
}

func (c *SlskClient) HandleMessageUser(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "messageUser"
	decoded["id"] = mr.ReadInt32()
	decoded["timestamp"] = time.Unix(int64(mr.ReadInt32()), 0)
	decoded["username"] = mr.ReadString()
	decoded["message"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandleFileSearch(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "fileSearch"
	decoded["username"] = mr.ReadString()
	decoded["token"] = mr.ReadInt32()
	decoded["query"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePing(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "ping"
	return decoded, nil
}

func (c *SlskClient) HandleGetUserStats(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "getUserStats"
	decoded["username"] = mr.ReadString()
	decoded["speed"] = mr.ReadInt32()
	decoded["downloadNum"] = mr.ReadInt64()
	decoded["files"] = mr.ReadInt32()
	decoded["directories"] = mr.ReadInt32()
	return decoded, nil
}

func (c *SlskClient) HandleRelog(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "relog"
	return decoded, nil
}

func (c *SlskClient) HandlePrivilegedUsers(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	var users []string
	decoded["type"] = "privilegedUsers"

	userCount := mr.ReadInt32()
	for i := uint32(0); i < userCount; i++ {
		user := mr.ReadString()
		users = append(users, user)
	}
	decoded["users"] = users
	return decoded, nil
}

func (c *SlskClient) HandleParentMinSpeed(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "parentMinSpeed"
	decoded["minSpeed"] = mr.ReadInt32()
	return decoded, nil
}

func (c *SlskClient) HandleParentSpeedRatio(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "parentSpeedRatio"
	decoded["ratio"] = mr.ReadInt32()
	return decoded, nil
}

func (c *SlskClient) HandleCheckPrivileges(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "checkPrivileges"
	decoded["timeLeft"] = mr.ReadInt32()
	return decoded, nil
}

func (c *SlskClient) HandleSearchRequest(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "searchRequest"
	decoded["distributedCode"] = mr.ReadInt8()
	decoded["unknown"] = mr.ReadInt32()
	decoded["username"] = mr.ReadString()
	decoded["token"] = mr.ReadInt32()
	decoded["query"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePossibleParents(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	var parents []map[string]interface{}
	decoded["type"] = "netInfo"

	parentCount := mr.ReadInt32()
	for i := uint32(0); i < parentCount; i++ {
		parent := make(map[string]interface{})
		parent["username"] = mr.ReadString()
		parent["ip"] = mr.ReadIp()
		parent["port"] = mr.ReadInt32()
		parents = append(parents, parent)
	}
	decoded["parents"] = parents
	return decoded, nil
}

func (c *SlskClient) HandleWishlistInterval(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "wishlistInterval"
	decoded["interval"] = mr.ReadInt32()
	return decoded, nil
}

func (c *SlskClient) HandleRoomTickerState(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
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

func (c *SlskClient) HandleRoomTickerAdd(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "roomTickerAdd"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	decoded["ticker"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandleRoomTickerRemove(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "roomTickerRemove"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomUsers(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
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

func (c *SlskClient) HandlePrivateRoomAddUser(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomAddUser"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomRemoveUser(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomRemoveUser"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomAdded(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomAdded"
	decoded["room"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomRemoved(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomRemoved"
	decoded["room"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomToggle(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomToggle"
	decoded["enable"] = mr.ReadInt8()
	return decoded, nil
}

func (c *SlskClient) HandleChangePassword(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "changePassword"
	decoded["password"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomAddOperator(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomAddOperator"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomRemoveOperator(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomRemoveOperator"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomOperatorAdded(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomOperatorAdded"
	decoded["room"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomOperatorRemoved(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomOperatorRemoved"
	decoded["room"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomOwned(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
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

func (c *SlskClient) HandleCantConnectToPeer(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "cantConnectToPeer"
	decoded["token"] = mr.ReadInt32()
	decoded["username"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandleExcludedSearchPhrases(mr *messages.ServerMessageReader) (map[string]interface{}, error) {
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
