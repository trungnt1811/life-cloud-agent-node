package types

// PaginatedResponse represents a paginated response structure
type PaginatedResponse[T any] struct {
	Items      []T  `json:"items"`
	TotalCount int  `json:"total_count"`
	Page       int  `json:"page"`
	PageSize   int  `json:"page_size"`
	NextPage   *int `json:"next_page,omitempty"`
}

// OrderBy represents ordering parameters for queries
type OrderBy struct {
	Field     string `json:"field"`
	Direction string `json:"direction"` // ASC or DESC
}

// Filter represents filtering parameters for queries
type Filter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"` // eq, ne, gt, lt, gte, lte, like, in
	Value    any    `json:"value"`
}

// QueryParams represents common query parameters
type QueryParams struct {
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
	OrderBy  []OrderBy `json:"order_by,omitempty"`
	Filters  []Filter  `json:"filters,omitempty"`
	Search   string    `json:"search,omitempty"`
}
