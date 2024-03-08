package serverMessages

import (
	"crypto/md5"
	"encoding/hex"
	"spotseek/src/slskClient/messages"
)

// Soulseek protocol required variables
const VERSION = 182
const MINOR = 157

// token to increment for each search 
var SEARCH_TOKEN int32 = 2

type ServerMessageBuilder struct {
    *messages.MessageBuilder
}

func (mb *ServerMessageBuilder) Login(username string, password string) []byte {
	// Example usage to construct a login message
	// Add message contents
	mb.AddString(username)     // Username
	mb.AddString(password)     // Password
	mb.AddInt32(VERSION)             // Version
	hash := md5.Sum([]byte(username+password))
	mb.AddString(hex.EncodeToString(hash[:]))   // Hash value
	mb.AddInt32(MINOR)              // Minor version

	// Build the message

	message := mb.Build(1)
	return message

}
func (mb *ServerMessageBuilder) SetWaitPort(port uint32) []byte {
	mb.AddInt32(port)     // The port
	return mb.Build(2)    // The message code for building should be the same as the one used above
}

func (mb *ServerMessageBuilder) GetPeerAddress(username string) []byte {
	mb.AddString(username)    // The username
	return mb.Build(3)        // The message code for building should be the same as the one used above
}

func (mb *ServerMessageBuilder) AddUser(username string) []byte {
	mb.AddString(username)    // The username
	return mb.Build(5)        // The message code for building should be the same as the one used above
}

func (mb *ServerMessageBuilder) GetUserStatus(username string) []byte {
	mb.AddString(username)    // The username
	return mb.Build(7)        // The message code for building should be the same as the one used above
}


func (mb *ServerMessageBuilder) SayChatroom(room, message string) []byte {
	mb.AddString(room)
	mb.AddString(message)
	return mb.Build(13)
}

func (mb *ServerMessageBuilder) JoinRoom(room string) []byte {
	mb.AddString(room)
	return mb.Build(14)
}

func (mb *ServerMessageBuilder) LeaveRoom(room string) []byte {
	mb.AddString(room)
	return mb.Build(15)
}

func (mb *ServerMessageBuilder) MessageUser(username, message string) []byte {
	mb.AddString(username)
	mb.AddString(message)
	return mb.Build(22)
}

func (mb *ServerMessageBuilder) MessageAcked(id uint32) []byte {
	mb.AddInt32(id)
	return mb.Build(23)
}

func (mb *ServerMessageBuilder) FileSearch(token uint32, query string) []byte {
	mb.AddInt32(token)
	mb.AddString(query)
	return mb.Build(26)
}

func (mb *ServerMessageBuilder) SetStatus(status uint32) []byte {
	mb.AddInt32(status)
	return mb.Build(28)
}

func (mb *ServerMessageBuilder) Ping() []byte {
	return mb.Build(32)
}

func (mb *ServerMessageBuilder) SharedFoldersFiles(folderCount, fileCount uint32) []byte {
	mb.AddInt32(folderCount)
	mb.AddInt32(fileCount)
	return mb.Build(35)
}


func (mb *ServerMessageBuilder) PrivilegedUsers() []byte {
	return mb.Build(69)
}

func (mb *ServerMessageBuilder) HaveNoParent(haveParent int8) []byte {
	mb.AddInt8(haveParent)
	return mb.Build(71)
}


func (mb *ServerMessageBuilder) CheckPrivileges() []byte {
	return mb.Build(92)
}

func (mb *ServerMessageBuilder) AcceptChildren(accept int8) []byte {
	mb.AddInt8(accept)
	return mb.Build(100)
}

func (mb *ServerMessageBuilder) RoomSearch(room string, token uint32, query string) []byte {
	mb.AddString(room)
	mb.AddInt32(token)
	mb.AddString(query)
	return mb.Build(120)
}


func (mb *ServerMessageBuilder) SendUploadSpeed(speed uint32) []byte {
	mb.AddInt32(speed)
	return mb.Build(121)
}

func (mb *ServerMessageBuilder) ChangePassword(password string) []byte {
	mb.AddString(password)
	return mb.Build(142)
}

func (mb *ServerMessageBuilder) PrivateRoomDisown(room string) []byte {
	mb.AddString(room)
	return mb.Build(137)
}

func (mb *ServerMessageBuilder) PrivateRoomToggle(enable int8) []byte {
	mb.AddInt8(enable)
	return mb.Build(141)
}

func (mb *ServerMessageBuilder) PrivateRoomAddOperator(room, username string) []byte {
	mb.AddString(room)
	mb.AddString(username)
	return mb.Build(143)
}

func (mb *ServerMessageBuilder) PrivateRoomRemoveOperator(room, username string) []byte {
	mb.AddString(room)
	mb.AddString(username)
	return mb.Build(144)
}

func (mb *ServerMessageBuilder) ConnectToPeer(token uint32, username string, connType string) []byte {
	mb.AddInt32(token)
	mb.AddString(username)
	mb.AddString(connType)
	return mb.Build(18)
}

func (mb *ServerMessageBuilder) CantConnectToPeer(token uint32, username string) []byte {
	mb.AddInt32(token)
	mb.AddString(username)
	return mb.Build(1001)
}


func (mb *ServerMessageBuilder) UserSearch(username string, token uint32, query string) []byte { 
	mb.AddString(username)
	mb.AddInt32(token)
	mb.AddString(query)
	return mb.Build(42)
}
