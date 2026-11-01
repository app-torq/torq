package servicelog

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/marekbeckmann/torq/internal/apperr"
	"github.com/marekbeckmann/torq/internal/clock"
	"github.com/marekbeckmann/torq/internal/db"
	"github.com/marekbeckmann/torq/internal/store"
	"github.com/marekbeckmann/torq/internal/vehicles"
)

// Cost types (data-model §3).
const (
	CostParts  = "parts"
	CostFluids = "fluids"
)

// WarningInconsistentOdometer is returned when the km does not fit the
// vehicle's other service entries.
const WarningInconsistentOdometer = "odometer.inconsistent_with_history"

// Service owns service logs and categories.
type Service struct {
	DB       *db.DB
	Clock    clock.Clock
	Vehicles *vehicles.Service
	// CollectFiles returns attachment storage keys of a log before deletion.
	CollectFiles func(ctx context.Context, tx *sql.Tx, serviceLogID int64) ([]string, error)
	// RemoveFiles deletes collected files after commit.
	RemoveFiles func(keys []string)
}

// ItemInput is one category worked on in a session.
type ItemInput struct {
	CategoryID            int64
	Brand                 string
	Specification         string
	QuantityMl            *int64
	QuantityUnit          string // "L" or "mL"; required when QuantityMl is set
	OEMPartNumber         string
	AftermarketPartNumber string
	CostCents             *int64
	CostType              string // optional; defaults from the category
}

// Input is a complete service session.
type Input struct {
	VehicleID      int64
	ServiceDate    string
	OdometerKm     int64
	Notes          string
	ExtraCostCents int64
	ExtraCostLabel string
	Items          []ItemInput
}

// Log is a service session with its items.
type Log struct {
	store.ServiceLog
	Items []store.ServiceLogItem
}

// TotalCents sums item costs and the extra cost.
func (l Log) TotalCents() int64 {
	total := l.ExtraCostCents
	for _, it := range l.Items {
		if it.CostCents.Valid {
			total += it.CostCents.Int64
		}
	}
	return total
}

func trimLen(v *apperr.Validation, s *string, field string, maxLen int) {
	*s = strings.TrimSpace(*s)
	v.Check(utf8.RuneCountInString(*s) > maxLen, field, "validation.length")
}

func (s *Service) validate(ctx context.Context, q *store.Queries, in *Input) error {
	var v apperr.Validation
	if _, ok := vehicles.ParseDate(in.ServiceDate); !ok {
		v.Add("service_date", "validation.date")
	} else {
		v.Check(in.ServiceDate > s.Clock.Today(), "service_date", "validation.future_date")
	}
	v.Check(in.OdometerKm < 0, "odometer_km", "validation.non_negative")
	v.Check(in.ExtraCostCents < 0, "extra_cost_cents", "validation.non_negative")
	trimLen(&v, &in.Notes, "notes", 10000)
	trimLen(&v, &in.ExtraCostLabel, "extra_cost_label", 120)
	if len(in.Items) == 0 {
		return apperr.Unprocessable("service_log.items_required", nil)
	}
	seen := map[int64]bool{}
	for i := range in.Items {
		it := &in.Items[i]
		prefix := "items." + strconv.Itoa(i) + "."
		if seen[it.CategoryID] {
			return apperr.Unprocessable("service_log.duplicate_category", map[string]any{"index": i})
		}
		seen[it.CategoryID] = true
		cat, err := q.GetCategory(ctx, it.CategoryID)
		if errors.Is(err, sql.ErrNoRows) {
			v.Add(prefix+"category_id", "validation.invalid_choice")
			continue
		}
		if err != nil {
			return err
		}
		if cat.ArchivedAt.Valid {
			return apperr.Unprocessable("service_log.category_archived", map[string]any{"index": i})
		}
		if it.CostType == "" {
			it.CostType = CostParts
			if cat.IsFluid == 1 {
				it.CostType = CostFluids
			}
		}
		v.Check(it.CostType != CostParts && it.CostType != CostFluids, prefix+"cost_type", "validation.invalid_choice")
		if it.QuantityMl != nil {
			v.Check(*it.QuantityMl <= 0, prefix+"quantity_ml", "validation.positive")
			v.Check(it.QuantityUnit != "L" && it.QuantityUnit != "mL", prefix+"quantity_unit", "validation.invalid_choice")
		} else {
			it.QuantityUnit = ""
		}
		v.Check(it.CostCents != nil && *it.CostCents < 0, prefix+"cost_cents", "validation.non_negative")
		trimLen(&v, &it.Brand, prefix+"brand", 120)
		trimLen(&v, &it.Specification, prefix+"specification", 120)
		trimLen(&v, &it.OEMPartNumber, prefix+"oem_part_number", 64)
		trimLen(&v, &it.AftermarketPartNumber, prefix+"aftermarket_part_number", 64)
	}
	return v.Err()
}

func (s *Service) writeItems(ctx context.Context, q *store.Queries, logID int64, items []ItemInput) error {
	for i, it := range items {
		qty, unit := sql.NullInt64{}, sql.NullString{}
		if it.QuantityMl != nil {
			qty = sql.NullInt64{Int64: *it.QuantityMl, Valid: true}
			unit = sql.NullString{String: it.QuantityUnit, Valid: true}
		}
		if err := q.InsertServiceLogItem(ctx, store.InsertServiceLogItemParams{
			ServiceLogID: logID, CategoryID: it.CategoryID, Brand: it.Brand, Specification: it.Specification,
			QuantityMl: qty, QuantityUnit: unit, OemPartNumber: it.OEMPartNumber, AftermarketPartNumber: it.AftermarketPartNumber,
			CostCents: nullInt(it.CostCents), CostType: it.CostType, SortOrder: int64(i + 1),
		}); err != nil {
			return err
		}
	}
	return nil
}

// recordReading replaces the log's odometer reading and returns any history warning.
func (s *Service) recordReading(ctx context.Context, tx *sql.Tx, l store.ServiceLog) ([]string, error) {
	q := store.New(tx)
	if err := q.DeleteServiceLogReading(ctx, sql.NullInt64{Int64: l.ID, Valid: true}); err != nil {
		return nil, err
	}
	readingID, err := s.Vehicles.RecordReading(ctx, tx, l.VehicleID, l.ServiceDate, l.OdometerKm, vehicles.SourceServiceLog)
	if err != nil {
		return nil, err
	}
	if err := q.LinkReadingToServiceLog(ctx, store.LinkReadingToServiceLogParams{ServiceLogID: sql.NullInt64{Int64: l.ID, Valid: true}, ID: readingID}); err != nil {
		return nil, err
	}
	n, err := q.CountInconsistentServiceLogs(ctx, store.CountInconsistentServiceLogsParams{
		VehicleID: l.VehicleID, ID: l.ID, ServiceDate: l.ServiceDate, OdometerKm: l.OdometerKm,
	})
	if err != nil {
		return nil, err
	}
	if n > 0 {
		return []string{WarningInconsistentOdometer}, nil
	}
	return nil, nil
}

func vehicleMustExist(ctx context.Context, q *store.Queries, id int64) error {
	if _, err := vehicles.Get(ctx, q, id); err != nil {
		if e, ok := apperr.As(err); ok && e.Code == "not_found" {
			var v apperr.Validation
			v.Add("vehicle_id", "validation.invalid_choice")
			return v.Err()
		}
		return err
	}
	return nil
}

// Create records a service session, updates the odometer and re-evaluates
// linked reminders in one transaction (FR-018, FR-019).
func (s *Service) Create(ctx context.Context, in Input, createdBy int64) (Log, []string, error) {
	var out Log
	var warnings []string
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		if err := vehicleMustExist(ctx, q, in.VehicleID); err != nil {
			return err
		}
		if err := s.validate(ctx, q, &in); err != nil {
			return err
		}
		now := clock.Timestamp(s.Clock.Now())
		l, err := q.CreateServiceLog(ctx, store.CreateServiceLogParams{
			VehicleID: in.VehicleID, ServiceDate: in.ServiceDate, OdometerKm: in.OdometerKm, Notes: in.Notes,
			ExtraCostCents: in.ExtraCostCents, ExtraCostLabel: in.ExtraCostLabel,
			CreatedBy: sql.NullInt64{Int64: createdBy, Valid: createdBy > 0}, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		if err := s.writeItems(ctx, q, l.ID, in.Items); err != nil {
			return err
		}
		if warnings, err = s.recordReading(ctx, tx, l); err != nil {
			return err
		}
		if err := s.Vehicles.AfterHistoryChange(ctx, tx, l.VehicleID); err != nil {
			return err
		}
		out, err = load(ctx, q, l.ID)
		return err
	})
	return out, warnings, err
}

// Replace overwrites a session including its items (PUT).
func (s *Service) Replace(ctx context.Context, id int64, in Input) (Log, []string, error) {
	var out Log
	var warnings []string
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		cur, err := q.GetServiceLog(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		if err := vehicleMustExist(ctx, q, in.VehicleID); err != nil {
			return err
		}
		if err := s.validate(ctx, q, &in); err != nil {
			return err
		}
		l, err := q.UpdateServiceLog(ctx, store.UpdateServiceLogParams{
			VehicleID: in.VehicleID, ServiceDate: in.ServiceDate, OdometerKm: in.OdometerKm, Notes: in.Notes,
			ExtraCostCents: in.ExtraCostCents, ExtraCostLabel: in.ExtraCostLabel, UpdatedAt: clock.Timestamp(s.Clock.Now()), ID: id,
		})
		if err != nil {
			return err
		}
		if err := q.DeleteServiceLogItems(ctx, id); err != nil {
			return err
		}
		if err := s.writeItems(ctx, q, id, in.Items); err != nil {
			return err
		}
		if warnings, err = s.recordReading(ctx, tx, l); err != nil {
			return err
		}
		if err := s.Vehicles.AfterHistoryChange(ctx, tx, l.VehicleID); err != nil {
			return err
		}
		if cur.VehicleID != l.VehicleID {
			if err := s.Vehicles.AfterHistoryChange(ctx, tx, cur.VehicleID); err != nil {
				return err
			}
		}
		out, err = load(ctx, q, id)
		return err
	})
	return out, warnings, err
}

// Delete removes a session; linked reminders fall back to the previous
// baseline. The current odometer is not lowered.
func (s *Service) Delete(ctx context.Context, id int64) (vehicleID int64, err error) {
	var files []string
	err = s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		cur, err := q.GetServiceLog(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		vehicleID = cur.VehicleID
		if s.CollectFiles != nil {
			if files, err = s.CollectFiles(ctx, tx, id); err != nil {
				return err
			}
		}
		if err := q.DeleteServiceLog(ctx, id); err != nil {
			return err
		}
		return s.Vehicles.AfterHistoryChange(ctx, tx, cur.VehicleID)
	})
	if err == nil && s.RemoveFiles != nil && len(files) > 0 {
		s.RemoveFiles(files)
	}
	return vehicleID, err
}

func load(ctx context.Context, q *store.Queries, id int64) (Log, error) {
	l, err := q.GetServiceLog(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Log{}, apperr.NotFound()
	}
	if err != nil {
		return Log{}, err
	}
	items, err := q.ListServiceLogItems(ctx, id)
	return Log{ServiceLog: l, Items: items}, err
}

// Get returns one session with items.
func (s *Service) Get(ctx context.Context, id int64) (Log, error) {
	return load(ctx, store.New(s.DB.Reader), id)
}

// Filter narrows the logbook list.
type Filter struct {
	VehicleID  int64
	CategoryID int64
	From, To   string
	Q          string
	Limit      int64
	Offset     int64
}

// List returns sessions (newest first) and the total count.
func (s *Service) List(ctx context.Context, f Filter) ([]Log, int64, error) {
	q := store.New(s.DB.Reader)
	rows, err := q.ListServiceLogs(ctx, store.ListServiceLogsParams{
		VehicleID: f.VehicleID, CategoryID: f.CategoryID, FromDate: f.From, ToDate: f.To, Q: f.Q, Lim: f.Limit, Off: f.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := q.CountServiceLogs(ctx, store.CountServiceLogsParams{
		VehicleID: f.VehicleID, CategoryID: f.CategoryID, FromDate: f.From, ToDate: f.To, Q: f.Q,
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Log, 0, len(rows))
	if len(rows) == 0 {
		return out, total, nil
	}
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	items, err := q.ListServiceLogItemsForLogs(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	byLog := map[int64][]store.ServiceLogItem{}
	for _, it := range items {
		byLog[it.ServiceLogID] = append(byLog[it.ServiceLogID], it)
	}
	for _, r := range rows {
		out = append(out, Log{ServiceLog: r, Items: byLog[r.ID]})
	}
	return out, total, nil
}
