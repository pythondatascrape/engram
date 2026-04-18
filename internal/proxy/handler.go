// internal/proxy/handler.go
package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/pythondatascrape/engram/internal/identity/codebook"
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
	windowSize     int
	sessionsDir    string
	upstream       string // e.g. "https://api.anthropic.com"
	openaiUpstream string // e.g. "https://api.openai.com"
	client         *http.Client
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
func NewHandler(windowSize int, sessionsDir, upstream, openaiUpstream string) *Handler {
	return &Handler{
		windowSize:     windowSize,
		sessionsDir:    sessionsDir,
		upstream:       upstream,
		openaiUpstream: openaiUpstream,
		client:         &http.Client{},
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

// claimPendingSession atomically reads the pending session ID.
// Returns "" if no session has been registered.
func (h *Handler) claimPendingSession() string {
	h.pendingMu.Lock()
	defer h.pendingMu.Unlock()
	id := h.pendingSession
	return id
}

// anthropicRequest is the subset of the Anthropic messages request we care about.
type anthropicRequest struct {
	Model     string             `json:"model"`
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

// countTokens calls the Anthropic /v1/messages/count_tokens endpoint to get an
// exact input token count for the given request body. Returns -1 on any error
// so callers can fall back to estimation.
func (h *Handler) countTokens(rawBody []byte, srcHeaders http.Header) int {
	url := h.upstream + "/v1/messages/count_tokens"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(rawBody))
	if err != nil {
		slog.Debug("proxy: countTokens request creation failed", "err", err)
		return -1
	}
	// Clone auth and version headers from the original request.
	for _, key := range []string{"Authorization", "X-Api-Key", "Anthropic-Version", "Content-Type"} {
		if v := srcHeaders.Get(key); v != "" {
			req.Header.Set(key, v)
		}
	}
	req.ContentLength = int64(len(rawBody))

	resp, err := h.client.Do(req)
	if err != nil {
		slog.Debug("proxy: countTokens request failed", "err", err)
		return -1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Debug("proxy: countTokens non-200", "status", resp.StatusCode)
		return -1
	}

	var result struct {
		InputTokens int `json:"input_tokens"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		slog.Debug("proxy: countTokens decode failed", "err", err)
		return -1
	}
	return result.InputTokens
}

// parseUsageInputTokens extracts usage.input_tokens from a captured response
// body. Handles both non-streaming JSON responses and SSE streaming responses
// (where the final message_delta event contains usage). Returns -1 on failure.
func parseUsageInputTokens(body []byte) int {
	// Try non-streaming JSON first.
	var resp struct {
		Usage struct {
			InputTokens int `json:"input_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(body, &resp) == nil && resp.Usage.InputTokens > 0 {
		return resp.Usage.InputTokens
	}

	// Try SSE: scan for the message_start event which contains the full usage.
	// Format: "data: {... "usage":{"input_tokens":N,...} ...}"
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		var event struct {
			Type    string `json:"type"`
			Message struct {
				Usage struct {
					InputTokens int `json:"input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if json.Unmarshal([]byte(data), &event) == nil && event.Type == "message_start" && event.Message.Usage.InputTokens > 0 {
			return event.Message.Usage.InputTokens
		}
	}
	return -1
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
		h.forwardVerbatim(w, r, nil, h.upstream)
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
		h.forwardVerbatim(w, r, rawBody, h.upstream)
		return
	}
	req.extractSystem()

	slog.Info("proxy: intercepted /v1/messages",
		"num_messages", len(req.Messages),
		"system_len", len(req.System),
		"raw_system_len", len(req.RawSystem),
		"model", req.Model,
		"body_len", len(rawBody))

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

	// Get exact uncompressed token count from the count_tokens API.
	// This runs before compression so it measures the original payload.
	ctxOrig := h.countTokens(rawBody, r.Header)
	if ctxOrig < 0 {
		// Fallback to estimation if count_tokens is unavailable.
		systemTokens := len(req.System) / 4
		ctxOrig = EstimateTokens(req.Messages) + systemTokens
		slog.Debug("proxy: countTokens unavailable, using estimate",
			"session", sessionID, "estimate", ctxOrig)
	} else {
		slog.Debug("proxy: exact token count (uncompressed)",
			"session", sessionID, "input_tokens", ctxOrig)
	}

	windowBefore := EstimateTokens(req.Messages)
	req.Messages = Compress(req.Messages, h.windowSize)
	windowSaved := windowBefore - EstimateTokens(req.Messages)

	const contextBudget = 24000
	budgetSaved := 0
	if EstimateTokens(req.Messages) > contextBudget {
		budgetBefore := EstimateTokens(req.Messages)
		req.Messages = CompressBudget(req.Messages, contextBudget)
		budgetSaved = budgetBefore - EstimateTokens(req.Messages)
	}

	// Re-encode by patching only the "messages" field in the original body.
	// This preserves all fields (max_tokens, tools, temperature, etc.) that
	// anthropicRequest does not model.
	compressedMsgs, err := json.Marshal(req.Messages)
	if err != nil {
		h.forwardVerbatim(w, r, rawBody, h.upstream)
		return
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &raw); err != nil {
		h.forwardVerbatim(w, r, rawBody, h.upstream)
		return
	}
	raw["messages"] = compressedMsgs
	forwardedSystem := req.System
	systemSaved := 0
	if req.System != "" {
		if compressed, ok := codebook.CompressIfSafe(req.System); ok {
			systemSaved = len(req.System)/4 - len(compressed)/4
			b, _ := json.Marshal(compressed)
			raw["system"] = b
			forwardedSystem = compressed
		}
	}
	if windowSaved > 0 || budgetSaved > 0 || systemSaved > 0 {
		slog.Info("proxy: compression savings",
			"session", sessionID,
			"window_saved_tokens", windowSaved,
			"budget_saved_tokens", budgetSaved,
			"system_saved_tokens", systemSaved,
			"total_saved_tokens", windowSaved+budgetSaved+systemSaved,
		)
	}
	newBody, err := json.Marshal(raw)
	if err != nil {
		h.forwardVerbatim(w, r, rawBody, h.upstream)
		return
	}

	respBody := h.forwardWithBody(w, r, newBody, h.upstream)

	// Extract exact compressed token count from the API response usage field.
	ctxComp := parseUsageInputTokens(respBody)
	if ctxComp < 0 {
		// Fallback to estimation.
		systemTokens := len(forwardedSystem) / 4
		ctxComp = EstimateTokens(req.Messages) + systemTokens
		slog.Debug("proxy: response usage unavailable, using estimate",
			"session", sessionID, "estimate", ctxComp)
	} else {
		slog.Debug("proxy: exact token count (compressed)",
			"session", sessionID, "input_tokens", ctxComp)
	}

	// Write stats asynchronously after response.
	go func() {
		_ = WriteStats(h.sessionsDir, sessionID, ctxOrig, ctxComp)
		if h.afterStats != nil {
			h.afterStats()
		}
	}()
}

// forwardVerbatim forwards the request with the given body (or the original body if nil).
func (h *Handler) forwardVerbatim(w http.ResponseWriter, r *http.Request, body []byte, upstream string) {
	if body == nil {
		var err error
		body, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadGateway)
			return
		}
	}
	h.forwardWithBody(w, r, body, upstream)
}

// forwardWithBody sends the request upstream with the provided body and streams
// the response back. Returns the captured response body for usage extraction.
func (h *Handler) forwardWithBody(w http.ResponseWriter, r *http.Request, body []byte, upstream string) []byte {
	upstreamURL := upstream + r.URL.RequestURI()
	upstreamReq, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "failed to create upstream request", http.StatusBadGateway)
		return nil
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
		return nil
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
	// Capture a copy for usage extraction.
	var captured bytes.Buffer
	flusher, canFlush := w.(http.Flusher)
	buf := make([]byte, 4096)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			_, _ = w.Write(buf[:n])
			captured.Write(buf[:n])
			if canFlush {
				flusher.Flush()
			}
		}
		if readErr != nil {
			break
		}
	}
	return captured.Bytes()
}

// ServeOpenAI handles all requests arriving on the OpenAI-compatible listener.
// POST /v1/chat/completions is compressed and forwarded to openaiUpstream;
// everything else is forwarded verbatim.
func (h *Handler) ServeOpenAI(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.URL.Path == "/v1/chat/completions" {
		rawBody, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadGateway)
			return
		}
		var req openaiRequest
		if err := json.Unmarshal(rawBody, &req); err != nil {
			h.forwardVerbatim(w, r, rawBody, h.openaiUpstream)
			return
		}
		h.handleOpenAI(w, r, rawBody, req)
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/v1/responses" {
		rawBody, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadGateway)
			return
		}
		var req openaiResponsesRequest
		if err := json.Unmarshal(rawBody, &req); err != nil {
			h.forwardVerbatim(w, r, rawBody, h.openaiUpstream)
			return
		}
		h.handleOpenAIResponses(w, r, rawBody, req)
		return
	}

	h.forwardVerbatim(w, r, nil, h.openaiUpstream)
}

func (h *Handler) handleOpenAI(w http.ResponseWriter, r *http.Request, rawBody []byte, req openaiRequest) {
	sessionID := r.Header.Get(engramSessionHeader)
	if sessionID == "" || isPlaceholder(sessionID) {
		if pending := h.claimPendingSession(); pending != "" {
			sessionID = pending
		} else {
			sessionID = fallbackOpenAISessionID(rawBody)
		}
	}

	slog.Info("proxy: intercepted /v1/chat/completions",
		"num_messages", len(req.Messages),
		"session_id", sessionID,
	)

	ctxOrig := EstimateTokens(req.Messages)
	if ctxOrig == 0 {
		ctxOrig = estimateOpenAITokens(rawBody)
	}

	identitySaved := 0
	req.Messages, identitySaved = compressOpenAIIdentityMessages(req.Messages)

	windowBefore := EstimateTokens(req.Messages)
	req.Messages = Compress(req.Messages, h.windowSize)
	windowSaved := windowBefore - EstimateTokens(req.Messages)

	const contextBudget = 24000
	budgetSaved := 0
	if EstimateTokens(req.Messages) > contextBudget {
		budgetBefore := EstimateTokens(req.Messages)
		req.Messages = CompressBudget(req.Messages, contextBudget)
		budgetSaved = budgetBefore - EstimateTokens(req.Messages)
	}

	if windowSaved > 0 || budgetSaved > 0 || identitySaved > 0 {
		slog.Info("proxy: openai compression savings",
			"session", sessionID,
			"identity_saved_tokens", identitySaved,
			"window_saved_tokens", windowSaved,
			"budget_saved_tokens", budgetSaved,
		)
	}

	// Patch only the "messages" field to preserve model, tools, stream, etc.
	compressedMsgs, err := json.Marshal(req.Messages)
	if err != nil {
		h.forwardVerbatim(w, r, rawBody, h.openaiUpstream)
		return
	}
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &rawMap); err != nil {
		h.forwardVerbatim(w, r, rawBody, h.openaiUpstream)
		return
	}
	rawMap["messages"] = compressedMsgs
	newBody, err := json.Marshal(rawMap)
	if err != nil {
		h.forwardVerbatim(w, r, rawBody, h.openaiUpstream)
		return
	}

	respBody := h.forwardWithBody(w, r, newBody, h.openaiUpstream)

	go func() {
		ctxComp := parseOpenAIUsage(respBody)
		if ctxComp < 0 {
			ctxComp = EstimateTokens(req.Messages)
		}
		_ = WriteStats(h.sessionsDir, sessionID, ctxOrig, ctxComp)
		if h.afterStats != nil {
			h.afterStats()
		}
	}()
}

func (h *Handler) handleOpenAIResponses(w http.ResponseWriter, r *http.Request, rawBody []byte, req openaiResponsesRequest) {
	sessionID := r.Header.Get(engramSessionHeader)
	if sessionID == "" || isPlaceholder(sessionID) {
		if pending := h.claimPendingSession(); pending != "" {
			sessionID = pending
		} else {
			sessionID = fallbackOpenAISessionID(rawBody)
		}
	}

	compressedInstructions, instructionSaved := compressOpenAIInstructions(req.Instructions)
	forwardedInput, inputMessages, compressedMessages, inputCompressible := compressOpenAIInput(req.Input, h.windowSize, 24000)
	if compressedMessages == nil {
		compressedMessages = inputMessages
	}

	slog.Info("proxy: intercepted /v1/responses",
		"num_messages", len(inputMessages),
		"session_id", sessionID,
	)

	ctxOrig := EstimateTokens(inputMessages) + estimateInstructionsTokens(req.Instructions)
	if ctxOrig == 0 {
		ctxOrig = estimateOpenAITokens(rawBody)
	}

	windowBefore := EstimateTokens(inputMessages)
	windowSaved := 0
	budgetSaved := 0
	if inputCompressible {
		windowCompressed := Compress(inputMessages, h.windowSize)
		windowSaved = windowBefore - EstimateTokens(windowCompressed)
		budgetSaved = EstimateTokens(windowCompressed) - EstimateTokens(compressedMessages)
		if budgetSaved < 0 {
			budgetSaved = 0
		}
	}

	if windowSaved > 0 || budgetSaved > 0 || instructionSaved > 0 {
		slog.Info("proxy: openai responses compression savings",
			"session", sessionID,
			"identity_saved_tokens", instructionSaved,
			"window_saved_tokens", windowSaved,
			"budget_saved_tokens", budgetSaved,
		)
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &rawMap); err != nil {
		h.forwardVerbatim(w, r, rawBody, h.openaiUpstream)
		return
	}
	if compressedInstructions != req.Instructions {
		rawMap["instructions"] = mustMarshalJSON(compressedInstructions)
	}
	if inputCompressible {
		rawMap["input"] = forwardedInput
	}
	newBody, err := json.Marshal(rawMap)
	if err != nil {
		h.forwardVerbatim(w, r, rawBody, h.openaiUpstream)
		return
	}

	respBody := h.forwardWithBody(w, r, newBody, h.openaiUpstream)

	go func() {
		ctxComp := parseOpenAIResponsesUsage(respBody)
		if ctxComp < 0 {
			ctxComp = EstimateTokens(compressedMessages) + estimateInstructionsTokens(req.Instructions)
		}
		_ = WriteStats(h.sessionsDir, sessionID, ctxOrig, ctxComp)
		if h.afterStats != nil {
			h.afterStats()
		}
	}()
}
