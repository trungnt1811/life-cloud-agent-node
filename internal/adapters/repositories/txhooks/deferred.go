package txhooks

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"gorm.io/gorm"
)

var (
	ErrDBRequired = errors.New("txhooks: database handle is required")
	ErrBegin      = errors.New("txhooks: begin failed")
	ErrCommit     = errors.New("txhooks: commit failed")
)

const deferredKey = "txhooks:deferred"

type deferred struct {
	mu  sync.Mutex
	fns []func()
}

func getOrCreate(tx *gorm.DB) *deferred {
	if tx == nil {
		return nil
	}
	// NOTE: gorm.DB.Set clones the DB (via getInstance) and returns a new *gorm.DB.
	// If we call tx.Set(...) without reassigning, the value is stored on a clone and lost.
	// For tx-scoped hooks we must attach to the tx instance that will be committed.
	if tx.Statement == nil {
		return nil
	}

	if v, ok := tx.Statement.Settings.Load(deferredKey); ok {
		if d, ok := v.(*deferred); ok && d != nil {
			return d
		}
	}

	d := &deferred{}
	tx.Statement.Settings.Store(deferredKey, d)
	return d
}

// Defer registers fn to be executed after a successful commit.
// Call Run(tx) after tx.Commit() succeeds.
func Defer(tx *gorm.DB, fn func()) {
	if tx == nil || fn == nil {
		return
	}

	d := getOrCreate(tx)
	if d == nil {
		return
	}

	d.mu.Lock()
	d.fns = append(d.fns, fn)
	d.mu.Unlock()
}

// Run executes all deferred functions exactly once.
// Intended to be called after tx.Commit() succeeds.
func Run(tx *gorm.DB) {
	if tx == nil {
		return
	}
	if tx.Statement == nil {
		return
	}

	v, ok := tx.Statement.Settings.Load(deferredKey)
	if !ok {
		return
	}
	d, ok := v.(*deferred)
	if !ok || d == nil {
		return
	}

	d.mu.Lock()
	fns := d.fns
	d.fns = nil
	d.mu.Unlock()

	for _, fn := range fns {
		if fn == nil {
			continue
		}
		fn()
	}
}

// Commit commits the transaction and runs deferred hooks only if commit succeeds.
// Use this instead of calling tx.Commit() directly when you rely on Defer/Run.
func Commit(tx *gorm.DB) error {
	if tx == nil {
		return nil
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("%w: %w", ErrCommit, err)
	}

	Run(tx)
	return nil
}

// WithTransaction is a single entrypoint for running a function inside a DB transaction.
// It guarantees:
// - Rollback on error
// - Rollback on panic (then re-panic)
// - Run deferred hooks only after a successful commit
func WithTransaction(ctx context.Context, db *gorm.DB, fn func(tx *gorm.DB) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if db == nil {
		return ErrDBRequired
	}
	if fn == nil {
		return nil
	}

	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("%w: %w", ErrBegin, tx.Error)
	}

	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback().Error
			panic(r)
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback().Error
		return err
	}

	return Commit(tx)
}
