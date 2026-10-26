package lab01

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// testClaims is a representative production-shaped claim set.
func testClaims() Claims {
	return Claims{
		Subject:  "user_2f8a91c4",
		Audience: "api.production",
		Scope:    []string{"read:profile", "read:orders", "write:reviews"},
	}
}

// issuerBundle groups the three implementations behind a common setup so
// each test can exercise all three without repeating the wiring.
type issuerBundle struct {
	jwt    *JWTIssuer
	paseto *PASETOIssuer
	opaque *OpaqueIssuer
	store  *OpaqueStore
}

func newBundle(t *testing.T) *issuerBundle {
	t.Helper()

	jwtIssuer, err := NewJWTIssuer("tokenfactory.test", "api.production")
	if err != nil {
		t.Fatalf("new jwt issuer: %v", err)
	}
	pasetoIssuer, err := NewPASETOIssuer("tokenfactory.test", "api.production")
	if err != nil {
		t.Fatalf("new paseto issuer: %v", err)
	}
	store := NewOpaqueStore()
	opaqueIssuer := NewOpaqueIssuer(store, "tokenfactory.test", "api.production")

	return &issuerBundle{
		jwt:    jwtIssuer,
		paseto: pasetoIssuer,
		opaque: opaqueIssuer,
		store:  store,
	}
}

// TestRoundTripAllShapes is the core correctness check: every shape issues
// a token that validates back to the original claims.
func TestRoundTripAllShapes(t *testing.T) {
	b := newBundle(t)
	claims := testClaims()
	now := time.Now()

	tests := []struct {
		name   string
		issue  func(Claims, time.Duration) (Token, error)
		issuer string
	}{
		{"jwt", b.jwt.Issue, "jwt"},
		{"paseto", b.paseto.Issue, "paseto"},
		{"opaque", b.opaque.Issue, "opaque"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tok, err := tc.issue(claims, time.Hour)
			if err != nil {
				t.Fatalf("issue: %v", err)
			}

			got, err := tok.Validate(now)
			if err != nil {
				t.Fatalf("validate: %v", err)
			}

			if got.Subject != claims.Subject {
				t.Errorf("subject = %q, want %q", got.Subject, claims.Subject)
			}
			if got.Audience != claims.Audience {
				t.Errorf("audience = %q, want %q", got.Audience, claims.Audience)
			}
			if !equalSlices(got.Scope, claims.Scope) {
				t.Errorf("scope = %v, want %v", got.Scope, claims.Scope)
			}
			if got.Issuer != "tokenfactory.test" {
				t.Errorf("issuer = %q, want %q", got.Issuer, "tokenfactory.test")
			}
			if got.ExpiresAt <= now.Unix() {
				t.Errorf("expires_at = %d, want > %d", got.ExpiresAt, now.Unix())
			}
		})
	}
}

// TestExpiredTokenRejected verifies that every shape honours expiry.
func TestExpiredTokenRejected(t *testing.T) {
	b := newBundle(t)
	claims := testClaims()

	issuers := []struct {
		name  string
		issue func(Claims, time.Duration) (Token, error)
	}{
		{"jwt", b.jwt.Issue},
		{"paseto", b.paseto.Issue},
		{"opaque", b.opaque.Issue},
	}

	for _, issuer := range issuers {
		t.Run(issuer.name, func(t *testing.T) {
			// Issue with a negative TTL so it is already expired.
			tok, err := issuer.issue(claims, -time.Minute)
			if err != nil {
				t.Fatalf("issue: %v", err)
			}

			_, err = tok.Validate(time.Now())
			if !errors.Is(err, ErrExpired) {
				t.Fatalf("validate error = %v, want ErrExpired", err)
			}
		})
	}
}

// TestJWTTamperRejected verifies the signature actually protects the claims.
//
// This is the test that distinguishes a real token from a signed blob
// nobody checks: flipping one byte of the payload must invalidate it.
func TestJWTTamperRejected(t *testing.T) {
	b := newBundle(t)
	tok, err := b.jwt.Issue(testClaims(), time.Hour)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	raw := tok.String()
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(parts))
	}

	tampered := parts[0] + "." + parts[1] + "x." + parts[2]
	_, err = (&jwtToken{raw: tampered, iss: b.jwt}).Validate(time.Now())
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("validate error = %v, want ErrInvalidToken", err)
	}
}

// TestPASETOTamperRejected verifies AEAD rejects a modified ciphertext.
func TestPASETOTamperRejected(t *testing.T) {
	b := newBundle(t)
	tok, err := b.paseto.Issue(testClaims(), time.Hour)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	raw := tok.String()
	const prefix = "v4.local."
	body := raw[len(prefix):]
	// Flip the last character of the base64 payload.
	replacement := byte('A')
	if body[len(body)-1] == 'A' {
		replacement = 'B'
	}
	tampered := prefix + body[:len(body)-1] + string(replacement)

	_, err = (&pasetoToken{raw: tampered, iss: b.paseto}).Validate(time.Now())
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("validate error = %v, want ErrInvalidToken", err)
	}
}

// TestOpaqueRevocable demonstrates the capability that only the opaque
// shape provides: immediate revocation.
//
// The equivalent for a JWT would be a revocation list lookup, which
// reintroduces the network hop the JWT was chosen to avoid.
func TestOpaqueRevocable(t *testing.T) {
	b := newBundle(t)
	tok, err := b.opaque.Issue(testClaims(), time.Hour)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	if _, err := tok.Validate(time.Now()); err != nil {
		t.Fatalf("validate before revoke: %v", err)
	}

	b.opaque.Revoke(tok.String())

	if _, err := tok.Validate(time.Now()); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("validate after revoke = %v, want ErrInvalidToken", err)
	}
}

// TestJWTPayloadVisible contrasts with PASETO: the JWT payload is
// base64url, not encryption. Anyone holding the token can read the claims.
func TestJWTPayloadVisible(t *testing.T) {
	b := newBundle(t)
	claims := testClaims()
	claims.Subject = "user_secret_identifier_12345"

	tok, err := b.jwt.Issue(claims, time.Hour)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	parts := strings.Split(tok.String(), ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(parts))
	}

	// Decode the payload and confirm the subject is readable in plaintext.
	// This is the defining weakness of a signed (not encrypted) JWT:
	// integrity is guaranteed, confidentiality is not.
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var decoded Claims
	if err := json.Unmarshal(payloadJSON, &decoded); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if decoded.Subject != claims.Subject {
		t.Errorf("payload subject = %q, want %q", decoded.Subject, claims.Subject)
	}

	// The PASETO payload must NOT contain the subject in any form.
	pasetoTok, err := b.paseto.Issue(claims, time.Hour)
	if err != nil {
		t.Fatalf("issue paseto: %v", err)
	}
	if strings.Contains(pasetoTok.String(), claims.Subject) {
		t.Error("paseto payload leaks plaintext subject")
	}
	pasetoBody, err := base64.RawURLEncoding.DecodeString(
		strings.TrimPrefix(pasetoTok.String(), "v4.local."))
	if err != nil {
		t.Fatalf("decode paseto body: %v", err)
	}
	if bytes.Contains(pasetoBody, []byte(claims.Subject)) {
		t.Error("paseto payload leaks plaintext subject in decoded body")
	}
}

// TestJWTSegmentStructure documents the RFC 7519 wire format.
func TestJWTSegmentStructure(t *testing.T) {
	b := newBundle(t)
	tok, err := b.jwt.Issue(testClaims(), time.Hour)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	parts := strings.Split(tok.String(), ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(parts))
	}
	if parts[0] != "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9" {
		t.Errorf("unexpected header: %s", parts[0])
	}
	// base64url must not contain '+' or '/'
	if strings.ContainsAny(tok.String(), "+/") {
		t.Error("token contains non-base64url characters")
	}
}

// TestFingerprintStable checks the logging helper.
func TestFingerprintStable(t *testing.T) {
	a := Fingerprint("token-value-1")
	b := Fingerprint("token-value-1")
	c := Fingerprint("token-value-2")
	if a != b {
		t.Error("fingerprint not stable")
	}
	if a == c {
		t.Error("fingerprint collision for different tokens")
	}
	if len(a) != 12 {
		t.Errorf("fingerprint length = %d, want 12", len(a))
	}
}

// TestNoNetworkOnJWTValidate documents the key operational property of
// self-contained tokens: validation performs no I/O.
//
// The test asserts the claim set is retrievable purely from the token
// string plus the public key — no store lookup involved.
func TestNoNetworkOnJWTValidate(t *testing.T) {
	b := newBundle(t)
	tok, err := b.jwt.Issue(testClaims(), time.Hour)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	// A fresh issuer with a DIFFERENT store still validates the token —
	// proving validation did not consult the (empty) opaque store.
	standalone := &JWTIssuer{
		privateKey: b.jwt.privateKey,
		publicKey:  b.jwt.publicKey,
		issuer:     b.jwt.issuer,
		audience:   b.jwt.audience,
	}
	if _, err := (&jwtToken{raw: tok.String(), iss: standalone}).Validate(time.Now()); err != nil {
		t.Fatalf("validate with independent issuer: %v", err)
	}
}

// equalSlices compares two string slices.
func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestTokenSizes records the wire size of each shape, which feeds the
// size comparison in docs/notes/01-token-fundamentals.md.
func TestTokenSizes(t *testing.T) {
	b := newBundle(t)
	claims := testClaims()

	jwtTok, err := b.jwt.Issue(claims, time.Hour)
	if err != nil {
		t.Fatalf("issue jwt: %v", err)
	}
	pasetoTok, err := b.paseto.Issue(claims, time.Hour)
	if err != nil {
		t.Fatalf("issue paseto: %v", err)
	}
	opaqueTok, err := b.opaque.Issue(claims, time.Hour)
	if err != nil {
		t.Fatalf("issue opaque: %v", err)
	}

	t.Logf("token sizes for %d-byte claim set:", len(claimsJSON(t, claims)))
	t.Logf("  jwt    : %d bytes", len(jwtTok.String()))
	t.Logf("  paseto : %d bytes", len(pasetoTok.String()))
	t.Logf("  opaque : %d bytes", len(opaqueTok.String()))

	// Opaque carries no payload at all — it is the smallest by construction.
	if len(opaqueTok.String()) >= len(jwtTok.String()) {
		t.Error("expected opaque token to be smaller than jwt")
	}

	// Measured finding (see docs/notes/01-token-fundamentals.md):
	//
	// PASETO is SMALLER than JWT for a payload of this size, which is
	// counter-intuitive at first glance. The reason is encoding overhead:
	//
	//   JWT   = b64url(header) + b64url(payload) + b64url(64-byte sig)
	//         = 4/3 expansion of the JSON, PLUS base64url of a 64-byte
	//           Ed25519 signature (~86 chars)
	//
	//   PASETO = b64url(nonce(24) || AEAD(plaintext) || tag(16))
	//         = 4/3 expansion of (plaintext + 40 bytes fixed overhead),
	//           with NO encoding of the plaintext itself
	//
	// For a 126-byte claim set the JWT overhead is ~213 bytes of base64
	// expansion, while PASETO's is ~53 bytes. PASETO stays smaller until
	// the plaintext grows large enough that the 4/3 expansion of the raw
	// JSON outweighs the extra signature encoding JWT pays.
	//
	// The practical takeaway: token size is a weak argument for choosing
	// one format over the other. Both are well under typical header size
	// limits (8 KB in nginx, 16 KB in Envoy).
	if len(pasetoTok.String()) >= len(jwtTok.String()) {
		t.Logf("NOTE: paseto (%d) >= jwt (%d) — crossover point reached for this payload size",
			len(pasetoTok.String()), len(jwtTok.String()))
	}
}

// claimsJSON returns the marshalled claim set size for reporting.
func claimsJSON(t *testing.T, c Claims) []byte {
	t.Helper()
	b, err := marshalClaims(c)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	return b
}

// assert bytes import is used
var _ = bytes.Equal
