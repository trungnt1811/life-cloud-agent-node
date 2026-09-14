package types

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrCacheMiss          = errors.New("cache: item not found")
	ErrInvalidDestination = errors.New("cache: destination must be a non-nil pointer")
	ErrTypeMismatch       = errors.New("cache: type mismatch between cached value and destination")
)

type Keyer struct {
	Raw string
}

type Value struct {
	Raw string
}

func (k *Keyer) String() string {
	return k.Raw
}

type ClientBatchItem struct {
	Key        string
	Value      any
	Expiration time.Duration
}

type RepoBatchItem struct {
	Key        fmt.Stringer
	Value      any
	Expiration time.Duration
}

//go:generate mockgen -source=cache.go -destination=../../mocks/mock_caching.go -package=mocks
type CacheClient interface {
	Set(ctx context.Context, key string, value any, expiration time.Duration) error
	// SetMany sets multiple key-value pairs, each with its own expiration.
	// Implementations should apply defaults the same way as Set (0 => default TTL, <0 => no expiration).
	SetMany(ctx context.Context, items []ClientBatchItem) error
	Get(ctx context.Context, key string, dest any) error
	Del(ctx context.Context, key string) error

	// Optional but recommended atomic ops
	Incr(ctx context.Context, key string, delta int64) (int64, error)
	Decr(ctx context.Context, key string, delta int64) (int64, error)

	// Optional TTL and ExpireNX
	TTL(ctx context.Context, key string) (time.Duration, error)
	ExpireNX(ctx context.Context, key string, expiration time.Duration) (bool, error)
}

type CacheRepository interface {
	SaveItem(key fmt.Stringer, val any, expire time.Duration) error
	// SaveItems saves multiple items, each with its own expiration. Best-effort if some fail.
	SaveItems(items []RepoBatchItem) error
	RetrieveItem(key fmt.Stringer, val any) error
	RemoveItem(key fmt.Stringer) error

	IncrementItem(key fmt.Stringer, delta int64) (int64, error)
	DecrementItem(key fmt.Stringer, delta int64) (int64, error)

	TTL(key fmt.Stringer) (time.Duration, error)
	ExpireNX(key fmt.Stringer, expire time.Duration) (bool, error)
}
