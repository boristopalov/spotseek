package messages

import (
	"crypto/md5"
	"encoding/hex"
)


const VERSION = 182
const MINOR = 157
var SEARCH_TOKEN int32 = 2

func (mb *MessageBuilder) Login(username string, password string) []byte {
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
func (mb *MessageBuilder) SetWaitPort(port uint32) []byte {
	mb.AddInt32(port)     // The port
	return mb.Build(2)    // The message code for building should be the same as the one used above
}

func (mb *MessageBuilder) GetPeerAddress(username string) []byte {
	mb.AddString(username)    // The username
	return mb.Build(3)        // The message code for building should be the same as the one used above
}

func (mb *MessageBuilder) AddUser(username string) []byte {
	mb.AddString(username)    // The username
	return mb.Build(5)        // The message code for building should be the same as the one used above
}

func (mb *MessageBuilder) GetUserStatus(username string) []byte {
	mb.AddString(username)    // The username
	return mb.Build(7)        // The message code for building should be the same as the one used above
}


func (mb *MessageBuilder) SayChatroom(room, message string) []byte {
	mb.AddString(room)
	mb.AddString(message)
	return mb.Build(13)
}

func (mb *MessageBuilder) JoinRoom(room string) []byte {
	mb.AddString(room)
	return mb.Build(14)
}

func (mb *MessageBuilder) LeaveRoom(room string) []byte {
	mb.AddString(room)
	return mb.Build(15)
}

func (mb *MessageBuilder) MessageUser(username, message string) []byte {
	mb.AddString(username)
	mb.AddString(message)
	return mb.Build(22)
}

func (mb *MessageBuilder) MessageAcked(id uint32) []byte {
	mb.AddInt32(id)
	return mb.Build(23)
}

func (mb *MessageBuilder) FileSearch(token uint32, query string) []byte {
	mb.AddInt32(token)
	mb.AddString(query)
	return mb.Build(26)
}

func (mb *MessageBuilder) SetStatus(status uint32) []byte {
	mb.AddInt32(status)
	return mb.Build(28)
}

func (mb *MessageBuilder) Ping() []byte {
	return mb.Build(32)
}

func (mb *MessageBuilder) SharedFoldersFiles(folderCount, fileCount uint32) []byte {
	mb.AddInt32(folderCount)
	mb.AddInt32(fileCount)
	return mb.Build(35)
}


func (mb *MessageBuilder) PrivilegedUsers() []byte {
	return mb.Build(69)
}

func (mb *MessageBuilder) HaveNoParent(haveParent int8) []byte {
	mb.AddInt8(haveParent)
	return mb.Build(71)
}


func (mb *MessageBuilder) CheckPrivileges() []byte {
	return mb.Build(92)
}

func (mb *MessageBuilder) AcceptChildren(accept int8) []byte {
	mb.AddInt8(accept)
	return mb.Build(100)
}

func (mb *MessageBuilder) RoomSearch(room string, token uint32, query string) []byte {
	mb.AddString(room)
	mb.AddInt32(token)
	mb.AddString(query)
	return mb.Build(120)
}


func (mb *MessageBuilder) SendUploadSpeed(speed uint32) []byte {
	mb.AddInt32(speed)
	return mb.Build(121)
}

func (mb *MessageBuilder) ChangePassword(password string) []byte {
	mb.AddString(password)
	return mb.Build(142)
}

func (mb *MessageBuilder) PrivateRoomDisown(room string) []byte {
	mb.AddString(room)
	return mb.Build(137)
}

func (mb *MessageBuilder) PrivateRoomToggle(enable int8) []byte {
	mb.AddInt8(enable)
	return mb.Build(141)
}

func (mb *MessageBuilder) PrivateRoomAddOperator(room, username string) []byte {
	mb.AddString(room)
	mb.AddString(username)
	return mb.Build(143)
}

func (mb *MessageBuilder) PrivateRoomRemoveOperator(room, username string) []byte {
	mb.AddString(room)
	mb.AddString(username)
	return mb.Build(144)
}

func (mb *MessageBuilder) ConnectToPeer(token uint32, username string, connType string) []byte {
	mb.AddInt32(token)
	mb.AddString(username)
	mb.AddString(connType)
	return mb.Build(18)
}

func (mb *MessageBuilder) CantConnectToPeer(token uint32, username string) []byte {
	mb.AddInt32(token)
	mb.AddString(username)
	return mb.Build(1001)
}


func (mb *MessageBuilder) UserSearch(username string, token uint32, query string) []byte { 
	mb.AddString(username)
	mb.AddInt32(token)
	mb.AddString(query)
	return mb.Build(42)
}