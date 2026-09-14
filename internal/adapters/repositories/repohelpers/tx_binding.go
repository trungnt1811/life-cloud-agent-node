package repohelpers

import "gorm.io/gorm"

// TxBinder is implemented by repository adapters that can bind themselves to a GORM transaction.
type TxBinder[T any] interface {
	WithTx(*gorm.DB) T
}

// BindTx returns repo bound to tx when repo supports TxBinder; otherwise it returns repo unchanged.
func BindTx[T any](repo T, tx *gorm.DB) T {
	if tx == nil {
		return repo
	}
	if binder, ok := any(repo).(TxBinder[T]); ok {
		return binder.WithTx(tx)
	}
	return repo
}
