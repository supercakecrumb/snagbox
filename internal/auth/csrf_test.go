package auth

import "testing"

func TestCSRFTokenForDeterministic(t *testing.T) {
	secret := []byte("server-secret")
	token := "session-token"

	first := CSRFTokenFor(secret, token)
	second := CSRFTokenFor(secret, token)
	if first != second {
		t.Fatalf("CSRFTokenFor not deterministic: %q != %q", first, second)
	}
	if first == "" {
		t.Fatal("CSRFTokenFor returned an empty token")
	}
}

func TestCSRFTokenForDiffersByToken(t *testing.T) {
	secret := []byte("server-secret")

	a := CSRFTokenFor(secret, "session-a")
	b := CSRFTokenFor(secret, "session-b")
	if a == b {
		t.Fatalf("CSRFTokenFor produced same token for different sessions: %q", a)
	}
}

func TestVerifyCSRF(t *testing.T) {
	secret := []byte("server-secret")
	token := "session-token"

	valid := CSRFTokenFor(secret, token)
	if !VerifyCSRF(secret, token, valid) {
		t.Fatal("VerifyCSRF rejected a matching token")
	}
	if VerifyCSRF(secret, token, valid+"tampered") {
		t.Fatal("VerifyCSRF accepted a mismatched token")
	}
	if VerifyCSRF(secret, token, CSRFTokenFor(secret, "other-session")) {
		t.Fatal("VerifyCSRF accepted a token derived from a different session")
	}
}
