// internal/proxy/handler.go
package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/pythondatascrape/engram/internal/smc"
)

// engramSessionHeader is the request header used to pass the session ID from
// Claude Code to the proxy. Stripped before forwarding to Anthropic.
const engramSessionHeader = "X-Engram-Session"

// isPlaceholder reports whether s looks like an unresolved shell template variable (e.g. "${session_id}").
func isPlaceholder(s string) bool {
	return len(s) > 3 && s[0] == '$' && s[1] == '{' && s[len(s)-1] == '}'
}

// Handler implements http.Handler for the Anthropic-compatible proxy.
type Handler struct {
	windowSize  int
	sessionsDir string
	upstream    string // e.g. "https://api.anthropic.com"
	client      *http.Client
	// afterStats is called after each WriteStats completes. Used in tests to
	// avoid time.Sleep races; nil in production.
	afterStats func()

	// pendingSession holds the Claude session UUID registered by the sessionstart
	// hook before the first /v1/messages request. First-request-claims: the next
	// intercepted /v1/messages call atomically reads and clears it. For concurrent
	// sessions (rare on single-user local software), the last registration wins the
	// store but the first /v1/messages request wins the claim — whichever session
	// gets its request in first will use the registered UUID.
	pendingMu      sync.Mutex
	pendingSession string

	// SMC fields — when smcEnabled is true, use matrix decomposition instead of
	// windowed compression.
	smcEnabled    bool
	smcSchema     smc.CategorySchema
	smcK          smc.KController
	smcDecomposer smc.Decomposer
	smcMatrices   map[string]*smc.ConversationMatrix // sessionID -> matrix
	smcMu         sync.Mutex
}

// NewHandler creates a new Handler.
func NewHandler(windowSize int, sessionsDir, upstream string) *Handler {
	return &Handler{
		windowSize:  windowSize,
		sessionsDir: sessionsDir,
		upstream:    upstream,
		client:      &http.Client{},
	}
}

// registerSession stores id as the pending session ID for the next /v1/messages
// request. Returns true if stored, false if id is empty or a placeholder.
func (h *Handler) registerSession(id string) bool {
	if id == "" || isPlaceholder(id) {
		return false
	}
	h.pendingMu.Lock()
	defer h.pendingMu.Unlock()
	h.pendingSession = id
	slog.Debug("proxy: registered session", "session_id", id)
	return true
}

// claimPendingSession atomically reads and clears the pending session ID.
// Returns "" if no session has been registered.
func (h *Handler) claimPendingSession() string {
	h.pendingMu.Lock()
	defer h.pendingMu.Unlock()
	id := h.pendingSession
	h.pendingSession = ""
	return id
}

// anthropicRequest is the subset of the Anthropic messages request we care about.
type anthropicRequest struct {
	Messages  []AnthropicMessage `json:"messages"`
	RawSystem json.RawMessage    `json:"system"`
	System    string             `json:"-"` // extracted text from RawSystem
	Stream    bool               `json:"stream"`
}

// extractSystem populates the System string field from RawSystem, handling
// both the plain string form and the content-block array form used by Claude Code:
//
//	"system": "text"
//	"system": [{"type":"text","text":"..."},...]
func (r *anthropicRequest) extractSystem() {
	if r.RawSystem == nil {
		return
	}
	// Try plain string first.
	var s string
	if json.Unmarshal(r.RawSystem, &s) == nil {
		r.System = s
		return
	}
	// Try array of content blocks.
	var blocks []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(r.RawSystem, &blocks) == nil {
		var parts []string
		for _, b := range blocks {
			if b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		r.System = strings.Join(parts, "\n")
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle session registration from the sessionstart hook.
	if r.URL.Path == "/internal/register-session" && r.Method == http.MethodPost {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}
		var body struct {
			SessionID string `json:"session_id"`
		}
		if json.Unmarshal(raw, &body) != nil {
			http.Error(w, "invalid session_id", http.StatusBadRequest)
			return
		}
		if !h.registerSession(body.SessionID) {
			http.Error(w, "invalid session_id", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	// Only intercept POST /v1/messages.
	if r.URL.Path != "/v1/messages" || r.Method != http.MethodPost {
		h.forwardVerbatim(w, r, nil)
		return
	}

	// Read the body once.
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadGateway)
		return
	}

	// Try to parse.
	var req anthropicRequest
	if err := json.Unmarshal(rawBody, &req); err != nil {
		// Fail-open: forward original body verbatim.
		h.forwardVerbatim(w, r, rawBody)
		return
	}
	req.extractSystem()

	// Determine session ID. Prefer the UUID registered by the sessionstart hook
	// (via /internal/register-session). Fall back to the system-prompt fingerprint
	// when no session was registered (e.g. proxy running without the hook).
	sessionID := r.Header.Get(engramSessionHeader)
	if sessionID == "" || isPlaceholder(sessionID) {
		if pending := h.claimPendingSession(); pending != "" {
			sessionID = pending
		} else {
			slog.Warn("proxy: no registered session, falling back to fingerprint ID",
				"fingerprint", SessionID(req.System))
			sessionID = SessionID(req.System)
		}
	}

	// Measure before and after compression.
	// Include the system prompt in token estimates — it's the largest payload
	// component but is not compressed by the proxy.
	systemTokens := len(req.System) / 4
	ctxOrig := EstimateTokens(req.Messages) + systemTokens
	if h.smcEnabled {
		req.Messages = h.smcCompress(sessionID, req.Messages)
	} else {
		req.Messages = Compress(req.Messages, h.windowSize)
	}
	ctxComp := EstimateTokens(req.Messages) + systemTokens

	// Re-encode.
	newBody, err := json.Marshal(req)
	if err != nil {
		h.forwardVerbatim(w, r, rawBody)
		return
	}

	h.forwardWithBody(w, r, newBody)

	// Write stats asynchronously after response.
	go func() {
		_ = WriteStats(h.sessionsDir, sessionID, ctxOrig, ctxComp)
		if h.afterStats != nil {
			h.afterStats()
		}
	}()
}

// forwardVerbatim forwards the request with the given body (or the original body if nil).
func (h *Handler) forwardVerbatim(w http.ResponseWriter, r *http.Request, body []byte) {
	if body == nil {
		var err error
		body, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadGateway)
			return
		}
	}
	h.forwardWithBody(w, r, body)
}

// forwardWithBody sends the request upstream with the provided body and streams the response back.
func (h *Handler) forwardWithBody(w http.ResponseWriter, r *http.Request, body []byte) {
	upstreamURL := h.upstream + r.URL.RequestURI()
	upstreamReq, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "failed to create upstream request", http.StatusBadGateway)
		return
	}

	// Clone headers, then drop internal/computed ones before forwarding.
	// X-Engram-Session is an internal routing header not meant for Anthropic.
	// Content-Length is set explicitly from the (possibly rewritten) body.
	upstreamReq.Header = r.Header.Clone()
	upstreamReq.Header.Del("Content-Length")
	upstreamReq.Header.Del(engramSessionHeader)
	upstreamReq.ContentLength = int64(len(body))

	resp, err := h.client.Do(upstreamReq)
	if err != nil {
		http.Error(w, "upstream request failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers.
	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Stream response body, flushing as we go (handles SSE and regular JSON).
	flusher, canFlush := w.(http.Flusher)
	buf := make([]byte, 4096)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			_, _ = w.Write(buf[:n])
			if canFlush {
				flusher.Flush()
			}
		}
		if readErr != nil {
			break
		}
	}
}

// EnableSMC activates structured matrix compression, replacing windowed compression.
func (h *Handler) EnableSMC(schema smc.CategorySchema, k smc.KController) {
	h.smcEnabled = true
	h.smcSchema = schema
	h.smcK = k
	h.smcDecomposer = smc.NewRuleDecomposer(k)
	h.smcMatrices = make(map[string]*smc.ConversationMatrix)
}

// getOrCreateMatrix returns the conversation matrix for a session, creating one if needed.
func (h *Handler) getOrCreateMatrix(sessionID string) *smc.ConversationMatrix {
	h.smcMu.Lock()
	defer h.smcMu.Unlock()
	m, ok := h.smcMatrices[sessionID]
	if !ok {
		m = smc.NewConversationMatrix(sessionID, h.smcSchema, h.smcK)
		h.smcMatrices[sessionID] = m
	}
	return m
}

// smcCompress decomposes older messages into the matrix and returns
// the matrix history + recent raw messages.
func (h *Handler) smcCompress(sessionID string, messages []AnthropicMessage) []AnthropicMessage {
	if len(messages) <= 2 {
		return messages
	}

	matrix := h.getOrCreateMatrix(sessionID)

	// Decompose all completed exchanges (pairs) except the last pair.
	alreadyDecomposed := matrix.Len()
	pairs := len(messages) / 2
	currentTail := messages[pairs*2:]

	for i := alreadyDecomposed; i < pairs; i++ {
		userIdx := i * 2
		assistIdx := i*2 + 1
		if assistIdx >= len(messages) {
			break
		}
		exchange := smc.Exchange{
			UserMessage:      messageText(messages[userIdx]),
			AssistantMessage: messageText(messages[assistIdx]),
			TurnIndex:        i,
		}
		row, err := h.smcDecomposer.Decompose(context.Background(), exchange, h.smcSchema)
		if err != nil {
			slog.Warn("smc: decomposition failed, keeping raw", "turn", i, "err", err)
			continue
		}
		matrix.Append(*row)
	}

	// Build output: matrix history messages + current raw tail
	matrixMsgs := matrix.Messages()
	result := make([]AnthropicMessage, 0, len(matrixMsgs)+len(currentTail)+2)

	for _, m := range matrixMsgs {
		result = append(result, AnthropicMessage{Role: m.Role, Content: m.Content})
	}
	if len(result) > 0 {
		result = append(result, AnthropicMessage{Role: "assistant", Content: "[compressed history above]"})
	}

	// Append the current raw tail
	if pairs > 0 && alreadyDecomposed < pairs {
		lastPairStart := (pairs - 1) * 2
		result = append(result, messages[lastPairStart:]...)
	} else {
		result = append(result, currentTail...)
	}

	return result
}
