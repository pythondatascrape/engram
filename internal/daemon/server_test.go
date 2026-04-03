package daemon

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dialAndSend(t *testing.T, sockPath string, req RPCRequest) RPCResponse {
	t.Helper()
	conn, err := net.DialTimeout("unix", sockPath, 2*time.Second)
	require.NoError(t, err)
	defer conn.Close()

	data, err := json.Marshal(req)
	require.NoError(t, err)
	data = append(data, '\n')
	_, err = conn.Write(data)
	require.NoError(t, err)

	buf := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buf)
	require.NoError(t, err)

	var resp RPCResponse
	require.NoError(t, json.Unmarshal(buf[:n], &resp))
	return resp
}

func TestServer_HealthMethod(t *testing.T) {
	sockPath := shortSock(t, "h.sock")
	l, err := NewListener(sockPath)
	require.NoError(t, err)

	srv := NewServer(l, nil)
	go srv.Serve()
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	resp := dialAndSend(t, sockPath, RPCRequest{
		JSONRPC: "2.0",
		Method:  "health",
		ID:      float64(1),
	})

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)
	assert.Equal(t, float64(1), resp.ID)

	result, err := json.Marshal(resp.Result)
	require.NoError(t, err)
	var health HealthResult
	require.NoError(t, json.Unmarshal(result, &health))
	assert.Equal(t, "ok", health.Status)
}

func TestServer_UnknownMethod(t *testing.T) {
	sockPath := shortSock(t, "u.sock")
	l, err := NewListener(sockPath)
	require.NoError(t, err)

	srv := NewServer(l, nil)
	go srv.Serve()
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	resp := dialAndSend(t, sockPath, RPCRequest{
		JSONRPC: "2.0",
		Method:  "nonexistent",
		ID:      float64(2),
	})

	assert.Equal(t, "2.0", resp.JSONRPC)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32601, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "nonexistent")
}

func TestServer_CompressWithoutHandler(t *testing.T) {
	sockPath := shortSock(t, "c.sock")
	l, err := NewListener(sockPath)
	require.NoError(t, err)

	srv := NewServer(l, nil)
	go srv.Serve()
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	resp := dialAndSend(t, sockPath, RPCRequest{
		JSONRPC: "2.0",
		Method:  "compress",
		Params:  mustMarshal(map[string]interface{}{"query": "hello"}),
		ID:      float64(3),
	})

	assert.Equal(t, "2.0", resp.JSONRPC)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32603, resp.Error.Code)
}
