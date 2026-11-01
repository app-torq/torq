package httpapi

import (
	"net/http"
	"strconv"

	"github.com/marekbeckmann/torq/internal/apperr"
	"github.com/marekbeckmann/torq/internal/servicelog"
	"github.com/marekbeckmann/torq/internal/store"
)

type categoryJSON struct {
	ID                    int64   `json:"id"`
	BuiltinKey            *string `json:"builtin_key"`
	CustomName            *string `json:"custom_name"`
	IsFluid               bool    `json:"is_fluid"`
	DefaultIntervalKm     *int64  `json:"default_interval_km"`
	DefaultIntervalMonths *int64  `json:"default_interval_months"`
	Archived              bool    `json:"archived"`
}

func toCategoryJSON(c store.ServiceCategory) categoryJSON {
	return categoryJSON{
		ID: c.ID, BuiltinKey: strPtr(c.BuiltinKey), CustomName: strPtr(c.CustomName), IsFluid: c.IsFluid == 1,
		DefaultIntervalKm: intPtr(c.DefaultIntervalKm), DefaultIntervalMonths: intPtr(c.DefaultIntervalMonths), Archived: c.ArchivedAt.Valid,
	}
}

func (s *Server) listCategories(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.servicelog.ListCategories(r.Context(), r.URL.Query().Get("include_archived") == "true")
	if err != nil {
		return err
	}
	items := make([]categoryJSON, 0, len(rows))
	for _, c := range rows {
		items = append(items, toCategoryJSON(c))
	}
	writeJSON(w, http.StatusOK, newList(items, int64(len(items))))
	return nil
}

type categoryInput struct {
	CustomName            *string `json:"custom_name"`
	IsFluid               *bool   `json:"is_fluid"`
	DefaultIntervalKm     *int64  `json:"default_interval_km"`
	DefaultIntervalMonths *int64  `json:"default_interval_months"`
}

func (in categoryInput) domain() servicelog.CategoryInput {
	return servicelog.CategoryInput{CustomName: in.CustomName, IsFluid: in.IsFluid, DefaultIntervalKm: in.DefaultIntervalKm, DefaultIntervalMonths: in.DefaultIntervalMonths}
}

func (s *Server) createCategory(w http.ResponseWriter, r *http.Request) error {
	var in categoryInput
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	c, err := s.servicelog.CreateCategory(r.Context(), in.domain())
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, map[string]any{"category": toCategoryJSON(c)})
	return nil
}

func (s *Server) updateCategory(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in categoryInput
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	c, err := s.servicelog.UpdateCategory(r.Context(), id, in.domain())
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"category": toCategoryJSON(c)})
	return nil
}

func (s *Server) setCategoryArchived(archived bool) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := pathID(r, "id")
		if err != nil {
			return err
		}
		c, err := s.servicelog.SetCategoryArchived(r.Context(), id, archived)
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, map[string]any{"category": toCategoryJSON(c)})
		return nil
	}
}

func (s *Server) deleteCategory(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	if err := s.servicelog.DeleteCategory(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type serviceItemJSON struct {
	ID                    int64   `json:"id"`
	CategoryID            int64   `json:"category_id"`
	Brand                 string  `json:"brand"`
	Specification         string  `json:"specification"`
	QuantityMl            *int64  `json:"quantity_ml"`
	QuantityUnit          *string `json:"quantity_unit"`
	OEMPartNumber         string  `json:"oem_part_number"`
	AftermarketPartNumber string  `json:"aftermarket_part_number"`
	CostCents             *int64  `json:"cost_cents"`
	CostType              string  `json:"cost_type"`
}

type serviceLogJSON struct {
	ID             int64             `json:"id"`
	VehicleID      int64             `json:"vehicle_id"`
	ServiceDate    string            `json:"service_date"`
	OdometerKm     int64             `json:"odometer_km"`
	Notes          string            `json:"notes"`
	ExtraCostCents int64             `json:"extra_cost_cents"`
	ExtraCostLabel string            `json:"extra_cost_label"`
	Items          []serviceItemJSON `json:"items"`
	TotalCents     int64             `json:"total_cents"`
	Attachments    []attachmentJSON  `json:"attachments,omitempty"`
	CreatedAt      string            `json:"created_at"`
	UpdatedAt      string            `json:"updated_at"`
}

func toServiceLogJSON(l servicelog.Log) serviceLogJSON {
	items := make([]serviceItemJSON, 0, len(l.Items))
	for _, it := range l.Items {
		items = append(items, serviceItemJSON{
			ID: it.ID, CategoryID: it.CategoryID, Brand: it.Brand, Specification: it.Specification,
			QuantityMl: intPtr(it.QuantityMl), QuantityUnit: strPtr(it.QuantityUnit),
			OEMPartNumber: it.OemPartNumber, AftermarketPartNumber: it.AftermarketPartNumber,
			CostCents: intPtr(it.CostCents), CostType: it.CostType,
		})
	}
	return serviceLogJSON{
		ID: l.ID, VehicleID: l.VehicleID, ServiceDate: l.ServiceDate, OdometerKm: l.OdometerKm, Notes: l.Notes,
		ExtraCostCents: l.ExtraCostCents, ExtraCostLabel: l.ExtraCostLabel, Items: items, TotalCents: l.TotalCents(),
		CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt,
	}
}

type serviceLogInput struct {
	VehicleID      int64  `json:"vehicle_id"`
	ServiceDate    string `json:"service_date"`
	OdometerKm     *int64 `json:"odometer_km"`
	Notes          string `json:"notes"`
	ExtraCostCents int64  `json:"extra_cost_cents"`
	ExtraCostLabel string `json:"extra_cost_label"`
	Items          []struct {
		CategoryID            int64  `json:"category_id"`
		Brand                 string `json:"brand"`
		Specification         string `json:"specification"`
		QuantityMl            *int64 `json:"quantity_ml"`
		QuantityUnit          string `json:"quantity_unit"`
		OEMPartNumber         string `json:"oem_part_number"`
		AftermarketPartNumber string `json:"aftermarket_part_number"`
		CostCents             *int64 `json:"cost_cents"`
		CostType              string `json:"cost_type"`
	} `json:"items"`
}

func (in serviceLogInput) domain() (servicelog.Input, error) {
	if in.OdometerKm == nil {
		var v apperr.Validation
		v.Add("odometer_km", "validation.required")
		return servicelog.Input{}, v.Err()
	}
	out := servicelog.Input{
		VehicleID: in.VehicleID, ServiceDate: in.ServiceDate, OdometerKm: *in.OdometerKm, Notes: in.Notes,
		ExtraCostCents: in.ExtraCostCents, ExtraCostLabel: in.ExtraCostLabel,
	}
	for _, it := range in.Items {
		out.Items = append(out.Items, servicelog.ItemInput{
			CategoryID: it.CategoryID, Brand: it.Brand, Specification: it.Specification, QuantityMl: it.QuantityMl,
			QuantityUnit: it.QuantityUnit, OEMPartNumber: it.OEMPartNumber, AftermarketPartNumber: it.AftermarketPartNumber,
			CostCents: it.CostCents, CostType: it.CostType,
		})
	}
	return out, nil
}

// pagination parses limit/offset with contract defaults (50, max 200).
func pagination(r *http.Request) (limit, offset int64, err error) {
	limit, offset = 50, 0
	q := r.URL.Query()
	if v := q.Get("limit"); v != "" {
		if limit, err = strconv.ParseInt(v, 10, 64); err != nil || limit < 1 || limit > 200 {
			return 0, 0, apperr.BadRequest("request.malformed")
		}
	}
	if v := q.Get("offset"); v != "" {
		if offset, err = strconv.ParseInt(v, 10, 64); err != nil || offset < 0 {
			return 0, 0, apperr.BadRequest("request.malformed")
		}
	}
	return limit, offset, nil
}

func queryInt(r *http.Request, key string) (int64, error) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		return 0, apperr.BadRequest("request.malformed")
	}
	return n, nil
}

func (s *Server) listServiceLogs(w http.ResponseWriter, r *http.Request) error {
	limit, offset, err := pagination(r)
	if err != nil {
		return err
	}
	vehicleID, err := queryInt(r, "vehicle_id")
	if err != nil {
		return err
	}
	categoryID, err := queryInt(r, "category_id")
	if err != nil {
		return err
	}
	q := r.URL.Query()
	logs, total, err := s.servicelog.List(r.Context(), servicelog.Filter{
		VehicleID: vehicleID, CategoryID: categoryID, From: q.Get("from"), To: q.Get("to"), Q: q.Get("q"), Limit: limit, Offset: offset,
	})
	if err != nil {
		return err
	}
	items := make([]serviceLogJSON, 0, len(logs))
	for _, l := range logs {
		items = append(items, toServiceLogJSON(l))
	}
	writeJSON(w, http.StatusOK, newList(items, total))
	return nil
}

func (s *Server) writeServiceLog(w http.ResponseWriter, r *http.Request, status int, l servicelog.Log, warnings []string) error {
	out := toServiceLogJSON(l)
	atts, err := s.serviceLogAttachments(r.Context(), l.ID)
	if err != nil {
		return err
	}
	out.Attachments = atts
	rems, err := s.vehicleReminderSummaries(r.Context(), l.VehicleID)
	if err != nil {
		return err
	}
	body := map[string]any{"service_log": out, "reminders": rems}
	if len(warnings) > 0 {
		body["warnings"] = warnings
	}
	writeJSON(w, status, body)
	return nil
}

func (s *Server) createServiceLog(w http.ResponseWriter, r *http.Request) error {
	var in serviceLogInput
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	d, err := in.domain()
	if err != nil {
		return err
	}
	l, warnings, err := s.servicelog.Create(r.Context(), d, currentUser(r).ID)
	if err != nil {
		return err
	}
	return s.writeServiceLog(w, r, http.StatusCreated, l, warnings)
}

func (s *Server) getServiceLog(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	l, err := s.servicelog.Get(r.Context(), id)
	if err != nil {
		return err
	}
	out := toServiceLogJSON(l)
	if out.Attachments, err = s.serviceLogAttachments(r.Context(), id); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"service_log": out})
	return nil
}

func (s *Server) replaceServiceLog(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in serviceLogInput
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	d, err := in.domain()
	if err != nil {
		return err
	}
	l, warnings, err := s.servicelog.Replace(r.Context(), id, d)
	if err != nil {
		return err
	}
	return s.writeServiceLog(w, r, http.StatusOK, l, warnings)
}

func (s *Server) deleteServiceLog(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	vehicleID, err := s.servicelog.Delete(r.Context(), id)
	if err != nil {
		return err
	}
	rems, err := s.vehicleReminderSummaries(r.Context(), vehicleID)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"reminders": rems})
	return nil
}
