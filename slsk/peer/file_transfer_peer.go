package peer

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path"
	"spotseek/config"
	"spotseek/slsk/messages"
	"strings"
	"time"
)

const uploadChunkSize = 8 * 1024

func (peer *FileTransferPeer) FileTransferInit(token uint32) {
	mb := messages.NewMessageBuilder()
	mb.AddInt32(token)
	peer.SendMessage(mb.Message)
}

func (peer *FileTransferPeer) FileOffset(offset uint64) {
	mb := messages.NewMessageBuilder()
	mb.AddInt64(offset)
	peer.SendMessage(mb.Message)
}

func (peer *FileTransferPeer) FileListen(transfer *FileTransfer) {
	if transfer == nil {
		peer.logger.Error("Tried listening for FileTransfer from peer but Transfer is nil")
		return
	}
	peer.logger.Info("Listening for file transfer", "peer", peer.Username)
	defer func() {
		peer.logger.Info("File transfer peer disconnected", "peer", peer.Username)
		peer.mgrCh <- PeerEvent{Type: PeerDisconnected, Username: peer.Username, Host: peer.Host, Port: peer.Port}
	}()

	if peer.Conn == nil {
		peer.logger.Error("Connection is nil")
		return
	}

	// Set initial timeout for reading the token (10 seconds for connection establishment)
	peer.Conn.Conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	tokenBuf := make([]byte, 4)
	if _, err := peer.Conn.Read(tokenBuf); err != nil {
		peer.logger.Error("failed to read transfer token", "error", err)
		return
	}

	token := binary.LittleEndian.Uint32(tokenBuf)
	peer.logger.Info("File transfer token received", "token", token)

	if transfer == nil {
		peer.logger.Error("No transfer info available for file transfer peer", "token", token)
		return
	}

	peer.FileOffset(transfer.Offset)

	transferBuf := bytes.NewBuffer(make([]byte, transfer.Size))

	// After sending FileOffset, give the peer up to 30 seconds to start sending data
	// This accounts for upload queue processing, file opening, and seeking to offset
	peer.Conn.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	readBuffer := make([]byte, 4096)
	for {
		n, err := peer.Conn.Read(readBuffer)
		if err != nil {
			if err == io.EOF {
				// Check if we received all expected bytes
				if uint64(transferBuf.Len()) >= transfer.Size {
					peer.logger.Info("File transfer completed (EOF after all bytes received)",
						"peer", peer.Username,
						"filename", transfer.Filename,
						"bytesReceived", transferBuf.Len(),
						"expectedSize", transfer.Size)

					// Write file to disk
					writeErr := writeToDownloadsDir(extractBasename(transfer.Filename), transferBuf.Bytes())
					if writeErr != nil {
						peer.logger.Error("error writing file to disk", "error", writeErr)
						peer.mgrCh <- PeerEvent{
							Type:     DownloadFailed,
							Username: peer.Username,
							Host:     peer.Host,
							Port:     peer.Port,
							Msg: DownloadFailedMsg{
								Username: peer.Username,
								Filename: transfer.Filename,
								Error:    fmt.Sprintf("disk write error: %v", writeErr),
							},
						}
						return
					}

					// Send download complete event
					peer.mgrCh <- PeerEvent{
						Type:     DownloadComplete,
						Username: peer.Username,
						Host:     peer.Host,
						Port:     peer.Port,
						Msg: DownloadCompleteMsg{
							Username: peer.Username,
							Filename: transfer.Filename,
						},
					}
					return
				}

				// EOF before receiving all bytes - incomplete transfer
				peer.logger.Warn("Incomplete file transfer (EOF)",
					"peer", peer.Username,
					"filename", transfer.Filename,
					"bytesReceived", transferBuf.Len(),
					"expectedSize", transfer.Size)

				peer.mgrCh <- PeerEvent{
					Type:     DownloadFailed,
					Username: peer.Username,
					Host:     peer.Host,
					Port:     peer.Port,
					Msg: DownloadFailedMsg{
						Username: peer.Username,
						Filename: transfer.Filename,
						Error:    fmt.Sprintf("Incomplete transfer: received %d/%d bytes", transferBuf.Len(), transfer.Size),
					},
				}
				return
			}
			peer.logger.Error("error reading file data", "error", err)

			// Send download failed event
			peer.mgrCh <- PeerEvent{
				Type:     DownloadFailed,
				Username: peer.Username,
				Host:     peer.Host,
				Port:     peer.Port,
				Msg: DownloadFailedMsg{
					Username: peer.Username,
					Filename: transfer.Filename,
					Error:    err.Error(),
				},
			}
			return
		}
		peer.logger.Debug("File transfer data received", "n", n, "token", token)

		// Reset read deadline after each successful chunk (30 seconds per chunk)
		// This keeps the connection alive during active transfer
		peer.Conn.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		// Write to transferBuf
		if _, err := transferBuf.Write(readBuffer[:n]); err != nil {
			peer.logger.Error("error writing to buffer", "error", err)

			// Send download failed event
			peer.mgrCh <- PeerEvent{
				Type:     DownloadFailed,
				Username: peer.Username,
				Host:     peer.Host,
				Port:     peer.Port,
				Msg: DownloadFailedMsg{
					Username: peer.Username,
					Filename: transfer.Filename,
					Error:    fmt.Sprintf("buffer write error: %v", err),
				},
			}
			return
		}

		transfer.Offset += uint64(n)

		// Send progress update
		peer.mgrCh <- PeerEvent{
			Type:     DownloadProgress,
			Username: peer.Username,
			Host:     peer.Host,
			Port:     peer.Port,
			Msg: DownloadProgressMsg{
				Username:      peer.Username,
				Filename:      transfer.Filename,
				BytesReceived: transfer.Offset,
			},
		}

		// Check if transfer is complete
		if uint64(transferBuf.Len()) >= transfer.Size {
			peer.logger.Info("File transfer complete", "filename", transfer.Filename, "token", token)
			err := writeToDownloadsDir(extractBasename(transfer.Filename), transferBuf.Bytes())
			if err != nil {
				peer.logger.Error("error writing file to disk", "error", err)

				// Send download failed event
				peer.mgrCh <- PeerEvent{
					Type:     DownloadFailed,
					Username: peer.Username,
					Host:     peer.Host,
					Port:     peer.Port,
					Msg: DownloadFailedMsg{
						Username: peer.Username,
						Filename: transfer.Filename,
						Error:    fmt.Sprintf("disk write error: %v", err),
					},
				}
				return
			}

			// Send download complete event
			peer.mgrCh <- PeerEvent{
				Type:     DownloadComplete,
				Username: peer.Username,
				Host:     peer.Host,
				Port:     peer.Port,
				Msg: DownloadCompleteMsg{
					Username: peer.Username,
					Filename: transfer.Filename,
				},
			}
			return
		}
	}
}

func (peer *FileTransferPeer) UploadFile(ft FileTransfer) {
	// Open the file - OS will automatically follow the symlink to the actual file
	peer.logger.Info("Uploading file", "peer", peer.Username, "filename", ft.Filename)
	start := time.Now()
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
	buffer := make([]byte, uploadChunkSize)

	// Calculate remaining bytes to send
	remainingBytes := ft.Size - ft.Offset
	bytesTransferred := uint64(0)

	// Send the file in chunks
	for remainingBytes > 0 {
		// Determine the size of the current chunk
		currentChunkSize := uploadChunkSize
		if remainingBytes < uint64(uploadChunkSize) {
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

		// Send progress update
		peer.mgrCh <- PeerEvent{
			Type:     UploadProgress,
			Username: peer.Username,
			Host:     peer.Host,
			Port:     peer.Port,
			Msg: UploadProgressMsg{
				Username:  peer.Username,
				Filename:  ft.Filename,
				BytesSent: bytesTransferred,
				Token:     ft.Token,
			},
		}
	}
	elapsed := time.Since(start)
	peer.logger.Info("Upload complete", "filename", ft.Filename)
	peer.mgrCh <- PeerEvent{
		Type:     UploadComplete,
		Username: peer.Username,
		Host:     peer.Host,
		Port:     peer.Port,
		Msg:      UploadCompleteMsg{Filename: ft.Filename, Token: ft.Token, TimeElapsed: elapsed},
	}
}

// extractBasename extracts the filename from a path that may use either
// Windows backslashes or Unix forward slashes, regardless of the current OS.
func extractBasename(fullPath string) string {
	normalized := strings.ReplaceAll(fullPath, "\\", "/")
	// Use path.Base which always works with forward slashes
	return path.Base(normalized)
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
