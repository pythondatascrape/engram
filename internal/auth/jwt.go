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
}

// JWTIssuer issues and validates Ed25519-signed JWTs.
type JWTIssuer struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	expiry     time.Duration
}

// NewJWTIssuer returns a new JWTIssuer.
func NewJWTIssuer(priv ed25519.PrivateKey, pub ed25519.PublicKey, expiry time.Duration) *JWTIssuer {
	return &JWTIssuer{privateKey: priv, publicKey: pub, expiry: expiry}
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

	return &claims, nil
}
