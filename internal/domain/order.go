package domain

import (
	"fmt"
	"time"
)

// Status of an order.
type Status string

const (
	StatusCreated   Status = "CREATED"
	StatusPaid      Status = "PAID"
	StatusShipped   Status = "SHIPPED"
	StatusDelivered Status = "DELIVERED"
	StatusCancelled Status = "CANCELLED"
)

// ParseStatus validates a status coming from storage.
func ParseStatus(s string) (Status, error) {
	switch st := Status(s); st {
	case StatusCreated, StatusPaid, StatusShipped, StatusDelivered, StatusCancelled:
		return st, nil
	default:
		return "", fmt.Errorf("%w: unknown status %q", ErrInvalidInput, s)
	}
}

// IsFinal reports whether the order reached a terminal state.
func (s Status) IsFinal() bool { return s == StatusDelivered || s == StatusCancelled }

// IsCancellable reports whether the order has not left the warehouse yet.
func (s Status) IsCancellable() bool { return s == StatusCreated || s == StatusPaid }

// Order is the aggregate. Money is kept in cents to avoid floating point errors.
type Order struct {
	ID         OrderID
	CustomerID CustomerID
	Status     Status
	TotalCents int64
	Currency   string
	UpdatedAt  time.Time
}

// Cancel applies the business rule: only orders that have not shipped can be cancelled.
// It returns a new Order; the receiver is not modified.
func (o Order) Cancel(now time.Time) (Order, error) {
	if !o.Status.IsCancellable() {
		return Order{}, fmt.Errorf("%w: order %s cannot be cancelled because it is %s",
			ErrCannotBeCancelled, o.ID, o.Status)
	}
	o.Status = StatusCancelled
	o.UpdatedAt = now
	return o, nil
}
