package rest_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cbuelvasc/orders-mcp-go/internal/adapter/in/rest"
	"github.com/cbuelvasc/orders-mcp-go/internal/adapter/out/memory"
	"github.com/cbuelvasc/orders-mcp-go/internal/app"
	"github.com/cbuelvasc/orders-mcp-go/internal/domain"
	"github.com/cbuelvasc/orders-mcp-go/internal/platform/auth"
)

func setup(t *testing.T) *http.ServeMux {
	t.Helper()
	repo := memory.NewRepository()
	id, _ := domain.ParseOrderID("ORD-1042")
	cid, _ := domain.ParseCustomerID("CUS-1")
	_ = repo.Save(context.Background(), domain.Order{ID: id, CustomerID: cid, Status: domain.StatusShipped,
		TotalCents: 5250, Currency: "USD", UpdatedAt: time.Now()})
	q := app.NewQueryService(repo)
	mux := http.NewServeMux()
	rest.NewHandler(q, q, app.NewCancelService(repo, time.Now)).
		Register(mux, func(_ string, h http.Handler) http.Handler { return h })
	return mux
}

func call(mux *http.ServeMux, method, path string, scopes ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req = req.WithContext(auth.WithClaims(req.Context(), auth.Claims{Scopes: scopes}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestRestMapsDomainToHTTP(t *testing.T) {
	mux := setup(t)
	cases := []struct {
		name, method, path string
		scopes             []string
		want               int
	}{
		{"found", "GET", "/api/orders/ORD-1042", []string{"orders:read"}, 200},
		{"not found", "GET", "/api/orders/ORD-9", []string{"orders:read"}, 404},
		{"bad id", "GET", "/api/orders/nope", []string{"orders:read"}, 400},
		{"no scope", "GET", "/api/orders/ORD-1042", nil, 403},
		{"reader cannot cancel", "POST", "/api/orders/ORD-1042/cancel", []string{"orders:read"}, 403},
		{"shipped cannot be cancelled", "POST", "/api/orders/ORD-1042/cancel", []string{"orders:write"}, 409},
		{"list", "GET", "/api/orders?customerId=CUS-1", []string{"orders:read"}, 200},
	}
	for _, c := range cases {
		if got := call(mux, c.method, c.path, c.scopes...).Code; got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}
