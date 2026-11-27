package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/store"
)

// ErrShareRefused is the one answer for every recipient a vehicle can't be
// shared with, so the answer doesn't reveal which accounts exist (#50).
func ErrShareRefused() *apperr.Error { return apperr.Unprocessable("sharing.refused", nil) }

// Share is a vehicle assignment one user gave another.
type Share struct {
	UserID      int64
	Username    string
	DisplayName string
	Level       Level
	CreatedAt   string
}

// SharesBy lists the shares sharerID made on the vehicle.
func (s *Roles) SharesBy(ctx context.Context, sharerID, vehicleID int64) ([]Share, error) {
	return sharesBy(ctx, store.New(s.DB.Reader), sharerID, vehicleID)
}

func sharesBy(ctx context.Context, q *store.Queries, sharerID, vehicleID int64) ([]Share, error) {
	rows, err := q.ListSharesBy(ctx, store.ListSharesByParams{GrantedBy: sql.NullInt64{Int64: sharerID, Valid: true}, VehicleID: vehicleID})
	if err != nil {
		return nil, err
	}
	out := make([]Share, 0, len(rows))
	for _, r := range rows {
		out = append(out, Share{UserID: r.UserID, Username: r.Username, DisplayName: r.DisplayName, Level: Level(r.Level), CreatedAt: r.CreatedAt})
	}
	return out, nil
}

// Share gives the user named username access to the vehicle on behalf of
// sharerID, or changes the level of a share sharerID made before. The caller
// has checked that the sharer may share the vehicle. Every recipient that
// can't receive it gets ErrShareRefused: unknown, disabled, administrators,
// roles that see all vehicles, the sharer, and users who hold the vehicle
// through someone else.
func (s *Roles) Share(ctx context.Context, sharerID, vehicleID int64, username string, level Level) (Share, error) {
	var v apperr.Validation
	username = strings.TrimSpace(username)
	v.Check(username == "", "username", "validation.required")
	v.Check(level != LevelView && level != LevelEdit, "level", "validation.invalid_choice")
	if err := v.Err(); err != nil {
		return Share{}, err
	}
	var out Share
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		recipient, err := q.GetUserByUsername(ctx, username)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrShareRefused()
		} else if err != nil {
			return err
		}
		if recipient.ID == sharerID || recipient.DisabledAt.Valid {
			return ErrShareRefused()
		}
		role, err := q.GetRole(ctx, recipient.RoleID.Int64)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrShareRefused()
		} else if err != nil {
			return err
		}
		if role.BuiltinKey.String == builtinAdmin || role.AllVehicles == 1 {
			return ErrShareRefused()
		}
		cur, err := q.GetVehicleAccess(ctx, store.GetVehicleAccessParams{UserID: recipient.ID, VehicleID: vehicleID})
		switch {
		case errors.Is(err, sql.ErrNoRows):
		case err != nil:
			return err
		case cur.Origin != OriginShared || cur.GrantedBy.Int64 != sharerID:
			return ErrShareRefused()
		}
		if err := q.AddVehicleAccess(ctx, store.AddVehicleAccessParams{
			UserID: recipient.ID, VehicleID: vehicleID, Level: string(level), CreatedAt: clock.Timestamp(s.Clock.Now()),
			Origin: OriginShared, GrantedBy: sql.NullInt64{Int64: sharerID, Valid: true},
		}); err != nil {
			return err
		}
		shares, err := sharesBy(ctx, q, sharerID, vehicleID)
		if err != nil {
			return err
		}
		for _, sh := range shares {
			if sh.UserID == recipient.ID {
				out = sh
			}
		}
		return nil
	})
	return out, err
}

// Unshare ends the share sharerID gave recipientID on the vehicle.
func (s *Roles) Unshare(ctx context.Context, sharerID, vehicleID, recipientID int64) error {
	return s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		cur, err := q.GetVehicleAccess(ctx, store.GetVehicleAccessParams{UserID: recipientID, VehicleID: vehicleID})
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound()
		} else if err != nil {
			return err
		}
		if cur.Origin != OriginShared || cur.GrantedBy.Int64 != sharerID {
			return apperr.NotFound()
		}
		return q.DeleteVehicleAccess(ctx, store.DeleteVehicleAccessParams{UserID: recipientID, VehicleID: vehicleID})
	})
}

// VehicleSharer returns the display name of the user who shared the vehicle
// with userID; ok is false when the vehicle didn't come through a share.
func (s *Roles) VehicleSharer(ctx context.Context, userID, vehicleID int64) (name string, ok bool, err error) {
	name, err = store.New(s.DB.Reader).GetVehicleSharer(ctx, store.GetVehicleSharerParams{UserID: userID, VehicleID: vehicleID})
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return name, err == nil, err
}

// pruneShares removes shares whose sharer no longer holds the vehicle in a
// way that lets them share it. Losing the share permission or being disabled
// keeps a user's shares; deleting the user removes them through the foreign
// key. Called inside every transaction that changes roles, a user's role or
// assignments.
func pruneShares(ctx context.Context, q *store.Queries) error {
	rows, err := q.ListAllShares(ctx)
	if err != nil {
		return err
	}
	access := map[int64]Access{}
	for _, r := range rows {
		a, ok := access[r.GrantedBy.Int64]
		if !ok {
			u, err := q.GetUser(ctx, r.GrantedBy.Int64)
			if err != nil {
				return err
			}
			if a, err = LoadAccess(ctx, q, FromStore(u)); err != nil {
				return err
			}
			access[r.GrantedBy.Int64] = a
		}
		if !a.holdsForSharing(r.VehicleID) {
			if err := q.DeleteVehicleAccess(ctx, store.DeleteVehicleAccessParams{UserID: r.UserID, VehicleID: r.VehicleID}); err != nil {
				return err
			}
		}
	}
	return nil
}
