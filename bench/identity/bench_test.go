package bench_test

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIdentityCompressionBenchmark(t *testing.T) {
	sockPath := os.ExpandEnv("$HOME/.engram/engram.sock")
	if _, err := os.Stat(sockPath); err != nil {
		t.Skip("engram daemon not running — start with: engram serve")
	}

	inputPath := filepath.Join("input", "syren_identity.txt")
	input, err := os.ReadFile(inputPath)
	require.NoError(t, err)

	conn, err := net.Dial("unix", sockPath)
	require.NoError(t, err)
	defer conn.Close()

	// Step 1: derive codebook from the identity text
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "engram.deriveCodebook",
		"params":  map[string]any{"content": string(input)},
	}
	reqBytes, _ := json.Marshal(req)
	_, err = conn.Write(append(reqBytes, '\n'))
	require.NoError(t, err)

	var resp map[string]any
	dec := json.NewDecoder(conn)
	require.NoError(t, dec.Decode(&resp))
	require.Nil(t, resp["error"], "deriveCodebook error: %v", resp["error"])

	result := resp["result"].(map[string]any)
	serialized := result["serialized"].(string)

	require.NotEmpty(t, serialized, "serialized output must not be empty")

	// Measure compression: prose input bytes vs compact key=value serialization
	inputBytes := len(input)
	outputBytes := len(serialized)

	t.Logf("Input bytes (prose identity): %d", inputBytes)
	t.Logf("Output bytes (codebook): %d", outputBytes)
	t.Logf("Dimension count: %d", len(strings.Split(serialized, " ")))

	if inputBytes > 0 {
		ratio := float64(inputBytes) / float64(outputBytes)
		t.Logf("Compression ratio: %.1fx", ratio)
		t.Logf("Savings: %.0f%%", (1-1/ratio)*100)
	}

	require.Greater(t, inputBytes, outputBytes,
		"codebook serialization must be smaller in bytes than the full prose input")

	// Write reference output
	os.MkdirAll("expected", 0755)
	os.WriteFile(filepath.Join("expected", "syren_identity_compressed.txt"), []byte(serialized), 0644)
}
