package quic

import (
	"fmt"
)

// StreamType represents the type of a QUIC stream.
type StreamType byte

// QUIC stream type constants.
const (
	StreamRequest StreamType = 0x01
	StreamState   StreamType = 0x02
	StreamEvents  StreamType = 0x03
	StreamClose   StreamType = 0x04
)

// ParseStreamType parses a byte into a StreamType.
// Returns an error if the byte does not correspond to a valid stream type.
func ParseStreamType(b byte) (StreamType, error) {
	st := StreamType(b)
	switch st {
	case StreamRequest, StreamState, StreamEvents, StreamClose:
		return st, nil
	default:
		return 0, fmt.Errorf("unknown stream type: 0x%02X", b)
	}
}

// String returns the string representation of the StreamType.
func (s StreamType) String() string {
	switch s {
	case StreamRequest:
		return "request"
	case StreamState:
		return "state"
	case StreamEvents:
		return "events"
	case StreamClose:
		return "close"
	default:
		return fmt.Sprintf("unknown(0x%02X)", byte(s))
	}
}
