package postgresql

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// IsUniqueViolation checks if the error is due to a unique constraint violation.
func IsUniqueViolation(err error) bool {
	var pqErr *pgconn.PgError
	// Check if the error is a PostgreSQL error and has a unique violation code
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return true
	}
	return false
}
