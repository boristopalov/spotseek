package peer

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path"
	"spotseek/config"
	"spotseek/slsk/messages"
)

type FileTransferPeer interface {
	BasePeer
	FileTransferInit(token uint32) error
	FileOffset(offset uint64) error
}

func (peer *Peer) FileTransferInit(token uint32) error {
	mb := messages.NewMessageBuilder()
	mb.AddInt32(token)
	err := peer.SendMessage(mb.Message)
	return err
}

func (peer *Peer) FileOffset(offset uint64) error {
	mb := messages.NewMessageBuilder()
	mb.AddInt64(offset)
	err := peer.SendMessage(mb.Message)
	return err
}

func (peer *Peer) FileListen() error {
	defer func() {
		peer.mgrCh <- PeerEvent{Type: PeerDisconnected, Peer: peer}
	}()

	tokenBuf := make([]byte, 4)
	if _, err := io.ReadFull(peer.PeerConnection, tokenBuf); err != nil {
		return fmt.Errorf("failed to read transfer token: %w", err)
	}
	token := binary.LittleEndian.Uint32(tokenBuf)

	// Find the pending transfer associated with this token
	peer.transfersMutex.Lock()
	transfer, exists := peer.pendingTransfers[token]
	peer.transfersMutex.RUnlock()

	if !exists {
		return fmt.Errorf("no pending transfer found for token: %d", token)
	}

	offsetBuf := make([]byte, 8)
	if _, err := io.ReadFull(peer.PeerConnection, offsetBuf); err != nil {
		return fmt.Errorf("failed to read transfer token: %w", err)
	}
	offset := binary.LittleEndian.Uint64(offsetBuf)

	peer.transfersMutex.Lock()
	transfer.Offset = offset
	peer.transfersMutex.RUnlock()

	readBuffer := make([]byte, 4096)
	for {
		n, err := peer.PeerConnection.Read(readBuffer)
		if err != nil {
			if err == io.EOF {
				log.Info("File transfer completed",
					"peer", peer.Username,
					"filename", transfer.Filename)
				return nil
			}
			return fmt.Errorf("error reading file data: %w", err)
		}

		// Write to transfer buffer
		peer.transfersMutex.Lock()
		defer peer.transfersMutex.Unlock()
		if _, err := transfer.Buffer.Write(readBuffer[:n]); err != nil {
			return fmt.Errorf("error writing to buffer: %w", err)
		}

		// Check if transfer is complete
		if uint64(transfer.Buffer.Len()) >= transfer.Size {
			err = writeToDownloadsDir(transfer.Filename, transfer.Buffer.Bytes())
			if err != nil {
				return err
			}
			delete(peer.pendingTransfers, token)
			return nil
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
