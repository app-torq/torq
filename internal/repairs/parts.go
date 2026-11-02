package repairs

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/store"
)

// PartInput is one BOM line (FR-024).
type PartInput struct {
	Name                  string
	OEMPartNumber         string
	AftermarketBrand      string
	AftermarketPartNumber string
	SupplierURL           string
	Quantity              int64
	UnitCostCents         *int64
	CostType              string
	Status                string
}

func validatePart(in *PartInput) error {
	var v apperr.Validation
	in.Name = strings.TrimSpace(in.Name)
	n := utf8.RuneCountInString(in.Name)
	v.Check(n < 1 || n > 120, "name", "validation.length")
	for field, p := range map[string]*string{
		"oem_part_number": &in.OEMPartNumber, "aftermarket_brand": &in.AftermarketBrand,
		"aftermarket_part_number": &in.AftermarketPartNumber,
	} {
		*p = strings.TrimSpace(*p)
		v.Check(utf8.RuneCountInString(*p) > 64, field, "validation.length")
	}
	in.SupplierURL = strings.TrimSpace(in.SupplierURL)
	v.Check(len(in.SupplierURL) > 2048 || !ValidURL(in.SupplierURL), "supplier_url", "validation.url")
	if in.Quantity == 0 {
		in.Quantity = 1
	}
	v.Check(in.Quantity < 1, "quantity", "validation.positive")
	v.Check(in.UnitCostCents != nil && *in.UnitCostCents < 0, "unit_cost_cents", "validation.non_negative")
	if in.CostType == "" {
		in.CostType = "parts"
	}
	v.Check(in.CostType != "parts" && in.CostType != "fluids", "cost_type", "validation.invalid_choice")
	if in.Status == "" {
		in.Status = "needed"
	}
	v.Check(!partStatuses[in.Status], "status", "validation.invalid_choice")
	return v.Err()
}

func (s *Service) touch(ctx context.Context, q *store.Queries, repairID int64) error {
	r, err := q.GetRepair(ctx, repairID)
	if errors.Is(err, sql.ErrNoRows) {
		return apperr.NotFound()
	}
	if err != nil {
		return err
	}
	_, err = q.UpdateRepair(ctx, store.UpdateRepairParams{
		Title: r.Title, Symptom: r.Symptom, RootCause: r.RootCause, ProcedureNotes: r.ProcedureNotes,
		SpecialTools: r.SpecialTools, StartedOn: r.StartedOn, CompletedOn: r.CompletedOn,
		CompletionOdometerKm: r.CompletionOdometerKm, UpdatedAt: clock.Timestamp(s.Clock.Now()), ID: repairID,
	})
	return err
}

// CreatePart appends a BOM line.
func (s *Service) CreatePart(ctx context.Context, repairID int64, in PartInput) (store.RepairPart, error) {
	if err := validatePart(&in); err != nil {
		return store.RepairPart{}, err
	}
	var out store.RepairPart
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		if err := s.touch(ctx, q, repairID); err != nil {
			return err
		}
		var err error
		out, err = q.CreatePart(ctx, store.CreatePartParams{
			RepairLogID: repairID, Name: in.Name, OemPartNumber: in.OEMPartNumber, AftermarketBrand: in.AftermarketBrand,
			AftermarketPartNumber: in.AftermarketPartNumber, SupplierUrl: in.SupplierURL, Quantity: in.Quantity,
			UnitCostCents: nullInt(in.UnitCostCents), CostType: in.CostType, Status: in.Status, ParentID: repairID,
		})
		return err
	})
	return out, err
}

// GetPart returns one BOM line.
func (s *Service) GetPart(ctx context.Context, id int64) (store.RepairPart, error) {
	p, err := store.New(s.DB.Reader).GetPart(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return store.RepairPart{}, apperr.NotFound()
	}
	return p, err
}

// UpdatePart replaces a BOM line's fields.
func (s *Service) UpdatePart(ctx context.Context, id int64, in PartInput) (store.RepairPart, error) {
	if err := validatePart(&in); err != nil {
		return store.RepairPart{}, err
	}
	var out store.RepairPart
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		cur, err := q.GetPart(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		out, err = q.UpdatePart(ctx, store.UpdatePartParams{
			Name: in.Name, OemPartNumber: in.OEMPartNumber, AftermarketBrand: in.AftermarketBrand,
			AftermarketPartNumber: in.AftermarketPartNumber, SupplierUrl: in.SupplierURL, Quantity: in.Quantity,
			UnitCostCents: nullInt(in.UnitCostCents), CostType: in.CostType, Status: in.Status, ID: id,
		})
		if err != nil {
			return err
		}
		return s.touch(ctx, q, cur.RepairLogID)
	})
	return out, err
}

// DeletePart removes a BOM line.
func (s *Service) DeletePart(ctx context.Context, id int64) error {
	return s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		cur, err := q.GetPart(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		if err := q.DeletePart(ctx, id); err != nil {
			return err
		}
		return s.touch(ctx, q, cur.RepairLogID)
	})
}

// ReorderParts rewrites sort_order; ids must list every part exactly once.
func (s *Service) ReorderParts(ctx context.Context, repairID int64, ids []int64) ([]store.RepairPart, error) {
	var out []store.RepairPart
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		parts, err := q.ListParts(ctx, repairID)
		if err != nil {
			return err
		}
		if len(parts) != len(ids) {
			return apperr.Unprocessable("parts.order_mismatch", nil)
		}
		known := map[int64]bool{}
		for _, p := range parts {
			known[p.ID] = true
		}
		seen := map[int64]bool{}
		for i, id := range ids {
			if !known[id] || seen[id] {
				return apperr.Unprocessable("parts.order_mismatch", nil)
			}
			seen[id] = true
			if err := q.SetPartOrder(ctx, store.SetPartOrderParams{SortOrder: int64(i + 1), ID: id, RepairLogID: repairID}); err != nil {
				return err
			}
		}
		out, err = q.ListParts(ctx, repairID)
		return err
	})
	return out, err
}

// OpenProjects returns up to 10 projects that are not completed.
func (s *Service) OpenProjects(ctx context.Context) ([]Repair, error) {
	rows, _, err := s.List(ctx, Filter{Statuses: []string{StatusInProgress, StatusWaitingOnParts, StatusPlanned}, Limit: 10})
	return rows, err
}
