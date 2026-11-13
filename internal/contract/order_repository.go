// Package contract holds the tests that EVERY implementation of the outbound port must pass.
// This is what makes swapping adapters safe.
package contract

import (
	"context"
	"testing"
	"time"

	"github.com/cbuelvasc/orders-mcp-go/internal/domain"
)

// RunOrderRepositoryContract runs the shared scenarios against a fresh repository.
func RunOrderRepositoryContract(t *testing.T, newRepo func(t *testing.T) domain.OrderRepository) {
	ctx := context.Background()
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)

	t.Run("saves and finds an order", func(t *testing.T) {
		repo := newRepo(t)
		order := newOrder(t, "ORD-9001", "CUS-99", domain.StatusPaid, now)
		mustSave(t, repo, order)

		got, found, err := repo.FindByID(ctx, order.ID)
		if err != nil || !found {
			t.Fatalf("expected order, got found=%v err=%v", found, err)
		}
		if got.Status != order.Status || got.TotalCents != order.TotalCents || !got.UpdatedAt.Equal(order.UpdatedAt) {
			t.Fatalf("got %+v, want %+v", got, order)
		}
	})

	t.Run("returns not found for unknown orders", func(t *testing.T) {
		repo := newRepo(t)
		id, _ := domain.ParseOrderID("ORD-9999999")
		if _, found, err := repo.FindByID(ctx, id); err != nil || found {
			t.Fatalf("expected not found, got found=%v err=%v", found, err)
		}
	})

	t.Run("lists the most recent orders of one customer, newest first", func(t *testing.T) {
		repo := newRepo(t)
		mustSave(t, repo, newOrder(t, "ORD-9101", "CUS-98", domain.StatusPaid, now.Add(-3*time.Hour)))
		mustSave(t, repo, newOrder(t, "ORD-9102", "CUS-98", domain.StatusPaid, now.Add(-1*time.Hour)))
		mustSave(t, repo, newOrder(t, "ORD-9103", "CUS-98", domain.StatusPaid, now.Add(-2*time.Hour)))
		mustSave(t, repo, newOrder(t, "ORD-9104", "CUS-97", domain.StatusPaid, now))

		customer, _ := domain.ParseCustomerID("CUS-98")
		recent, err := repo.FindRecentByCustomer(ctx, customer, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(recent) != 2 || recent[0].ID.String() != "ORD-9102" || recent[1].ID.String() != "ORD-9103" {
			t.Fatalf("unexpected order: %v", ids(recent))
		}
	})

	t.Run("save updates an existing order", func(t *testing.T) {
		repo := newRepo(t)
		order := newOrder(t, "ORD-9201", "CUS-96", domain.StatusPaid, now)
		mustSave(t, repo, order)
		cancelled, err := order.Cancel(now.Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		mustSave(t, repo, cancelled)

		got, _, _ := repo.FindByID(ctx, order.ID)
		if got.Status != domain.StatusCancelled {
			t.Fatalf("expected CANCELLED, got %s", got.Status)
		}
	})
}

func newOrder(t *testing.T, id, customer string, status domain.Status, updatedAt time.Time) domain.Order {
	t.Helper()
	oid, err := domain.ParseOrderID(id)
	if err != nil {
		t.Fatal(err)
	}
	cid, err := domain.ParseCustomerID(customer)
	if err != nil {
		t.Fatal(err)
	}
	return domain.Order{ID: oid, CustomerID: cid, Status: status, TotalCents: 4250, Currency: "USD", UpdatedAt: updatedAt}
}

func mustSave(t *testing.T, repo domain.OrderRepository, o domain.Order) {
	t.Helper()
	if err := repo.Save(context.Background(), o); err != nil {
		t.Fatal(err)
	}
}

func ids(orders []domain.Order) []string {
	out := make([]string, len(orders))
	for i, o := range orders {
		out[i] = o.ID.String()
	}
	return out
}
