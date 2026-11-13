// Package mcp is the driving adapter for AI agents.
// tools.go holds the tool logic (plain Go, unit-testable); server.go binds it to the MCP SDK.
package mcp

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/cbuelvasc/orders-mcp-go/internal/domain"
)

// Scopes required per tool: least privilege per capability.
const (
	ScopeRead  = "orders:read"
	ScopeWrite = "orders:write"
)

// ErrInsufficientScope is returned when the token lacks the scope a tool needs.
var ErrInsufficientScope = errors.New("insufficient scope")

// Metrics records one observation per tool call (implemented with Prometheus).
type Metrics interface {
	ObserveTool(tool string, elapsed time.Duration, errKind string)
}

// Tools depends on inbound ports only, never on the application services or the repository.
type Tools struct {
	find    domain.FindOrder
	list    domain.ListRecentOrders
	cancel  domain.CancelOrder
	metrics Metrics
}

func NewTools(find domain.FindOrder, list domain.ListRecentOrders, cancel domain.CancelOrder, m Metrics) *Tools {
	return &Tools{find: find, list: list, cancel: cancel, metrics: m}
}

// --- Inputs and outputs: the JSON schema the model sees is generated from these types ---

type FindOrderInput struct {
	OrderID string `json:"orderId" jsonschema:"Order id, format ORD-<digits>, e.g. ORD-1042"`
}

type ListRecentOrdersInput struct {
	CustomerID string `json:"customerId" jsonschema:"Customer id, format CUS-<digits>, e.g. CUS-7"`
	Limit      int    `json:"limit,omitempty" jsonschema:"How many orders to return, from 1 to 20 (default 5)"`
}

type CancelOrderInput struct {
	OrderID string `json:"orderId" jsonschema:"Order id to cancel, format ORD-<digits>"`
}

// OrderView is what the model sees: only the fields it needs, nothing sensitive.
type OrderView struct {
	OrderID   string    `json:"orderId"`
	Status    string    `json:"status"`
	IsFinal   bool      `json:"isFinal"`
	Total     string    `json:"total"`
	Currency  string    `json:"currency"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// OrderList wraps the list: MCP structured output must be a JSON object.
type OrderList struct {
	Orders []OrderView `json:"orders"`
}

func toView(o domain.Order) OrderView {
	return OrderView{
		OrderID:   o.ID.String(),
		Status:    string(o.Status),
		IsFinal:   o.Status.IsFinal(),
		Total:     fmt.Sprintf("%d.%02d", o.TotalCents/100, o.TotalCents%100),
		Currency:  o.Currency,
		UpdatedAt: o.UpdatedAt,
	}
}

// --- Tool logic ---

func (t *Tools) FindOrder(ctx context.Context, scopes []string, in FindOrderInput) (OrderView, error) {
	return observe(t.metrics, "findOrder", func() (OrderView, error) {
		if err := require(scopes, ScopeRead); err != nil {
			return OrderView{}, err
		}
		id, err := domain.ParseOrderID(in.OrderID)
		if err != nil {
			return OrderView{}, err
		}
		o, err := t.find.FindByID(ctx, id)
		if err != nil {
			return OrderView{}, err
		}
		return toView(o), nil
	})
}

func (t *Tools) ListRecentOrders(ctx context.Context, scopes []string, in ListRecentOrdersInput) (OrderList, error) {
	return observe(t.metrics, "listRecentOrders", func() (OrderList, error) {
		if err := require(scopes, ScopeRead); err != nil {
			return OrderList{}, err
		}
		customer, err := domain.ParseCustomerID(in.CustomerID)
		if err != nil {
			return OrderList{}, err
		}
		limit := in.Limit
		if limit == 0 {
			limit = 5
		}
		orders, err := t.list.ListRecent(ctx, customer, limit)
		if err != nil {
			return OrderList{}, err
		}
		views := make([]OrderView, 0, len(orders))
		for _, o := range orders {
			views = append(views, toView(o))
		}
		return OrderList{Orders: views}, nil
	})
}

func (t *Tools) CancelOrder(ctx context.Context, scopes []string, in CancelOrderInput) (OrderView, error) {
	return observe(t.metrics, "cancelOrder", func() (OrderView, error) {
		if err := require(scopes, ScopeWrite); err != nil {
			return OrderView{}, err
		}
		id, err := domain.ParseOrderID(in.OrderID)
		if err != nil {
			return OrderView{}, err
		}
		o, err := t.cancel.Cancel(ctx, id)
		if err != nil {
			return OrderView{}, err
		}
		return toView(o), nil
	})
}

func require(scopes []string, scope string) error {
	if !slices.Contains(scopes, scope) {
		return fmt.Errorf("%w: this tool requires the %q scope", ErrInsufficientScope, scope)
	}
	return nil
}

// observe records latency and errors per tool. The error message is returned to the
// model as a tool error, so it must be readable (e.g. "order not found: ORD-1042").
func observe[T any](m Metrics, tool string, call func() (T, error)) (T, error) {
	start := time.Now()
	out, err := call()
	m.ObserveTool(tool, time.Since(start), errKind(err))
	return out, err
}

func errKind(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrInsufficientScope):
		return "InsufficientScope"
	case errors.Is(err, domain.ErrOrderNotFound):
		return "OrderNotFound"
	case errors.Is(err, domain.ErrCannotBeCancelled):
		return "CannotBeCancelled"
	case errors.Is(err, domain.ErrInvalidInput):
		return "InvalidInput"
	default:
		return "Internal"
	}
}
