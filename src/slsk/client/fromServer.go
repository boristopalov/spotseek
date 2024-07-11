package client

import (
	"errors"
	"fmt"
	"log"
	"spotseek/src/slsk/messages/serverMessages"
	"spotseek/src/slsk/peer"
	"time"
)

// the SlskClient type is listening for messages
// probably simplest to just move this into slskClient package
// peer package is separate since it creates a Peer type for messages ingress/egress
// these methods are being directly listened to by the client
func (c *SlskClient) HandleServerMessage(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	var decoded map[string]interface{}
	var err error
	code := mr.ReadInt32()
	log.Println("Received message from server with code", code)
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
	case 32:
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

func (c *SlskClient) HandleLogin(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
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

func (c *SlskClient) HandleGetPeerAddress(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	username := mr.ReadString()
	ip := mr.ReadIp()
	port := mr.ReadInt32()
	decoded["username"] = username
	decoded["ip"] = ip
	decoded["port"] = port
	if ip == "0.0.0.0" {
		log.Println("Can't get IP because user", username, "is offline")
		return decoded, nil
	}
	uIP := IP{IP: ip, port: port}
	c.UsernameIps[username] = uIP
	// _, ok := c.PendingUsernameIps[username]
	// if ok {
	// 	delete(c.PendingUsernameIps, username)
	// }
	// connType, ok := c.PendingUsernameConnTypes[username]
	// if !ok {
	// 	log.Println("no pending connection for", username)
	// 	return decoded, nil
	// }
	// log.Println("Attempting direct connection to", username, ip, port)
	// peer := NewPeer(username, c.Listener, connType, 0, ip, port) // attempt direct connection
	// if peer != nil {
	// c.ListenForPeerMessages(peer)
	// err := peer.PeerInit(username, connType, 0)
	// log.Println(peer.UserInfoRequest())
	// if err != nil {
	// 	log.Println(err)
	// } else {
	// log.Println("sent PeerInit to", username)
	// }
	// }
	return decoded, nil
}

func (c *SlskClient) HandleAddUser(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
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

func (c *SlskClient) HandleGetUserStatus(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "getUserStatus"
	decoded["username"] = mr.ReadString()
	decoded["status"] = mr.ReadInt32()
	decoded["privileged"] = mr.ReadInt8()
	return decoded, nil
}

func (c *SlskClient) HandleSayChatroom(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "sayChatroom"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	decoded["message"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandleJoinRoom(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "joinRoom"
	decoded["room"] = mr.ReadString()
	var users []map[string]interface{}
	numUsers := mr.ReadInt32()
	for i := uint32(0); i < numUsers; i++ {
		user := make(map[string]interface{})
		username := mr.ReadString()
		user["username"] = username
		users = append(users, user)
	}

	_ = mr.ReadInt32()

	for i := uint32(0); i < numUsers; i++ {
		status := mr.ReadInt32()
		users[i]["status"] = status
	}

	_ = mr.ReadInt32()

	for i := uint32(0); i < numUsers; i++ {
		users[i]["speed"] = mr.ReadInt32()
		users[i]["downloadNum"] = mr.ReadInt64()
		users[i]["files"] = mr.ReadInt32()
		users[i]["folders"] = mr.ReadInt32()
	}

	_ = mr.ReadInt32()

	for i := uint32(0); i < numUsers; i++ {
		users[i]["slotsFree"] = mr.ReadInt32()
	}

	_ = mr.ReadInt32()

	for i := uint32(0); i < numUsers; i++ {
		users[i]["country"] = mr.ReadString()
	}
	return decoded, nil
}

func (c *SlskClient) HandleRoom(mr *serverMessages.ServerMessageReader) ([]map[string]interface{}, error) {
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

func (c *SlskClient) HandleRoomList(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
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

func (c *SlskClient) HandleLeaveRoom(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "leaveRoom"
	decoded["room"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandleUserJoinedRoom(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "userJoinedRoom"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	decoded["status"] = mr.ReadInt32()
	decoded["speed"] = mr.ReadInt32()
	decoded["downloadNum"] = mr.ReadInt64()
	decoded["files"] = mr.ReadInt32()
	decoded["folders"] = mr.ReadInt32()
	decoded["slotsFree"] = mr.ReadInt32()
	decoded["country"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandleUserLeftRoom(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "userLeftRoom"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandleConnectToPeer(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
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

	log.Printf("received request from server to connect to %s with connection type %s", username, cType)

	_, ok := c.ConnectedPeers[username]
	if ok && cType == c.ConnectedPeers[username].ConnType {
		log.Printf("Already connected to %s", username)
		return decoded, nil // we are already connected to this peer
	}

	// _, ok = c.PendingPeerInits[username]
	// if ok && cType == c.PendingPeerInits[username].ConnType {
	// 	// we have a pending connection to this peer and need to establish it
	// }

	log.Println("-----received info about peer--------\n", decoded)

	go func() {
		peer := peer.NewPeer(username, c.Listener, cType, token, ip, port)
		if peer == nil {
			log.Printf("Failed to establish connection to %s", username)
			c.CantConnectToPeer(token, username)
			return
		}

		// try sending PierceFirewall every 10 seconds for 1 minute (6 times)
		piercedFirewall := false
		for i := 0; i < 6; i++ {
			err := peer.PierceFirewall(token)
			if err != nil {
				log.Printf("Failed to pierce firewall for %s: %v... Attempt %d of 6", username, err, i+1)
				time.Sleep(10 * time.Second)
			} else {
				piercedFirewall = true
				break
			}
		}
		if !piercedFirewall {
			c.CantConnectToPeer(token, username)
			return
		}

		err := peer.PeerInit(c.User, cType, token)
		if err != nil {
			log.Printf("Failed to send PeerInit to %s: %v", username, err)
			c.CantConnectToPeer(token, username)
			return
		}

		c.ConnectedPeers[username] = *peer
		log.Printf("Successfully connected to peer %s", username)
		go c.ListenForPeerMessages(peer)
	}()
	return decoded, nil
}

func (c *SlskClient) HandleMessageUser(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "messageUser"
	decoded["id"] = mr.ReadInt32()
	decoded["timestamp"] = time.Unix(int64(mr.ReadInt32()), 0)
	decoded["username"] = mr.ReadString()
	decoded["message"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandleFileSearch(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "fileSearch"
	decoded["username"] = mr.ReadString()
	decoded["query"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePing(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "ping"
	return decoded, nil
}

func (c *SlskClient) HandleGetUserStats(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "getUserStats"
	decoded["username"] = mr.ReadString()
	decoded["speed"] = mr.ReadInt32()
	decoded["downloadNum"] = mr.ReadInt64()
	decoded["files"] = mr.ReadInt32()
	decoded["directories"] = mr.ReadInt32()
	return decoded, nil
}

func (c *SlskClient) HandleRelog(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "relog"
	return decoded, nil
}

func (c *SlskClient) HandlePrivilegedUsers(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
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

func (c *SlskClient) HandleParentMinSpeed(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "parentMinSpeed"
	decoded["minSpeed"] = mr.ReadInt32()
	return decoded, nil
}

func (c *SlskClient) HandleParentSpeedRatio(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "parentSpeedRatio"
	decoded["ratio"] = mr.ReadInt32()
	return decoded, nil
}

func (c *SlskClient) HandleCheckPrivileges(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "checkPrivileges"
	decoded["timeLeft"] = mr.ReadInt32()
	return decoded, nil
}

func (c *SlskClient) HandleSearchRequest(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "searchRequest"
	decoded["distributedCode"] = mr.ReadInt8()
	decoded["unknown"] = mr.ReadInt32()
	decoded["username"] = mr.ReadString()
	decoded["token"] = mr.ReadInt32()
	decoded["query"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePossibleParents(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
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

func (c *SlskClient) HandleWishlistInterval(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "wishlistInterval"
	decoded["interval"] = mr.ReadInt32()
	return decoded, nil
}

func (c *SlskClient) HandleRoomTickerState(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
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

func (c *SlskClient) HandleRoomTickerAdd(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "roomTickerAdd"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	decoded["ticker"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandleRoomTickerRemove(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "roomTickerRemove"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomUsers(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
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

func (c *SlskClient) HandlePrivateRoomAddUser(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomAddUser"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomRemoveUser(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomRemoveUser"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomAdded(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomAdded"
	decoded["room"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomRemoved(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomRemoved"
	decoded["room"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomToggle(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomToggle"
	decoded["enable"] = mr.ReadInt8()
	return decoded, nil
}

func (c *SlskClient) HandleChangePassword(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "changePassword"
	decoded["password"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomAddOperator(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomAddOperator"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomRemoveOperator(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomRemoveOperator"
	decoded["room"] = mr.ReadString()
	decoded["username"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomOperatorAdded(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomOperatorAdded"
	decoded["room"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomOperatorRemoved(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "privateRoomOperatorRemoved"
	decoded["room"] = mr.ReadString()
	return decoded, nil
}

func (c *SlskClient) HandlePrivateRoomOwned(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
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

func (c *SlskClient) HandleCantConnectToPeer(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
	decoded := make(map[string]interface{})
	decoded["type"] = "cantConnectToPeer"
	decoded["token"] = mr.ReadInt32()
	return decoded, nil
}

func (c *SlskClient) HandleExcludedSearchPhrases(mr *serverMessages.ServerMessageReader) (map[string]interface{}, error) {
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
