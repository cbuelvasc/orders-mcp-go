// Package auth validates OAuth 2.1 access tokens (JWT, RS256) against the issuer's JWKS,
// using only the standard library. It is infrastructure: the core never imports it.
package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
)

// Claims is the subset of the JWT we care about.
type Claims struct {
	Subject   string
	Issuer    string
	Scopes    []string
	ExpiresAt time.Time
}

// HasScope reports whether the token carries the given scope.
func (c Claims) HasScope(scope string) bool {
	for _, s := range c.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// Verifier validates tokens issued by one issuer. Keys are fetched from the JWKS
// endpoint and cached; an unknown kid triggers a refresh (key rotation).
type Verifier struct {
	issuer  string
	jwksURL string
	client  *http.Client
	now     func() time.Time
	leeway  time.Duration

	mu   sync.RWMutex
	keys map[string]*rsa.PublicKey
}

func NewVerifier(issuer, jwksURL string) *Verifier {
	return &Verifier{
		issuer:  issuer,
		jwksURL: jwksURL,
		client:  &http.Client{Timeout: 5 * time.Second},
		now:     time.Now,
		leeway:  30 * time.Second,
		keys:    map[string]*rsa.PublicKey{},
	}
}

type header struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
}

type payload struct {
	Sub   string          `json:"sub"`
	Iss   string          `json:"iss"`
	Exp   int64           `json:"exp"`
	Nbf   int64           `json:"nbf"`
	Scope string          `json:"scope"`
	Scp   json.RawMessage `json:"scp"`
}

// Verify checks signature (RS256 only), issuer, exp/nbf and returns the claims.
func (v *Verifier) Verify(ctx context.Context, token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, fmt.Errorf("%w: malformed", ErrInvalidToken)
	}
	var h header
	if err := decodeSegment(parts[0], &h); err != nil {
		return Claims{}, err
	}
	if h.Alg != "RS256" { // never trust "none" or HMAC with a public key
		return Claims{}, fmt.Errorf("%w: unsupported alg %q", ErrInvalidToken, h.Alg)
	}
	key, err := v.key(ctx, h.Kid)
	if err != nil {
		return Claims{}, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, fmt.Errorf("%w: bad signature encoding", ErrInvalidToken)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], sig); err != nil {
		return Claims{}, fmt.Errorf("%w: bad signature", ErrInvalidToken)
	}

	var p payload
	if err := decodeSegment(parts[1], &p); err != nil {
		return Claims{}, err
	}
	now := v.now()
	if p.Iss != v.issuer {
		return Claims{}, fmt.Errorf("%w: unexpected issuer %q", ErrInvalidToken, p.Iss)
	}
	if p.Exp == 0 || now.After(time.Unix(p.Exp, 0).Add(v.leeway)) {
		return Claims{}, ErrExpiredToken
	}
	if p.Nbf != 0 && now.Add(v.leeway).Before(time.Unix(p.Nbf, 0)) {
		return Claims{}, fmt.Errorf("%w: not valid yet", ErrInvalidToken)
	}
	return Claims{Subject: p.Sub, Issuer: p.Iss, Scopes: scopes(p), ExpiresAt: time.Unix(p.Exp, 0)}, nil
}

// scopes supports both "scope": "a b" (RFC 9068) and "scp": ["a","b"].
func scopes(p payload) []string {
	if p.Scope != "" {
		return strings.Fields(p.Scope)
	}
	var list []string
	if len(p.Scp) > 0 && json.Unmarshal(p.Scp, &list) == nil {
		return list
	}
	return nil
}

func (v *Verifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	k, ok := v.keys[kid]
	v.mu.RUnlock()
	if ok {
		return k, nil
	}
	if err := v.refresh(ctx); err != nil {
		return nil, err
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	if k, ok := v.keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("%w: unknown key id %q", ErrInvalidToken, kid)
}

type jwks struct {
	Keys []struct {
		Kid string `json:"kid"`
		Kty string `json:"kty"`
		Use string `json:"use"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

func (v *Verifier) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return err
	}
	res, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetching JWKS: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("fetching JWKS: status %d", res.StatusCode)
	}
	var set jwks
	if err := json.NewDecoder(res.Body).Decode(&set); err != nil {
		return fmt.Errorf("decoding JWKS: %w", err)
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range set.Keys {
		if k.Kty != "RSA" || (k.Use != "" && k.Use != "sig") {
			continue
		}
		n, errN := base64.RawURLEncoding.DecodeString(k.N)
		e, errE := base64.RawURLEncoding.DecodeString(k.E)
		if errN != nil || errE != nil {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	v.mu.Lock()
	v.keys = keys
	v.mu.Unlock()
	return nil
}

func decodeSegment(seg string, into any) error {
	raw, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		return fmt.Errorf("%w: bad encoding", ErrInvalidToken)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return fmt.Errorf("%w: bad json", ErrInvalidToken)
	}
	return nil
}
