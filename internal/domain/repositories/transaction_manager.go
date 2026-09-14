package repositories

import "context"

// TxRepositories exposes repository ports bound to the same transaction.
type TxRepositories interface {
	Examples() ExampleRepository
}

// TransactionManager runs a function inside one atomic repository transaction.
//
//go:generate mockgen -source=transaction_manager.go -destination=../../mocks/mock_transaction_manager.go -package=mocks
type TransactionManager interface {
	WithinTx(ctx context.Context, fn func(TxRepositories) error) error
}
