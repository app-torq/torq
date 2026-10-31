// Package repairs owns the Self-Repair & Project Log: unscheduled repairs,
// overhauls and modifications with diagnostics and a parts BOM. It must not
// import the servicelog package (Constitution II).
package repairs

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/marekbeckmann/torq/internal/apperr"
	"github.com/marekbeckmann/torq/internal/clock"
	"github.com/marekbeckmann/torq/internal/db"
	"github.com/marekbeckmann/torq/internal/store"
	"github.com/marekbeckmann/torq/internal/vehicles"
)

// Workflow states (FR-022).
const (
	StatusPlanned        = "planned"
	StatusInProgress     = "in_progress"
	StatusWaitingOnParts = "waiting_on_parts"
	StatusCompleted      = "completed"
)

// AllStatuses lists every workflow state.
var AllStatuses = []string{StatusPlanned, StatusInProgress, StatusWaitingOnParts, StatusCompleted}

// Part statuses.
var partStatuses = map[string]bool{"needed": true, "ordered": true, "in_stock": true, "installed": true}

// Service owns repair projects.
type Service struct {
	DB           *db.DB
	Clock        clock.Clock
	Vehicles     *vehicles.Service
	CollectFiles func(ctx context.Context, tx *sql.Tx, repairID int64) ([]string, error)
	RemoveFiles  func(keys []string)
}

// TorqueSpec is a critical torque value recorded for future reference.
type TorqueSpec struct {
	Label   string
	ValueNm float64
	Note    string
}

// Input holds the editable project fields.
type Input struct {
	VehicleID      int64
	Title          string
	Status         string // create only; use SetStatus afterwards
	Symptom        string
	RootCause      string
	ProcedureNotes string
	SpecialTools   string
	StartedOn      string
	CompletedOn    string // edit only, for completed projects
	CompletionKm   *int64 // edit only, for completed projects
	FaultCodes     []string
	TorqueSpecs    []TorqueSpec
}

// Totals summarizes the BOM.
type Totals struct {
	TotalCents   int64
	StatusCounts map[string]int64
}

// Repair is a project with its children.
type Repair struct {
	store.RepairLog
	FaultCodes  []string
	TorqueSpecs []store.RepairTorqueSpec
	Parts       []store.RepairPart
	Totals      Totals
}

func nullStr(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

func nullInt(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

func (s *Service) validate(in *Input) error {
	var v apperr.Validation
	in.Title = strings.TrimSpace(in.Title)
	n := utf8.RuneCountInString(in.Title)
	v.Check(n < 1 || n > 120, "title", "validation.length")
	for field, p := range map[string]*string{"symptom": &in.Symptom, "root_cause": &in.RootCause, "special_tools": &in.SpecialTools} {
		*p = strings.TrimSpace(*p)
		v.Check(utf8.RuneCountInString(*p) > 10000, field, "validation.length")
	}
	v.Check(utf8.RuneCountInString(in.ProcedureNotes) > 50000, "procedure_notes", "validation.length")
	today := s.Clock.Today()
	for field, d := range map[string]string{"started_on": in.StartedOn, "completed_on": in.CompletedOn} {
		if d == "" {
			continue
		}
		if _, ok := vehicles.ParseDate(d); !ok {
			v.Add(field, "validation.date")
		} else {
			v.Check(d > today, field, "validation.future_date")
		}
	}
	if in.StartedOn != "" && in.CompletedOn != "" && in.CompletedOn < in.StartedOn {
		v.Add("completed_on", "repair.completed_before_started")
	}
	codes := make([]string, 0, len(in.FaultCodes))
	seen := map[string]bool{}
	for _, c := range in.FaultCodes {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c == "" || seen[c] {
			continue
		}
		v.Check(len(c) > 16, "fault_codes", "validation.length")
		seen[c] = true
		codes = append(codes, c)
	}
	in.FaultCodes = codes
	for i := range in.TorqueSpecs {
		ts := &in.TorqueSpecs[i]
		ts.Label = strings.TrimSpace(ts.Label)
		ts.Note = strings.TrimSpace(ts.Note)
		n := utf8.RuneCountInString(ts.Label)
		if n < 1 || n > 120 || ts.ValueNm <= 0 || utf8.RuneCountInString(ts.Note) > 200 {
			v.Add("torque_specs", "repair.torque_spec_invalid")
		}
	}
	return v.Err()
}

func (s *Service) writeChildren(ctx context.Context, q *store.Queries, id int64, in Input) error {
	if err := q.DeleteFaultCodes(ctx, id); err != nil {
		return err
	}
	for _, c := range in.FaultCodes {
		if err := q.InsertFaultCode(ctx, store.InsertFaultCodeParams{RepairLogID: id, Code: c}); err != nil {
			return err
		}
	}
	if err := q.DeleteTorqueSpecs(ctx, id); err != nil {
		return err
	}
	for i, ts := range in.TorqueSpecs {
		if err := q.InsertTorqueSpec(ctx, store.InsertTorqueSpecParams{
			RepairLogID: id, Label: ts.Label, ValueNm: ts.ValueNm, Note: ts.Note, SortOrder: int64(i + 1),
		}); err != nil {
			return err
		}
	}
	return nil
}

// Create starts a project. Completed projects must be created via SetStatus.
func (s *Service) Create(ctx context.Context, in Input, createdBy int64) (Repair, error) {
	if in.Status == "" {
		in.Status = StatusPlanned
	}
	if in.Status == StatusCompleted || !validStatus(in.Status) {
		var v apperr.Validation
		v.Add("status", "validation.invalid_choice")
		return Repair{}, v.Err()
	}
	in.CompletedOn, in.CompletionKm = "", nil
	if err := s.validate(&in); err != nil {
		return Repair{}, err
	}
	if in.Status == StatusInProgress && in.StartedOn == "" {
		in.StartedOn = s.Clock.Today()
	}
	var out Repair
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		if _, err := vehicles.Get(ctx, q, in.VehicleID); err != nil {
			if e, ok := apperr.As(err); ok && e.Code == "not_found" {
				var v apperr.Validation
				v.Add("vehicle_id", "validation.invalid_choice")
				return v.Err()
			}
			return err
		}
		now := clock.Timestamp(s.Clock.Now())
		r, err := q.CreateRepair(ctx, store.CreateRepairParams{
			VehicleID: in.VehicleID, Title: in.Title, Status: in.Status, Symptom: in.Symptom, RootCause: in.RootCause,
			ProcedureNotes: in.ProcedureNotes, SpecialTools: in.SpecialTools, StartedOn: nullStr(in.StartedOn),
			CreatedBy: sql.NullInt64{Int64: createdBy, Valid: createdBy > 0}, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		if err := s.writeChildren(ctx, q, r.ID, in); err != nil {
			return err
		}
		out, err = load(ctx, q, r.ID)
		return err
	})
	return out, err
}

func validStatus(s string) bool {
	for _, x := range AllStatuses {
		if x == s {
			return true
		}
	}
	return false
}

// Update edits a project's fields (not its status or vehicle).
func (s *Service) Update(ctx context.Context, id int64, in Input) (Repair, error) {
	if err := s.validate(&in); err != nil {
		return Repair{}, err
	}
	var out Repair
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		cur, err := q.GetRepair(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		completedOn, completionKm := cur.CompletedOn, cur.CompletionOdometerKm
		if cur.Status == StatusCompleted {
			if in.CompletedOn != "" {
				completedOn = nullStr(in.CompletedOn)
			}
			if in.CompletionKm != nil {
				completionKm = nullInt(in.CompletionKm)
			}
		}
		if _, err := q.UpdateRepair(ctx, store.UpdateRepairParams{
			Title: in.Title, Symptom: in.Symptom, RootCause: in.RootCause, ProcedureNotes: in.ProcedureNotes,
			SpecialTools: in.SpecialTools, StartedOn: nullStr(in.StartedOn), CompletedOn: completedOn,
			CompletionOdometerKm: completionKm, UpdatedAt: clock.Timestamp(s.Clock.Now()), ID: id,
		}); err != nil {
			if strings.Contains(err.Error(), "CHECK constraint failed") {
				return apperr.Unprocessable("repair.completed_before_started", nil)
			}
			return err
		}
		if err := s.writeChildren(ctx, q, id, in); err != nil {
			return err
		}
		if cur.Status == StatusCompleted && (completedOn != cur.CompletedOn || completionKm != cur.CompletionOdometerKm) {
			if err := s.recordCompletion(ctx, tx, id, cur.VehicleID, completedOn.String, completionKm.Int64); err != nil {
				return err
			}
		}
		out, err = load(ctx, q, id)
		return err
	})
	return out, err
}

func (s *Service) recordCompletion(ctx context.Context, tx *sql.Tx, repairID, vehicleID int64, date string, km int64) error {
	q := store.New(tx)
	if err := q.DeleteRepairReading(ctx, sql.NullInt64{Int64: repairID, Valid: true}); err != nil {
		return err
	}
	readingID, err := s.Vehicles.RecordReading(ctx, tx, vehicleID, date, km, vehicles.SourceRepairCompletion)
	if err != nil {
		return err
	}
	if err := q.LinkReadingToRepair(ctx, store.LinkReadingToRepairParams{RepairLogID: sql.NullInt64{Int64: repairID, Valid: true}, ID: readingID}); err != nil {
		return err
	}
	return s.Vehicles.AfterHistoryChange(ctx, tx, vehicleID)
}

// SetStatus moves a project through the workflow (data-model §4):
// completing requires a date and odometer and records an odometer reading;
// reopening removes that reading but keeps the completion values.
func (s *Service) SetStatus(ctx context.Context, id int64, status, completedOn string, completionKm *int64) (Repair, error) {
	if !validStatus(status) {
		var v apperr.Validation
		v.Add("status", "validation.invalid_choice")
		return Repair{}, v.Err()
	}
	var out Repair
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		cur, err := q.GetRepair(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		started, done, km := cur.StartedOn, cur.CompletedOn, cur.CompletionOdometerKm
		today := s.Clock.Today()
		if status == StatusInProgress && !started.Valid {
			start := today
			// A reopened project keeps its completion date; the start must not come after it.
			if done.Valid && done.String < start {
				start = done.String
			}
			started = sql.NullString{String: start, Valid: true}
		}
		if status == StatusCompleted {
			var v apperr.Validation
			if completedOn == "" {
				v.Add("completed_on", "validation.required")
			} else if _, ok := vehicles.ParseDate(completedOn); !ok {
				v.Add("completed_on", "validation.date")
			} else if completedOn > today {
				v.Add("completed_on", "validation.future_date")
			} else if started.Valid && completedOn < started.String {
				v.Add("completed_on", "repair.completed_before_started")
			}
			if completionKm == nil {
				v.Add("completion_odometer_km", "validation.required")
			} else if *completionKm < 0 {
				v.Add("completion_odometer_km", "validation.non_negative")
			}
			if err := v.Err(); err != nil {
				return err
			}
			done = sql.NullString{String: completedOn, Valid: true}
			km = nullInt(completionKm)
		}
		if _, err := q.SetRepairStatus(ctx, store.SetRepairStatusParams{
			Status: status, StartedOn: started, CompletedOn: done, CompletionOdometerKm: km,
			UpdatedAt: clock.Timestamp(s.Clock.Now()), ID: id,
		}); err != nil {
			return err
		}
		switch {
		case status == StatusCompleted:
			if err := s.recordCompletion(ctx, tx, id, cur.VehicleID, done.String, km.Int64); err != nil {
				return err
			}
		case cur.Status == StatusCompleted:
			if err := q.DeleteRepairReading(ctx, sql.NullInt64{Int64: id, Valid: true}); err != nil {
				return err
			}
			if err := s.Vehicles.AfterHistoryChange(ctx, tx, cur.VehicleID); err != nil {
				return err
			}
		}
		out, err = load(ctx, q, id)
		return err
	})
	return out, err
}

// Delete removes a project with its parts, specs and attachments.
func (s *Service) Delete(ctx context.Context, id int64) error {
	var files []string
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		cur, err := q.GetRepair(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		if s.CollectFiles != nil {
			if files, err = s.CollectFiles(ctx, tx, id); err != nil {
				return err
			}
		}
		if err := q.DeleteRepair(ctx, id); err != nil {
			return err
		}
		return s.Vehicles.AfterHistoryChange(ctx, tx, cur.VehicleID)
	})
	if err == nil && s.RemoveFiles != nil && len(files) > 0 {
		s.RemoveFiles(files)
	}
	return err
}

// Get returns a project with children and totals.
func (s *Service) Get(ctx context.Context, id int64) (Repair, error) {
	return load(ctx, store.New(s.DB.Reader), id)
}

func load(ctx context.Context, q *store.Queries, id int64) (Repair, error) {
	r, err := q.GetRepair(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Repair{}, apperr.NotFound()
	}
	if err != nil {
		return Repair{}, err
	}
	out := Repair{RepairLog: r}
	codes, err := q.ListFaultCodes(ctx, []int64{id})
	if err != nil {
		return Repair{}, err
	}
	for _, c := range codes {
		out.FaultCodes = append(out.FaultCodes, c.Code)
	}
	if out.TorqueSpecs, err = q.ListTorqueSpecs(ctx, id); err != nil {
		return Repair{}, err
	}
	if out.Parts, err = q.ListParts(ctx, id); err != nil {
		return Repair{}, err
	}
	totals, err := totalsFor(ctx, q, []int64{id})
	if err != nil {
		return Repair{}, err
	}
	out.Totals = totals[id]
	return out, nil
}

func totalsFor(ctx context.Context, q *store.Queries, ids []int64) (map[int64]Totals, error) {
	rows, err := q.RepairTotals(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]Totals, len(ids))
	for _, id := range ids {
		out[id] = Totals{StatusCounts: map[string]int64{"needed": 0, "ordered": 0, "in_stock": 0, "installed": 0}}
	}
	for _, r := range rows {
		out[r.RepairLogID] = Totals{TotalCents: r.TotalCents, StatusCounts: map[string]int64{
			"needed": r.Needed, "ordered": r.Ordered, "in_stock": r.InStock, "installed": r.Installed,
		}}
	}
	return out, nil
}

// Filter narrows the project list.
type Filter struct {
	VehicleID int64
	Statuses  []string
	Q         string
	Limit     int64
	Offset    int64
}

// List returns projects (active first) with fault codes and BOM totals, plus the total count.
func (s *Service) List(ctx context.Context, f Filter) ([]Repair, int64, error) {
	statuses := f.Statuses
	if len(statuses) == 0 {
		statuses = AllStatuses
	}
	for _, st := range statuses {
		if !validStatus(st) {
			return nil, 0, apperr.BadRequest("request.malformed")
		}
	}
	q := store.New(s.DB.Reader)
	joined, query := strings.Join(statuses, ","), strings.TrimSpace(f.Q)
	rows, err := q.ListRepairs(ctx, store.ListRepairsParams{VehicleID: f.VehicleID, Statuses: joined, Q: query, Lim: f.Limit, Off: f.Offset})
	if err != nil {
		return nil, 0, err
	}
	total, err := q.CountRepairs(ctx, store.CountRepairsParams{VehicleID: f.VehicleID, Statuses: joined, Q: query})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Repair, 0, len(rows))
	if len(rows) == 0 {
		return out, total, nil
	}
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	codes, err := q.ListFaultCodes(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	byRepair := map[int64][]string{}
	for _, c := range codes {
		byRepair[c.RepairLogID] = append(byRepair[c.RepairLogID], c.Code)
	}
	totals, err := totalsFor(ctx, q, ids)
	if err != nil {
		return nil, 0, err
	}
	for _, r := range rows {
		out = append(out, Repair{RepairLog: r, FaultCodes: byRepair[r.ID], Totals: totals[r.ID]})
	}
	return out, total, nil
}

// ValidURL reports whether s is empty or an absolute http(s) URL.
func ValidURL(s string) bool {
	if s == "" {
		return true
	}
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
