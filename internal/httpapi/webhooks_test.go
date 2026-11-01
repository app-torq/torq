package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebhookContract(t *testing.T) {
	var gotAuth string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()

	e := newEnv(t)
	adm := e.admin()
	adm.call(http.MethodPost, "/users", map[string]string{"username": "partner", "display_name": "P", "password": testPassword, "role": "member"}, http.StatusCreated, nil)
	member := e.login("partner", testPassword)
	if code := member.errorCode(http.MethodGet, "/webhooks", nil, http.StatusForbidden); code != "auth.forbidden" {
		t.Fatalf("code %q", code)
	}

	var created struct {
		Webhook webhookJSON `json:"webhook"`
	}
	rec := adm.do(http.MethodPost, "/webhooks", map[string]any{
		"name": "ntfy", "url": target.URL, "preset": "ntfy",
		"events":  map[string]bool{"due_soon": true, "overdue": true},
		"headers": []map[string]string{{"name": "Authorization", "value": "Bearer test-secret"}},
	})
	if rec.Code != http.StatusCreated || strings.Contains(rec.Body.String(), "test-secret") {
		t.Fatalf("create %d %s", rec.Code, rec.Body)
	}
	adm.call(http.MethodGet, "/webhooks", nil, http.StatusOK, nil)
	got := adm.do(http.MethodGet, "/webhooks", nil)
	if strings.Contains(got.Body.String(), "test-secret") {
		t.Fatal("header value leaked in list")
	}
	var list listResponse[webhookJSON]
	adm.call(http.MethodGet, "/webhooks", nil, http.StatusOK, &list)
	created.Webhook = list.Items[0]
	if len(created.Webhook.Headers) != 1 || !created.Webhook.Headers[0].ValueSet || !created.Webhook.Enabled {
		t.Fatalf("webhook %+v", created.Webhook)
	}
	id := itoa(created.Webhook.ID)

	// PATCH without a header value keeps the stored secret.
	adm.call(http.MethodPatch, "/webhooks/"+id, map[string]any{"name": "ntfy phone", "headers": []map[string]any{{"name": "Authorization"}}}, http.StatusOK, &created)
	var test struct {
		Delivery struct {
			Status          string `json:"status"`
			StatusCode      *int   `json:"status_code"`
			ResponseExcerpt string `json:"response_excerpt"`
			DurationMs      int64  `json:"duration_ms"`
		} `json:"delivery"`
	}
	adm.call(http.MethodPost, "/webhooks/"+id+"/test", nil, http.StatusOK, &test)
	if test.Delivery.Status != "succeeded" || *test.Delivery.StatusCode != 200 || test.Delivery.ResponseExcerpt != `{"ok":true}` || gotAuth != "Bearer test-secret" {
		t.Fatalf("test %+v auth %q", test.Delivery, gotAuth)
	}

	// Omitting a header from the list deletes it.
	adm.call(http.MethodPatch, "/webhooks/"+id, map[string]any{"headers": []any{}}, http.StatusOK, &created)
	if len(created.Webhook.Headers) != 0 || created.Webhook.Name != "ntfy phone" {
		t.Fatalf("after header removal %+v", created.Webhook)
	}

	var bad struct {
		Error struct {
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	adm.call(http.MethodPost, "/webhooks", map[string]any{"name": "x", "url": "ftp://x", "preset": "pager", "headers": []map[string]string{{"name": "Host", "value": "x"}}}, http.StatusUnprocessableEntity, &bad)
	if bad.Error.Fields["url"] != "validation.url" || bad.Error.Fields["preset"] != "validation.invalid_choice" || bad.Error.Fields["headers"] != "validation.header_name" {
		t.Fatalf("fields %v", bad.Error.Fields)
	}

	var deliveries listResponse[deliveryJSON]
	adm.call(http.MethodGet, "/webhooks/"+id+"/deliveries", nil, http.StatusOK, &deliveries)
	if deliveries.Total != 1 || deliveries.Items[0].Event != "test" {
		t.Fatalf("deliveries %+v", deliveries)
	}

	var inst instanceSettingsJSON
	adm.call(http.MethodGet, "/settings/instance", nil, http.StatusOK, &inst)
	if inst.WeeklyDigestWeekday != 1 || inst.MaxUploadMB != 25 {
		t.Fatalf("instance %+v", inst)
	}
	adm.call(http.MethodPatch, "/settings/instance", map[string]int{"monthly_digest_day": 15}, http.StatusOK, &inst)
	if inst.MonthlyDigestDay != 15 || inst.WeeklyDigestHour != 8 {
		t.Fatalf("patched %+v", inst)
	}
	adm.errorCode(http.MethodPatch, "/settings/instance", map[string]int{"monthly_digest_day": 31}, http.StatusUnprocessableEntity)
	adm.call(http.MethodDelete, "/webhooks/"+id, nil, http.StatusNoContent, nil)
}
