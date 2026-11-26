package httpapi

import (
	"context"
	"net/http"
	"slices"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/repairs"
	"github.com/app-torq/torq/internal/store"
)

type torqueSpecJSON struct {
	Label   string  `json:"label"`
	ValueNm float64 `json:"value_nm"`
	Note    string  `json:"note"`
}

type partJSON struct {
	ID                    int64  `json:"id"`
	Name                  string `json:"name"`
	OEMPartNumber         string `json:"oem_part_number"`
	AftermarketBrand      string `json:"aftermarket_brand"`
	AftermarketPartNumber string `json:"aftermarket_part_number"`
	SupplierURL           string `json:"supplier_url"`
	Quantity              int64  `json:"quantity"`
	UnitCostCents         *int64 `json:"unit_cost_cents"`
	LineTotalCents        int64  `json:"line_total_cents"`
	CostType              string `json:"cost_type"`
	Status                string `json:"status"`
	SortOrder             int64  `json:"sort_order"`
}

func toPartJSON(p store.RepairPart) partJSON {
	out := partJSON{
		ID: p.ID, Name: p.Name, OEMPartNumber: p.OemPartNumber, AftermarketBrand: p.AftermarketBrand,
		AftermarketPartNumber: p.AftermarketPartNumber, SupplierURL: p.SupplierUrl, Quantity: p.Quantity,
		UnitCostCents: intPtr(p.UnitCostCents), CostType: p.CostType, Status: p.Status, SortOrder: p.SortOrder,
	}
	if p.UnitCostCents.Valid {
		out.LineTotalCents = p.Quantity * p.UnitCostCents.Int64
	}
	return out
}

type repairSummaryJSON struct {
	ID                   int64   `json:"id"`
	VehicleID            int64   `json:"vehicle_id"`
	Title                string  `json:"title"`
	Status               string  `json:"status"`
	PriorityID           *int64  `json:"priority_id"`
	StartedOn            *string `json:"started_on"`
	CompletedOn          *string `json:"completed_on"`
	CompletionOdometerKm *int64  `json:"completion_odometer_km"`
	// CompletionOdometerRecorded is false when the project was completed on a
	// vehicle without an odometer; completion_odometer_km is then 0.
	CompletionOdometerRecorded bool               `json:"completion_odometer_recorded"`
	CompletionMeterReadings    []meterReadingJSON `json:"completion_meter_readings"`
	FaultCodes                 []string           `json:"fault_codes"`
	Tags                       []string           `json:"tags"`
	PartsTotalCents            int64              `json:"parts_total_cents"`
	PartStatusCounts           map[string]int64   `json:"part_status_counts"`
	UpdatedAt                  string             `json:"updated_at"`
}

type repairJSON struct {
	repairSummaryJSON
	Symptom        string           `json:"symptom"`
	RootCause      string           `json:"root_cause"`
	ProcedureNotes string           `json:"procedure_notes"`
	SpecialTools   string           `json:"special_tools"`
	TorqueSpecs    []torqueSpecJSON `json:"torque_specs"`
	Parts          []partJSON       `json:"parts"`
	Attachments    []attachmentJSON `json:"attachments"`
	CreatedAt      string           `json:"created_at"`
}

func toRepairSummary(r repairs.Repair) repairSummaryJSON {
	codes, tags := r.FaultCodes, r.Tags
	if codes == nil {
		codes = []string{}
	}
	if tags == nil {
		tags = []string{}
	}
	return repairSummaryJSON{
		ID: r.ID, VehicleID: r.VehicleID, Title: r.Title, Status: r.Status, PriorityID: intPtr(r.PriorityID), StartedOn: strPtr(r.StartedOn),
		CompletedOn: strPtr(r.CompletedOn), CompletionOdometerKm: intPtr(r.CompletionOdometerKm), FaultCodes: codes, Tags: tags,
		CompletionOdometerRecorded: r.CompletionOdometerRecorded == 1, CompletionMeterReadings: readingsOut(r.CompletionMeterReadings),
		PartsTotalCents: r.Totals.TotalCents, PartStatusCounts: r.Totals.StatusCounts, UpdatedAt: r.UpdatedAt,
	}
}

func (s *Server) toRepairJSON(ctx context.Context, r repairs.Repair) (repairJSON, error) {
	out := repairJSON{
		repairSummaryJSON: toRepairSummary(r), Symptom: r.Symptom, RootCause: r.RootCause,
		ProcedureNotes: r.ProcedureNotes, SpecialTools: r.SpecialTools, CreatedAt: r.CreatedAt,
		TorqueSpecs: []torqueSpecJSON{}, Parts: []partJSON{},
	}
	for _, ts := range r.TorqueSpecs {
		out.TorqueSpecs = append(out.TorqueSpecs, torqueSpecJSON{Label: ts.Label, ValueNm: ts.ValueNm, Note: ts.Note})
	}
	for _, p := range r.Parts {
		out.Parts = append(out.Parts, toPartJSON(p))
	}
	var err error
	out.Attachments, err = s.repairAttachments(ctx, r.ID)
	visibilityOf(ctx).repair(&out)
	return out, err
}

func (s *Server) openRepairs(ctx context.Context, visible []int64) ([]repairSummaryJSON, error) {
	rows, err := s.repairs.OpenProjects(ctx, visible)
	if err != nil {
		return nil, err
	}
	v := visibilityOf(ctx)
	out := make([]repairSummaryJSON, 0, len(rows))
	for _, r := range rows {
		j := toRepairSummary(r)
		v.repairSummary(&j)
		out = append(out, j)
	}
	return out, nil
}

type repairInput struct {
	VehicleID            int64            `json:"vehicle_id"`
	Title                string           `json:"title"`
	Status               string           `json:"status"`
	Symptom              string           `json:"symptom"`
	RootCause            string           `json:"root_cause"`
	ProcedureNotes       string           `json:"procedure_notes"`
	SpecialTools         string           `json:"special_tools"`
	StartedOn            *string          `json:"started_on"`
	CompletedOn          *string          `json:"completed_on"`
	CompletionOdometerKm *int64           `json:"completion_odometer_km"`
	FaultCodes           []string         `json:"fault_codes"`
	Tags                 []string         `json:"tags"`
	TorqueSpecs          []torqueSpecJSON `json:"torque_specs"`
	// CompletionMeterReadings replaces the hours of a completed project when sent.
	CompletionMeterReadings *[]meterReadingJSON `json:"completion_meter_readings"`
	// PriorityID: omitted means the garage default on create and unchanged on update; null means none.
	PriorityID optionalID `json:"priority_id"`
}

func (in repairInput) domain() (repairs.Input, error) {
	out := repairs.Input{
		VehicleID: in.VehicleID, Title: in.Title, Status: in.Status, Symptom: in.Symptom, RootCause: in.RootCause,
		ProcedureNotes: in.ProcedureNotes, SpecialTools: in.SpecialTools, FaultCodes: in.FaultCodes, Tags: in.Tags,
		CompletionKm: in.CompletionOdometerKm, PriorityID: in.PriorityID.Value, DefaultPriority: !in.PriorityID.Set,
	}
	if in.StartedOn != nil {
		out.StartedOn = *in.StartedOn
	}
	if in.CompletedOn != nil {
		out.CompletedOn = *in.CompletedOn
	}
	for _, ts := range in.TorqueSpecs {
		out.TorqueSpecs = append(out.TorqueSpecs, repairs.TorqueSpec{Label: ts.Label, ValueNm: ts.ValueNm, Note: ts.Note})
	}
	if in.CompletionMeterReadings != nil {
		var v apperr.Validation
		out.CompletionMeterReadings = readingsIn(*in.CompletionMeterReadings, "completion_meter_readings", &v)
		if err := v.Err(); err != nil {
			return repairs.Input{}, err
		}
	}
	return out, nil
}

func (s *Server) writeRepair(w http.ResponseWriter, r *http.Request, status int, rep repairs.Repair) error {
	out, err := s.toRepairJSON(r.Context(), rep)
	if err != nil {
		return err
	}
	writeJSON(w, status, map[string]any{"repair": out})
	return nil
}

func (s *Server) listRepairs(w http.ResponseWriter, r *http.Request) error {
	limit, offset, err := pagination(r)
	if err != nil {
		return err
	}
	vehicleID, err := queryInt(r, "vehicle_id")
	if err != nil {
		return err
	}
	q := r.URL.Query()
	sort := q.Get("sort")
	if sort != "" && !slices.Contains(repairs.Sorts, sort) {
		return badQuery()
	}
	rows, total, err := s.repairs.List(r.Context(), repairs.Filter{
		VehicleID: vehicleID, Statuses: q["status"], Priorities: q["priority"], Tags: q["tag"], Sort: sort, Q: q.Get("q"), Limit: limit, Offset: offset,
		VehicleIDs: visibleVehicles(r),
	})
	if err != nil {
		return err
	}
	items := make([]repairSummaryJSON, 0, len(rows))
	for _, rep := range rows {
		j := toRepairSummary(rep)
		visibilityOf(r.Context()).repairSummary(&j)
		items = append(items, j)
	}
	writeJSON(w, http.StatusOK, newList(items, total))
	return nil
}

func (s *Server) createRepair(w http.ResponseWriter, r *http.Request) error {
	var in repairInput
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	if in.VehicleID != 0 { // 0 fails validation below
		if err := s.guard(r, in.VehicleID); err != nil {
			return err
		}
	}
	if !visibilityOf(r.Context()).notes {
		in.ProcedureNotes = "" // what the user can't see stays empty (#49)
	}
	d, err := in.domain()
	if err != nil {
		return err
	}
	rep, err := s.repairs.Create(r.Context(), d, currentUser(r).ID)
	if err != nil {
		return err
	}
	return s.writeRepair(w, r, http.StatusCreated, rep)
}

func (s *Server) getRepair(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathRepair(r)
	if err != nil {
		return err
	}
	rep, err := s.repairs.Get(r.Context(), id)
	if err != nil {
		return err
	}
	return s.writeRepair(w, r, http.StatusOK, rep)
}

func (s *Server) updateRepair(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathRepair(r)
	if err != nil {
		return err
	}
	cur, err := s.repairs.Get(r.Context(), id)
	if err != nil {
		return err
	}
	// PATCH: overlay the body on the current values; lists present in the body replace.
	in := repairInput{
		VehicleID: cur.VehicleID, Title: cur.Title, Symptom: cur.Symptom, RootCause: cur.RootCause,
		ProcedureNotes: cur.ProcedureNotes, SpecialTools: cur.SpecialTools, StartedOn: strPtr(cur.StartedOn),
		FaultCodes: cur.FaultCodes, Tags: cur.Tags, PriorityID: optionalID{Set: true, Value: intPtr(cur.PriorityID)},
	}
	for _, ts := range cur.TorqueSpecs {
		in.TorqueSpecs = append(in.TorqueSpecs, torqueSpecJSON{Label: ts.Label, ValueNm: ts.ValueNm, Note: ts.Note})
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	if in.Status != "" {
		return apperr.BadRequest("repair.status_use_status_endpoint")
	}
	if in.VehicleID != cur.VehicleID {
		return apperr.BadRequest("repair.vehicle_immutable")
	}
	if !visibilityOf(r.Context()).notes {
		in.ProcedureNotes = cur.ProcedureNotes // can't change what it can't see (#49)
	}
	d, err := in.domain()
	if err != nil {
		return err
	}
	rep, err := s.repairs.Update(r.Context(), id, d)
	if err != nil {
		return err
	}
	return s.writeRepair(w, r, http.StatusOK, rep)
}

func (s *Server) setRepairStatus(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathRepair(r)
	if err != nil {
		return err
	}
	var in struct {
		Status                  string             `json:"status"`
		CompletedOn             string             `json:"completed_on"`
		CompletionOdometerKm    *int64             `json:"completion_odometer_km"`
		CompletionMeterReadings []meterReadingJSON `json:"completion_meter_readings"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	var verr apperr.Validation
	readings := readingsIn(in.CompletionMeterReadings, "completion_meter_readings", &verr)
	if err := verr.Err(); err != nil {
		return err
	}
	rep, err := s.repairs.SetStatusWithReadings(r.Context(), id, in.Status, in.CompletedOn, in.CompletionOdometerKm, readings)
	if err != nil {
		return err
	}
	out, err := s.toRepairJSON(r.Context(), rep)
	if err != nil {
		return err
	}
	v, err := s.vehicles.Get(r.Context(), rep.VehicleID)
	if err != nil {
		return err
	}
	vj, err := s.vehicleOut(r.Context(), v)
	if err != nil {
		return err
	}
	rems, err := s.vehicleReminderSummaries(r.Context(), rep.VehicleID)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"repair": out, "vehicle": vj, "reminders": visibilityOf(r.Context()).reminderList(rems)})
	return nil
}

func (s *Server) deleteRepair(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathRepair(r)
	if err != nil {
		return err
	}
	if err := s.repairs.Delete(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type partInput struct {
	Name                  string `json:"name"`
	OEMPartNumber         string `json:"oem_part_number"`
	AftermarketBrand      string `json:"aftermarket_brand"`
	AftermarketPartNumber string `json:"aftermarket_part_number"`
	SupplierURL           string `json:"supplier_url"`
	Quantity              int64  `json:"quantity"`
	UnitCostCents         *int64 `json:"unit_cost_cents"`
	CostType              string `json:"cost_type"`
	Status                string `json:"status"`
}

func (in partInput) domain() repairs.PartInput {
	return repairs.PartInput{
		Name: in.Name, OEMPartNumber: in.OEMPartNumber, AftermarketBrand: in.AftermarketBrand,
		AftermarketPartNumber: in.AftermarketPartNumber, SupplierURL: in.SupplierURL, Quantity: in.Quantity,
		UnitCostCents: in.UnitCostCents, CostType: in.CostType, Status: in.Status,
	}
}

func (s *Server) createPart(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathRepair(r)
	if err != nil {
		return err
	}
	var in partInput
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	if !visibilityOf(r.Context()).costs {
		in.UnitCostCents = nil // what the user can't see stays empty (#49)
	}
	p, err := s.repairs.CreatePart(r.Context(), id, in.domain())
	if err != nil {
		return err
	}
	j := toPartJSON(p)
	visibilityOf(r.Context()).part(&j)
	writeJSON(w, http.StatusCreated, map[string]any{"part": j})
	return nil
}

func (s *Server) updatePart(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathPart(r)
	if err != nil {
		return err
	}
	cur, err := s.repairs.GetPart(r.Context(), id)
	if err != nil {
		return err
	}
	in := partInput{
		Name: cur.Name, OEMPartNumber: cur.OemPartNumber, AftermarketBrand: cur.AftermarketBrand,
		AftermarketPartNumber: cur.AftermarketPartNumber, SupplierURL: cur.SupplierUrl, Quantity: cur.Quantity,
		UnitCostCents: intPtr(cur.UnitCostCents), CostType: cur.CostType, Status: cur.Status,
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	if !visibilityOf(r.Context()).costs {
		in.UnitCostCents, in.CostType = intPtr(cur.UnitCostCents), cur.CostType // can't change what it can't see (#49)
	}
	p, err := s.repairs.UpdatePart(r.Context(), id, in.domain())
	if err != nil {
		return err
	}
	j := toPartJSON(p)
	visibilityOf(r.Context()).part(&j)
	writeJSON(w, http.StatusOK, map[string]any{"part": j})
	return nil
}

func (s *Server) deletePart(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathPart(r)
	if err != nil {
		return err
	}
	if err := s.repairs.DeletePart(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) reorderParts(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathRepair(r)
	if err != nil {
		return err
	}
	var in struct {
		IDs []int64 `json:"ids"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	parts, err := s.repairs.ReorderParts(r.Context(), id, in.IDs)
	if err != nil {
		return err
	}
	out := make([]partJSON, 0, len(parts))
	for _, p := range parts {
		j := toPartJSON(p)
		visibilityOf(r.Context()).part(&j)
		out = append(out, j)
	}
	writeJSON(w, http.StatusOK, map[string]any{"parts": out})
	return nil
}

// pathRepair parses the {id} wildcard as a repair project and checks its
// vehicle for the route's permission.
func (s *Server) pathRepair(r *http.Request) (int64, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return 0, err
	}
	return id, s.guardRepair(r, id)
}

func (s *Server) guardRepair(r *http.Request, id int64) error {
	rep, err := s.repairs.Get(r.Context(), id)
	if err != nil {
		return err
	}
	return s.guard(r, rep.VehicleID)
}

// pathPart parses the {id} wildcard as a project part and checks the
// project's vehicle for the route's permission.
func (s *Server) pathPart(r *http.Request) (int64, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return 0, err
	}
	p, err := s.repairs.GetPart(r.Context(), id)
	if err != nil {
		return 0, err
	}
	return id, s.guardRepair(r, p.RepairLogID)
}
