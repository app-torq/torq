package httpapi

import (
	"context"
	"net/http"

	"github.com/marekbeckmann/torq/internal/store"
	"github.com/marekbeckmann/torq/internal/vehicles"
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
}

func toSpecsJSON(s store.VehicleSpec) specsJSON {
	return specsJSON{
		EngineOilViscosity: s.EngineOilViscosity, EngineOilSpec: s.EngineOilSpec, EngineOilCapacityMl: intPtr(s.EngineOilCapacityMl),
		TransmissionFluidSpec: s.TransmissionFluidSpec, TransmissionFluidCapacityMl: intPtr(s.TransmissionFluidCapacityMl),
		DifferentialFluidSpec: s.DifferentialFluidSpec, DifferentialFluidCapacityMl: intPtr(s.DifferentialFluidCapacityMl),
		BrakeFluidSpec: s.BrakeFluidSpec, BrakeFluidCapacityMl: intPtr(s.BrakeFluidCapacityMl),
		CoolantSpec: s.CoolantSpec, CoolantCapacityMl: intPtr(s.CoolantCapacityMl),
		WheelLugTorqueNm: floatPtr(s.WheelLugTorqueNm), TirePressureFrontBar: floatPtr(s.TirePressureFrontBar),
		TirePressureRearBar: floatPtr(s.TirePressureRearBar),
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
