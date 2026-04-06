// internal/proxy/handler.go
package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync"
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
	Messages []AnthropicMessage `json:"messages"`
	System   string             `json:"system"`
	Stream   bool               `json:"stream"`
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
	ctxOrig := EstimateTokens(req.Messages)
	req.Messages = Compress(req.Messages, h.windowSize)
	ctxComp := EstimateTokens(req.Messages)

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
