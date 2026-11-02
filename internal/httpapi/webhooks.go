package httpapi

import (
	"net/http"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/notify"
	"github.com/app-torq/torq/internal/store"
)

type webhookEventsJSON struct {
	DueSoon       bool `json:"due_soon"`
	Overdue       bool `json:"overdue"`
	WeeklyDigest  bool `json:"weekly_digest"`
	MonthlyDigest bool `json:"monthly_digest"`
}

type webhookHeaderJSON struct {
	Name     string `json:"name"`
	ValueSet bool   `json:"value_set"`
}

type webhookJSON struct {
	ID          int64               `json:"id"`
	Name        string              `json:"name"`
	URL         string              `json:"url"`
	Preset      string              `json:"preset"`
	Enabled     bool                `json:"enabled"`
	Events      webhookEventsJSON   `json:"events"`
	Headers     []webhookHeaderJSON `json:"headers"`
	OwnerUserID *int64              `json:"owner_user_id"`
	CreatedAt   string              `json:"created_at"`
	UpdatedAt   string              `json:"updated_at"`
}

// toWebhookJSON never includes header values (FR-042).
func toWebhookJSON(w notify.Webhook) webhookJSON {
	out := webhookJSON{
		ID: w.ID, Name: w.Name, URL: w.Url, Preset: w.Preset, Enabled: w.Enabled == 1,
		Events:      webhookEventsJSON{DueSoon: w.OnDueSoon == 1, Overdue: w.OnOverdue == 1, WeeklyDigest: w.OnWeeklyDigest == 1, MonthlyDigest: w.OnMonthlyDigest == 1},
		Headers:     []webhookHeaderJSON{},
		OwnerUserID: intPtr(w.OwnerUserID), CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt,
	}
	for _, n := range w.HeaderNames {
		out.Headers = append(out.Headers, webhookHeaderJSON{Name: n, ValueSet: true})
	}
	return out
}

type webhookInput struct {
	Name    string            `json:"name"`
	URL     string            `json:"url"`
	Preset  string            `json:"preset"`
	Enabled *bool             `json:"enabled"`
	Events  webhookEventsJSON `json:"events"`
	Headers *[]struct {
		Name  string  `json:"name"`
		Value *string `json:"value"`
	} `json:"headers"`
}

func (in webhookInput) domain() notify.WebhookInput {
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	out := notify.WebhookInput{
		Name: in.Name, URL: in.URL, Preset: in.Preset, Enabled: enabled,
		DueSoon: in.Events.DueSoon, Overdue: in.Events.Overdue, WeeklyDigest: in.Events.WeeklyDigest, MonthlyDigest: in.Events.MonthlyDigest,
	}
	if in.Headers != nil {
		hs := make([]notify.HeaderInput, 0, len(*in.Headers))
		for _, h := range *in.Headers {
			hs = append(hs, notify.HeaderInput{Name: h.Name, Value: h.Value})
		}
		out = out.WithHeaders(hs)
	}
	return out
}

func (s *Server) listWebhooks(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.notify.List(r.Context())
	if err != nil {
		return err
	}
	items := make([]webhookJSON, 0, len(rows))
	for _, wh := range rows {
		items = append(items, toWebhookJSON(wh))
	}
	writeJSON(w, http.StatusOK, newList(items, int64(len(items))))
	return nil
}

func (s *Server) createWebhook(w http.ResponseWriter, r *http.Request) error {
	var in webhookInput
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	d := in.domain()
	if in.Headers == nil {
		d = d.WithHeaders(nil)
	}
	wh, err := s.notify.Create(r.Context(), d, currentUser(r).ID)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, map[string]any{"webhook": toWebhookJSON(wh)})
	return nil
}

func (s *Server) getWebhook(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	wh, err := s.notify.Get(r.Context(), id)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"webhook": toWebhookJSON(wh)})
	return nil
}

func (s *Server) updateWebhook(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	cur, err := s.notify.Get(r.Context(), id)
	if err != nil {
		return err
	}
	enabled := cur.Enabled == 1
	in := webhookInput{
		Name: cur.Name, URL: cur.Url, Preset: cur.Preset, Enabled: &enabled,
		Events: webhookEventsJSON{DueSoon: cur.OnDueSoon == 1, Overdue: cur.OnOverdue == 1, WeeklyDigest: cur.OnWeeklyDigest == 1, MonthlyDigest: cur.OnMonthlyDigest == 1},
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	wh, err := s.notify.Update(r.Context(), id, in.domain())
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"webhook": toWebhookJSON(wh)})
	return nil
}

func (s *Server) deleteWebhook(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	if err := s.notify.Delete(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) testWebhook(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	res, err := s.notify.SendTest(r.Context(), id)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"delivery": res})
	return nil
}

type deliveryJSON struct {
	ID              int64   `json:"id"`
	Event           string  `json:"event"`
	ReminderID      *int64  `json:"reminder_id"`
	Status          string  `json:"status"`
	AttemptCount    int64   `json:"attempt_count"`
	NextAttemptAt   *string `json:"next_attempt_at"`
	LastStatusCode  *int64  `json:"last_status_code"`
	LastError       string  `json:"last_error"`
	ResponseExcerpt string  `json:"response_excerpt"`
	DurationMs      *int64  `json:"duration_ms"`
	CreatedAt       string  `json:"created_at"`
	CompletedAt     *string `json:"completed_at"`
}

func toDeliveryJSON(d store.WebhookDelivery) deliveryJSON {
	return deliveryJSON{
		ID: d.ID, Event: d.Event, ReminderID: intPtr(d.ReminderID), Status: d.Status, AttemptCount: d.AttemptCount,
		NextAttemptAt: strPtr(d.NextAttemptAt), LastStatusCode: intPtr(d.LastStatusCode), LastError: d.LastError,
		ResponseExcerpt: d.ResponseExcerpt, DurationMs: intPtr(d.DurationMs), CreatedAt: d.CreatedAt, CompletedAt: strPtr(d.CompletedAt),
	}
}

func (s *Server) listDeliveries(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	limit, offset, err := pagination(r)
	if err != nil {
		return err
	}
	rows, total, err := s.notify.Deliveries(r.Context(), id, limit, offset)
	if err != nil {
		return err
	}
	items := make([]deliveryJSON, 0, len(rows))
	for _, d := range rows {
		items = append(items, toDeliveryJSON(d))
	}
	writeJSON(w, http.StatusOK, newList(items, total))
	return nil
}

// --- Instance settings -------------------------------------------------------

type instanceSettingsJSON struct {
	WeeklyDigestWeekday int64  `json:"weekly_digest_weekday"`
	WeeklyDigestHour    int64  `json:"weekly_digest_hour"`
	MonthlyDigestDay    int64  `json:"monthly_digest_day"`
	MonthlyDigestHour   int64  `json:"monthly_digest_hour"`
	BaseURLConfigured   bool   `json:"base_url_configured"`
	TimeZone            string `json:"time_zone"`
	MaxUploadMB         int    `json:"max_upload_mb"`
	Version             string `json:"version"`
}

func (s *Server) instanceSettingsOut(c store.InstanceSetting) instanceSettingsJSON {
	return instanceSettingsJSON{
		WeeklyDigestWeekday: c.WeeklyDigestWeekday, WeeklyDigestHour: c.WeeklyDigestHour,
		MonthlyDigestDay: c.MonthlyDigestDay, MonthlyDigestHour: c.MonthlyDigestHour,
		BaseURLConfigured: s.Config.BaseURL != "", TimeZone: s.Clock.Now().Location().String(),
		MaxUploadMB: s.Config.MaxUploadMB, Version: s.Version,
	}
}

func (s *Server) getInstanceSettings(w http.ResponseWriter, r *http.Request) error {
	c, err := store.New(s.DB.Reader).GetInstanceSettings(r.Context())
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, s.instanceSettingsOut(c))
	return nil
}

func (s *Server) patchInstanceSettings(w http.ResponseWriter, r *http.Request) error {
	cur, err := store.New(s.DB.Reader).GetInstanceSettings(r.Context())
	if err != nil {
		return err
	}
	in := struct {
		WeeklyDigestWeekday int64 `json:"weekly_digest_weekday"`
		WeeklyDigestHour    int64 `json:"weekly_digest_hour"`
		MonthlyDigestDay    int64 `json:"monthly_digest_day"`
		MonthlyDigestHour   int64 `json:"monthly_digest_hour"`
	}{cur.WeeklyDigestWeekday, cur.WeeklyDigestHour, cur.MonthlyDigestDay, cur.MonthlyDigestHour}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	var v apperr.Validation
	v.Check(in.WeeklyDigestWeekday < 1 || in.WeeklyDigestWeekday > 7, "weekly_digest_weekday", "validation.range")
	v.Check(in.WeeklyDigestHour < 0 || in.WeeklyDigestHour > 23, "weekly_digest_hour", "validation.range")
	v.Check(in.MonthlyDigestDay < 1 || in.MonthlyDigestDay > 28, "monthly_digest_day", "validation.range")
	v.Check(in.MonthlyDigestHour < 0 || in.MonthlyDigestHour > 23, "monthly_digest_hour", "validation.range")
	if err := v.Err(); err != nil {
		return err
	}
	c, err := store.New(s.DB.Writer).UpdateInstanceSettings(r.Context(), store.UpdateInstanceSettingsParams{
		WeeklyDigestWeekday: in.WeeklyDigestWeekday, WeeklyDigestHour: in.WeeklyDigestHour,
		MonthlyDigestDay: in.MonthlyDigestDay, MonthlyDigestHour: in.MonthlyDigestHour,
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, s.instanceSettingsOut(c))
	return nil
}
