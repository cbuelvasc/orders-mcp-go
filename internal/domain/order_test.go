package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/cbuelvasc/orders-mcp-go/internal/domain"
)

func TestParseOrderIDNormalizesAndValidates(t *testing.T) {
	id, err := domain.ParseOrderID("  ord-1042 ")
	if err != nil || id.String() != "ORD-1042" {
		t.Fatalf("got %v, %v", id, err)
	}
	if _, err := domain.ParseOrderID("DROP TABLE orders"); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCancelOnlyBeforeShipping(t *testing.T) {
	now := time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)
	id, _ := domain.ParseOrderID("ORD-1")
	cases := map[domain.Status]bool{
		domain.StatusCreated: true, domain.StatusPaid: true,
		domain.StatusShipped: false, domain.StatusDelivered: false, domain.StatusCancelled: false,
	}
	for status, allowed := range cases {
		cancelled, err := domain.Order{ID: id, Status: status}.Cancel(now)
		if allowed && (err != nil || cancelled.Status != domain.StatusCancelled || !cancelled.UpdatedAt.Equal(now)) {
			t.Errorf("%s: expected cancel, got %v %v", status, cancelled.Status, err)
		}
		if !allowed && !errors.Is(err, domain.ErrCannotBeCancelled) {
			t.Errorf("%s: expected ErrCannotBeCancelled, got %v", status, err)
		}
	}
}
