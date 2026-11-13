// Command server wires the hexagon together. This is the only place that knows every piece.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"

	mcpadapter "github.com/cbuelvasc/orders-mcp-go/internal/adapter/in/mcp"
	"github.com/cbuelvasc/orders-mcp-go/internal/adapter/in/rest"
	"github.com/cbuelvasc/orders-mcp-go/internal/adapter/out/postgres"
	"github.com/cbuelvasc/orders-mcp-go/internal/app"
	"github.com/cbuelvasc/orders-mcp-go/internal/platform/auth"
	"github.com/cbuelvasc/orders-mcp-go/internal/platform/observability"
)

const version = "0.1.0"

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := loadConfig()

	// Driven side: PostgreSQL
	pool, err := pgxpool.New(ctx, cfg.databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := postgres.Migrate(ctx, pool); err != nil {
		return err
	}
	repo := postgres.NewRepository(pool)

	// Core
	queries := app.NewQueryService(repo)
	cancel := app.NewCancelService(repo, time.Now)

	// Platform
	metrics := observability.New()
	verifier := auth.NewVerifier(cfg.issuer, cfg.jwksURL)
	metadataURL, err := auth.MetadataURL(cfg.resource)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()

	// Driving side 1: MCP for AI agents, protected by the SDK's bearer middleware.
	tools := mcpadapter.NewTools(queries, queries, cancel, metrics)
	mcpHandler := mcpadapter.Handler(mcpadapter.NewServer(tools, version))
	requireToken := sdkauth.RequireBearerToken(
		func(ctx context.Context, token string, _ *http.Request) (*sdkauth.TokenInfo, error) {
			claims, err := verifier.Verify(ctx, token)
			if err != nil {
				return nil, sdkauth.ErrInvalidToken
			}
			return &sdkauth.TokenInfo{Scopes: claims.Scopes, Expiration: claims.ExpiresAt}, nil
		},
		&sdkauth.RequireBearerTokenOptions{ResourceMetadataURL: metadataURL},
	)
	mux.Handle("/mcp", metrics.Instrument("/mcp", requireToken(mcpHandler)))

	// Driving side 2: REST for humans, same ports.
	restAuth := auth.RequireBearer(verifier, metadataURL)
	rest.NewHandler(queries, queries, cancel).Register(mux, func(uri string, h http.Handler) http.Handler {
		return metrics.Instrument(uri, restAuth(h))
	})

	// Public endpoints
	metadata := auth.MetadataHandler(auth.ProtectedResourceMetadata{
		Resource:               cfg.resource,
		AuthorizationServers:   []string{cfg.issuer},
		ScopesSupported:        []string{mcpadapter.ScopeRead, mcpadapter.ScopeWrite},
		BearerMethodsSupported: []string{"header"},
	})
	mux.Handle("GET /.well-known/oauth-protected-resource", metadata)
	mux.Handle("GET /.well-known/oauth-protected-resource/mcp", metadata)
	mux.Handle("GET /metrics", metrics.Handler())
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "db down", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.addr, "mcp", cfg.resource)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelShutdown()
		return srv.Shutdown(shutdownCtx)
	}
	return nil
}

type config struct {
	addr, databaseURL, issuer, jwksURL, resource string
}

func loadConfig() config {
	issuer := env("ISSUER_URI", "http://localhost:8180/realms/orders")
	return config{
		addr:        env("ADDR", ":8080"),
		databaseURL: env("DATABASE_URL", "postgres://orders:orders@localhost:5432/orders?sslmode=disable"),
		issuer:      issuer,
		jwksURL:     env("JWKS_URL", issuer+"/protocol/openid-connect/certs"),
		resource:    env("MCP_RESOURCE_URL", "http://localhost:8080/mcp"),
	}
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
