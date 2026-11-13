package memory_test

import (
	"testing"

	"github.com/cbuelvasc/orders-mcp-go/internal/adapter/out/memory"
	"github.com/cbuelvasc/orders-mcp-go/internal/contract"
	"github.com/cbuelvasc/orders-mcp-go/internal/domain"
)

func TestInMemoryRepositoryContract(t *testing.T) {
	contract.RunOrderRepositoryContract(t, func(*testing.T) domain.OrderRepository {
		return memory.NewRepository()
	})
}
