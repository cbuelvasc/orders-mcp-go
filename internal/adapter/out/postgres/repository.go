// Package postgres is the driven adapter for PostgreSQL (pgx). Plain SQL, no ORM:
// the mapping between rows and the Order aggregate lives here and only here.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cbuelvasc/orders-mcp-go/internal/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

var _ domain.OrderRepository = (*Repository)(nil)

// Money is NUMERIC(12,2) in the table and cents (int64) in the domain.
const columns = "id, customer_id, status, (total * 100)::bigint, currency, updated_at"

func (r *Repository) FindByID(ctx context.Context, id domain.OrderID) (domain.Order, bool, error) {
	row := r.pool.QueryRow(ctx, "SELECT "+columns+" FROM orders WHERE id = $1", id.String())
	o, err := scan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Order{}, false, nil
	}
	if err != nil {
		return domain.Order{}, false, err
	}
	return o, true, nil
}

func (r *Repository) FindRecentByCustomer(ctx context.Context, c domain.CustomerID, limit int) ([]domain.Order, error) {
	rows, err := r.pool.Query(ctx, "SELECT "+columns+` FROM orders
		WHERE customer_id = $1
		ORDER BY updated_at DESC
		LIMIT $2`, c.String(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Order
	for rows.Next() {
		o, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (r *Repository) Save(ctx context.Context, o domain.Order) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO orders (id, customer_id, status, total, currency, updated_at)
		VALUES ($1, $2, $3, $4::bigint / 100.0, $5, $6)
		ON CONFLICT (id) DO UPDATE
		SET status = EXCLUDED.status, total = EXCLUDED.total, updated_at = EXCLUDED.updated_at`,
		o.ID.String(), o.CustomerID.String(), string(o.Status), o.TotalCents, o.Currency, o.UpdatedAt)
	return err
}

func scan(row pgx.Row) (domain.Order, error) {
	var (
		id, customer, status, currency string
		cents                          int64
		updatedAt                      time.Time
	)
	if err := row.Scan(&id, &customer, &status, &cents, &currency, &updatedAt); err != nil {
		return domain.Order{}, err
	}
	oid, err := domain.ParseOrderID(id)
	if err != nil {
		return domain.Order{}, fmt.Errorf("corrupt row: %w", err)
	}
	cid, err := domain.ParseCustomerID(customer)
	if err != nil {
		return domain.Order{}, fmt.Errorf("corrupt row: %w", err)
	}
	st, err := domain.ParseStatus(status)
	if err != nil {
		return domain.Order{}, fmt.Errorf("corrupt row: %w", err)
	}
	return domain.Order{ID: oid, CustomerID: cid, Status: st, TotalCents: cents, Currency: currency,
		UpdatedAt: updatedAt.UTC()}, nil
}
