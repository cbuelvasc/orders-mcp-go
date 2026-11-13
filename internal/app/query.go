// Package app holds the use cases. It depends only on the domain (standard library otherwise).
package app

import (
	"context"
	"fmt"

	"github.com/cbuelvasc/orders-mcp-go/internal/domain"
)

// QueryService implements the read use cases.
type QueryService struct {
	repo domain.OrderRepository
}

func NewQueryService(repo domain.OrderRepository) *QueryService {
	return &QueryService{repo: repo}
}

var (
	_ domain.FindOrder        = (*QueryService)(nil)
	_ domain.ListRecentOrders = (*QueryService)(nil)
)

func (s *QueryService) FindByID(ctx context.Context, id domain.OrderID) (domain.Order, error) {
	order, found, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return domain.Order{}, err
	}
	if !found {
		return domain.Order{}, fmt.Errorf("%w: %s", domain.ErrOrderNotFound, id)
	}
	return order, nil
}

func (s *QueryService) ListRecent(ctx context.Context, customer domain.CustomerID, limit int) ([]domain.Order, error) {
	// Business rule lives in the core, so it protects EVERY adapter (REST, MCP, ...)
	return s.repo.FindRecentByCustomer(ctx, customer, min(max(limit, 1), domain.MaxRecentOrders))
}
