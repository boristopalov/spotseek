package peer

import (
	"encoding/binary"
	"fmt"
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

type fileTransferPeer = Peer

func (peer *fileTransferPeer) FileTransferInit(token uint32) {
	mb := messages.NewMessageBuilder()
	mb.AddInt32(token)
	peer.SendMessage(mb.Message)
}

func (peer *fileTransferPeer) FileOffset(offset uint64) {
	mb := messages.NewMessageBuilder()
	mb.AddInt64(offset)
	peer.SendMessage(mb.Message)
}

func (peer *fileTransferPeer) FileListen() {
	peer.logger.Info("Listening for file transfer", "peer", peer.Username)
	defer func() {
		peer.logger.Info("File transfer peer disconnected", "peer", peer.Username)
		peer.mgrCh <- PeerEvent{Type: PeerDisconnected, Peer: peer}
	}()

	if peer.PeerConnection == nil {
		peer.logger.Error("PeerConnection is nil")
		return
	}

	// this might not work
	peer.PeerConnection.Conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	tokenBuf := make([]byte, 4)
	if _, err := peer.PeerConnection.Read(tokenBuf); err != nil {
		peer.logger.Error("failed to read transfer token", "error", err)
		return
	}

	token := binary.LittleEndian.Uint32(tokenBuf)
	peer.logger.Info("File transfer token received", "token", token)

	// Find the pending transfer associated with this token
	peer.transfersMutex.RLock()
	transfer, exists := peer.pendingTransfers[token]
	peer.transfersMutex.RUnlock()

	if !exists {
		peer.logger.Error("No pending transfer found for token", "token", token, "pendingTransfers", peer.pendingTransfers)
		return
	}
	peer.logger.Info("File transfer found", "token", token)

	peer.FileOffset(transfer.Offset)

	readBuffer := make([]byte, 4096)
	for {
		n, err := peer.PeerConnection.Read(readBuffer)
		if err != nil {
			if err == io.EOF {
				peer.logger.Info("File transfer completed",
					"peer", peer.Username,
					"filename", transfer.Filename)

				// Send download failed event (unexpected EOF)
				peer.mgrCh <- PeerEvent{
					Type: DownloadFailed,
					Peer: peer,
					Msg: DownloadFailedMsg{
						Token: token,
						Error: "Unexpected EOF",
					},
				}
				return
			}
			peer.logger.Error("error reading file data", "error", err)

			// Send download failed event
			peer.mgrCh <- PeerEvent{
				Type: DownloadFailed,
				Peer: peer,
				Msg: DownloadFailedMsg{
					Token: token,
					Error: err.Error(),
				},
			}
			return
		}
		peer.logger.Debug("File transfer data received", "n", n, "token", token)

		// Write to transfer buffer
		if _, err := transfer.Buffer.Write(readBuffer[:n]); err != nil {
			peer.logger.Error("error writing to buffer", "error", err)

			// Send download failed event
			peer.mgrCh <- PeerEvent{
				Type: DownloadFailed,
				Peer: peer,
				Msg: DownloadFailedMsg{
					Token: token,
					Error: fmt.Sprintf("buffer write error: %v", err),
				},
			}
			return
		}

		transfer.Offset += uint64(n)

		// Send progress update
		peer.mgrCh <- PeerEvent{
			Type: DownloadProgress,
			Peer: peer,
			Msg: DownloadProgressMsg{
				Token:         token,
				BytesReceived: transfer.Offset,
			},
		}

		// Check if transfer is complete
		if uint64(transfer.Buffer.Len()) >= transfer.Size {
			peer.logger.Info("File transfer complete", "filename", transfer.Filename, "token", token)
			err := writeToDownloadsDir(transfer.Filename, transfer.Buffer.Bytes())
			if err != nil {
				peer.logger.Error("error writing file to disk", "error", err)

				// Send download failed event
				peer.mgrCh <- PeerEvent{
					Type: DownloadFailed,
					Peer: peer,
					Msg: DownloadFailedMsg{
						Token: token,
						Error: fmt.Sprintf("disk write error: %v", err),
					},
				}
				return
			}

			// Send download complete event
			peer.mgrCh <- PeerEvent{
				Type: DownloadComplete,
				Peer: peer,
				Msg: DownloadCompleteMsg{
					Token:    token,
					Filename: transfer.Filename,
				},
			}
			return
		}
	}
}

func (peer *fileTransferPeer) UploadFile(ft FileTransfer) {
	// Open the file - OS will automatically follow the symlink to the actual file
	peer.logger.Info("Uploading file", "peer", peer.Username, "filename", ft.Filename)
	file, err := os.Open(ft.Filename)
	if err != nil {
		peer.logger.Error("failed to open file for upload (virtual path: %s)", "error", err)
		return
	}
	defer file.Close()

	// If there's an offset, seek to that position
	if ft.Offset > 0 {
		_, err = file.Seek(int64(ft.Offset), io.SeekStart)
		if err != nil {
			peer.logger.Error("failed to seek to offset %d", "error", err)
		}
	}

	// Define chunk size (e.g., 8KB)
	const chunkSize = 8 * 1024
	buffer := make([]byte, chunkSize)

	// Calculate remaining bytes to send
	remainingBytes := ft.Size - ft.Offset
	bytesTransferred := uint64(0)

	// Send the file in chunks
	for remainingBytes > 0 {
		// Determine the size of the current chunk
		currentChunkSize := chunkSize
		if remainingBytes < uint64(chunkSize) {
			currentChunkSize = int(remainingBytes)
		}

		// Read a chunk from the file
		n, err := file.Read(buffer[:currentChunkSize])
		if err != nil && err != io.EOF {
			peer.logger.Error("error reading file chunk", "error", err)
		}
		if n == 0 {
			break // End of file
		}

		// Send the chunk
		err = peer.SendMessage(buffer[:n])
		if err != nil {
			peer.logger.Error("error sending file chunk", "error", err)
		}

		// Update tracking variables
		remainingBytes -= uint64(n)
		bytesTransferred += uint64(n)

	}
	peer.logger.Info("Upload complete", "filename", ft.Filename)
	peer.mgrCh <- PeerEvent{Type: UploadComplete, Peer: peer, Msg: UploadCompleteMsg{Filename: ft.Filename, Token: ft.Token}}
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
