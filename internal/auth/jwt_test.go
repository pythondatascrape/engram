package auth_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/pythondatascrape/engram/internal/auth"
)

func generateKeys(t *testing.T) (ed25519.PrivateKey, ed25519.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate keys: %v", err)
	}
	return priv, pub
}

func TestIssueAndValidateRoundTrip(t *testing.T) {
	priv, pub := generateKeys(t)
	issuer := auth.NewJWTIssuer(priv, pub, 5*time.Minute)

	clientID := "test-client-123"
	providers := []string{"anthropic", "openai"}

	token, err := issuer.Issue(clientID, providers)
	if err != nil {
		t.Fatalf("Issue() error: %v", err)
	}
	if token == "" {
		t.Fatal("Issue() returned empty token")
	}

	claims, err := issuer.Validate(token)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
	if claims.ClientID != clientID {
		t.Errorf("ClientID = %q, want %q", claims.ClientID, clientID)
	}
	if len(claims.Providers) != len(providers) {
		t.Errorf("Providers len = %d, want %d", len(claims.Providers), len(providers))
	}
	for i, p := range providers {
		if claims.Providers[i] != p {
			t.Errorf("Providers[%d] = %q, want %q", i, claims.Providers[i], p)
		}
	}
	if claims.ExpiresAt <= claims.IssuedAt {
		t.Errorf("ExpiresAt (%d) should be after IssuedAt (%d)", claims.ExpiresAt, claims.IssuedAt)
	}
}

func TestExpiredToken(t *testing.T) {
	priv, pub := generateKeys(t)
	// Use a negative expiry so the token is immediately expired
	issuer := auth.NewJWTIssuer(priv, pub, -1*time.Second)

	token, err := issuer.Issue("client-id", nil)
	if err != nil {
		t.Fatalf("Issue() error: %v", err)
	}

	_, err = issuer.Validate(token)
	if err == nil {
		t.Fatal("Validate() should return error for expired token")
	}
}

func TestTamperedToken(t *testing.T) {
	priv, pub := generateKeys(t)
	issuer := auth.NewJWTIssuer(priv, pub, 5*time.Minute)

	token, err := issuer.Issue("client-id", []string{"anthropic"})
	if err != nil {
		t.Fatalf("Issue() error: %v", err)
	}

	// Tamper: flip last character of the signature
	tampered := token[:len(token)-1] + "X"
	if tampered[len(tampered)-1] == token[len(token)-1] {
		tampered = token[:len(token)-1] + "Y"
	}

	_, err = issuer.Validate(tampered)
	if err == nil {
		t.Fatal("Validate() should return error for tampered token")
	}
}

func TestWrongFormatToken(t *testing.T) {
	priv, pub := generateKeys(t)
	issuer := auth.NewJWTIssuer(priv, pub, 5*time.Minute)

	_, err := issuer.Validate("not.a.valid.jwt.format")
	if err == nil {
		t.Fatal("Validate() should return error for wrong format")
	}

	_, err = issuer.Validate("only.two")
	if err == nil {
		t.Fatal("Validate() should return error for two-part token")
	}
}

func TestIssueNoProviders(t *testing.T) {
	priv, pub := generateKeys(t)
	issuer := auth.NewJWTIssuer(priv, pub, 5*time.Minute)

	token, err := issuer.Issue("client-id", nil)
	if err != nil {
		t.Fatalf("Issue() error: %v", err)
	}

	claims, err := issuer.Validate(token)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
	if len(claims.Providers) != 0 {
		t.Errorf("Providers = %v, want empty", claims.Providers)
	}
}
