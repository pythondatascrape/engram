package server

import (
	"context"
	"strings"
	"unicode/utf8"

	engramErrors "github.com/pythondatascrape/engram/internal/errors"
	"github.com/pythondatascrape/engram/internal/identity/codebook"
	"github.com/pythondatascrape/engram/internal/identity/serializer"
	"github.com/pythondatascrape/engram/internal/provider"
	"github.com/pythondatascrape/engram/internal/provider/pool"
	"github.com/pythondatascrape/engram/internal/session"
)

// IncomingRequest holds all fields sent by the client for a single turn.
type IncomingRequest struct {
	ClientID  string
	APIKey    string
	SessionID string
	Query     string
	Identity  map[string]string
	Opts      session.Opts
}

// Response is the result returned to the caller after a turn is processed.
type Response struct {
	SessionID   string
	FullText    string
	TotalTokens int
}

// Handler orchestrates identity serialization, session management, prompt
// assembly, and LLM provider calls for a single request.
type Handler struct {
	sessions   *session.Manager
	serializer *serializer.Serializer
	codebook   *codebook.Codebook
	pool       *pool.Pool
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

// HandleRequest processes one client turn end-to-end:
//  1. First request (no SessionID): validate identity, create session, serialize identity.
//  2. Subsequent request (SessionID present): look up session, verify ownership.
//  3. Assemble prompt from identity + query.
//  4. Acquire a provider connection from the pool.
//  5. Stream the LLM response and collect full text.
//  6. Record the turn on the session.
//  7. Return the connection to the pool.
func (h *Handler) HandleRequest(ctx context.Context, req IncomingRequest) (Response, error) {
	var sess *session.Session

	if req.SessionID == "" {
		// First request — identity is mandatory.
		if len(req.Identity) == 0 {
			return Response{}, engramErrors.IDENTITY_REQUIRED
		}

		// Create a new session.
		s, err := h.sessions.Create(ctx, req.ClientID, req.Opts)
		if err != nil {
			return Response{}, err
		}

		// Serialize and store the identity.
		serialized, err := h.serializer.Serialize(h.codebook, req.Identity)
		if err != nil {
			return Response{}, err
		}
		if err := h.sessions.SetIdentity(s.ID, serialized); err != nil {
			return Response{}, err
		}

		sess = s
	} else {
		// Subsequent request — look up and verify ownership.
		s, err := h.sessions.Get(req.SessionID)
		if err != nil {
			return Response{}, err
		}
		if err := h.sessions.CheckOwnership(req.SessionID, req.ClientID); err != nil {
			return Response{}, err
		}
		sess = s
	}

	// Snapshot the session to safely read SerializedIdentity.
	snap := sess.Snapshot()

	// Assemble the structured prompt.
	prompt := AssemblePrompt(PromptParts{
		Identity: snap.SerializedIdentity,
		Query:    req.Query,
	})

	// Acquire a provider connection.
	conn, err := h.pool.Get(ctx, req.APIKey)
	if err != nil {
		return Response{}, err
	}

	// Send to the LLM and collect streaming chunks.
	chunks, err := conn.Provider.Send(ctx, &provider.Request{
		Model:        snap.Opts.Model,
		SystemPrompt: prompt,
		Query:        req.Query,
	})
	if err != nil {
		h.pool.Return(conn)
		return Response{}, err
	}

	var sb strings.Builder
	for chunk := range chunks {
		sb.WriteString(chunk.Text)
		if chunk.Done {
			break
		}
	}
	fullText := sb.String()

	// Return the connection to the pool as soon as streaming is done.
	h.pool.Return(conn)

	// Record the turn; token counts are not available from all providers,
	// so we pass 0 for tokensSaved and use the prompt length as a rough proxy
	// for tokensSent when a real count is unavailable.
	tokensSent := utf8.RuneCountInString(prompt)
	if err := h.sessions.RecordTurn(snap.ID, tokensSent, 0); err != nil {
		return Response{}, err
	}

	return Response{
		SessionID:   snap.ID,
		FullText:    fullText,
		TotalTokens: tokensSent,
	}, nil
}
