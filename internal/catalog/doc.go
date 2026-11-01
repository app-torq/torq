// Package catalog holds the embedded, offline vehicle reference data
// (Constitution VII): KBA HSN/TSN keys and the make/model/generation/engine
// taxonomy. Lookups never leave the process; the data ships with the binary
// and is imported into SQLite at startup when it changes.
package catalog
