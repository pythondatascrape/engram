package errors_test

import (
	"strings"
	"testing"

	"github.com/pythondatascrape/engram/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetail_FieldAccess(t *testing.T) {
	e := errors.New(1000, "TRANSPORT", "something failed", true, 500)

	assert.Equal(t, 1000, e.Code)
	assert.Equal(t, "TRANSPORT", e.Category)
	assert.Equal(t, "something failed", e.Message)
	assert.True(t, e.Retryable)
	assert.Equal(t, 500, e.RetryAfterMs)
}

func TestDetail_ErrorInterface(t *testing.T) {
	e := errors.New(1000, "TRANSPORT", "something failed", false, 0)

	// Must satisfy error interface
	var err error = e
	require.NotNil(t, err)

	msg := err.Error()
	assert.True(t, strings.Contains(msg, "1000"), "Error() should contain code")
	assert.True(t, strings.Contains(msg, "something failed"), "Error() should contain message")
}

func TestDetail_WithRetryAfter(t *testing.T) {
	e := errors.New(1000, "TRANSPORT", "timeout", true, 0)
	e2 := e.WithRetryAfter(2000)

	assert.Equal(t, 2000, e2.RetryAfterMs)
	// Original unchanged
	assert.Equal(t, 0, e.RetryAfterMs)
}

func TestDetail_WithMessage(t *testing.T) {
	e := errors.New(1000, "TRANSPORT", "generic", false, 0)
	e2 := e.WithMessage("specific detail")

	assert.Equal(t, "specific detail", e2.Message)
	// Original unchanged
	assert.Equal(t, "generic", e.Message)
}

func TestPredefined_Transport(t *testing.T) {
	assert.Equal(t, 1000, errors.TRANSPORT_UNKNOWN.Code)
	assert.Equal(t, "TRANSPORT", errors.TRANSPORT_UNKNOWN.Category)

	assert.Equal(t, 1001, errors.CONNECTION_FAILED.Code)
	assert.Equal(t, 1002, errors.CONNECTION_TIMEOUT.Code)
	assert.Equal(t, 1003, errors.CONNECTION_CLOSED.Code)
	assert.Equal(t, 1004, errors.CONNECTION_REFUSED.Code)
	assert.Equal(t, 1005, errors.PROTOCOL_ERROR.Code)
}

func TestPredefined_Auth(t *testing.T) {
	assert.Equal(t, 1100, errors.AUTH_REQUIRED.Code)
	assert.Equal(t, "AUTH", errors.AUTH_REQUIRED.Category)

	assert.Equal(t, 1101, errors.AUTH_EXPIRED.Code)
	assert.Equal(t, 1102, errors.AUTH_INVALID.Code)
	assert.Equal(t, 1103, errors.PERMISSION_DENIED.Code)
	assert.Equal(t, 1104, errors.INVALID_CREDENTIALS.Code)
}

func TestPredefined_Session(t *testing.T) {
	assert.Equal(t, 1200, errors.SESSION_NOT_FOUND.Code)
	assert.Equal(t, "SESSION", errors.SESSION_NOT_FOUND.Category)

	assert.Equal(t, 1201, errors.SESSION_EXPIRED.Code)
	assert.Equal(t, 1202, errors.SESSION_LIMIT_REACHED.Code)
	assert.Equal(t, 1203, errors.SESSION_BUSY.Code)
}

func TestPredefined_Identity(t *testing.T) {
	assert.Equal(t, 1300, errors.CODEBOOK_NOT_FOUND.Code)
	assert.Equal(t, "IDENTITY", errors.CODEBOOK_NOT_FOUND.Category)

	assert.Equal(t, 1301, errors.CODEBOOK_VERSION_MISMATCH.Code)
	assert.Equal(t, 1302, errors.IDENTITY_INVALID.Code)
	assert.Equal(t, 1303, errors.IDENTITY_REQUIRED.Code)
}

func TestPredefined_Provider(t *testing.T) {
	assert.Equal(t, 1400, errors.PROVIDER_UNKNOWN.Code)
	assert.Equal(t, "PROVIDER", errors.PROVIDER_UNKNOWN.Category)

	assert.Equal(t, 1401, errors.PROVIDER_UNAVAILABLE.Code)
	assert.Equal(t, 1402, errors.PROVIDER_TIMEOUT.Code)
	assert.Equal(t, 1403, errors.PROVIDER_RATE_LIMITED.Code)
	assert.Equal(t, 1404, errors.MODEL_NOT_FOUND.Code)
	assert.Equal(t, 1405, errors.NO_API_KEY.Code)
}

func TestPredefined_Knowledge(t *testing.T) {
	assert.Equal(t, 1500, errors.KNOWLEDGE_RESOLUTION_FAILED.Code)
	assert.Equal(t, "KNOWLEDGE", errors.KNOWLEDGE_RESOLUTION_FAILED.Category)

	assert.Equal(t, 1501, errors.KNOWLEDGE_NOT_FOUND.Code)
	assert.Equal(t, 1502, errors.KNOWLEDGE_UNAVAILABLE.Code)
	assert.Equal(t, 1503, errors.TOO_MANY_REFS.Code)
}

func TestPredefined_Security(t *testing.T) {
	assert.Equal(t, 1600, errors.INJECTION_DETECTED.Code)
	assert.Equal(t, "SECURITY", errors.INJECTION_DETECTED.Category)

	assert.Equal(t, 1601, errors.CONTENT_POLICY_VIOLATION.Code)
}

func TestPredefined_Plugin(t *testing.T) {
	assert.Equal(t, 1700, errors.PLUGIN_UNAVAILABLE.Code)
	assert.Equal(t, "PLUGIN", errors.PLUGIN_UNAVAILABLE.Category)

	assert.Equal(t, 1701, errors.PLUGIN_TIMEOUT.Code)
	assert.Equal(t, 1702, errors.PLUGIN_ERROR.Code)
}

func TestPredefined_Admin(t *testing.T) {
	assert.Equal(t, 1800, errors.ADMIN_AUTH_REQUIRED.Code)
	assert.Equal(t, "ADMIN", errors.ADMIN_AUTH_REQUIRED.Category)

	assert.Equal(t, 1801, errors.ADMIN_PERMISSION_DENIED.Code)
	assert.Equal(t, 1802, errors.CLIENT_NOT_FOUND.Code)
}

func TestPredefined_Server(t *testing.T) {
	assert.Equal(t, 1900, errors.INTERNAL_ERROR.Code)
	assert.Equal(t, "SERVER", errors.INTERNAL_ERROR.Category)

	assert.Equal(t, 1901, errors.NOT_IMPLEMENTED.Code)
	assert.Equal(t, 1902, errors.SERVER_OVERLOADED.Code)
	assert.Equal(t, 1903, errors.SERVER_MAINTENANCE.Code)
}

func TestDetail_RetryableDefaults(t *testing.T) {
	// Transport errors are generally retryable
	assert.True(t, errors.CONNECTION_TIMEOUT.Retryable)
	assert.True(t, errors.PROVIDER_TIMEOUT.Retryable)
	assert.True(t, errors.PROVIDER_RATE_LIMITED.Retryable)

	// Auth errors are not retryable
	assert.False(t, errors.AUTH_INVALID.Retryable)
	assert.False(t, errors.INVALID_CREDENTIALS.Retryable)
	assert.False(t, errors.INJECTION_DETECTED.Retryable)
}
