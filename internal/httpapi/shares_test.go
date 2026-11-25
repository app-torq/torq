package httpapi

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Share links (#38, specs/007-vehicle-share-link).

type shareResp struct {
	Share *shareJSON `json:"share"`
}

// sharedVehicle creates a vehicle with every kind of data the public record
// must leave out unless included: a license plate, a VIN, notes, costs, an
// attachment and a repair.
func sharedVehicle(t *testing.T, e *env, c *client) int64 {
	t.Helper()
	var out vehicleResp
	c.call(http.MethodPost, "/vehicles", map[string]any{
		"make": "Mercedes-Benz", "model": "C 220 CDI", "license_plate": "HH-SC 4711", "vin": "WDB2030161A987654",
		"current_odometer_km": 210000,
	}, http.StatusCreated, &out)
	id := out.Vehicle.ID
	cats := categoryIDs(t, c)
	var log struct {
		ServiceLog struct {
			ID int64 `json:"id"`
		} `json:"service_log"`
	}
	c.call(http.MethodPost, "/service-logs", map[string]any{
		"vehicle_id": id, "service_date": "2026-09-01", "odometer_km": 212000, "notes": "Paid cash to Klaus, call 0170-5550199",
		"items": []map[string]any{{"category_id": cats["engine_oil"], "brand": "Mobil 1", "specification": "MB 229.51", "cost_cents": 8765}},
	}, http.StatusCreated, &log)
	c.call(http.MethodPost, "/repairs", map[string]any{"vehicle_id": id, "title": "Secret repair project"}, http.StatusCreated, nil)
	c.call(http.MethodPost, "/reminders", map[string]any{"vehicle_id": id, "service_category_id": cats["engine_oil"], "interval_km": 15000}, http.StatusCreated, nil)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "invoice-klaus.png")
	_, _ = fw.Write(append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 64)...))
	_ = mw.Close()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/service-logs/"+itoa(log.ServiceLog.ID)+"/attachments", &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set("X-Torq-Request", "1")
	r.AddCookie(c.cookie)
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, r)
	if rec.Code != http.StatusCreated {
		t.Fatalf("attachment %d %s", rec.Code, rec.Body)
	}
	c.call(http.MethodPut, "/vehicles/"+itoa(id)+"/specs", map[string]any{"engine_oil_spec": "MB 229.51", "engine_oil_capacity_ml": 6500}, http.StatusOK, nil)
	return id
}

// publicGet opens a share link like a visitor, from ip, optionally with a language.
func publicGet(e *env, ip, token, lang string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/shared/"+token, nil)
	r.RemoteAddr = ip + ":1234"
	if lang != "" {
		r.Header.Set("Accept-Language", lang)
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, r)
	if err := contract(e.t).checkResponse(http.MethodGet, r.URL.Path, rec.Code, rec.Header().Get("Content-Type"), rec.Body.Bytes()); err != nil {
		e.t.Errorf("shared record does not match openapi.yaml: %v\n%s", err, rec.Body)
	}
	return rec
}

func TestShareRecordShowsOnlyAllowedData(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	id := sharedVehicle(t, e, c)
	path := "/vehicles/" + itoa(id) + "/share"

	var got shareResp
	c.call(http.MethodGet, path, nil, http.StatusOK, &got)
	if got.Share != nil {
		t.Fatal("vehicle shared by default")
	}
	c.call(http.MethodPut, path, map[string]any{}, http.StatusCreated, &got)
	sh := got.Share
	if len(sh.Token) != 43 || sh.Path != "/s/"+sh.Token || sh.IncludeCosts || sh.IncludeVIN || sh.IncludeLicensePlate {
		t.Fatalf("share %+v", sh)
	}

	rec := publicGet(e, "198.51.100.1", sh.Token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("public %d %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	for _, hidden := range []string{"HH-SC 4711", "WDB2030161A987654", "Klaus", "0170", "8765", "invoice", "Secret repair", "marek", "Marek", `"vehicle_id"`, `"id":` + itoa(id) + ","} {
		if strings.Contains(body, hidden) {
			t.Errorf("public record contains %q", hidden)
		}
	}
	var pub publicRecordJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &pub); err != nil {
		t.Fatal(err)
	}
	if pub.Vehicle.Make != "Mercedes-Benz" || len(pub.ServiceLogs) != 1 || pub.ServiceLogs[0].Items[0].Brand != "Mobil 1" ||
		pub.ServiceLogs[0].Items[0].CostCents != nil || len(pub.Reminders) != 1 || pub.Reminders[0].Name != "Engine Oil & Filter" ||
		pub.Specs["engine_oil_spec"] != "MB 229.51" {
		t.Fatalf("record %+v", pub)
	}
	h := rec.Header()
	if h.Get("Cache-Control") != "no-store" || h.Get("X-Robots-Tag") != "noindex, nofollow, noarchive" || h.Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("headers %v", h)
	}

	// German names with a German browser; the page shell gets the same headers.
	var de publicRecordJSON
	_ = json.Unmarshal(publicGet(e, "198.51.100.1", sh.Token, "de-AT,de;q=0.9,en;q=0.5").Body.Bytes(), &de)
	if len(de.Reminders) != 1 || de.Reminders[0].Name == pub.Reminders[0].Name {
		t.Errorf("not localized: %q", de.Reminders[0].Name)
	}
	page := httptest.NewRecorder()
	e.h.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/s/"+sh.Token, nil))
	if page.Code != http.StatusOK || page.Header().Get("Cache-Control") != "no-store" || page.Header().Get("X-Robots-Tag") == "" {
		t.Fatalf("page %d %v", page.Code, page.Header())
	}

	// Options: same token, the data appears.
	c.call(http.MethodPut, path, map[string]any{"include_costs": true, "include_vin": true, "include_license_plate": true}, http.StatusOK, &got)
	if got.Share.Token != sh.Token {
		t.Fatal("options changed the token")
	}
	body = publicGet(e, "198.51.100.1", sh.Token, "").Body.String()
	for _, shown := range []string{"HH-SC 4711", "WDB2030161A987654", "8765"} {
		if !strings.Contains(body, shown) {
			t.Errorf("included %q missing", shown)
		}
	}
	// Notes, attachments and repairs stay out whatever the options say.
	for _, hidden := range []string{"Klaus", "invoice", "Secret repair"} {
		if strings.Contains(body, hidden) {
			t.Errorf("public record contains %q with all options on", hidden)
		}
	}
	// A partial update keeps the other options.
	c.call(http.MethodPut, path, map[string]any{"include_costs": false}, http.StatusOK, &got)
	if got.Share.IncludeCosts || !got.Share.IncludeVIN || !got.Share.IncludeLicensePlate {
		t.Fatalf("partial update %+v", got.Share)
	}

	// Signed in, the record names the vehicle for "Open in torq".
	r := httptest.NewRequest(http.MethodGet, "/api/v1/shared/"+sh.Token, nil)
	r.AddCookie(c.cookie)
	signed := httptest.NewRecorder()
	e.h.ServeHTTP(signed, r)
	var withID publicRecordJSON
	_ = json.Unmarshal(signed.Body.Bytes(), &withID)
	if withID.VehicleID == nil || *withID.VehicleID != id {
		t.Fatalf("signed in: %s", signed.Body)
	}
}

func TestShareMissesLookAlikeAndAreThrottled(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	id := sharedVehicle(t, e, c)
	var first, second shareResp
	c.call(http.MethodPut, "/vehicles/"+itoa(id)+"/share", map[string]any{}, http.StatusCreated, &first)
	c.call(http.MethodPost, "/vehicles/"+itoa(id)+"/share/renew", nil, http.StatusOK, &second)

	revoked := publicGet(e, "203.0.113.1", first.Share.Token, "")
	unknown := publicGet(e, "203.0.113.2", strings.Repeat("A", 43), "")
	malformed := publicGet(e, "203.0.113.3", "not-a-token", "")
	for _, rec := range []*httptest.ResponseRecorder{revoked, unknown, malformed} {
		if rec.Code != http.StatusNotFound || rec.Body.String() != revoked.Body.String() {
			t.Fatalf("miss %d %s", rec.Code, rec.Body)
		}
	}

	// Ten failures from one address, then it has to wait; others don't.
	for range 9 {
		publicGet(e, "203.0.113.50", strings.Repeat("B", 43), "")
	}
	if rec := publicGet(e, "203.0.113.50", strings.Repeat("B", 43), ""); rec.Code != http.StatusNotFound {
		t.Fatalf("10th failure %d", rec.Code)
	}
	blocked := publicGet(e, "203.0.113.50", second.Share.Token, "")
	if blocked.Code != http.StatusTooManyRequests || !strings.Contains(blocked.Body.String(), "auth.throttled") {
		t.Fatalf("blocked %d %s", blocked.Code, blocked.Body)
	}
	if rec := publicGet(e, "203.0.113.51", second.Share.Token, ""); rec.Code != http.StatusOK {
		t.Fatalf("other address %d", rec.Code)
	}
	c.call(http.MethodGet, "/vehicles/"+itoa(id)+"/share", nil, http.StatusOK, nil)
}

func TestShareRevokeRenewArchive(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	id := sharedVehicle(t, e, c)
	path := "/vehicles/" + itoa(id) + "/share"
	ok := func(token string) bool { return publicGet(e, "192.0.2.1", token, "").Code == http.StatusOK }

	if code := c.errorCode(http.MethodPost, path+"/renew", nil, http.StatusNotFound); code != "not_found" {
		t.Fatalf("renew without link: %s", code)
	}
	var a, b shareResp
	c.call(http.MethodPut, path, map[string]any{}, http.StatusCreated, &a)
	c.call(http.MethodPost, path+"/renew", nil, http.StatusOK, &b)
	if ok(a.Share.Token) || !ok(b.Share.Token) {
		t.Fatal("renew did not replace the token")
	}

	c.call(http.MethodPost, "/vehicles/"+itoa(id)+"/archive", nil, http.StatusOK, nil)
	if ok(b.Share.Token) {
		t.Fatal("archived vehicle shared")
	}
	c.call(http.MethodPost, "/vehicles/"+itoa(id)+"/unarchive", nil, http.StatusOK, nil)
	if !ok(b.Share.Token) {
		t.Fatal("restored vehicle not shared again")
	}

	c.call(http.MethodDelete, path, nil, http.StatusNoContent, nil)
	c.call(http.MethodDelete, path, nil, http.StatusNoContent, nil)
	if ok(b.Share.Token) {
		t.Fatal("turned-off link works")
	}
	var fresh shareResp
	c.call(http.MethodPut, path, map[string]any{}, http.StatusCreated, &fresh)
	if fresh.Share.Token == a.Share.Token || fresh.Share.Token == b.Share.Token || !ok(fresh.Share.Token) {
		t.Fatal("turning on again reused a token")
	}

	c.call(http.MethodDelete, "/vehicles/"+itoa(id), map[string]string{"confirm_name": "Mercedes-Benz C 220 CDI"}, http.StatusNoContent, nil)
	if ok(fresh.Share.Token) {
		t.Fatal("deleted vehicle shared")
	}
}

func TestShareAdminListAndSwitch(t *testing.T) {
	e := newEnv(t)
	adm := e.admin()
	adm.call(http.MethodPost, "/users", map[string]string{"username": "lena", "display_name": "Lena", "password": testPassword, "role": "member"}, http.StatusCreated, nil)
	member := e.login("lena", testPassword)
	id := sharedVehicle(t, e, adm)

	var sh shareResp
	member.call(http.MethodPut, "/vehicles/"+itoa(id)+"/share", map[string]any{}, http.StatusCreated, &sh)
	member.errorCode(http.MethodGet, "/shares", nil, http.StatusForbidden)
	var list struct {
		Items []shareListItemJSON `json:"items"`
	}
	adm.call(http.MethodGet, "/shares", nil, http.StatusOK, &list)
	if len(list.Items) != 1 || list.Items[0].VehicleID != id || list.Items[0].CreatedByName == nil || *list.Items[0].CreatedByName != "Lena" {
		t.Fatalf("list %+v", list.Items)
	}
	raw, _ := json.Marshal(list)
	if strings.Contains(string(raw), sh.Share.Token) {
		t.Fatal("admin list contains the token")
	}

	var info struct {
		PublicLinks bool `json:"public_links"`
	}
	e.client().call(http.MethodGet, "/instance", nil, http.StatusOK, &info)
	if !info.PublicLinks {
		t.Fatal("public_links off by default")
	}
	adm.call(http.MethodPatch, "/settings/instance", map[string]any{"public_links_enabled": false}, http.StatusOK, nil)
	e.client().call(http.MethodGet, "/instance", nil, http.StatusOK, &info)
	if info.PublicLinks || publicGet(e, "192.0.2.9", sh.Share.Token, "").Code != http.StatusNotFound {
		t.Fatal("switch did not stop the link")
	}
	if code := member.errorCode(http.MethodPost, "/vehicles/"+itoa(id)+"/share/renew", nil, http.StatusConflict); code != "share.disabled" {
		t.Fatalf("renew while off: %s", code)
	}
	adm.call(http.MethodPatch, "/settings/instance", map[string]any{"public_links_enabled": true}, http.StatusOK, nil)
	if publicGet(e, "192.0.2.9", sh.Share.Token, "").Code != http.StatusOK {
		t.Fatal("link not back after switching on")
	}
}
