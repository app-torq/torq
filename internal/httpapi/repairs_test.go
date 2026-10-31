package httpapi

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type repairResp struct {
	Repair repairJSON `json:"repair"`
}

func (c *client) upload(path, filename string, content []byte, caption string) *httptest.ResponseRecorder {
	c.e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if caption != "" {
		_ = mw.WriteField("caption", caption)
	}
	fw, _ := mw.CreateFormFile("file", filename)
	_, _ = fw.Write(content)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1"+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Torq-Request", "1")
	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	rec := httptest.NewRecorder()
	c.e.h.ServeHTTP(rec, req)
	return rec
}

var testPDF = []byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\n")

func TestRepairContract(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	v := createVehicle(t, c, 215200)

	var created repairResp
	c.call(http.MethodPost, "/repairs", map[string]any{
		"vehicle_id": v.ID, "title": "Injector seal replacement", "symptom": "rough idle, P0201",
		"fault_codes":  []string{"P0201", "p0300"},
		"torque_specs": []map[string]any{{"label": "Injector clamp bolt", "value_nm": 7, "note": "+ 90°"}},
	}, http.StatusCreated, &created)
	r := created.Repair
	if r.Status != "planned" || len(r.FaultCodes) != 2 || len(r.TorqueSpecs) != 1 || r.PartStatusCounts["needed"] != 0 {
		t.Fatalf("created %+v", r)
	}
	base := "/repairs/" + itoa(r.ID)

	for _, name := range []string{"Seal", "Bolt"} {
		c.call(http.MethodPost, base+"/parts", map[string]any{"name": name, "quantity": 4, "unit_cost_cents": 250, "oem_part_number": "A611-" + name}, http.StatusCreated, nil)
	}
	var part struct {
		Part partJSON `json:"part"`
	}
	c.call(http.MethodPost, base+"/parts", map[string]any{"name": "Return line", "unit_cost_cents": 1999}, http.StatusCreated, &part)
	if part.Part.LineTotalCents != 1999 || part.Part.Status != "needed" {
		t.Fatalf("part %+v", part.Part)
	}
	c.call(http.MethodPatch, "/repair-parts/"+itoa(part.Part.ID), map[string]any{"status": "ordered"}, http.StatusOK, &part)
	if part.Part.Status != "ordered" || part.Part.Name != "Return line" {
		t.Fatalf("patched part %+v", part.Part)
	}

	// PATCH keeps omitted lists, replaces provided ones.
	var got repairResp
	c.call(http.MethodPatch, base, map[string]any{"root_cause": "worn copper seals", "special_tools": "E12 Torx\nInjector puller"}, http.StatusOK, &got)
	if len(got.Repair.FaultCodes) != 2 || got.Repair.RootCause != "worn copper seals" || got.Repair.PartsTotalCents != 4*250*2+1999 {
		t.Fatalf("patched %+v", got.Repair)
	}
	c.call(http.MethodPatch, base, map[string]any{"fault_codes": []string{"P0201"}}, http.StatusOK, &got)
	if len(got.Repair.FaultCodes) != 1 {
		t.Fatalf("fault codes %v", got.Repair.FaultCodes)
	}
	if code := c.errorCode(http.MethodPatch, base, map[string]any{"status": "completed"}, http.StatusBadRequest); code != "repair.status_use_status_endpoint" {
		t.Fatalf("code %q", code)
	}

	c.call(http.MethodPost, base+"/status", map[string]any{"status": "waiting_on_parts"}, http.StatusOK, nil)
	var list listResponse[repairSummaryJSON]
	c.call(http.MethodGet, "/repairs?status=planned&status=waiting_on_parts", nil, http.StatusOK, &list)
	if list.Total != 1 || list.Items[0].PartStatusCounts["ordered"] != 1 {
		t.Fatalf("list %+v", list)
	}
	c.call(http.MethodGet, "/repairs?q=P0201", nil, http.StatusOK, &list)
	if list.Total != 1 {
		t.Fatal("fault code search failed")
	}
	c.call(http.MethodGet, "/repairs?q=A611-Bolt", nil, http.StatusOK, &list)
	if list.Total != 1 {
		t.Fatal("part number search failed")
	}

	if code := c.errorCode(http.MethodPost, base+"/status", map[string]any{"status": "completed", "completed_on": "2026-10-30"}, http.StatusUnprocessableEntity); code != "validation.failed" {
		t.Fatalf("code %q", code)
	}
	var done struct {
		Repair  repairJSON  `json:"repair"`
		Vehicle vehicleJSON `json:"vehicle"`
	}
	c.call(http.MethodPost, base+"/status", map[string]any{"status": "completed", "completed_on": "2026-10-30", "completion_odometer_km": 216000}, http.StatusOK, &done)
	if done.Vehicle.CurrentOdometerKm != 216000 || done.Repair.Status != "completed" {
		t.Fatalf("done %+v %+v", done.Repair.Status, done.Vehicle.CurrentOdometerKm)
	}

	// Dashboard open repairs excludes completed ones.
	var dash struct {
		OpenRepairs []repairSummaryJSON `json:"open_repairs"`
	}
	c.call(http.MethodGet, "/dashboard", nil, http.StatusOK, &dash)
	if len(dash.OpenRepairs) != 0 {
		t.Fatalf("open repairs %+v", dash.OpenRepairs)
	}

	// Attachment upload, serving headers, file cleanup on owner delete.
	rec := c.upload(base+"/attachments", "manual.pdf", testPDF, "before")
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload %d %s", rec.Code, rec.Body)
	}
	var att struct {
		Attachment attachmentJSON `json:"attachment"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &att)
	if att.Attachment.Caption != "before" || att.Attachment.MimeType != "application/pdf" {
		t.Fatalf("attachment %+v", att.Attachment)
	}
	serve := c.do(http.MethodGet, "/attachments/"+itoa(att.Attachment.ID), nil)
	h := serve.Header()
	if serve.Code != 200 || h.Get("Content-Type") != "application/pdf" || h.Get("X-Content-Type-Options") != "nosniff" ||
		h.Get("Content-Security-Policy") != "sandbox" || !strings.HasPrefix(h.Get("Content-Disposition"), "inline") {
		t.Fatalf("serve %d %v", serve.Code, h)
	}
	if rec := c.upload(base+"/attachments", "evil.png", []byte("<html><script>x</script></html>"), ""); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("html upload: %d", rec.Code)
	}
	files := func() int {
		n := 0
		_ = filepath.Walk(filepath.Join(e.cfg.DataDir, "attachments"), func(_ string, info os.FileInfo, _ error) error {
			if info != nil && !info.IsDir() {
				n++
			}
			return nil
		})
		return n
	}
	if files() != 1 {
		t.Fatalf("files on disk: %d", files())
	}
	c.call(http.MethodDelete, base, nil, http.StatusNoContent, nil)
	if files() != 0 {
		t.Fatal("attachment file not removed with repair")
	}
	c.errorCode(http.MethodGet, "/attachments/"+itoa(att.Attachment.ID), nil, http.StatusNotFound)

	// Reopened/completed odometer reading is removed when the repair is deleted; odometer is not lowered.
	var veh vehicleResp
	c.call(http.MethodGet, "/vehicles/"+itoa(v.ID), nil, http.StatusOK, &veh)
	if veh.Vehicle.CurrentOdometerKm != 216000 {
		t.Fatalf("odometer lowered to %d", veh.Vehicle.CurrentOdometerKm)
	}
}

func TestAvatarUpload(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	v := createVehicle(t, c, 1)
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 64)...)
	req := func() *httptest.ResponseRecorder {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("file", "car.png")
		_, _ = fw.Write(png)
		_ = mw.Close()
		r := httptest.NewRequest(http.MethodPut, "/api/v1/vehicles/"+itoa(v.ID)+"/avatar", &buf)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		r.Header.Set("X-Torq-Request", "1")
		r.AddCookie(c.cookie)
		rec := httptest.NewRecorder()
		e.h.ServeHTTP(rec, r)
		return rec
	}
	for i := 0; i < 2; i++ { // second upload replaces the first
		if rec := req(); rec.Code != http.StatusOK {
			t.Fatalf("avatar %d %s", rec.Code, rec.Body)
		}
	}
	var got vehicleResp
	c.call(http.MethodGet, "/vehicles/"+itoa(v.ID), nil, http.StatusOK, &got)
	if got.Vehicle.AvatarURL == nil {
		t.Fatal("avatar_url missing")
	}
	c.call(http.MethodDelete, "/vehicles/"+itoa(v.ID)+"/avatar", nil, http.StatusNoContent, nil)
	c.call(http.MethodGet, "/vehicles/"+itoa(v.ID), nil, http.StatusOK, &got)
	if got.Vehicle.AvatarURL != nil {
		t.Fatal("avatar not removed")
	}
}
