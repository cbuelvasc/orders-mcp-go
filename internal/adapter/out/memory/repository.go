// Package memory is an in-memory implementation of the outbound port.
// It is a test double: it must pass the same contract as the PostgreSQL adapter.
package memory

import (
	"context"
	"slices"
	"sync"

	"github.com/cbuelvasc/orders-mcp-go/internal/domain"
)

type Repository struct {
	mu     sync.RWMutex
	orders map[domain.OrderID]domain.Order
}

func NewRepository() *Repository {
	return &Repository{orders: map[domain.OrderID]domain.Order{}}
}

var _ domain.OrderRepository = (*Repository)(nil)

func (r *Repository) FindByID(_ context.Context, id domain.OrderID) (domain.Order, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	o, ok := r.orders[id]
	return o, ok, nil
}

func (r *Repository) FindRecentByCustomer(_ context.Context, c domain.CustomerID, limit int) ([]domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []domain.Order
	for _, o := range r.orders {
		if o.CustomerID == c {
			out = append(out, o)
		}
	}
	slices.SortFunc(out, func(a, b domain.Order) int { return b.UpdatedAt.Compare(a.UpdatedAt) })
	return out[:min(limit, len(out))], nil
}

func (r *Repository) Save(_ context.Context, o domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.orders[o.ID] = o
	return nil
}
