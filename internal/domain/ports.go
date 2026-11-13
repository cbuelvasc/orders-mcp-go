package domain

import "context"

// MaxRecentOrders is the business limit for listRecentOrders, enforced in the core.
const MaxRecentOrders = 20

// --- Inbound ports (driving side): what the adapters can ask the core to do ---

// FindOrder returns one order or ErrOrderNotFound.
type FindOrder interface {
	FindByID(ctx context.Context, id OrderID) (Order, error)
}

// ListRecentOrders returns the newest orders of a customer. The limit is clamped by the core.
type ListRecentOrders interface {
	ListRecent(ctx context.Context, customer CustomerID, limit int) ([]Order, error)
}

// CancelOrder cancels an order that has not shipped yet.
type CancelOrder interface {
	Cancel(ctx context.Context, id OrderID) (Order, error)
}

// --- Outbound port (driven side): what the core needs from the outside world ---

// OrderRepository is implemented by PostgreSQL in production and by an in-memory fake in tests.
type OrderRepository interface {
	FindByID(ctx context.Context, id OrderID) (Order, bool, error)
	FindRecentByCustomer(ctx context.Context, customer CustomerID, limit int) ([]Order, error)
	Save(ctx context.Context, order Order) error
}
