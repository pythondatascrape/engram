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
// engram.deriveCodebook request followed by one engram.compressIdentity request.
// It captures both param payloads and returns a response with "serialized"
// set to the provided returnVal.
func startMockDaemon(t *testing.T, returnVal string) (sockPath string, gotParams func() []map[string]interface{}) {
	t.Helper()
	dir, err := os.MkdirTemp(".", "oc-")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })

	sockPath = filepath.Join(dir, "engram.sock")
	ln, err := net.Listen("unix", sockPath)
	require.NoError(t, err)

	paramsCh := make(chan []map[string]interface{}, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		defer ln.Close()

		scanner := bufio.NewScanner(conn)
		encoder := json.NewEncoder(conn)

		var seen []map[string]interface{}
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
			seen = append(seen, req.Params)
			switch req.Method {
			case "engram.deriveCodebook":
				resp := map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      req.ID,
					"result": map[string]interface{}{
						"codebook": map[string]string{
							"name": "Alice",
							"role": "engineer",
						},
					},
				}
				_ = encoder.Encode(resp)
			case "engram.compressIdentity":
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
				paramsCh <- seen
				return
			}
		}
	}()

	gotParams = func() []map[string]interface{} {
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

	params := gotParams()
	require.Len(t, params, 2, "expected derive + compress requests")

	deriveParams := params[0]
	require.Equal(t, "name=Alice role=engineer", deriveParams["content"])

	// Verify the second request sends dimensions to compressIdentity.
	compressParams := params[1]
	dims, ok := compressParams["dimensions"]
	require.True(t, ok, "compress params should have 'dimensions' key, got: %v", compressParams)
	_, hasContent := compressParams["content"]
	require.False(t, hasContent, "compress params must NOT have 'content' key")

	// dimensions should be a map
	dimsMap, ok := dims.(map[string]interface{})
	require.True(t, ok, "dimensions should be a map, got %T", dims)
	require.Equal(t, "Alice", dimsMap["name"])
	require.Equal(t, "engineer", dimsMap["role"])
}

func TestCompressContext_ProseContentGoesThroughDeriveFirst(t *testing.T) {
	const wantSerialized = "name=Alice role=engineer"
	sockPath, gotParams := startMockDaemon(t, wantSerialized)

	a := &Adapter{
		socketPath: sockPath,
		nextID:     1,
	}
	ctx := t.Context()
	require.NoError(t, a.Connect(ctx))
	defer a.Close()

	content := "I am Alice and I am an engineer who prefers concise answers."
	got, err := a.CompressContext(ctx, content)
	require.NoError(t, err)
	require.Equal(t, wantSerialized, got)

	params := gotParams()
	require.Len(t, params, 2)
	require.Equal(t, content, params[0]["content"])
	_, ok := params[1]["originalTokens"]
	require.True(t, ok, "compressIdentity should receive originalTokens for prose baseline")
}
