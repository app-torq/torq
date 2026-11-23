package httpapi

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testSVG = `<?xml version="1.0"?>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`

func TestSniffLogo(t *testing.T) {
	cases := map[string]string{
		"\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 16): "image/png",
		"\xFF\xD8\xFF\xE0" + strings.Repeat("\x00", 16):  "image/jpeg",
		testSVG: "image/svg+xml",
		`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`: "image/svg+xml",
		`<svg><rect/></svg>`: "",
		`<!DOCTYPE svg [<!ENTITY x "y">]><svg xmlns="http://www.w3.org/2000/svg">&x;</svg>`: "",
		`<html xmlns="http://www.w3.org/2000/svg"></html>`:                                  "",
		`<svg xmlns="http://www.w3.org/2000/svg">`:                                          "",
		`hello <svg xmlns="http://www.w3.org/2000/svg"/>`:                                   "",
		"GIF89a" + strings.Repeat("\x00", 16):                                               "",
		"%PDF-1.7\n":                                                                        "",
	}
	for in, want := range cases {
		if got := sniffLogo([]byte(in)); got != want {
			t.Errorf("sniffLogo(%.40q) = %q, want %q", in, got, want)
		}
	}
}

// uploadLogo sends file as the multipart "file" part.
func uploadLogo(c *client, file []byte) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "logo")
	_, _ = fw.Write(file)
	_ = mw.Close()
	r := httptest.NewRequest(http.MethodPut, "/api/v1/settings/instance/logo", &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set("X-Torq-Request", "1")
	if c.cookie != nil {
		r.AddCookie(c.cookie)
	}
	rec := httptest.NewRecorder()
	c.e.h.ServeHTTP(rec, r)
	return rec
}

type brandingResp struct {
	Name    string  `json:"name"`
	LogoURL *string `json:"logo_url"`
	Contact *struct {
		Address string `json:"address"`
		Phone   string `json:"phone"`
		Email   string `json:"email"`
		Website string `json:"website"`
	} `json:"contact"`
}

func TestBranding(t *testing.T) {
	e := newEnv(t)
	adm := e.admin()
	adm.call(http.MethodPost, "/users", map[string]string{
		"username": "lena", "display_name": "Lena", "password": testPassword, "role": "member",
	}, http.StatusCreated, nil)
	member := e.login("lena", testPassword)

	branding := map[string]string{
		"name": "  Garage Nord  ", "address": "Hauptstr. 1\n12345 Musterstadt", "phone": "+49 123 456",
		"email": "info@garage-nord.example", "website": "garage-nord.example",
	}
	member.errorCode(http.MethodPatch, "/settings/instance", map[string]any{"branding": branding}, http.StatusForbidden)
	var settings struct {
		WeeklyDigestHour int64        `json:"weekly_digest_hour"`
		Branding         brandingResp `json:"branding"`
	}
	adm.call(http.MethodPatch, "/settings/instance", map[string]any{"branding": branding}, http.StatusOK, &settings)
	if settings.Branding.Name != "Garage Nord" || settings.Branding.Contact == nil || settings.Branding.Contact.Phone != "+49 123 456" {
		t.Fatalf("settings branding %+v", settings.Branding)
	}
	// Changing only the digest keeps the branding.
	adm.call(http.MethodPatch, "/settings/instance", map[string]any{"weekly_digest_hour": 9}, http.StatusOK, &settings)
	if settings.Branding.Name != "Garage Nord" || settings.WeeklyDigestHour != 9 {
		t.Fatalf("branding lost: %+v", settings)
	}
	var bad struct {
		Error struct {
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	adm.call(http.MethodPatch, "/settings/instance", map[string]any{"branding": map[string]string{"name": strings.Repeat("x", 61)}}, http.StatusUnprocessableEntity, &bad)
	if bad.Error.Fields["branding.name"] != "validation.length" {
		t.Fatalf("fields %v", bad.Error.Fields)
	}

	// Logo: admins only, PNG/JPEG/SVG up to 512 KB.
	if rec := uploadLogo(member, []byte(testSVG)); rec.Code != http.StatusForbidden {
		t.Fatalf("member upload %d", rec.Code)
	}
	for _, f := range [][]byte{[]byte("<html><script>alert(1)</script></html>"), []byte("%PDF-1.7\n")} {
		if rec := uploadLogo(adm, f); rec.Code != http.StatusUnsupportedMediaType || !strings.Contains(rec.Body.String(), "instance.logo_type_not_allowed") {
			t.Fatalf("bad logo %d %s", rec.Code, rec.Body)
		}
	}
	big := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, maxLogoBytes)...)
	if rec := uploadLogo(adm, big); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "instance.logo_too_large") {
		t.Fatalf("big logo %d %s", rec.Code, rec.Body)
	}
	if rec := uploadLogo(adm, []byte(testSVG)); rec.Code != http.StatusOK {
		t.Fatalf("svg logo %d %s", rec.Code, rec.Body)
	}

	// Public: name and logo, no contact details.
	var info struct {
		Branding brandingResp `json:"branding"`
	}
	e.client().call(http.MethodGet, "/instance", nil, http.StatusOK, &info)
	if info.Branding.Name != "Garage Nord" || info.Branding.LogoURL == nil || info.Branding.Contact != nil {
		t.Fatalf("public branding %+v", info.Branding)
	}
	var me struct {
		Instance struct {
			Branding brandingResp `json:"branding"`
		} `json:"instance"`
	}
	member.call(http.MethodGet, "/auth/me", nil, http.StatusOK, &me)
	if c := me.Instance.Branding.Contact; c == nil || c.Address != branding["address"] || c.Website != "garage-nord.example" {
		t.Fatalf("me branding %+v", me.Instance.Branding)
	}

	// The logo is served without a session, sandboxed.
	rec := e.client().do(http.MethodGet, strings.TrimPrefix(*info.Branding.LogoURL, "/api/v1"), nil)
	h := rec.Header()
	if rec.Code != http.StatusOK || h.Get("Content-Type") != "image/svg+xml" || h.Get("X-Content-Type-Options") != "nosniff" ||
		!strings.Contains(h.Get("Content-Security-Policy"), "sandbox") || rec.Body.String() != testSVG {
		t.Fatalf("logo %d %v %s", rec.Code, h, rec.Body)
	}

	// Exports carry the name.
	v := createVehicle(t, adm, 1)
	var hist struct {
		Instance struct {
			Name string `json:"name"`
		} `json:"instance"`
	}
	member.call(http.MethodGet, "/vehicles/"+itoa(v.ID)+"/history?download=0", nil, http.StatusOK, &hist)
	if hist.Instance.Name != "Garage Nord" {
		t.Fatalf("history instance %+v", hist.Instance)
	}
	csv := member.do(http.MethodGet, "/vehicles/"+itoa(v.ID)+"/history?format=csv", nil).Body.String()
	if lines := strings.Split(csv, "\n"); len(lines) < 2 || !strings.HasPrefix(lines[1], "instance,,,Garage Nord,") {
		t.Fatalf("csv %q", csv)
	}

	// Removing the logo.
	var removed struct {
		Branding brandingResp `json:"branding"`
	}
	adm.call(http.MethodDelete, "/settings/instance/logo", nil, http.StatusOK, &removed)
	if removed.Branding.LogoURL != nil {
		t.Fatal("logo not removed")
	}
	e.client().errorCode(http.MethodGet, "/instance/logo", nil, http.StatusNotFound)
}
