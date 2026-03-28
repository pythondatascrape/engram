// Package errors defines structured error types for the Engram server.
package errors

import "fmt"

// Detail is the canonical error type for Engram.
type Detail struct {
	Code         int
	Category     string
	Message      string
	Retryable    bool
	RetryAfterMs int
}

func (d *Detail) Error() string {
	return fmt.Sprintf("[%d %s] %s", d.Code, d.Category, d.Message)
}

// New constructs a Detail error.
func New(code int, category, message string, retryable bool, retryAfterMs int) *Detail {
	return &Detail{
		Code:         code,
		Category:     category,
		Message:      message,
		Retryable:    retryable,
		RetryAfterMs: retryAfterMs,
	}
}

// WithRetryAfter returns a copy with RetryAfterMs set.
func (d *Detail) WithRetryAfter(ms int) *Detail {
	copy := *d
	copy.RetryAfterMs = ms
	return &copy
}

// WithMessage returns a copy with Message replaced.
func (d *Detail) WithMessage(msg string) *Detail {
	copy := *d
	copy.Message = msg
	return &copy
}

// Sentinel errors grouped by category. Code ranges: Transport 1000–1099,
// Auth 1100–1199, Session 1200–1299, Identity 1300–1399, Provider 1400–1499,
// Knowledge 1500–1599, Security 1600–1699, Plugin 1700–1799, Admin 1800–1899,
// Server 1900–1999.
var (
	// Transport
	TRANSPORT_UNKNOWN  = New(1000, "TRANSPORT", "unknown transport error", true, 0)
	CONNECTION_FAILED  = New(1001, "TRANSPORT", "connection failed", true, 0)
	CONNECTION_TIMEOUT = New(1002, "TRANSPORT", "connection timed out", true, 0)
	CONNECTION_CLOSED  = New(1003, "TRANSPORT", "connection closed", true, 0)
	CONNECTION_REFUSED = New(1004, "TRANSPORT", "connection refused", true, 0)
	PROTOCOL_ERROR     = New(1005, "TRANSPORT", "protocol error", false, 0)

	// Auth
	AUTH_REQUIRED       = New(1100, "AUTH", "authentication required", false, 0)
	AUTH_EXPIRED        = New(1101, "AUTH", "authentication token expired", false, 0)
	AUTH_INVALID        = New(1102, "AUTH", "authentication token invalid", false, 0)
	PERMISSION_DENIED   = New(1103, "AUTH", "permission denied", false, 0)
	INVALID_CREDENTIALS = New(1104, "AUTH", "invalid credentials", false, 0)

	// Session
	SESSION_NOT_FOUND     = New(1200, "SESSION", "session not found", false, 0)
	SESSION_EXPIRED       = New(1201, "SESSION", "session expired", false, 0)
	SESSION_LIMIT_REACHED = New(1202, "SESSION", "session limit reached", true, 0)
	SESSION_BUSY          = New(1203, "SESSION", "session busy", true, 0)

	// Identity
	CODEBOOK_NOT_FOUND        = New(1300, "IDENTITY", "codebook not found", false, 0)
	CODEBOOK_VERSION_MISMATCH = New(1301, "IDENTITY", "codebook version mismatch", false, 0)
	IDENTITY_INVALID          = New(1302, "IDENTITY", "identity invalid", false, 0)
	IDENTITY_REQUIRED         = New(1303, "IDENTITY", "identity required", false, 0)

	// Provider
	PROVIDER_UNKNOWN      = New(1400, "PROVIDER", "unknown provider", false, 0)
	PROVIDER_UNAVAILABLE  = New(1401, "PROVIDER", "provider unavailable", true, 0)
	PROVIDER_TIMEOUT      = New(1402, "PROVIDER", "provider timed out", true, 0)
	PROVIDER_RATE_LIMITED = New(1403, "PROVIDER", "provider rate limited", true, 0)
	MODEL_NOT_FOUND       = New(1404, "PROVIDER", "model not found", false, 0)
	NO_API_KEY            = New(1405, "PROVIDER", "no API key configured", false, 0)

	// Knowledge
	KNOWLEDGE_RESOLUTION_FAILED = New(1500, "KNOWLEDGE", "knowledge resolution failed", false, 0)
	KNOWLEDGE_NOT_FOUND         = New(1501, "KNOWLEDGE", "knowledge not found", false, 0)
	KNOWLEDGE_UNAVAILABLE       = New(1502, "KNOWLEDGE", "knowledge unavailable", true, 0)
	TOO_MANY_REFS               = New(1503, "KNOWLEDGE", "too many knowledge references", false, 0)

	// Security
	INJECTION_DETECTED       = New(1600, "SECURITY", "prompt injection detected", false, 0)
	CONTENT_POLICY_VIOLATION = New(1601, "SECURITY", "content policy violation", false, 0)

	// Plugin
	PLUGIN_UNAVAILABLE = New(1700, "PLUGIN", "plugin unavailable", true, 0)
	PLUGIN_TIMEOUT     = New(1701, "PLUGIN", "plugin timed out", true, 0)
	PLUGIN_ERROR       = New(1702, "PLUGIN", "plugin error", false, 0)

	// Admin
	ADMIN_AUTH_REQUIRED     = New(1800, "ADMIN", "admin authentication required", false, 0)
	ADMIN_PERMISSION_DENIED = New(1801, "ADMIN", "admin permission denied", false, 0)
	CLIENT_NOT_FOUND        = New(1802, "ADMIN", "client not found", false, 0)

	// Server
	INTERNAL_ERROR     = New(1900, "SERVER", "internal server error", false, 0)
	NOT_IMPLEMENTED    = New(1901, "SERVER", "not implemented", false, 0)
	SERVER_OVERLOADED  = New(1902, "SERVER", "server overloaded", true, 0)
	SERVER_MAINTENANCE = New(1903, "SERVER", "server under maintenance", true, 0)
)
