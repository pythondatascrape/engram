package server

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	engramErrors "github.com/pythondatascrape/engram/internal/errors"
	"github.com/pythondatascrape/engram/internal/identity/codebook"
	"github.com/pythondatascrape/engram/internal/identity/serializer"
	"github.com/pythondatascrape/engram/internal/provider"
	"github.com/pythondatascrape/engram/internal/provider/pool"
	"github.com/pythondatascrape/engram/internal/security"
	"github.com/pythondatascrape/engram/internal/session"
	engramctx "github.com/pythondatascrape/engram/internal/context"
)

// IncomingRequest holds all fields sent by the client for a single turn.
type IncomingRequest struct {
	ClientID  string
	APIKey    string
	SessionID string
	Query     string
	Identity            map[string]string
	ContextSchema       map[string]string // optional — field name → type hint for context codebook
	Opts                session.Opts
	VerboseIdentitySize int // baseline char count for savings calculation (optional)
}

// Response is the result returned to the caller after a turn is processed.
type Response struct {
	SessionID   string
	FullText    string
	TotalTokens int
}

const (
	maxQueryBytes    = 32 * 1024   // 32 KB
	maxIdentityBytes = 4 * 1024    // 4 KB total across all k/v pairs
	maxResponseBytes = 1024 * 1024 // 1 MB
)

// Handler orchestrates identity serialization, session management, prompt
// assembly, and LLM provider calls for a single request.
type Handler struct {
	sessions   *session.Manager
	serializer *serializer.Serializer
	codebook   *codebook.Codebook
	pool       *pool.Pool
	detector   *security.InjectionDetector
}

// NewHandler constructs a Handler with its dependencies.
func NewHandler(
	sessions *session.Manager,
	ser *serializer.Serializer,
	cb *codebook.Codebook,
	p *pool.Pool,
) *Handler {
	return &Handler{
		sessions:   sessions,
		serializer: ser,
		codebook:   cb,
		pool:       p,
	}
}

// NewHandlerWithSecurity constructs a Handler with an injection detector.
func NewHandlerWithSecurity(
	sessions *session.Manager,
	ser *serializer.Serializer,
	cb *codebook.Codebook,
	p *pool.Pool,
	det *security.InjectionDetector,
) *Handler {
	return &Handler{
		sessions:   sessions,
		serializer: ser,
		codebook:   cb,
		pool:       p,
		detector:   det,
	}
}

// HandleRequest processes one client turn: resolve/create session, assemble
// prompt, call the LLM provider, and record the turn.
func (h *Handler) HandleRequest(ctx context.Context, req IncomingRequest) (Response, error) {
	if len(req.Query) > maxQueryBytes {
		return Response{}, fmt.Errorf("query exceeds maximum size of %d bytes", maxQueryBytes)
	}
	if req.Identity != nil {
		total := 0
		for k, v := range req.Identity {
			total += len(k) + len(v)
		}
		if total > maxIdentityBytes {
			return Response{}, fmt.Errorf("identity exceeds maximum size of %d bytes", maxIdentityBytes)
		}
	}

	if h.detector != nil {
		if result := h.detector.Check(req.Query); result.Detected {
			slog.Warn("injection detected", "client_id", req.ClientID, "pattern", result.Pattern, "source", "query")
			return Response{}, engramErrors.INJECTION_DETECTED
		}
		if req.Identity != nil {
			if result := h.detector.CheckIdentityValues(req.Identity); result.Detected {
				slog.Warn("injection detected", "client_id", req.ClientID, "pattern", result.Pattern, "source", "identity")
				return Response{}, engramErrors.INJECTION_DETECTED
			}
		}
	}

	slog.Debug("request received", "client_id", req.ClientID, "has_session", req.SessionID != "")

	var sess *session.Session

	if req.SessionID == "" {
		if len(req.Identity) == 0 {
			return Response{}, engramErrors.IDENTITY_REQUIRED
		}
		s, err := h.sessions.Create(ctx, req.ClientID, req.Opts)
		if err != nil {
			return Response{}, err
		}
		serialized, err := h.serializer.Serialize(h.codebook, req.Identity)
		if err != nil {
			return Response{}, err
		}
		s.SetIdentity(serialized)
		rawSize := req.VerboseIdentitySize
		if rawSize == 0 {
			for k, v := range req.Identity {
				rawSize += len(k) + len(v) + 2
			}
		}
		s.SetIdentityTokens(rawSize)
		if len(req.ContextSchema) > 0 {
			ctxCB, err := engramctx.DeriveCodebook(req.ClientID, req.ContextSchema)
			if err != nil {
				return Response{}, fmt.Errorf("context codebook: %w", err)
			}
			s.SetContextCodebook(ctxCB)
			s.SetHistory(engramctx.NewHistory())
		}
		slog.Info("session created", "session_id", s.ID, "client_id", req.ClientID)
		sess = s
	} else {
		s, err := h.sessions.Get(req.SessionID)
		if err != nil {
			return Response{}, err
		}
		if err := s.CheckOwnership(req.ClientID); err != nil {
			return Response{}, err
		}
		sess = s
	}

	rctx := sess.RequestCtx()

	var history []provider.Message
	if rctx.History != nil {
		history = rctx.History.Messages()
	}

	var ctxCodebookDef, respCodebookDef string
	if rctx.ContextCodebook != nil {
		ctxCodebookDef = rctx.ContextCodebook.Definition()
		if sess.Opts.Provider == "openai" {
			respCodebookDef = engramctx.OpenAIResponseCodebook().Definition()
		} else {
			respCodebookDef = engramctx.AnthropicResponseCodebook().Definition()
		}
	}

	prompt := AssemblePrompt(PromptParts{
		Identity:            rctx.SerializedIdentity,
		ContextCodebookDef:  ctxCodebookDef,
		ResponseCodebookDef: respCodebookDef,
		History:             history,
		Query:               req.Query,
	})

	conn, err := h.pool.Get(ctx, req.APIKey)
	if err != nil {
		return Response{}, err
	}

	chunks, err := conn.Provider.Send(ctx, &provider.Request{
		Model:               rctx.Model,
		SystemPrompt:        prompt,
		Query:               req.Query,
		ConversationHistory: history,
	})
	if err != nil {
		h.pool.Return(conn)
		return Response{}, err
	}

	var sb strings.Builder
	for chunk := range chunks {
		if len(chunk.Text) > 0 {
			if sb.Len()+len(chunk.Text) > maxResponseBytes {
				sb.WriteString(chunk.Text[:maxResponseBytes-sb.Len()])
				break
			}
			sb.WriteString(chunk.Text)
		}
		if chunk.Done {
			break
		}
	}
	fullText := sb.String()

	// Baseline for this turn: what would have been sent without any Engram compression.
	// = verbose identity size + raw history accumulated so far (JSON format) + query
	// JSON overhead per prior turn pair: {"role":"user","content":"..."} + {"role":"assistant",...} = 54 chars
	const jsonMsgOverhead = 54
	rawHistoryNow := sess.RawHistory()
	baselineThisTurn := sess.IdentityBaseline() + rawHistoryNow + len(req.Query)

	var contextSaved int
	rawTurnBytes := jsonMsgOverhead + len(req.Query) + len(fullText)
	if sess.ContextCodebook != nil && sess.History != nil {
		requestTurn := map[string]string{
			"role":    "user",
			"content": req.Query,
		}
		compressedResp := "role=assistant content=" + fullText
		before := sess.History.TokenCount()
		_ = sess.History.Append(sess.ContextCodebook, requestTurn, compressedResp)
		compressedSize := sess.History.TokenCount() - before
		if rawTurnBytes > compressedSize {
			contextSaved = rawTurnBytes - compressedSize
		}
	}

	h.pool.Return(conn)

	// prompt already contains the query via AssemblePrompt.
	tokensSent := len(prompt)
	identitySaved := 0
	if baseline := sess.IdentityBaseline(); baseline > 0 {
		if saved := baseline - len(rctx.SerializedIdentity); saved > 0 {
			identitySaved = saved
		}
	}
	sess.RecordTurn(tokensSent, identitySaved, contextSaved, baselineThisTurn, rawTurnBytes)

	slog.Info("turn recorded",
		"session_id", rctx.ID,
		"tokens_sent", tokensSent,
		"identity_saved", identitySaved,
		"context_saved", contextSaved,
		"identity_baseline", sess.IdentityBaseline(),
		"serialized_len", len(rctx.SerializedIdentity),
	)

	return Response{
		SessionID:   rctx.ID,
		FullText:    fullText,
		TotalTokens: tokensSent,
	}, nil
}
