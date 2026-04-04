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

func TestCheckRedundancyEndToEnd(t *testing.T) {
	sockPath := shortSock(t, "red.sock")
	l, err := NewListener(sockPath)
	require.NoError(t, err)

	srv := NewServer(l, nil)
	go srv.Serve()
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	params1 := mustMarshal(map[string]interface{}{"content": "lang=go arch=monolith"})

	// First call: not redundant
	resp1 := dialAndSend(t, sockPath, RPCRequest{
		JSONRPC: "2.0",
		Method:  "engram.checkRedundancy",
		Params:  params1,
		ID:      float64(20),
	})
	require.Nil(t, resp1.Error)
	var result1 map[string]interface{}
	require.NoError(t, json.Unmarshal(resp1.Result, &result1))
	assert.Equal(t, false, result1["isRedundant"])

	// Second identical call: redundant (exact)
	resp2 := dialAndSend(t, sockPath, RPCRequest{
		JSONRPC: "2.0",
		Method:  "engram.checkRedundancy",
		Params:  params1,
		ID:      float64(21),
	})
	require.Nil(t, resp2.Error)
	var result2 map[string]interface{}
	require.NoError(t, json.Unmarshal(resp2.Result, &result2))
	assert.Equal(t, true, result2["isRedundant"])
	assert.Equal(t, "exact", result2["kind"])
}

func TestCompressAlias(t *testing.T) {
	sockPath := shortSock(t, "alias.sock")
	l, err := NewListener(sockPath)
	require.NoError(t, err)

	srv := NewServer(l, nil)
	go srv.Serve()
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	params := mustMarshal(map[string]interface{}{
		"client_id": "test-client",
		"api_key":   "test-key",
		"query":     "hello",
	})

	// "compress" should return handler-not-configured error (not method-not-found)
	resp1 := dialAndSend(t, sockPath, RPCRequest{
		JSONRPC: "2.0",
		Method:  "compress",
		Params:  params,
		ID:      float64(10),
	})
	require.NotNil(t, resp1.Error)
	assert.Equal(t, -32603, resp1.Error.Code, "compress: expected handler-not-configured, not method-not-found")

	// "engram.compress" must route to the same handler — same error code, not -32601
	resp2 := dialAndSend(t, sockPath, RPCRequest{
		JSONRPC: "2.0",
		Method:  "engram.compress",
		Params:  params,
		ID:      float64(11),
	})
	require.NotNil(t, resp2.Error)
	assert.Equal(t, -32603, resp2.Error.Code, "engram.compress: expected handler-not-configured, not method-not-found")
}

func TestGenerateReportHasRealData(t *testing.T) {
	sockPath := shortSock(t, "rep.sock")
	l, err := NewListener(sockPath)
	require.NoError(t, err)

	srv := NewServer(l, nil)
	go srv.Serve()
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Make 2 compressIdentity calls to build up stats.
	dims := mustMarshal(map[string]interface{}{
		"dimensions": map[string]string{
			"lang": "go",
			"arch": "modular_monolith",
			"db":   "postgresql",
		},
	})
	for i := 0; i < 2; i++ {
		resp := dialAndSend(t, sockPath, RPCRequest{
			JSONRPC: "2.0",
			Method:  "engram.compressIdentity",
			Params:  dims,
			ID:      float64(100 + i),
		})
		require.Nil(t, resp.Error, "compressIdentity call %d failed", i)
	}

	// Call generateReport.
	resp := dialAndSend(t, sockPath, RPCRequest{
		JSONRPC: "2.0",
		Method:  "engram.generateReport",
		Params:  mustMarshal(map[string]interface{}{}),
		ID:      float64(200),
	})
	require.Nil(t, resp.Error)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(resp.Result, &result))

	// Must not be a placeholder.
	if msg, ok := result["message"].(string); ok {
		assert.NotContains(t, msg, "not fully implemented")
	}
	if report, ok := result["report"].(string); ok {
		assert.NotContains(t, report, "not yet fully implemented")
	}

	// compressionEvents must be >= 2.
	ce, ok := result["compressionEvents"].(float64)
	require.True(t, ok, "compressionEvents must be numeric, got %T", result["compressionEvents"])
	assert.GreaterOrEqual(t, ce, float64(2))

	// tokensBefore must be > 0.
	tb, ok := result["tokensBefore"].(float64)
	require.True(t, ok, "tokensBefore must be numeric")
	assert.Greater(t, tb, float64(0))

	// estimatedSavingsPct must be > 0.
	pct, ok := result["estimatedSavingsPct"].(float64)
	require.True(t, ok, "estimatedSavingsPct must be numeric")
	assert.Greater(t, pct, float64(0))

	// markdown must mention the compression event count.
	md, ok := result["markdown"].(string)
	require.True(t, ok, "markdown must be a string")
	assert.Contains(t, md, "2")
}

func TestDeriveCodebookRealImplementation(t *testing.T) {
	sockPath := shortSock(t, "dcb.sock")
	l, err := NewListener(sockPath)
	require.NoError(t, err)

	srv := NewServer(l, nil)
	go srv.Serve()
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	params := mustMarshal(map[string]interface{}{
		"content": "lang=go arch=modular_monolith db=postgresql",
	})

	resp := dialAndSend(t, sockPath, RPCRequest{
		JSONRPC: "2.0",
		Method:  "engram.deriveCodebook",
		Params:  params,
		ID:      float64(30),
	})

	require.Nil(t, resp.Error)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(resp.Result, &result))

	assert.Equal(t, "derived", result["status"])
	assert.NotContains(t, result, "message", "stub message should not be present")

	serialized, ok := result["serialized"].(string)
	require.True(t, ok, "serialized should be a string")
	assert.Contains(t, serialized, "lang=go")

	codebook, ok := result["codebook"].(map[string]interface{})
	require.True(t, ok, "codebook should be a map")
	assert.Equal(t, "go", codebook["lang"])
}
