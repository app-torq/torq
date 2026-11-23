package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/attachments"
	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/store"
)

// maxLogoBytes caps the instance logo, which is stored in the database.
const maxLogoBytes = 512 << 10

// contactJSON holds the contact details printed on the vehicle history.
type contactJSON struct {
	Address string `json:"address"`
	Phone   string `json:"phone"`
	Email   string `json:"email"`
	Website string `json:"website"`
}

// brandingJSON is the instance name and logo (#32). Contact is set only for
// signed-in users, since GET /instance is public.
type brandingJSON struct {
	Name    string       `json:"name"`
	LogoURL *string      `json:"logo_url"`
	Contact *contactJSON `json:"contact,omitempty"`
}

func contactOf(c store.InstanceSetting) *contactJSON {
	return &contactJSON{Address: c.BrandAddress, Phone: c.BrandPhone, Email: c.BrandEmail, Website: c.BrandWebsite}
}

// logoURL returns the logo's URL, versioned so a new logo is never served
// from the browser cache, or nil without a logo.
func (s *Server) logoURL(ctx context.Context) (*string, error) {
	updated, err := store.New(s.DB.Reader).GetInstanceLogoUpdatedAt(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u := "/api/v1/instance/logo?v=" + url.QueryEscape(updated)
	return &u, nil
}

func (s *Server) branding(ctx context.Context, withContact bool) (brandingJSON, error) {
	c, err := store.New(s.DB.Reader).GetInstanceSettings(ctx)
	if err != nil {
		return brandingJSON{}, err
	}
	logo, err := s.logoURL(ctx)
	if err != nil {
		return brandingJSON{}, err
	}
	b := brandingJSON{Name: c.BrandName, LogoURL: logo}
	if withContact {
		b.Contact = contactOf(c)
	}
	return b, nil
}

// brandingIn is the branding part of PATCH /settings/instance.
type brandingIn struct {
	Name string `json:"name"`
	contactJSON
}

func (in *brandingIn) validate(v *apperr.Validation) {
	for _, f := range []struct {
		field string
		value *string
		max   int
	}{
		{"branding.name", &in.Name, 60}, {"branding.address", &in.Address, 300}, {"branding.phone", &in.Phone, 40},
		{"branding.email", &in.Email, 120}, {"branding.website", &in.Website, 200},
	} {
		*f.value = strings.TrimSpace(*f.value)
		v.Check(utf8.RuneCountInString(*f.value) > f.max, f.field, "validation.length")
	}
}

// getLogo serves the logo to anyone, since the sign-in page shows it. An SVG
// opened directly gets a sandbox without scripts; in the app it is only ever
// shown through <img>, where scripts never run.
func (s *Server) getLogo(w http.ResponseWriter, r *http.Request) error {
	logo, err := store.New(s.DB.Reader).GetInstanceLogo(r.Context())
	if errors.Is(err, sql.ErrNoRows) {
		return apperr.NotFound()
	}
	if err != nil {
		return err
	}
	h := w.Header()
	h.Set("Content-Type", logo.MimeType)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	h.Set("Cache-Control", "public, max-age=86400")
	modified, _ := time.Parse(time.RFC3339, logo.UpdatedAt)
	http.ServeContent(w, r, "", modified, bytes.NewReader(logo.Data))
	return nil
}

func (s *Server) putLogo(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxLogoBytes+1<<20)
	mr, err := r.MultipartReader()
	if err != nil {
		return apperr.BadRequest("request.malformed")
	}
	var data []byte
	for data == nil {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return apperr.Unprocessable("attachment.missing_file", nil)
		}
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				return logoTooLarge()
			}
			return apperr.BadRequest("request.malformed")
		}
		if part.FormName() != "file" {
			continue
		}
		if data, err = io.ReadAll(io.LimitReader(part, maxLogoBytes+1)); err != nil {
			return apperr.BadRequest("request.malformed")
		}
	}
	if len(data) == 0 {
		return apperr.Unprocessable("attachment.empty", nil)
	}
	if len(data) > maxLogoBytes {
		return logoTooLarge()
	}
	mt := sniffLogo(data)
	if mt == "" {
		return apperr.Unsupported("instance.logo_type_not_allowed")
	}
	err = store.New(s.DB.Writer).PutInstanceLogo(r.Context(), store.PutInstanceLogoParams{
		MimeType: mt, Data: data, UpdatedAt: clock.Timestamp(s.Clock.Now()),
	})
	if err != nil {
		return err
	}
	return s.writeBranding(w, r)
}

func (s *Server) deleteLogo(w http.ResponseWriter, r *http.Request) error {
	if err := store.New(s.DB.Writer).DeleteInstanceLogo(r.Context()); err != nil {
		return err
	}
	return s.writeBranding(w, r)
}

func (s *Server) writeBranding(w http.ResponseWriter, r *http.Request) error {
	b, err := s.branding(r.Context(), true)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"branding": b})
	return nil
}

func logoTooLarge() error {
	return apperr.Unprocessable("instance.logo_too_large", map[string]any{"max_kb": maxLogoBytes >> 10})
}

// sniffLogo returns the logo's content type: PNG or JPEG by their magic
// bytes, or SVG when the file is well-formed XML with an <svg> root in the SVG
// namespace. A document type declaration is refused, so no entities either.
func sniffLogo(b []byte) string {
	if mt := attachments.Sniff(b); mt == "image/png" || mt == "image/jpeg" {
		return mt
	}
	d := xml.NewDecoder(bytes.NewReader(b))
	root := false
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			if root {
				return "image/svg+xml"
			}
			return ""
		}
		if err != nil {
			return ""
		}
		switch t := tok.(type) {
		case xml.Directive:
			return ""
		case xml.StartElement:
			if !root && (t.Name.Local != "svg" || t.Name.Space != "http://www.w3.org/2000/svg") {
				return ""
			}
			root = true
		case xml.CharData:
			if !root && len(bytes.TrimSpace(t)) > 0 {
				return ""
			}
		}
	}
}
