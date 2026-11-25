package vehicles

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/store"
)

// Public read-only links (#38, specs/007-vehicle-share-link). A vehicle has at
// most one; its token is shown to signed-in users so labels can be reprinted,
// and found only through its SHA-256 (research S1, S2).

// ShareOptions are the data a link may show besides the always-public record.
type ShareOptions struct {
	IncludeCosts        bool
	IncludeLicensePlate bool
	IncludeVIN          bool
}

// NewShareToken returns 32 random bytes as 43 characters of unpadded base64url.
func NewShareToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// ShareHash is the lookup key of a token.
func ShareHash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// Share returns the vehicle's link; apperr not found when there is none.
func (s *Service) Share(ctx context.Context, vehicleID int64) (store.VehicleShare, error) {
	sh, err := store.New(s.DB.Reader).GetVehicleShare(ctx, vehicleID)
	if errors.Is(err, sql.ErrNoRows) {
		return store.VehicleShare{}, apperr.NotFound()
	}
	return sh, err
}

// linksEnabled reports the instance switch.
func linksEnabled(ctx context.Context, q *store.Queries) (bool, error) {
	c, err := q.GetInstanceSettings(ctx)
	return c.PublicLinksEnabled == 1, err
}

// EnableShare turns the vehicle's link on with opts, or changes the options of
// the link it has; created reports which. The token of an existing link stays.
func (s *Service) EnableShare(ctx context.Context, vehicleID int64, opts ShareOptions, userID int64) (sh store.VehicleShare, created bool, err error) {
	err = s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		if ok, err := linksEnabled(ctx, q); err != nil || !ok {
			return cmpErr(err, apperr.Conflict("share.disabled", nil))
		}
		if _, err := Get(ctx, q, vehicleID); err != nil {
			return err
		}
		sh, err = q.UpdateVehicleShareOptions(ctx, store.UpdateVehicleShareOptionsParams{
			IncludeCosts: boolInt(opts.IncludeCosts), IncludeLicensePlate: boolInt(opts.IncludeLicensePlate),
			IncludeVin: boolInt(opts.IncludeVIN), VehicleID: vehicleID,
		})
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		token, err := NewShareToken()
		if err != nil {
			return err
		}
		created = true
		sh, err = q.InsertVehicleShare(ctx, store.InsertVehicleShareParams{
			VehicleID: vehicleID, Token: token, TokenSha256: ShareHash(token),
			IncludeCosts: boolInt(opts.IncludeCosts), IncludeLicensePlate: boolInt(opts.IncludeLicensePlate),
			IncludeVin: boolInt(opts.IncludeVIN), CreatedBy: sql.NullInt64{Int64: userID, Valid: userID > 0},
			CreatedAt: clock.Timestamp(s.Clock.Now()),
		})
		return err
	})
	return sh, created, err
}

// cmpErr returns err if set, else fallback.
func cmpErr(err, fallback error) error {
	if err != nil {
		return err
	}
	return fallback
}

// RenewShare gives the vehicle's link a new token; the old one stops working.
func (s *Service) RenewShare(ctx context.Context, vehicleID int64, userID int64) (store.VehicleShare, error) {
	var sh store.VehicleShare
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		if ok, err := linksEnabled(ctx, q); err != nil || !ok {
			return cmpErr(err, apperr.Conflict("share.disabled", nil))
		}
		token, err := NewShareToken()
		if err != nil {
			return err
		}
		sh, err = q.RenewVehicleShare(ctx, store.RenewVehicleShareParams{
			Token: token, TokenSha256: ShareHash(token), CreatedBy: sql.NullInt64{Int64: userID, Valid: userID > 0},
			CreatedAt: clock.Timestamp(s.Clock.Now()), VehicleID: vehicleID,
		})
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound()
		}
		return err
	})
	return sh, err
}

// DisableShare turns the vehicle's link off; turning it on again creates a
// new token. Disabling a vehicle without a link is not an error.
func (s *Service) DisableShare(ctx context.Context, vehicleID int64) error {
	return store.New(s.DB.Writer).DeleteVehicleShare(ctx, vehicleID)
}

// ResolveShare finds the link for a token taken from a public request. Every
// failure is the same not found: unknown, revoked or malformed tokens,
// archived vehicles and a disabled instance (FR-004). The token is always
// hashed and looked up, whatever it looks like, so all misses take the same
// path.
func (s *Service) ResolveShare(ctx context.Context, token string) (store.VehicleShare, error) {
	row, err := store.New(s.DB.Reader).ResolveVehicleShare(ctx, ShareHash(token))
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (row.VehicleArchivedAt.Valid || row.LinksEnabled != 1)) {
		return store.VehicleShare{}, apperr.NotFound()
	}
	if err != nil {
		return store.VehicleShare{}, err
	}
	return store.VehicleShare{
		VehicleID: row.VehicleID, Token: row.Token, TokenSha256: row.TokenSha256, IncludeCosts: row.IncludeCosts,
		IncludeLicensePlate: row.IncludeLicensePlate, IncludeVin: row.IncludeVin, CreatedBy: row.CreatedBy,
		CreatedAt: row.CreatedAt, LastOpenedAt: row.LastOpenedAt,
	}, nil
}

// TouchShare records a view, at most once an hour.
func (s *Service) TouchShare(ctx context.Context, vehicleID int64) error {
	now := s.Clock.Now()
	return store.New(s.DB.Writer).TouchVehicleShare(ctx, store.TouchVehicleShareParams{
		Now:       sql.NullString{String: clock.Timestamp(now), Valid: true},
		VehicleID: vehicleID,
		HourAgo:   sql.NullString{String: clock.Timestamp(now.Add(-time.Hour)), Valid: true},
	})
}

// SetPublicLinks turns public links on or off for the whole instance.
func (s *Service) SetPublicLinks(ctx context.Context, enabled bool) (store.InstanceSetting, error) {
	return store.New(s.DB.Writer).SetPublicLinksEnabled(ctx, boolInt(enabled))
}
