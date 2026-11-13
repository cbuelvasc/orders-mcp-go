//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/cbuelvasc/orders-mcp-go/internal/adapter/out/postgres"
	"github.com/cbuelvasc/orders-mcp-go/internal/contract"
	"github.com/cbuelvasc/orders-mcp-go/internal/domain"
)

// Same contract as the in-memory fake, now against real PostgreSQL + the same migrations.
// Run with: go test -tags=integration ./...
func TestPostgresRepositoryContract(t *testing.T) {
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("orders"), tcpostgres.WithUsername("orders"), tcpostgres.WithPassword("orders"),
		tcpostgres.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	testcontainers.CleanupContainer(t, container)

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := postgres.Migrate(ctx, pool); err != nil { // idempotent
		t.Fatal(err)
	}

	repo := postgres.NewRepository(pool)
	contract.RunOrderRepositoryContract(t, func(*testing.T) domain.OrderRepository { return repo })

	t.Run("migrations seed the demo data", func(t *testing.T) {
		id, _ := domain.ParseOrderID("ORD-1042")
		o, found, err := repo.FindByID(ctx, id)
		if err != nil || !found || o.TotalCents != 5250 {
			t.Fatalf("got %+v found=%v err=%v", o, found, err)
		}
	})
}
