package render

import (
	"net/http"
	"strconv"
)

// Pagination constants for list endpoints.
const (
	DefaultLimit  = 50
	MaxLimit      = 500
	MinLimit      = 1
	DefaultOffset = 0
	MinOffset     = 0
)

// Page holds bounded pagination parameters parsed from a request.
type Page struct {
	Limit  int
	Offset int
}

// ParsePage extracts and clamps `limit` and `offset` query params from the request.
// Non-integer or out-of-range values fall back to defaults / clamped bounds.
func ParsePage(r *http.Request) Page {
	limit := DefaultLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if limit < MinLimit {
		limit = MinLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}

	offset := DefaultOffset
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}
	if offset < MinOffset {
		offset = MinOffset
	}

	return Page{Limit: limit, Offset: offset}
}
