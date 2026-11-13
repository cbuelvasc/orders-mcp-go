package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cbuelvasc/orders-mcp-go/internal/adapter/out/memory"
	"github.com/cbuelvasc/orders-mcp-go/internal/app"
	"github.com/cbuelvasc/orders-mcp-go/internal/domain"
)

var now = time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)

type limitSpy struct {
	domain.OrderRepository
	lastLimit int
}

func (s *limitSpy) FindRecentByCustomer(ctx context.Context, c domain.CustomerID, limit int) ([]domain.Order, error) {
	s.lastLimit = limit
	return nil, nil
}

func TestListRecentClampsTheLimitToProtectEveryAdapter(t *testing.T) {
	spy := &limitSpy{OrderRepository: memory.NewRepository()}
	svc := app.NewQueryService(spy)
	customer, _ := domain.ParseCustomerID("CUS-1")

	for limit, want := range map[int]int{500: 20, -3: 1, 5: 5} {
		_, _ = svc.ListRecent(context.Background(), customer, limit)
		if spy.lastLimit != want {
			t.Errorf("limit %d: got %d, want %d", limit, spy.lastLimit, want)
		}
	}
}

func TestFindByIDReturnsReadableNotFound(t *testing.T) {
	svc := app.NewQueryService(memory.NewRepository())
	id, _ := domain.ParseOrderID("ORD-404")

	_, err := svc.FindByID(context.Background(), id)
	if !errors.Is(err, domain.ErrOrderNotFound) || err.Error() != "order not found: ORD-404" {
		t.Fatalf("got %v", err)
	}
}

func TestCancel(t *testing.T) {
	repo := memory.NewRepository()
	svc := app.NewCancelService(repo, func() time.Time { return now })
	ctx := context.Background()
	paid := seed(t, repo, "ORD-1", domain.StatusPaid)
	shipped := seed(t, repo, "ORD-2", domain.StatusShipped)

	cancelled, err := svc.Cancel(ctx, paid)
	if err != nil || cancelled.Status != domain.StatusCancelled || !cancelled.UpdatedAt.Equal(now) {
		t.Fatalf("got %+v %v", cancelled, err)
	}
	if stored, _, _ := repo.FindByID(ctx, paid); stored.Status != domain.StatusCancelled {
		t.Fatal("cancellation was not saved")
	}
	if _, err := svc.Cancel(ctx, shipped); !errors.Is(err, domain.ErrCannotBeCancelled) {
		t.Fatalf("expected ErrCannotBeCancelled, got %v", err)
	}
	unknown, _ := domain.ParseOrderID("ORD-404")
	if _, err := svc.Cancel(ctx, unknown); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("expected ErrOrderNotFound, got %v", err)
	}
}

func seed(t *testing.T, repo domain.OrderRepository, id string, status domain.Status) domain.OrderID {
	t.Helper()
	oid, _ := domain.ParseOrderID(id)
	cid, _ := domain.ParseCustomerID("CUS-1")
	if err := repo.Save(context.Background(), domain.Order{ID: oid, CustomerID: cid, Status: status,
		TotalCents: 1000, Currency: "USD", UpdatedAt: now.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	return oid
}
