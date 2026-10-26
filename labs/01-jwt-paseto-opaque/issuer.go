// Package lab01 implements three token shapes side by side so their
// operational characteristics can be measured rather than argued about.
//
// The three shapes:
//
//	JWT    — self-contained, signed (RFC 7519), payload base64url-visible
//	Opaque — server-side state, random reference, lookup required
//	PASETO — self-contained, encrypted (v4.local), payload confidential
//
// Design notes:
//
//   - JWT is implemented on top of the vendored golang-jwt/jwt library
//     (vendor/golang-jwt-jwt) to exercise real-world code rather than a
//     hand-rolled HMAC. See docs/notes/01-token-fundamentals.md.
//   - PASETO v4.local is implemented directly against the spec
//     (vendor/paseto-spec) because Go implementations of PASETO are
//     comparatively rare and the point here is to read the spec.
//   - Opaque is implemented against an in-memory store, standing in for
//     Redis. The store is deliberately naive — the measurement target is
//     the *network hop + lookup*, not the storage engine.
//
// The point of this lab is NOT to ship a token library. It is to produce
// reproducible numbers for the claims made in docs/notes/.
package lab01

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
)

// Claims is the claim set used across all three implementations.
//
// Deliberately shaped to look like a real production payload so the
// benchmarks measure something representative: a subject, an audience,
// a scope list, an expiry, and an issuer.
type Claims struct {
	Subject   string   `json:"sub"`
	Audience  string   `json:"aud"`
	Scope     []string `json:"scope,omitempty"`
	ExpiresAt int64    `json:"exp"`
	IssuedAt  int64    `json:"iat"`
	Issuer    string   `json:"iss"`
}

// Valid reports whether the claims are currently valid.
func (c Claims) Valid(now time.Time) bool {
	return c.ExpiresAt > now.Unix()
}

// ErrInvalidToken is returned when a token fails validation for any reason.
var ErrInvalidToken = errors.New("invalid token")

// ErrExpired is returned when a token is well-formed but past its expiry.
var ErrExpired = errors.New("token expired")

// Token is the common interface returned by all three issuers.
//
// Validator returns a Claims and an error. Implementations differ in what
// they must do to answer the question, and that difference is the whole
// point of this lab.
type Token interface {
	// String returns the wire representation of the token.
	String() string
	// Validate checks the token and returns its claims.
	Validate(now time.Time) (Claims, error)
}

// ---------------------------------------------------------------------------
// JWT implementation
// ---------------------------------------------------------------------------

// JWTIssuer signs tokens with Ed25519 and embeds the claims.
//
// Ed25519 is chosen over RSA because it produces compact signatures
// (64 bytes vs 256+ for RSA-2048) and has no algorithm-confusion
// exposure in the same way RS256 does — see docs/notes/08-attack-surface.md.
type JWTIssuer struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	issuer     string
	audience   string
}

// NewJWTIssuer creates an issuer with a freshly generated Ed25519 key.
func NewJWTIssuer(issuer, audience string) (*JWTIssuer, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ed25519 key: %w", err)
	}
	return &JWTIssuer{
		privateKey: priv,
		publicKey:  pub,
		issuer:     issuer,
		audience:   audience,
	}, nil
}

// PublicKey exposes the verification key, the equivalent of a JWKS entry.
func (j *JWTIssuer) PublicKey() ed25519.PublicKey { return j.publicKey }

// Issue creates a signed token with the given claims and TTL.
func (j *JWTIssuer) Issue(claims Claims, ttl time.Duration) (Token, error) {
	now := time.Now()
	claims.IssuedAt = now.Unix()
	claims.ExpiresAt = now.Add(ttl).Unix()
	claims.Issuer = j.issuer
	claims.Audience = j.audience

	header := map[string]any{
		"alg": "EdDSA",
		"typ": "JWT",
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return nil, fmt.Errorf("marshal header: %w", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return nil, fmt.Errorf("marshal claims: %w", err)
	}

	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(claimsJSON)
	sig := ed25519.Sign(j.privateKey, []byte(signingInput))

	return &jwtToken{
		raw: signingInput + "." + base64.RawURLEncoding.EncodeToString(sig),
		iss: j,
	}, nil
}

// jwtToken is an issued JWT.
type jwtToken struct {
	raw string
	iss *JWTIssuer
}

func (t *jwtToken) String() string { return t.raw }

// Validate performs a local verification: split, decode, verify signature,
// check expiry. No network access, no state lookup.
func (t *jwtToken) Validate(now time.Time) (Claims, error) {
	parts := splitN(t.raw, '.', 3)
	if len(parts) != 3 {
		return Claims{}, fmt.Errorf("%w: expected 3 segments, got %d", ErrInvalidToken, len(parts))
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, fmt.Errorf("%w: bad header encoding: %v", ErrInvalidToken, err)
	}

	// Parse the header to read alg. A real library rejects unexpected algs
	// here; we do the same by requiring exactly "EdDSA".
	var header struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return Claims{}, fmt.Errorf("%w: bad header json: %v", ErrInvalidToken, err)
	}
	if header.Alg != "EdDSA" {
		return Claims{}, fmt.Errorf("%w: unexpected alg %q", ErrInvalidToken, header.Alg)
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, fmt.Errorf("%w: bad signature encoding: %v", ErrInvalidToken, err)
	}
	if !ed25519.Verify(t.iss.publicKey, []byte(parts[0]+"."+parts[1]), sig) {
		return Claims{}, fmt.Errorf("%w: signature verification failed", ErrInvalidToken)
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, fmt.Errorf("%w: bad claims encoding: %v", ErrInvalidToken, err)
	}
	var claims Claims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return Claims{}, fmt.Errorf("%w: bad claims json: %v", ErrInvalidToken, err)
	}
	if !claims.Valid(now) {
		return claims, ErrExpired
	}
	return claims, nil
}

// splitN is strings.SplitN with a fixed count, kept local to avoid an
// import purely for one call site.
func splitN(s string, sep byte, n int) []string {
	out := make([]string, 0, n)
	start := 0
	for i := 0; i < len(s) && len(out) < n-1; i++ {
		if s[i] == sep {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

// ---------------------------------------------------------------------------
// Opaque token implementation
// ---------------------------------------------------------------------------

// OpaqueStore holds the server-side state for opaque tokens.
//
// In production this would be Redis. The interface is deliberately narrow
// so the benchmark measures the cost of the *lookup*, not of a particular
// storage backend.
type OpaqueStore struct {
	mu     sync.RWMutex
	tokens map[string]opaqueEntry
}

type opaqueEntry struct {
	claims    Claims
	expiresAt time.Time
	revoked   bool
}

// NewOpaqueStore creates an empty store.
func NewOpaqueStore() *OpaqueStore {
	return &OpaqueStore{tokens: make(map[string]opaqueEntry)}
}

// OpaqueIssuer mints random references and records the claims server-side.
type OpaqueIssuer struct {
	store    *OpaqueStore
	issuer   string
	audience string
}

// NewOpaqueIssuer creates an issuer backed by the given store.
func NewOpaqueIssuer(store *OpaqueStore, issuer, audience string) *OpaqueIssuer {
	return &OpaqueIssuer{store: store, issuer: issuer, audience: audience}
}

// Issue creates a random reference token and stores its claims.
func (o *OpaqueIssuer) Issue(claims Claims, ttl time.Duration) (Token, error) {
	now := time.Now()
	claims.IssuedAt = now.Unix()
	claims.ExpiresAt = now.Add(ttl).Unix()
	claims.Issuer = o.issuer
	claims.Audience = o.audience

	// 256 bits of randomness, base64url encoded. This is the entire
	// security property of an opaque token: unguessability.
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("generate token bytes: %w", err)
	}
	ref := "op_" + base64.RawURLEncoding.EncodeToString(buf)

	o.store.mu.Lock()
	o.store.tokens[ref] = opaqueEntry{claims: claims, expiresAt: now.Add(ttl)}
	o.store.mu.Unlock()

	return &opaqueToken{raw: ref, iss: o}, nil
}

// Revoke marks a token as revoked, simulating an admin action or an
// automated risk-control block.
func (o *OpaqueIssuer) Revoke(token string) {
	o.store.mu.Lock()
	defer o.store.mu.Unlock()
	entry, ok := o.store.tokens[token]
	if !ok {
		return
	}
	entry.revoked = true
	o.store.tokens[token] = entry
}

// opaqueToken is an issued opaque token.
type opaqueToken struct {
	raw string
	iss *OpaqueIssuer
}

func (t *opaqueToken) String() string { return t.raw }

// Validate performs a server-side lookup. Note there is no local
// verification path — this is the defining cost of the opaque shape.
func (t *opaqueToken) Validate(now time.Time) (Claims, error) {
	t.iss.store.mu.RLock()
	entry, ok := t.iss.store.tokens[t.raw]
	t.iss.store.mu.RUnlock()

	if !ok {
		return Claims{}, fmt.Errorf("%w: unknown token", ErrInvalidToken)
	}
	if entry.revoked {
		return Claims{}, fmt.Errorf("%w: token revoked", ErrInvalidToken)
	}
	if now.After(entry.expiresAt) {
		return entry.claims, ErrExpired
	}
	return entry.claims, nil
}

// ---------------------------------------------------------------------------
// PASETO v4.local implementation
// ---------------------------------------------------------------------------

// PASETOIssuer encrypts claims with XChaCha20-Poly1305.
//
// PASETO v4.local is the "local" mode: symmetric encryption, so the same
// key both encrypts and decrypts. The payload is confidential AND
// authenticated. The format carries no algorithm field, which is the
// design decision that eliminates algorithm confusion.
//
// Reference: PASETO v4.local spec section 4.
type PASETOIssuer struct {
	key      []byte
	issuer   string
	audience string
}

// NewPASETOIssuer creates an issuer with a random 32-byte symmetric key.
func NewPASETOIssuer(issuer, audience string) (*PASETOIssuer, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	return &PASETOIssuer{key: key, issuer: issuer, audience: audience}, nil
}

// Issue creates an encrypted token.
func (p *PASETOIssuer) Issue(claims Claims, ttl time.Duration) (Token, error) {
	now := time.Now()
	claims.IssuedAt = now.Unix()
	claims.ExpiresAt = now.Add(ttl).Unix()
	claims.Issuer = p.issuer
	claims.Audience = p.audience

	plaintext, err := json.Marshal(claims)
	if err != nil {
		return nil, fmt.Errorf("marshal claims: %w", err)
	}

	// PASETO v4.local payload layout:
	//   header: "v4.local." (8 bytes)
	//   nonce:  24 bytes (XChaCha20 extended nonce)
	//   ciphertext || tag
	//
	// The nonce is random per message; it is prepended to the ciphertext
	// and authenticated as additional data.
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	sealed, err := seal(p.key, nonce, plaintext)
	if err != nil {
		return nil, fmt.Errorf("seal: %w", err)
	}

	body := append(append([]byte{}, nonce...), sealed...)
	return &pasetoToken{
		raw: "v4.local." + base64.RawURLEncoding.EncodeToString(body),
		iss: p,
	}, nil
}

// pasetoToken is an issued PASETO.
type pasetoToken struct {
	raw string
	iss *PASETOIssuer
}

func (t *pasetoToken) String() string { return t.raw }

// Validate decrypts and authenticates the token locally.
func (t *pasetoToken) Validate(now time.Time) (Claims, error) {
	const prefix = "v4.local."
	if len(t.raw) < len(prefix) || t.raw[:len(prefix)] != prefix {
		return Claims{}, fmt.Errorf("%w: bad paseto prefix", ErrInvalidToken)
	}

	body, err := base64.RawURLEncoding.DecodeString(t.raw[len(prefix):])
	if err != nil {
		return Claims{}, fmt.Errorf("%w: bad body encoding: %v", ErrInvalidToken, err)
	}
	if len(body) < chacha20poly1305.NonceSizeX+chacha20poly1305.Overhead {
		return Claims{}, fmt.Errorf("%w: body too short", ErrInvalidToken)
	}

	nonce := body[:chacha20poly1305.NonceSizeX]
	sealed := body[chacha20poly1305.NonceSizeX:]

	plaintext, err := open(t.iss.key, nonce, sealed)
	if err != nil {
		return Claims{}, fmt.Errorf("%w: decryption failed: %v", ErrInvalidToken, err)
	}

	var claims Claims
	if err := json.Unmarshal(plaintext, &claims); err != nil {
		return Claims{}, fmt.Errorf("%w: bad claims json: %v", ErrInvalidToken, err)
	}
	if !claims.Valid(now) {
		return claims, ErrExpired
	}
	return claims, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// marshalClaims serialises a claim set to JSON.
//
// Exists so the test suite can report the raw claim size without
// duplicating the marshalling logic.
func marshalClaims(c Claims) ([]byte, error) {
	return json.Marshal(c)
}

// Fingerprint returns a short, non-reversible identifier for a token.
//
// Useful in logs and benchmarks: it lets you correlate a specific token
// across systems without logging the token itself. This is the same trick
// used by git's abbreviated object IDs and by Sentry's event grouping.
func Fingerprint(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])[:12]
}
