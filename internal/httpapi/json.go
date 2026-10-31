package httpapi

import "database/sql"

// JSON helpers for nullable database columns.

func strPtr(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	s := n.String
	return &s
}

func intPtr(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

func floatPtr(n sql.NullFloat64) *float64 {
	if !n.Valid {
		return nil
	}
	v := n.Float64
	return &v
}

// listResponse is the contract's list envelope.
type listResponse[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
}

func newList[T any](items []T, total int64) listResponse[T] {
	if items == nil {
		items = []T{}
	}
	return listResponse[T]{Items: items, Total: total}
}
