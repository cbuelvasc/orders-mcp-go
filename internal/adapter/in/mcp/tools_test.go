package mcp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cbuelvasc/orders-mcp-go/internal/adapter/out/memory"
	"github.com/cbuelvasc/orders-mcp-go/internal/app"
	"github.com/cbuelvasc/orders-mcp-go/internal/domain"
)

type recorder struct{ calls []string }

func (r *recorder) ObserveTool(tool string, _ time.Duration, errKind string) {
	r.calls = append(r.calls, tool+":"+errKind)
}

func newTools(t *testing.T) (*Tools, *recorder) {
	t.Helper()
	repo := memory.NewRepository()
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	for _, s := range []struct {
		id     string
		status domain.Status
	}{{"ORD-1042", domain.StatusShipped}, {"ORD-1040", domain.StatusCreated}} {
		id, _ := domain.ParseOrderID(s.id)
		cid, _ := domain.ParseCustomerID("CUS-1")
		_ = repo.Save(context.Background(), domain.Order{ID: id, CustomerID: cid, Status: s.status,
			TotalCents: 12050, Currency: "USD", UpdatedAt: now})
	}
	q := app.NewQueryService(repo)
	rec := &recorder{}
	return NewTools(q, q, app.NewCancelService(repo, time.Now), rec), rec
}

var reader = []string{ScopeRead}
var writer = []string{ScopeRead, ScopeWrite}

func TestFindOrderReturnsAModelFriendlyView(t *testing.T) {
	tools, rec := newTools(t)

	view, err := tools.FindOrder(context.Background(), reader, FindOrderInput{OrderID: "ord-1042"})
	if err != nil {
		t.Fatal(err)
	}
	if view.OrderID != "ORD-1042" || view.Status != "SHIPPED" || view.Total != "120.50" || view.IsFinal {
		t.Fatalf("unexpected view %+v", view)
	}
	if rec.calls[0] != "findOrder:" {
		t.Fatalf("metrics not recorded: %v", rec.calls)
	}
}

func TestErrorsAreReadableAndCounted(t *testing.T) {
	tools, rec := newTools(t)

	_, err := tools.FindOrder(context.Background(), reader, FindOrderInput{OrderID: "ORD-9"})
	if !errors.Is(err, domain.ErrOrderNotFound) || err.Error() != "order not found: ORD-9" {
		t.Fatalf("got %v", err)
	}
	if rec.calls[0] != "findOrder:OrderNotFound" {
		t.Fatalf("got %v", rec.calls)
	}
}

func TestEachToolRequiresItsScope(t *testing.T) {
	tools, _ := newTools(t)
	ctx := context.Background()

	if _, err := tools.FindOrder(ctx, nil, FindOrderInput{OrderID: "ORD-1042"}); !errors.Is(err, ErrInsufficientScope) {
		t.Errorf("anonymous read: got %v", err)
	}
	if _, err := tools.CancelOrder(ctx, reader, CancelOrderInput{OrderID: "ORD-1040"}); !errors.Is(err, ErrInsufficientScope) {
		t.Errorf("reader cancel: got %v", err)
	}
	view, err := tools.CancelOrder(ctx, writer, CancelOrderInput{OrderID: "ORD-1040"})
	if err != nil || view.Status != "CANCELLED" || !view.IsFinal {
		t.Errorf("writer cancel: got %+v %v", view, err)
	}
}

func TestListRecentOrdersUsesADefaultLimit(t *testing.T) {
	tools, _ := newTools(t)

	list, err := tools.ListRecentOrders(context.Background(), reader, ListRecentOrdersInput{CustomerID: "CUS-1"})
	if err != nil || len(list.Orders) != 2 {
		t.Fatalf("got %+v %v", list, err)
	}
}
