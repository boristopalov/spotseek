package peer

import (
	"spotseek/src/slsk/messages"
)

type DefaultPeer interface {
	BasePeer
	SharedFileListRequest()
	SharedFileListResponse()
	FileSearchResponse()
	UserInfoRequest()
	UserInfoResponse()
	FolderContentsRequest()
	FolderContentsResponse()
	TransferRequest()
	UploadResponse()
	QueueUpload(filename string) error
	UploadFailed()
	UploadDenied()
	PlaceInQueueRequest(filename string) error
}

func (peer *Peer) QueueUpload(filename string) error {
	mb := messages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.QueueUpload(filename)
	err := peer.SendMessage(msg)
	return err
}

func (peer *Peer) SharedFileListRequest() error {
	mb := messages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.SharedFileListRequest()
	err := peer.SendMessage(msg)
	return err
}

func (peer *Peer) UserInfoRequest() error {
	mb := messages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.UserInfoRequest()
	err := peer.SendMessage(msg)
	return err
}

func (peer *Peer) PlaceInQueueRequest(filename string) error {
	mb := messages.PeerMessageBuilder{
		MessageBuilder: messages.NewMessageBuilder(),
	}
	msg := mb.PlaceInQueueRequest(filename)
	err := peer.SendMessage(msg)
	return err
}
