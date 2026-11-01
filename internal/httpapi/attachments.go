package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/marekbeckmann/torq/internal/apperr"
	"github.com/marekbeckmann/torq/internal/attachments"
	"github.com/marekbeckmann/torq/internal/store"
)

type attachmentJSON struct {
	ID           int64  `json:"id"`
	OriginalName string `json:"original_name"`
	MimeType     string `json:"mime_type"`
	SizeBytes    int64  `json:"size_bytes"`
	Caption      string `json:"caption"`
	URL          string `json:"url"`
	CreatedAt    string `json:"created_at"`
}

func attachmentURL(id int64) string { return "/api/v1/attachments/" + strconv.FormatInt(id, 10) }

func toAttachmentJSON(a store.Attachment) attachmentJSON {
	return attachmentJSON{
		ID: a.ID, OriginalName: a.OriginalName, MimeType: a.MimeType, SizeBytes: a.SizeBytes,
		Caption: a.Caption, URL: attachmentURL(a.ID), CreatedAt: a.CreatedAt,
	}
}

func toAttachmentList(rows []store.Attachment, err error) ([]attachmentJSON, error) {
	if err != nil {
		return nil, err
	}
	out := make([]attachmentJSON, 0, len(rows))
	for _, a := range rows {
		out = append(out, toAttachmentJSON(a))
	}
	return out, nil
}

func nid(id int64) sql.NullInt64 { return sql.NullInt64{Int64: id, Valid: true} }

func (s *Server) serviceLogAttachments(ctx context.Context, id int64) ([]attachmentJSON, error) {
	return toAttachmentList(store.New(s.DB.Reader).ListAttachmentsForServiceLog(ctx, nid(id)))
}

func (s *Server) repairAttachments(ctx context.Context, id int64) ([]attachmentJSON, error) {
	return toAttachmentList(store.New(s.DB.Reader).ListAttachmentsForRepair(ctx, nid(id)))
}

func (s *Server) specEntryAttachments(ctx context.Context, id int64) ([]attachmentJSON, error) {
	return toAttachmentList(store.New(s.DB.Reader).ListAttachmentsForSpecEntry(ctx, nid(id)))
}

func (s *Server) vehicleAvatars(ctx context.Context) (map[int64]string, error) {
	rows, err := store.New(s.DB.Reader).ListVehicleAvatars(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]string, len(rows))
	for _, r := range rows {
		out[r.VehicleID.Int64] = attachmentURL(r.ID)
	}
	return out, nil
}

func (s *Server) collectSpecEntryFiles(id int64) func(context.Context, *sql.Tx) ([]string, error) {
	return func(ctx context.Context, tx *sql.Tx) ([]string, error) { return s.files.KeysForSpecEntry(ctx, tx, id) }
}

func (s *Server) removeFiles(keys []string) { s.files.RemoveFiles(keys) }

// receiveUpload streams the multipart "file" part (with an optional preceding
// "caption" field) into the attachment store.
func (s *Server) receiveUpload(w http.ResponseWriter, r *http.Request, owner attachments.Owner, purpose string) (store.Attachment, error) {
	r.Body = http.MaxBytesReader(w, r.Body, s.Config.MaxUploadBytes()+1<<20)
	mr, err := r.MultipartReader()
	if err != nil {
		return store.Attachment{}, apperr.BadRequest("request.malformed")
	}
	caption := ""
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return store.Attachment{}, apperr.Unprocessable("attachment.missing_file", nil)
		}
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				return store.Attachment{}, apperr.TooLarge(s.Config.MaxUploadMB)
			}
			return store.Attachment{}, apperr.BadRequest("request.malformed")
		}
		switch part.FormName() {
		case "caption":
			b, err := io.ReadAll(io.LimitReader(part, 1024))
			if err != nil {
				return store.Attachment{}, apperr.BadRequest("request.malformed")
			}
			caption = string(b)
		case "file":
			return s.files.Save(r.Context(), part, part.FileName(), owner, purpose, caption, currentUser(r).ID)
		}
	}
}

func (s *Server) uploadTo(owner func(id int64) attachments.Owner) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := pathID(r, "id")
		if err != nil {
			return err
		}
		a, err := s.receiveUpload(w, r, owner(id), attachments.PurposeFile)
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusCreated, map[string]any{"attachment": toAttachmentJSON(a)})
		return nil
	}
}

func (s *Server) putAvatar(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	if _, err := s.vehicles.Get(r.Context(), id); err != nil {
		return err
	}
	if _, err := s.receiveUpload(w, r, attachments.Owner{VehicleID: id}, attachments.PurposeAvatar); err != nil {
		return err
	}
	v, err := s.vehicles.Get(r.Context(), id)
	if err != nil {
		return err
	}
	out, err := s.vehicleOut(r.Context(), v)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"vehicle": out})
	return nil
}

func (s *Server) deleteAvatar(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	a, err := store.New(s.DB.Reader).GetVehicleAvatar(r.Context(), nid(id))
	if errors.Is(err, sql.ErrNoRows) {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
	if err != nil {
		return err
	}
	if err := s.files.Delete(r.Context(), a.ID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// getAttachment serves a file to signed-in users with headers that prevent the
// browser from interpreting it as anything but the sniffed type (R18).
func (s *Server) getAttachment(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	a, f, err := s.files.Open(r.Context(), id)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	disposition := "inline"
	if r.URL.Query().Get("download") == "1" || a.MimeType == "image/heic" || a.MimeType == "image/heif" {
		disposition = "attachment"
	}
	h := w.Header()
	h.Set("Content-Type", a.MimeType)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox")
	h.Set("Cache-Control", "private, max-age=86400")
	h.Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": a.OriginalName}))
	modified, _ := time.Parse(time.RFC3339, a.CreatedAt)
	http.ServeContent(w, r, "", modified, f)
	return nil
}

func (s *Server) patchAttachment(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Caption string `json:"caption"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	a, err := s.files.UpdateCaption(r.Context(), id, in.Caption)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"attachment": toAttachmentJSON(a)})
	return nil
}

func (s *Server) deleteAttachment(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	if err := s.files.Delete(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
