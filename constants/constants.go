package constants

import (
	"time"
)

// HTTP Headers
const (
	HeaderKeyContentType  = "Content-Type"
	HeaderContentTypeJson = "application/json"
	HeaderContentTypeXML  = "application/xml"
	HeaderContentTypeForm = "application/x-www-form-urlencoded"
)

// Pagination
const (
	DefaultPageText    = "page"
	DefaultSizeText    = "size"
	AltPageSizeText    = "page_size"
	DefaultPage        = 1
	DefaultPageSize    = 20
	DefaultMinPageSize = 5
	DefaultMaxPageSize = 100
)

// Cache config
const (
	DefaultExpiration = 30 * time.Minute
	CleanupInterval   = 5 * time.Minute
	CacheTypeRedis    = "redis"
	CacheTypeInMemory = "in-memory"
)

// HTTP Timeouts
const (
	DefaultTimeout          = 30 * time.Second
	ReadHeaderTimeout       = 5 * time.Second
	ReadTimeout             = 15 * time.Second
	WriteTimeout            = 15 * time.Second
	DefaultHTTPRequestMs    = 30_000
	DefaultGracefulShutdown = 15 * time.Second
)

// Order direction
type OrderDirection string

func (t OrderDirection) String() string {
	return string(t)
}

const (
	Asc  OrderDirection = "ASC"
	Desc OrderDirection = "DESC"
)

// Retry and backoff constants
const (
	BaseRetryDuration        = 1 * time.Second
	DefaultChallengeDuration = 30 * time.Second
)
