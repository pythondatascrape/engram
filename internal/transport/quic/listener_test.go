package quic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStreamType(t *testing.T) {
	tests := []struct {
		name    string
		byte    byte
		want    StreamType
		wantErr bool
	}{
		{
			name:    "StreamRequest",
			byte:    0x01,
			want:    StreamRequest,
			wantErr: false,
		},
		{
			name:    "StreamState",
			byte:    0x02,
			want:    StreamState,
			wantErr: false,
		},
		{
			name:    "StreamEvents",
			byte:    0x03,
			want:    StreamEvents,
			wantErr: false,
		},
		{
			name:    "StreamClose",
			byte:    0x04,
			want:    StreamClose,
			wantErr: false,
		},
		{
			name:    "InvalidByte",
			byte:    0xFF,
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStreamType(tt.byte)
			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, tt.want, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestStreamTypeString(t *testing.T) {
	tests := []struct {
		name string
		st   StreamType
		want string
	}{
		{
			name: "StreamRequest",
			st:   StreamRequest,
			want: "request",
		},
		{
			name: "StreamState",
			st:   StreamState,
			want: "state",
		},
		{
			name: "StreamEvents",
			st:   StreamEvents,
			want: "events",
		},
		{
			name: "StreamClose",
			st:   StreamClose,
			want: "close",
		},
		{
			name: "UnknownType",
			st:   StreamType(0xFF),
			want: "unknown(0xFF)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.st.String()
			assert.Equal(t, tt.want, got)
		})
	}
}
