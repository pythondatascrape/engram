package openclaw

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// startMockDaemon starts a mock Unix socket server that handles one
// engram.compressIdentity request. It captures the params it receives and
// returns a response with "serialized" set to the provided returnVal.
func startMockDaemon(t *testing.T, returnVal string) (sockPath string, gotParams func() map[string]interface{}) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "oc-")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })

	sockPath = filepath.Join(dir, "engram.sock")
	ln, err := net.Listen("unix", sockPath)
	require.NoError(t, err)

	paramsCh := make(chan map[string]interface{}, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		defer ln.Close()

		scanner := bufio.NewScanner(conn)
		encoder := json.NewEncoder(conn)

		for scanner.Scan() {
			var req struct {
				JSONRPC string                 `json:"jsonrpc"`
				ID      interface{}            `json:"id"`
				Method  string                 `json:"method"`
				Params  map[string]interface{} `json:"params"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
				continue
			}
			paramsCh <- req.Params
			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]interface{}{
					"serialized": returnVal,
					"stats": map[string]int{
						"originalTokens":   10,
						"compressedTokens": 5,
						"saved":            5,
					},
				},
			}
			_ = encoder.Encode(resp)
			return
		}
	}()

	gotParams = func() map[string]interface{} {
		select {
		case p := <-paramsCh:
			return p
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for params")
			return nil
		}
	}
	return sockPath, gotParams
}

func TestCompressContext_SendsDimensionsAndReadsSerializedField(t *testing.T) {
	const wantSerialized = "name=Alice role=engineer"
	sockPath, gotParams := startMockDaemon(t, wantSerialized)

	a := &Adapter{
		socketPath: sockPath,
		nextID:     1,
	}
	ctx := t.Context()
	require.NoError(t, a.Connect(ctx))
	defer a.Close()

	got, err := a.CompressContext(ctx, "name=Alice role=engineer")
	require.NoError(t, err)

	// Verify the returned value equals what the daemon put in "serialized".
	require.Equal(t, wantSerialized, got)

	// Verify the params sent had "dimensions" (a map), not "content".
	params := gotParams()
	dims, ok := params["dimensions"]
	require.True(t, ok, "params should have 'dimensions' key, got: %v", params)
	_, hasContent := params["content"]
	require.False(t, hasContent, "params must NOT have 'content' key")

	// dimensions should be a map
	dimsMap, ok := dims.(map[string]interface{})
	require.True(t, ok, "dimensions should be a map, got %T", dims)
	require.Equal(t, "Alice", dimsMap["name"])
	require.Equal(t, "engineer", dimsMap["role"])
}
