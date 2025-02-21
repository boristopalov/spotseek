package peer

import (
	"testing"

	"spotseek/slsk/messages"
	"spotseek/slsk/shared"

	"github.com/stretchr/testify/assert"
)

func TestHandleTransferRequest(t *testing.T) {
	tests := []struct {
		name      string
		direction uint32
		token     uint32
		filename  string
		filesize  uint64
		wantErr   bool
	}{
		{
			name:      "valid upload request",
			direction: 1,
			token:     12345,
			filename:  "test.mp3",
			filesize:  1024,
			wantErr:   false,
		},
		{
			name:      "invalid direction",
			direction: 2,
			token:     12345,
			filename:  "test.mp3",
			filesize:  1024,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mb := messages.NewMessageBuilder()
			mb.AddInt32(tt.direction)
			mb.AddInt32(tt.token)
			mb.AddString(tt.filename)
			if tt.direction == 1 {
				mb.AddInt64(tt.filesize)
			}

			peer := &Peer{
				Username: "testuser",
				mgrCh:    make(chan PeerEvent, 1),
			}
			mr := messages.NewMessageReader(mb.Message)

			decoded, err := peer.handleTransferRequest(mr)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, "TransferRequest", decoded["type"])
			result := decoded["result"].(map[string]interface{})
			assert.Equal(t, tt.direction, result["direction"])
			assert.Equal(t, tt.token, result["token"])
			assert.Equal(t, tt.filename, result["filename"])
			if tt.direction == 1 {
				assert.Equal(t, tt.filesize, result["filesize"])
			}
		})
	}
}

func TestTransferResponse(t *testing.T) {
	tests := []struct {
		name      string
		token     uint32
		allowed   bool
		filesize  uint64
		wantBytes []byte
	}{
		{
			name:     "allowed transfer",
			token:    12345,
			allowed:  true,
			filesize: 1024,
		},
		{
			name:     "denied transfer",
			token:    12345,
			allowed:  false,
			filesize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			peer := &Peer{
				Username:       "testuser",
				PeerConnection: &shared.Connection{},
			}

			msg := peer.TransferResponse(tt.token, tt.allowed, tt.filesize)
			reader := messages.NewMessageReader(msg[4:]) // Skip message length

			code := reader.ReadInt32()
			assert.Equal(t, uint32(41), code) // Transfer response code

			token := reader.ReadInt32()
			assert.Equal(t, tt.token, token)

			allowed := reader.ReadInt8()
			expectedAllowed := uint8(0)
			if tt.allowed {
				expectedAllowed = 1
			}
			assert.Equal(t, expectedAllowed, allowed)

			if tt.allowed {
				size := reader.ReadInt64()
				assert.Equal(t, tt.filesize, size)
			}
		})
	}
}

func TestHandleTransferResponse(t *testing.T) {
	tests := []struct {
		name     string
		token    uint32
		allowed  bool
		filesize uint64
		wantErr  bool
	}{
		{
			name:     "allowed transfer",
			token:    12345,
			allowed:  true,
			filesize: 1024,
			wantErr:  false,
		},
		{
			name:     "denied transfer",
			token:    12345,
			allowed:  false,
			filesize: 0,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mb := messages.NewMessageBuilder()
			mb.AddInt32(tt.token)
			if tt.allowed {
				mb.AddInt8(1)
				mb.AddInt64(tt.filesize)
			} else {
				mb.AddInt8(0)
			}

			peer := &Peer{
				Username: "testuser",
				mgrCh:    make(chan PeerEvent, 1),
			}

			mr := messages.NewMessageReader(mb.Message)
			decoded, err := peer.handleTransferResponse(mr)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, "TransferResponse", decoded["type"])
			result := decoded["result"].(map[string]interface{})
			assert.Equal(t, tt.token, result["token"])
			assert.Equal(t, tt.allowed, result["allowed"])
			if tt.allowed {
				assert.Equal(t, tt.filesize, result["filesize"])
			}
		})
	}
}
