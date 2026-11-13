package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type ctxKey struct{}

// FromContext returns the verified claims stored by the middleware.
func FromContext(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(ctxKey{}).(Claims)
	return c, ok
}

// WithClaims stores claims in the context (used by the middleware and by tests).
func WithClaims(ctx context.Context, c Claims) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

// RequireBearer rejects requests without a valid token with 401 and a
// WWW-Authenticate header that points to the Protected Resource Metadata (RFC 9728).
func RequireBearer(v *Verifier, resourceMetadataURL string) func(http.Handler) http.Handler {
	challenge := fmt.Sprintf(`Bearer resource_metadata=%q`, resourceMetadataURL)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || token == "" {
				w.Header().Set("WWW-Authenticate", challenge)
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			claims, err := v.Verify(r.Context(), token)
			if err != nil {
				// RFC 6750: expired and malformed tokens are both "invalid_token"
				w.Header().Set("WWW-Authenticate", challenge+`, error="invalid_token"`)
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
		})
	}
}

// ProtectedResourceMetadata is the RFC 9728 document.
type ProtectedResourceMetadata struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers"`
	ScopesSupported        []string `json:"scopes_supported"`
	BearerMethodsSupported []string `json:"bearer_methods_supported"`
}

// MetadataHandler serves the RFC 9728 document. It must be public.
func MetadataHandler(m ProtectedResourceMetadata) http.Handler {
	body, _ := json.Marshal(m)
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
}

// MetadataURL derives /.well-known/oauth-protected-resource/<path> from the resource URL.
func MetadataURL(resource string) (string, error) {
	u, err := url.Parse(resource)
	if err != nil {
		return "", err
	}
	return u.Scheme + "://" + u.Host + "/.well-known/oauth-protected-resource" + u.Path, nil
}
