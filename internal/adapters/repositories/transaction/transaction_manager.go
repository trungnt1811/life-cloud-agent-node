package transaction

import (
	"context"

	"gorm.io/gorm"

	"github.com/lifenetwork-ai/go-backend-template/internal/adapters/repositories/repohelpers"
	"github.com/lifenetwork-ai/go-backend-template/internal/adapters/repositories/txhooks"
	domainrepos "github.com/lifenetwork-ai/go-backend-template/internal/domain/repositories"
)

// TransactionManagerDeps contains repositories that can be rebound to a transaction.
type TransactionManagerDeps struct {
	ExampleRepo domainrepos.ExampleRepository
}

type gormTransactionManager struct {
	db   *gorm.DB
	deps TransactionManagerDeps
}

type gormTxRepositories struct {
	exampleRepo domainrepos.ExampleRepository
}

// NewTransactionManager creates a GORM-backed transaction manager.
func NewTransactionManager(db *gorm.DB, deps TransactionManagerDeps) domainrepos.TransactionManager {
	return &gormTransactionManager{db: db, deps: deps}
}

func (m *gormTransactionManager) WithinTx(ctx context.Context, fn func(domainrepos.TxRepositories) error) error {
	if fn == nil {
		return nil
	}
	if m == nil || m.db == nil {
		return txhooks.ErrDBRequired
	}
	return txhooks.WithTransaction(ctx, m.db, func(tx *gorm.DB) error {
		return fn(m.bindTx(tx))
	})
}

func (m *gormTransactionManager) bindTx(tx *gorm.DB) domainrepos.TxRepositories {
	if m == nil {
		return gormTxRepositories{}
	}
	return gormTxRepositories{
		exampleRepo: repohelpers.BindTx(m.deps.ExampleRepo, tx),
	}
}

func (r gormTxRepositories) Examples() domainrepos.ExampleRepository {
	return r.exampleRepo
}
