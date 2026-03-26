package auth

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Claims represents the JWT payload.
type Claims struct {
	ClientID  string   `json:"sub"`
	Providers []string `json:"providers,omitempty"`
	IssuedAt  int64    `json:"iat"`
	ExpiresAt int64    `json:"exp"`
	Role      string   `json:"role,omitempty"`
	Issuer    string   `json:"iss,omitempty"`
	Audience  string   `json:"aud,omitempty"`
}

// JWTIssuer issues and validates Ed25519-signed JWTs.
type JWTIssuer struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	expiry     time.Duration
	issuer     string
	audience   string
	isRevoked  func(*Claims) bool
}

// NewJWTIssuer returns a new JWTIssuer.
func NewJWTIssuer(priv ed25519.PrivateKey, pub ed25519.PublicKey, expiry time.Duration) *JWTIssuer {
	return &JWTIssuer{privateKey: priv, publicKey: pub, expiry: expiry}
}

// NewJWTIssuerWithIdentity returns a new JWTIssuer that stamps iss and aud on every token.
func NewJWTIssuerWithIdentity(priv ed25519.PrivateKey, pub ed25519.PublicKey, expiry time.Duration, issuer, audience string) *JWTIssuer {
	return &JWTIssuer{privateKey: priv, publicKey: pub, expiry: expiry, issuer: issuer, audience: audience}
}

// SetRevocationCheck registers a callback that returns true when a token should be rejected.
func (j *JWTIssuer) SetRevocationCheck(fn func(*Claims) bool) {
	j.isRevoked = fn
}

var enc = base64.RawURLEncoding

// Issue creates a signed JWT for the given client and providers.
func (j *JWTIssuer) Issue(clientID string, providers []string) (string, error) {
	header := map[string]string{"alg": "EdDSA", "typ": "JWT"}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	headerEnc := enc.EncodeToString(headerJSON)

	now := time.Now()
	claims := Claims{
		ClientID:  clientID,
		Providers: providers,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(j.expiry).Unix(),
		Issuer:    j.issuer,
		Audience:  j.audience,
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payloadEnc := enc.EncodeToString(payloadJSON)

	signingInput := headerEnc + "." + payloadEnc
	sig := ed25519.Sign(j.privateKey, []byte(signingInput))
	sigEnc := enc.EncodeToString(sig)

	return signingInput + "." + sigEnc, nil
}

// Validate parses and verifies a JWT, returning its claims.
func (j *JWTIssuer) Validate(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("auth: invalid token format")
	}

	signingInput := parts[0] + "." + parts[1]
	sig, err := enc.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("auth: invalid token signature encoding")
	}

	if !ed25519.Verify(j.publicKey, []byte(signingInput), sig) {
		return nil, errors.New("auth: invalid token signature")
	}

	payloadJSON, err := enc.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("auth: invalid token payload encoding")
	}

	var claims Claims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, errors.New("auth: invalid token payload")
	}

	if time.Now().Unix() > claims.ExpiresAt {
		return nil, errors.New("auth: token expired")
	}

	if j.issuer != "" && claims.Issuer != j.issuer {
		return nil, errors.New("auth: token issuer mismatch")
	}
	if j.audience != "" && claims.Audience != j.audience {
		return nil, errors.New("auth: token audience mismatch")
	}

	if j.isRevoked != nil && j.isRevoked(&claims) {
		return nil, errors.New("auth: token revoked")
	}

	return &claims, nil
}
