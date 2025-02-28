package peer

import (
	"encoding/binary"
	"io"
	"os"
	"path"
	"spotseek/config"
	"spotseek/slsk/messages"
	"time"
)

type FileTransferPeer interface {
	FileTransferInit(token uint32)
	FileOffset(offset uint64)
	FileListen()
}

type fileTransferPeer struct {
	Peer *Peer
}

func NewFileTransferPeer(peer *Peer) FileTransferPeer {
	return &fileTransferPeer{
		Peer: peer,
	}
}

func (peer *fileTransferPeer) FileTransferInit(token uint32) {
	mb := messages.NewMessageBuilder()
	mb.AddInt32(token)
	peer.Peer.SendMessage(mb.Message)
}

func (peer *fileTransferPeer) FileOffset(offset uint64) {
	mb := messages.NewMessageBuilder()
	mb.AddInt64(offset)
	peer.Peer.SendMessage(mb.Message)
}

func (peer *fileTransferPeer) FileListen() {

	defer func() {
		peer.Peer.logger.Info("File transfer peer disconnected", "peer", peer.Peer.Username)
		peer.Peer.mgrCh <- PeerEvent{Type: PeerDisconnected, Peer: peer.Peer}
	}()

	if peer.Peer.PeerConnection == nil {
		peer.Peer.logger.Error("PeerConnection is nil")
		return
	}

	// this might not work
	peer.Peer.PeerConnection.Conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	tokenBuf := make([]byte, 4)
	if _, err := peer.Peer.PeerConnection.Read(tokenBuf); err != nil {
		peer.Peer.logger.Error("failed to read transfer token", "error", err)
		return
	}

	token := binary.LittleEndian.Uint32(tokenBuf)
	peer.Peer.logger.Info("File transfer token received", "token", token)

	// Find the pending transfer associated with this token
	peer.Peer.transfersMutex.RLock()
	transfer, exists := peer.Peer.pendingTransfers[token]
	peer.Peer.transfersMutex.RUnlock()

	if !exists {
		peer.Peer.logger.Error("No pending transfer found for token", "token", token, "pendingTransfers", peer.Peer.pendingTransfers)
		return
	}
	peer.Peer.logger.Info("File transfer found", "token", token)

	peer.FileOffset(transfer.Offset)

	readBuffer := make([]byte, 4096)
	for {
		n, err := peer.Peer.PeerConnection.Read(readBuffer)
		if err != nil {
			if err == io.EOF {
				peer.Peer.logger.Info("File transfer completed",
					"peer", peer.Peer.Username,
					"filename", transfer.Filename)
				return
			}
			peer.Peer.logger.Error("error reading file data", "error", err)
			return
		}
		peer.Peer.logger.Info("File transfer data received", "n", n)
		// Write to transfer buffer
		if _, err := transfer.Buffer.Write(readBuffer[:n]); err != nil {
			peer.Peer.logger.Error("error writing to buffer", "error", err)
			return
		}

		transfer.Offset += uint64(n)

		// Check if transfer is complete
		if uint64(transfer.Buffer.Len()) >= transfer.Size {
			peer.Peer.logger.Info("File transfer complete", "filename", transfer.Filename)
			writeToDownloadsDir(transfer.Filename, transfer.Buffer.Bytes())
			return
		}
	}
}

func writeToDownloadsDir(filename string, data []byte) error {
	dir := config.GetSettings().DownloadPath

	file, err := os.Create(path.Join(dir, filename))
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		return err
	}

	return nil
}
