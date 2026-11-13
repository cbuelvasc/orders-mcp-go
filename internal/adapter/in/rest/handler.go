// Package rest is the driving adapter for humans: same inbound ports as the MCP adapter.
package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"

	"github.com/cbuelvasc/orders-mcp-go/internal/domain"
	"github.com/cbuelvasc/orders-mcp-go/internal/platform/auth"
)

type Handler struct {
	find   domain.FindOrder
	list   domain.ListRecentOrders
	cancel domain.CancelOrder
}

func NewHandler(find domain.FindOrder, list domain.ListRecentOrders, cancel domain.CancelOrder) *Handler {
	return &Handler{find: find, list: list, cancel: cancel}
}

// Register mounts the routes (Go 1.22+ method and wildcard patterns).
func (h *Handler) Register(mux *http.ServeMux, wrap func(uri string, next http.Handler) http.Handler) {
	mux.Handle("GET /api/orders/{id}", wrap("/api/orders/{id}", h.scoped("orders:read", h.findOrder)))
	mux.Handle("GET /api/orders", wrap("/api/orders", h.scoped("orders:read", h.listRecent)))
	mux.Handle("POST /api/orders/{id}/cancel", wrap("/api/orders/{id}/cancel", h.scoped("orders:write", h.cancelOrder)))
}

func (h *Handler) findOrder(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseOrderID(r.PathValue("id"))
	if err != nil {
		problem(w, err)
		return
	}
	o, err := h.find.FindByID(r.Context(), id)
	if err != nil {
		problem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toJSON(o))
}

func (h *Handler) listRecent(w http.ResponseWriter, r *http.Request) {
	customer, err := domain.ParseCustomerID(r.URL.Query().Get("customerId"))
	if err != nil {
		problem(w, err)
		return
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil {
		limit = 5
	}
	orders, err := h.list.ListRecent(r.Context(), customer, limit)
	if err != nil {
		problem(w, err)
		return
	}
	out := make([]orderJSON, 0, len(orders))
	for _, o := range orders {
		out = append(out, toJSON(o))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) cancelOrder(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseOrderID(r.PathValue("id"))
	if err != nil {
		problem(w, err)
		return
	}
	o, err := h.cancel.Cancel(r.Context(), id)
	if err != nil {
		problem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toJSON(o))
}

// scoped enforces one scope per route, like @PreAuthorize in the Java version.
func (h *Handler) scoped(scope string, next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.FromContext(r.Context())
		if !ok || !slices.Contains(claims.Scopes, scope) {
			writeProblem(w, http.StatusForbidden, "insufficient scope: "+scope+" required")
			return
		}
		next(w, r)
	})
}

type orderJSON struct {
	ID         string `json:"id"`
	CustomerID string `json:"customerId"`
	Status     string `json:"status"`
	TotalCents int64  `json:"totalCents"`
	Currency   string `json:"currency"`
	UpdatedAt  string `json:"updatedAt"`
}

func toJSON(o domain.Order) orderJSON {
	return orderJSON{o.ID.String(), o.CustomerID.String(), string(o.Status), o.TotalCents, o.Currency,
		o.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")}
}

// problem maps domain errors to RFC 9457 Problem Details.
func problem(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		writeProblem(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrOrderNotFound):
		writeProblem(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrCannotBeCancelled):
		writeProblem(w, http.StatusConflict, err.Error())
	default:
		writeProblem(w, http.StatusInternalServerError, "internal error")
	}
}

func writeProblem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"status": status, "title": http.StatusText(status), "detail": detail})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
