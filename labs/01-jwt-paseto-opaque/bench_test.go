package lab01

import (
	"testing"
	"time"
)

// BenchmarkIssue measures the cost of minting a token for each shape.
//
// The opaque shape's cost is dominated by random-number generation, not
// by any cryptographic work — the token carries no payload at all.
func BenchmarkIssue(b *testing.B) {
	claims := testClaims()
	ttl := time.Hour

	b.Run("jwt", func(b *testing.B) {
		issuer, err := NewJWTIssuer("tokenfactory.test", "api.production")
		if err != nil {
			b.Fatalf("new issuer: %v", err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := issuer.Issue(claims, ttl); err != nil {
				b.Fatalf("issue: %v", err)
			}
		}
	})

	b.Run("paseto", func(b *testing.B) {
		issuer, err := NewPASETOIssuer("tokenfactory.test", "api.production")
		if err != nil {
			b.Fatalf("new issuer: %v", err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := issuer.Issue(claims, ttl); err != nil {
				b.Fatalf("issue: %v", err)
			}
		}
	})

	b.Run("opaque", func(b *testing.B) {
		issuer := NewOpaqueIssuer(NewOpaqueStore(), "tokenfactory.test", "api.production")
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := issuer.Issue(claims, ttl); err != nil {
				b.Fatalf("issue: %v", err)
			}
		}
	})
}

// BenchmarkValidate measures the cost of verifying a token.
//
// This is the number that matters at scale, because validation happens
// on every authenticated request while issuance happens once.
//
// The opaque case measures only the in-process map lookup. A production
// deployment pays an additional network round-trip to Redis — see
// docs/notes/01-token-fundamentals.md for why that changes the picture
// entirely.
func BenchmarkValidate(b *testing.B) {
	claims := testClaims()
	ttl := time.Hour
	now := time.Now()

	jwtIssuer, err := NewJWTIssuer("tokenfactory.test", "api.production")
	if err != nil {
		b.Fatalf("new jwt issuer: %v", err)
	}
	pasetoIssuer, err := NewPASETOIssuer("tokenfactory.test", "api.production")
	if err != nil {
		b.Fatalf("new paseto issuer: %v", err)
	}
	opaqueIssuer := NewOpaqueIssuer(NewOpaqueStore(), "tokenfactory.test", "api.production")

	jwtTok, err := jwtIssuer.Issue(claims, ttl)
	if err != nil {
		b.Fatalf("issue jwt: %v", err)
	}
	pasetoTok, err := pasetoIssuer.Issue(claims, ttl)
	if err != nil {
		b.Fatalf("issue paseto: %v", err)
	}
	opaqueTok, err := opaqueIssuer.Issue(claims, ttl)
	if err != nil {
		b.Fatalf("issue opaque: %v", err)
	}

	b.Run("jwt", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := jwtTok.Validate(now); err != nil {
				b.Fatalf("validate: %v", err)
			}
		}
	})

	b.Run("paseto", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := pasetoTok.Validate(now); err != nil {
				b.Fatalf("validate: %v", err)
			}
		}
	})

	b.Run("opaque", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := opaqueTok.Validate(now); err != nil {
				b.Fatalf("validate: %v", err)
			}
		}
	})
}

// BenchmarkValidateParallel measures throughput under concurrency.
//
// Verification is on the hot path of every request, and the difference
// between a mutex-guarded map and lock-free local verification is exactly
// why the opaque shape becomes a scaling bottleneck.
func BenchmarkValidateParallel(b *testing.B) {
	claims := testClaims()
	ttl := time.Hour
	now := time.Now()

	jwtIssuer, err := NewJWTIssuer("tokenfactory.test", "api.production")
	if err != nil {
		b.Fatalf("new jwt issuer: %v", err)
	}
	opaqueIssuer := NewOpaqueIssuer(NewOpaqueStore(), "tokenfactory.test", "api.production")

	jwtTok, err := jwtIssuer.Issue(claims, ttl)
	if err != nil {
		b.Fatalf("issue jwt: %v", err)
	}
	opaqueTok, err := opaqueIssuer.Issue(claims, ttl)
	if err != nil {
		b.Fatalf("issue opaque: %v", err)
	}

	b.Run("jwt", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if _, err := jwtTok.Validate(now); err != nil {
					b.Fatalf("validate: %v", err)
				}
			}
		})
	})

	b.Run("opaque", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if _, err := opaqueTok.Validate(now); err != nil {
					b.Fatalf("validate: %v", err)
				}
			}
		})
	})
}

// BenchmarkTamperCheck measures the cost of the rejection path.
//
// A validator that is slow on the happy path but fast on the failure path
// is a potential DoS amplifier, so both matter.
func BenchmarkTamperCheck(b *testing.B) {
	claims := testClaims()
	now := time.Now()

	issuer, err := NewJWTIssuer("tokenfactory.test", "api.production")
	if err != nil {
		b.Fatalf("new issuer: %v", err)
	}
	tok, err := issuer.Issue(claims, time.Hour)
	if err != nil {
		b.Fatalf("issue: %v", err)
	}

	// Corrupt the signature segment.
	raw := tok.String()
	corrupted := raw[:len(raw)-4] + "AAAA"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = (&jwtToken{raw: corrupted, iss: issuer}).Validate(now)
	}
}
