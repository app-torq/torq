// Package presets renders notification messages into the native payload
// format of each supported target (contracts/webhook-payloads.md).
package presets

import "time"

// Severity of a notification.
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

// Reminder describes the reminder a notification is about.
type Reminder struct {
	ID            int64   `json:"id"`
	Name          string  `json:"name"`
	State         string  `json:"state"`
	Trigger       *string `json:"trigger"`
	NextDueKm     *int64  `json:"next_due_km"`
	NextDueDate   *string `json:"next_due_date"`
	RemainingKm   *int64  `json:"remaining_km"`
	RemainingDays *int64  `json:"remaining_days"`
}

// Vehicle describes the vehicle a notification is about.
type Vehicle struct {
	ID                int64   `json:"id"`
	Name              string  `json:"name"`
	LicensePlate      *string `json:"license_plate"`
	CurrentOdometerKm int64   `json:"current_odometer_km"`
}

// DigestItem is one reminder listed in a digest.
type DigestItem struct {
	Reminder
	Vehicle Vehicle `json:"vehicle"`
	// Line is the localized one-line summary used by text presets.
	Line string `json:"-"`
}

// Digest lists due reminders across the garage.
type Digest struct {
	Period  string       `json:"period"`
	Overdue []DigestItem `json:"overdue"`
	DueSoon []DigestItem `json:"due_soon"`
}

// Field is a localized label/value pair (Discord embed fields).
type Field struct {
	Name  string
	Value string
}

// Message is a fully localized notification, independent of the target.
type Message struct {
	Event    string
	Severity string
	Title    string
	Body     string
	URL      string
	Reminder *Reminder
	Vehicle  *Vehicle
	Digest   *Digest
	Fields   []Field
	// MoreText is the localized "+N more" template result for truncated digests (Discord).
	MoreText func(n int) string
	SentAt   time.Time
}

// Rendered is the HTTP request a preset produces (always POST).
type Rendered struct {
	Headers map[string]string
	Body    []byte
}
