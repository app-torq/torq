package db

import (
	"database/sql"
	"encoding/json"
)

// Scope turns a list of visible vehicle IDs into the scope parameter of the
// list queries: NULL (nil) means all vehicles, otherwise a JSON array, so an
// empty non-nil list matches nothing.
func Scope(vehicleIDs []int64) sql.NullString {
	if vehicleIDs == nil {
		return sql.NullString{}
	}
	b, _ := json.Marshal(vehicleIDs) // []int64 always marshals
	return sql.NullString{String: string(b), Valid: true}
}
