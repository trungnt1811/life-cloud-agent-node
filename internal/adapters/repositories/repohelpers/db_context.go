package repohelpers

import (
	"context"

	"gorm.io/gorm"
)

// DBWithContext binds db to ctx via WithContext, or returns db unchanged
// when ctx is nil. Shared by repository adapters instead of each defining
// its own copy of this one-line plumbing.
func DBWithContext(ctx context.Context, db *gorm.DB) *gorm.DB {
	if ctx == nil {
		return db
	}
	return db.WithContext(ctx)
}
