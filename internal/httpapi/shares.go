package httpapi

import (
	"math"
	"net/http"
	"strings"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/auth"
	"github.com/app-torq/torq/internal/i18n"
	"github.com/app-torq/torq/internal/reminders"
	"github.com/app-torq/torq/internal/reports"
	"github.com/app-torq/torq/internal/store"
	"github.com/app-torq/torq/internal/vehicles"
)

// Public read-only links (#38, specs/007-vehicle-share-link). The public
// record is built only from the allow-list types below, so a field added to
// vehicles, services or the history export never becomes public by accident
// (research S7).

type shareJSON struct {
	Token               string  `json:"token"`
	Path                string  `json:"path"`
	IncludeCosts        bool    `json:"include_costs"`
	IncludeLicensePlate bool    `json:"include_license_plate"`
	IncludeVIN          bool    `json:"include_vin"`
	CreatedAt           string  `json:"created_at"`
	LastOpenedAt        *string `json:"last_opened_at"`
}

func toShareJSON(sh store.VehicleShare) shareJSON {
	return shareJSON{
		Token: sh.Token, Path: "/s/" + sh.Token, IncludeCosts: sh.IncludeCosts == 1,
		IncludeLicensePlate: sh.IncludeLicensePlate == 1, IncludeVIN: sh.IncludeVin == 1,
		CreatedAt: sh.CreatedAt, LastOpenedAt: strPtr(sh.LastOpenedAt),
	}
}

func (s *Server) getShare(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathVehicle(r)
	if err != nil {
		return err
	}
	if _, err := s.vehicles.Get(r.Context(), id); err != nil {
		return err
	}
	sh, err := s.vehicles.Share(r.Context(), id)
	if e, ok := apperr.As(err); ok && e.Status == http.StatusNotFound {
		writeJSON(w, http.StatusOK, map[string]any{"share": nil})
		return nil
	}
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"share": toShareJSON(sh)})
	return nil
}

// putShare turns the link on, or changes its options; the token stays.
func (s *Server) putShare(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathVehicle(r)
	if err != nil {
		return err
	}
	var in struct {
		IncludeCosts        *bool `json:"include_costs"`
		IncludeLicensePlate *bool `json:"include_license_plate"`
		IncludeVIN          *bool `json:"include_vin"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	// Only who sees costs, VIN and plate may publish them (#49).
	v := visibilityOf(r.Context())
	if (in.IncludeCosts != nil && !v.costs) || ((in.IncludeVIN != nil || in.IncludeLicensePlate != nil) && !v.vehicleIDs) {
		return apperr.Forbidden("auth.forbidden")
	}
	// Omitted options keep their current value (off for a new link).
	var opts vehicles.ShareOptions
	if cur, err := s.vehicles.Share(r.Context(), id); err == nil {
		opts = vehicles.ShareOptions{IncludeCosts: cur.IncludeCosts == 1, IncludeLicensePlate: cur.IncludeLicensePlate == 1, IncludeVIN: cur.IncludeVin == 1}
	}
	for _, o := range []struct {
		in  *bool
		out *bool
	}{{in.IncludeCosts, &opts.IncludeCosts}, {in.IncludeLicensePlate, &opts.IncludeLicensePlate}, {in.IncludeVIN, &opts.IncludeVIN}} {
		if o.in != nil {
			*o.out = *o.in
		}
	}
	sh, created, err := s.vehicles.EnableShare(r.Context(), id, opts, currentUser(r).ID)
	if err != nil {
		return err
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, map[string]any{"share": toShareJSON(sh)})
	return nil
}

func (s *Server) renewShare(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathVehicle(r)
	if err != nil {
		return err
	}
	sh, err := s.vehicles.RenewShare(r.Context(), id, currentUser(r).ID)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"share": toShareJSON(sh)})
	return nil
}

func (s *Server) deleteShare(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathVehicle(r)
	if err != nil {
		return err
	}
	if err := s.vehicles.DisableShare(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type shareListItemJSON struct {
	VehicleID     int64   `json:"vehicle_id"`
	VehicleName   string  `json:"vehicle_name"`
	Archived      bool    `json:"archived"`
	CreatedAt     string  `json:"created_at"`
	CreatedByName *string `json:"created_by_name"`
	LastOpenedAt  *string `json:"last_opened_at"`
}

// listShares lists every link for administrators, without tokens.
func (s *Server) listShares(w http.ResponseWriter, r *http.Request) error {
	rows, err := store.New(s.DB.Reader).ListVehicleShares(r.Context())
	if err != nil {
		return err
	}
	items := make([]shareListItemJSON, 0, len(rows))
	for _, row := range rows {
		items = append(items, shareListItemJSON{
			VehicleID: row.VehicleID, VehicleName: strings.TrimSpace(row.Make + " " + row.Model), Archived: row.ArchivedAt.Valid,
			CreatedAt: row.CreatedAt, CreatedByName: strPtr(row.CreatedByName), LastOpenedAt: strPtr(row.LastOpenedAt),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
	return nil
}

// --- The public record ---------------------------------------------------

type publicHourMeterJSON struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	CurrentHours float64 `json:"current_hours"`
}

type publicVehicleJSON struct {
	VehicleType       string                `json:"vehicle_type"`
	Make              string                `json:"make"`
	Model             string                `json:"model"`
	ChassisCode       string                `json:"chassis_code"`
	ModelYear         *int64                `json:"model_year"`
	EngineCode        string                `json:"engine_code"`
	HSN               *string               `json:"hsn"`
	TSN               *string               `json:"tsn"`
	DisplacementCC    *int64                `json:"displacement_cc"`
	PowerKW           *int64                `json:"power_kw"`
	PowerW            *int64                `json:"power_w"`
	FuelType          *string               `json:"fuel_type"`
	HasOdometer       bool                  `json:"has_odometer"`
	CurrentOdometerKm int64                 `json:"current_odometer_km"`
	OdometerUnit      string                `json:"odometer_unit"`
	DisplacementUnit  *string               `json:"displacement_unit"`
	PowerUnit         *string               `json:"power_unit"`
	HourMeters        []publicHourMeterJSON `json:"hour_meters"`
	// LicensePlate and VIN are null unless the owner included them.
	LicensePlate *string `json:"license_plate"`
	VIN          *string `json:"vin"`
}

type publicReadingJSON struct {
	HourMeterID int64   `json:"hour_meter_id"`
	Hours       float64 `json:"hours"`
}

type publicItemJSON struct {
	Category              string  `json:"category"`
	Brand                 string  `json:"brand"`
	Specification         string  `json:"specification"`
	QuantityMl            *int64  `json:"quantity_ml"`
	QuantityUnit          *string `json:"quantity_unit"`
	OEMPartNumber         string  `json:"oem_part_number"`
	AftermarketPartNumber string  `json:"aftermarket_part_number"`
	CostCents             *int64  `json:"cost_cents"`
}

type publicServiceJSON struct {
	ServiceDate      string              `json:"service_date"`
	OdometerKm       int64               `json:"odometer_km"`
	OdometerRecorded bool                `json:"odometer_recorded"`
	MeterReadings    []publicReadingJSON `json:"meter_readings"`
	Items            []publicItemJSON    `json:"items"`
	ExtraCostCents   *int64              `json:"extra_cost_cents"`
	ExtraCostLabel   string              `json:"extra_cost_label"`
	TotalCents       *int64              `json:"total_cents"`
}

type publicReminderJSON struct {
	Name           string   `json:"name"`
	State          string   `json:"state"`
	Trigger        *string  `json:"trigger"`
	NextDueDate    *string  `json:"next_due_date"`
	NextDueKm      *int64   `json:"next_due_km"`
	NextDueHours   *float64 `json:"next_due_hours"`
	RemainingDays  *int64   `json:"remaining_days"`
	RemainingKm    *int64   `json:"remaining_km"`
	RemainingHours *float64 `json:"remaining_hours"`
	HourMeter      *string  `json:"hour_meter"`
}

type publicEntryJSON struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type publicBrandingJSON struct {
	Name    string  `json:"name"`
	LogoURL *string `json:"logo_url"`
}

type publicRecordJSON struct {
	// VehicleID is only sent to signed-in users, for "Open in torq".
	VehicleID   *int64               `json:"vehicle_id,omitempty"`
	Vehicle     publicVehicleJSON    `json:"vehicle"`
	ServiceLogs []publicServiceJSON  `json:"service_logs"`
	Reminders   []publicReminderJSON `json:"reminders"`
	Specs       map[string]any       `json:"specs"`
	SpecEntries []publicEntryJSON    `json:"spec_entries"`
	Formats     formatsJSON          `json:"formats"`
	Branding    publicBrandingJSON   `json:"branding"`
}

// shareLang picks the language of category names from Accept-Language.
func shareLang(r *http.Request) string {
	for part := range strings.SplitSeq(r.Header.Get("Accept-Language"), ",") {
		tag := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		switch {
		case strings.HasPrefix(tag, "de"):
			return "de"
		case strings.HasPrefix(tag, "en"):
			return "en"
		}
	}
	return "en"
}

// setShareHeaders keeps shared pages out of caches, search engines and
// referrers (FR-010).
func setShareHeaders(h http.Header) {
	h.Set("Cache-Control", "no-store")
	h.Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	h.Set("Referrer-Policy", "no-referrer")
}

// getSharedRecord serves the public record of a link. Every failure is the
// same 404 (FR-004); an IP that keeps failing is slowed down (FR-005).
func (s *Server) getSharedRecord(w http.ResponseWriter, r *http.Request) error {
	setShareHeaders(w.Header())
	key := "share:" + auth.ClientIP(r, s.Config.TrustedProxies)
	if wait := s.shareThrottle.RetryAfter(key); wait > 0 {
		return apperr.Throttled(int(math.Ceil(wait.Seconds())))
	}
	sh, err := s.vehicles.ResolveShare(r.Context(), r.PathValue("token"))
	if err != nil {
		if e, ok := apperr.As(err); ok && e.Status == http.StatusNotFound {
			s.shareThrottle.Fail(key)
		}
		return err
	}
	rec, err := s.publicRecord(r, sh)
	if err != nil {
		return err
	}
	if _, err := s.authorize(r, member); err == nil {
		id := sh.VehicleID
		rec.VehicleID = &id
	}
	if err := s.vehicles.TouchShare(r.Context(), sh.VehicleID); err != nil {
		s.Log.WarnContext(r.Context(), "record share view", "err", err)
	}
	writeJSON(w, http.StatusOK, rec)
	return nil
}

func (s *Server) publicRecord(r *http.Request, sh store.VehicleShare) (publicRecordJSON, error) {
	ctx := r.Context()
	q := store.New(s.DB.Reader)
	settings, err := q.GetInstanceSettings(ctx)
	if err != nil {
		return publicRecordJSON{}, err
	}
	defaults := auth.DefaultsFromStore(settings)
	loc := i18n.Locale{Lang: shareLang(r), DateFormat: defaults.DateFormat, NumberFormat: defaults.NumberFormat}
	h, err := s.reports.History(ctx, sh.VehicleID, reports.ExportOptions{IncludeCosts: sh.IncludeCosts == 1, IncludeNotes: false, Locale: loc})
	if err != nil {
		return publicRecordJSON{}, err
	}
	v := h.Vehicle
	rec := publicRecordJSON{
		Vehicle: publicVehicleJSON{
			VehicleType: v.VehicleType, Make: v.Make, Model: v.Model, ChassisCode: v.ChassisCode, ModelYear: v.ModelYear,
			EngineCode: v.EngineCode, HSN: v.HSN, TSN: v.TSN, DisplacementCC: v.DisplacementCC, PowerKW: v.PowerKW,
			PowerW: v.PowerW, FuelType: v.FuelType, HasOdometer: v.HasOdometer, CurrentOdometerKm: v.CurrentOdometerKm,
			OdometerUnit: v.OdometerUnit, DisplacementUnit: v.DisplacementUnit, PowerUnit: v.PowerUnit,
			HourMeters: []publicHourMeterJSON{},
		},
		ServiceLogs: []publicServiceJSON{}, Reminders: []publicReminderJSON{}, Specs: h.Specs, SpecEntries: []publicEntryJSON{},
		Formats: toFormatsJSON(defaults),
	}
	if sh.IncludeLicensePlate == 1 {
		rec.Vehicle.LicensePlate = v.LicensePlate
	}
	if sh.IncludeVin == 1 {
		rec.Vehicle.VIN = v.VIN
	}
	meterNames := map[int64]string{}
	for _, m := range v.HourMeters {
		rec.Vehicle.HourMeters = append(rec.Vehicle.HourMeters, publicHourMeterJSON{ID: m.ID, Name: m.Name, CurrentHours: m.CurrentHours})
		meterNames[m.ID] = m.Name
	}
	for _, sv := range h.Services {
		out := publicServiceJSON{
			ServiceDate: sv.Date, OdometerKm: sv.OdometerKm, OdometerRecorded: sv.OdometerRecorded,
			MeterReadings: []publicReadingJSON{}, Items: []publicItemJSON{},
		}
		for _, m := range sv.MeterReadings {
			out.MeterReadings = append(out.MeterReadings, publicReadingJSON{HourMeterID: m.HourMeterID, Hours: m.Hours})
		}
		for _, it := range sv.Items {
			out.Items = append(out.Items, publicItemJSON{
				Category: it.Category, Brand: it.Brand, Specification: it.Specification, QuantityMl: it.QuantityMl,
				QuantityUnit: it.QuantityUnit, OEMPartNumber: it.OEMPartNumber, AftermarketPartNumber: it.AftermarketPartNumber,
			})
		}
		// The history export leaves costs out unless asked; only copy them then.
		if sh.IncludeCosts == 1 {
			for i, it := range sv.Items {
				out.Items[i].CostCents = it.CostCents
			}
			out.ExtraCostCents, out.ExtraCostLabel, out.TotalCents = sv.ExtraCostCents, sv.ExtraCostLabel, sv.TotalCents
		}
		rec.ServiceLogs = append(rec.ServiceLogs, out)
	}
	for _, e := range h.Entries {
		rec.SpecEntries = append(rec.SpecEntries, publicEntryJSON{Label: e.Label, Value: e.Value})
	}
	if err := s.addPublicReminders(r, &rec, sh.VehicleID, loc, meterNames); err != nil {
		return publicRecordJSON{}, err
	}
	b, err := s.branding(ctx, false)
	if err != nil {
		return publicRecordJSON{}, err
	}
	rec.Branding = publicBrandingJSON{Name: b.Name, LogoURL: b.LogoURL}
	return rec, nil
}

// addPublicReminders adds the vehicle's active reminders, named like in the
// app: the category's (translated) name, or the reminder's own title.
func (s *Server) addPublicReminders(r *http.Request, rec *publicRecordJSON, vehicleID int64, loc i18n.Locale, meterNames map[int64]string) error {
	views, err := s.reminders.List(r.Context(), reminders.Filter{VehicleID: vehicleID})
	if err != nil {
		return err
	}
	cats, err := s.servicelog.ListCategories(r.Context(), true, "")
	if err != nil {
		return err
	}
	names := map[int64]string{}
	for _, c := range cats {
		if c.BuiltinKey.Valid {
			names[c.ID] = loc.T("categories."+c.BuiltinKey.String, nil)
		} else {
			names[c.ID] = c.CustomName.String
		}
	}
	for _, v := range views {
		out := publicReminderJSON{
			Name: v.Title, State: v.Status.State, NextDueDate: strPtr(v.NextDueDate), NextDueKm: intPtr(v.NextDueKm),
			NextDueHours: hoursPtr(v.NextDueHoursTenths), RemainingDays: v.Status.RemainingDays, RemainingKm: v.Status.RemainingKm,
			RemainingHours: hoursOf(v.Status.RemainingHoursTenths),
		}
		if v.ServiceCategoryID.Valid {
			if n := names[v.ServiceCategoryID.Int64]; n != "" {
				out.Name = n
			}
		}
		if v.Status.Trigger != "" {
			tr := v.Status.Trigger
			out.Trigger = &tr
		}
		if v.HourMeterID.Valid {
			if n, ok := meterNames[v.HourMeterID.Int64]; ok {
				out.HourMeter = &n
			}
		}
		rec.Reminders = append(rec.Reminders, out)
	}
	return nil
}
