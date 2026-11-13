package app

import (
	"context"
	"fmt"
	"time"

	"github.com/cbuelvasc/orders-mcp-go/internal/domain"
)

// CancelService implements the command side. The rule lives in the aggregate (Order.Cancel).
type CancelService struct {
	repo domain.OrderRepository
	now  func() time.Time
}

func NewCancelService(repo domain.OrderRepository, now func() time.Time) *CancelService {
	return &CancelService{repo: repo, now: now}
}

var _ domain.CancelOrder = (*CancelService)(nil)

func (s *CancelService) Cancel(ctx context.Context, id domain.OrderID) (domain.Order, error) {
	order, found, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return domain.Order{}, err
	}
	if !found {
		return domain.Order{}, fmt.Errorf("%w: %s", domain.ErrOrderNotFound, id)
	}
	cancelled, err := order.Cancel(s.now().UTC())
	if err != nil {
		return domain.Order{}, err
	}
	if err := s.repo.Save(ctx, cancelled); err != nil {
		return domain.Order{}, err
	}
	return cancelled, nil
}
