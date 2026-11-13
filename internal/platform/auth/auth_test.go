package auth_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cbuelvasc/orders-mcp-go/internal/platform/auth"
)

const issuer = "http://localhost:8180/realms/orders"

type testIssuer struct {
	key  *rsa.PrivateKey
	jwks *httptest.Server
}

func newIssuer(t *testing.T) *testIssuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ti := &testIssuer{key: key}
	ti.jwks = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": "k1", "kty": "RSA", "use": "sig", "alg": "RS256",
			"n": b64(key.N.Bytes()), "e": b64(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	t.Cleanup(ti.jwks.Close)
	return ti
}

func (ti *testIssuer) token(t *testing.T, claims map[string]any, alg string) string {
	t.Helper()
	h, _ := json.Marshal(map[string]string{"alg": alg, "kid": "k1", "typ": "JWT"})
	p, _ := json.Marshal(claims)
	signing := b64(h) + "." + b64(p)
	digest := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, ti.key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return signing + "." + b64(sig)
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func validClaims() map[string]any {
	return map[string]any{"iss": issuer, "sub": "agent", "exp": time.Now().Add(5 * time.Minute).Unix(),
		"scope": "openid orders:read"}
}

func TestVerifyAcceptsAValidTokenAndReadsScopes(t *testing.T) {
	ti := newIssuer(t)
	v := auth.NewVerifier(issuer, ti.jwks.URL)

	claims, err := v.Verify(context.Background(), ti.token(t, validClaims(), "RS256"))
	if err != nil {
		t.Fatal(err)
	}
	if !claims.HasScope("orders:read") || claims.HasScope("orders:write") {
		t.Fatalf("unexpected scopes %v", claims.Scopes)
	}
}

func TestVerifyRejectsBadTokens(t *testing.T) {
	ti := newIssuer(t)
	v := auth.NewVerifier(issuer, ti.jwks.URL)
	expired := validClaims()
	expired["exp"] = time.Now().Add(-time.Hour).Unix()
	otherIssuer := validClaims()
	otherIssuer["iss"] = "https://evil.example.com"
	good := ti.token(t, validClaims(), "RS256")
	tampered := good[:strings.LastIndex(good, ".")-2] + "xx" + good[strings.LastIndex(good, "."):]

	cases := map[string]struct {
		token string
		want  error
	}{
		"expired":      {ti.token(t, expired, "RS256"), auth.ErrExpiredToken},
		"wrong issuer": {ti.token(t, otherIssuer, "RS256"), auth.ErrInvalidToken},
		"alg none":     {ti.token(t, validClaims(), "none"), auth.ErrInvalidToken},
		"tampered":     {tampered, auth.ErrInvalidToken},
		"not a jwt":    {"hello", auth.ErrInvalidToken},
	}
	for name, c := range cases {
		if _, err := v.Verify(context.Background(), c.token); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", name, err, c.want)
		}
	}
}

func TestMiddlewareChallengesWithResourceMetadata(t *testing.T) {
	ti := newIssuer(t)
	mw := auth.RequireBearer(auth.NewVerifier(issuer, ti.jwks.URL),
		"http://localhost:8080/.well-known/oauth-protected-resource/mcp")
	var seen auth.Claims
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seen, _ = auth.FromContext(r.Context()) }))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))
	if rec.Code != http.StatusUnauthorized ||
		rec.Header().Get("WWW-Authenticate") != `Bearer resource_metadata="http://localhost:8080/.well-known/oauth-protected-resource/mcp"` {
		t.Fatalf("got %d %q", rec.Code, rec.Header().Get("WWW-Authenticate"))
	}

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+ti.token(t, validClaims(), "RS256"))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || seen.Subject != "agent" {
		t.Fatalf("got %d, claims %+v", rec.Code, seen)
	}
}

func TestMetadataURL(t *testing.T) {
	got, _ := auth.MetadataURL("http://localhost:8080/mcp")
	if got != "http://localhost:8080/.well-known/oauth-protected-resource/mcp" {
		t.Fatal(got)
	}
}
