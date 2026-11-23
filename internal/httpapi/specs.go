package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/specfill"
	"github.com/app-torq/torq/internal/store"
	"github.com/app-torq/torq/internal/vehicles"
)

type specsJSON struct {
	EngineOilViscosity          string   `json:"engine_oil_viscosity"`
	EngineOilSpec               string   `json:"engine_oil_spec"`
	EngineOilCapacityMl         *int64   `json:"engine_oil_capacity_ml"`
	TransmissionFluidSpec       string   `json:"transmission_fluid_spec"`
	TransmissionFluidCapacityMl *int64   `json:"transmission_fluid_capacity_ml"`
	DifferentialFluidSpec       string   `json:"differential_fluid_spec"`
	DifferentialFluidCapacityMl *int64   `json:"differential_fluid_capacity_ml"`
	BrakeFluidSpec              string   `json:"brake_fluid_spec"`
	BrakeFluidCapacityMl        *int64   `json:"brake_fluid_capacity_ml"`
	CoolantSpec                 string   `json:"coolant_spec"`
	CoolantCapacityMl           *int64   `json:"coolant_capacity_ml"`
	WheelLugTorqueNm            *float64 `json:"wheel_lug_torque_nm"`
	TirePressureFrontBar        *float64 `json:"tire_pressure_front_bar"`
	TirePressureRearBar         *float64 `json:"tire_pressure_rear_bar"`
	TirePressureFrontMaxBar     *float64 `json:"tire_pressure_front_max_bar"`
	TirePressureRearMaxBar      *float64 `json:"tire_pressure_rear_max_bar"`
	// Meta is the per-field provenance (auto-filled from a web source or
	// adjusted by hand); fields without an entry were entered manually.
	Meta map[string]vehicles.SpecMeta `json:"meta"`
	// Motorcycle is always sent; on PUT, omitting it keeps the stored values.
	Motorcycle *motorcycleSpecsJSON `json:"motorcycle"`
}

type motorcycleSpecsJSON struct {
	FinalDrive                 *string  `json:"final_drive"`
	SprocketFrontTeeth         *int64   `json:"sprocket_front_teeth"`
	SprocketRearTeeth          *int64   `json:"sprocket_rear_teeth"`
	ChainSize                  string   `json:"chain_size"`
	ChainLinks                 *int64   `json:"chain_links"`
	ForkOilSpec                string   `json:"fork_oil_spec"`
	ForkOilCapacityMl          *int64   `json:"fork_oil_capacity_ml"`
	ValveClearanceIntakeMinMm  *float64 `json:"valve_clearance_intake_min_mm"`
	ValveClearanceIntakeMaxMm  *float64 `json:"valve_clearance_intake_max_mm"`
	ValveClearanceExhaustMinMm *float64 `json:"valve_clearance_exhaust_min_mm"`
	ValveClearanceExhaustMaxMm *float64 `json:"valve_clearance_exhaust_max_mm"`
}

func (m *motorcycleSpecsJSON) domain() *vehicles.MotorcycleSpecs {
	if m == nil {
		return nil
	}
	return &vehicles.MotorcycleSpecs{
		FinalDrive: m.FinalDrive, SprocketFrontTeeth: m.SprocketFrontTeeth, SprocketRearTeeth: m.SprocketRearTeeth,
		ChainSize: m.ChainSize, ChainLinks: m.ChainLinks, ForkOilSpec: m.ForkOilSpec, ForkOilCapacityMl: m.ForkOilCapacityMl,
		ValveClearanceIntakeMinMm: m.ValveClearanceIntakeMinMm, ValveClearanceIntakeMaxMm: m.ValveClearanceIntakeMaxMm,
		ValveClearanceExhaustMinMm: m.ValveClearanceExhaustMinMm, ValveClearanceExhaustMaxMm: m.ValveClearanceExhaustMaxMm,
	}
}

func toSpecsJSON(s store.VehicleSpec) specsJSON {
	return specsJSON{
		EngineOilViscosity: s.EngineOilViscosity, EngineOilSpec: s.EngineOilSpec, EngineOilCapacityMl: intPtr(s.EngineOilCapacityMl),
		TransmissionFluidSpec: s.TransmissionFluidSpec, TransmissionFluidCapacityMl: intPtr(s.TransmissionFluidCapacityMl),
		DifferentialFluidSpec: s.DifferentialFluidSpec, DifferentialFluidCapacityMl: intPtr(s.DifferentialFluidCapacityMl),
		BrakeFluidSpec: s.BrakeFluidSpec, BrakeFluidCapacityMl: intPtr(s.BrakeFluidCapacityMl),
		CoolantSpec: s.CoolantSpec, CoolantCapacityMl: intPtr(s.CoolantCapacityMl),
		WheelLugTorqueNm: floatPtr(s.WheelLugTorqueNm), TirePressureFrontBar: floatPtr(s.TirePressureFrontBar),
		TirePressureRearBar: floatPtr(s.TirePressureRearBar), TirePressureFrontMaxBar: floatPtr(s.TirePressureFrontMaxBar),
		TirePressureRearMaxBar: floatPtr(s.TirePressureRearMaxBar), Meta: vehicles.ParseSpecsMeta(s.SpecsMeta),
		Motorcycle: &motorcycleSpecsJSON{
			FinalDrive: strPtr(s.FinalDrive), SprocketFrontTeeth: intPtr(s.SprocketFrontTeeth), SprocketRearTeeth: intPtr(s.SprocketRearTeeth),
			ChainSize: s.ChainSize, ChainLinks: intPtr(s.ChainLinks), ForkOilSpec: s.ForkOilSpec, ForkOilCapacityMl: intPtr(s.ForkOilCapacityMl),
			ValveClearanceIntakeMinMm: floatPtr(s.ValveClearanceIntakeMinMm), ValveClearanceIntakeMaxMm: floatPtr(s.ValveClearanceIntakeMaxMm),
			ValveClearanceExhaustMinMm: floatPtr(s.ValveClearanceExhaustMinMm), ValveClearanceExhaustMaxMm: floatPtr(s.ValveClearanceExhaustMaxMm),
		},
	}
}

type specEntryJSON struct {
	ID          int64            `json:"id"`
	Label       string           `json:"label"`
	Value       string           `json:"value"`
	SortOrder   int64            `json:"sort_order"`
	Attachments []attachmentJSON `json:"attachments"`
}

func (s *Server) toSpecEntriesJSON(ctx context.Context, entries []store.VehicleSpecEntry) ([]specEntryJSON, error) {
	out := make([]specEntryJSON, 0, len(entries))
	for _, e := range entries {
		atts, err := s.specEntryAttachments(ctx, e.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, specEntryJSON{ID: e.ID, Label: e.Label, Value: e.Value, SortOrder: e.SortOrder, Attachments: atts})
	}
	return out, nil
}

func (s *Server) getSpecs(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	spec, entries, err := s.vehicles.GetSpecs(r.Context(), id)
	if err != nil {
		return err
	}
	ej, err := s.toSpecEntriesJSON(r.Context(), entries)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"specs": toSpecsJSON(spec), "entries": ej})
	return nil
}

func (s *Server) putSpecs(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in specsJSON
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	spec, err := s.vehicles.PutSpecs(r.Context(), id, vehicles.Specs{
		EngineOilViscosity: in.EngineOilViscosity, EngineOilSpec: in.EngineOilSpec, EngineOilCapacityMl: in.EngineOilCapacityMl,
		TransmissionFluidSpec: in.TransmissionFluidSpec, TransmissionFluidCapacityMl: in.TransmissionFluidCapacityMl,
		DifferentialFluidSpec: in.DifferentialFluidSpec, DifferentialFluidCapacityMl: in.DifferentialFluidCapacityMl,
		BrakeFluidSpec: in.BrakeFluidSpec, BrakeFluidCapacityMl: in.BrakeFluidCapacityMl,
		CoolantSpec: in.CoolantSpec, CoolantCapacityMl: in.CoolantCapacityMl,
		WheelLugTorqueNm: in.WheelLugTorqueNm, TirePressureFrontBar: in.TirePressureFrontBar, TirePressureRearBar: in.TirePressureRearBar,
		TirePressureFrontMaxBar: in.TirePressureFrontMaxBar, TirePressureRearMaxBar: in.TirePressureRearMaxBar,
		Meta: in.Meta, Motorcycle: in.Motorcycle.domain(),
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"specs": toSpecsJSON(spec)})
	return nil
}

func (s *Server) createSpecEntry(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Label string `json:"label"`
		Value string `json:"value"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	e, err := s.vehicles.CreateSpecEntry(r.Context(), id, in.Label, in.Value)
	if err != nil {
		return err
	}
	ej, err := s.toSpecEntriesJSON(r.Context(), []store.VehicleSpecEntry{e})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, map[string]any{"entry": ej[0]})
	return nil
}

func (s *Server) updateSpecEntry(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Label *string `json:"label"`
		Value *string `json:"value"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	e, err := s.vehicles.UpdateSpecEntry(r.Context(), id, in.Label, in.Value)
	if err != nil {
		return err
	}
	ej, err := s.toSpecEntriesJSON(r.Context(), []store.VehicleSpecEntry{e})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"entry": ej[0]})
	return nil
}

func (s *Server) deleteSpecEntry(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	files, err := s.vehicles.DeleteSpecEntry(r.Context(), id, s.collectSpecEntryFiles(id))
	if err != nil {
		return err
	}
	s.removeFiles(files)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) reorderSpecEntries(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		IDs []int64 `json:"ids"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	entries, err := s.vehicles.ReorderSpecEntries(r.Context(), id, in.IDs)
	if err != nil {
		return err
	}
	ej, err := s.toSpecEntriesJSON(r.Context(), entries)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": ej})
	return nil
}

// autofillSpecs looks up the vehicle's fluid and wheel specifications on the
// web (optional feature, TAVILY_API_KEY and GEMINI_API_KEY). It saves nothing: the edit dialog
// shows the values with their sources, and the user saves them.
func (s *Server) autofillSpecs(w http.ResponseWriter, r *http.Request) error {
	if s.specfill == nil {
		return apperr.Forbidden("specs.autofill_disabled")
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	v, err := s.vehicles.Get(r.Context(), id)
	if err != nil {
		return err
	}
	fields, err := s.specfill.Lookup(r.Context(), specfill.Vehicle{
		Make: v.Make, Model: v.Model, ChassisCode: v.ChassisCode, EngineCode: v.EngineCode,
		FuelType: v.FuelType.String, HSN: v.Hsn.String, TSN: v.Tsn.String,
		ModelYear: v.ModelYear.Int64, DisplacementCC: v.DisplacementCc.Int64, PowerKW: v.PowerKw.Int64,
	})
	switch {
	case errors.Is(err, specfill.ErrNoSources):
		fields = map[string]specfill.Value{}
		for _, k := range specfill.Keys() {
			fields[k] = specfill.Value{}
		}
	case err != nil:
		s.Log.ErrorContext(r.Context(), "spec auto-fill failed", "vehicle", id, "err", err)
		return autofillError(err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"fields": fields})
	return nil
}

// autofillError tells the admin what to fix: a retired model (GEMINI_MODEL),
// a used-up quota, or a rejected key, for Tavily or Gemini.
func autofillError(err error) error {
	if errors.Is(err, specfill.ErrTimeout) {
		return apperr.BadGateway("specs.autofill_timeout")
	}
	var search *specfill.SearchError
	if errors.As(err, &search) {
		switch search.Status {
		case http.StatusUnauthorized, http.StatusForbidden:
			return apperr.BadGateway("specs.autofill_search_key_invalid")
		case http.StatusTooManyRequests, 432, 433: // 432/433: Tavily plan or pay-as-you-go limit
			return apperr.BadGateway("specs.autofill_search_quota")
		}
		return apperr.BadGateway("specs.autofill_failed")
	}
	var api *specfill.APIError
	if !errors.As(err, &api) {
		return apperr.BadGateway("specs.autofill_failed")
	}
	code := "specs.autofill_failed"
	switch {
	case api.Status == http.StatusNotFound:
		code = "specs.autofill_model_unavailable"
	case api.Status == http.StatusTooManyRequests:
		code = "specs.autofill_quota"
	case api.Status == http.StatusServiceUnavailable:
		code = "specs.autofill_busy"
	case api.Status == http.StatusUnauthorized || api.Status == http.StatusForbidden ||
		(api.Status == http.StatusBadRequest && strings.Contains(strings.ToLower(api.Message), "api key")):
		code = "specs.autofill_key_invalid"
	}
	return apperr.BadGateway(code)
}
