package httpapi

import (
	"errors"
	"math"
	"net/http"
	"strings"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/auth"
	"github.com/app-torq/torq/internal/auth/local"
)

// formatsJSON carries the format and unit preferences. In requests an
// omitted (empty) field stays unchanged.
type formatsJSON struct {
	DateFormat       string `json:"date_format"`
	NumberFormat     string `json:"number_format"`
	TimeFormat       string `json:"time_format"`
	CurrencyCode     string `json:"currency_code"`
	PowerUnit        string `json:"power_unit"`
	DistanceUnit     string `json:"distance_unit"`
	VolumeUnit       string `json:"volume_unit"`
	TorqueUnit       string `json:"torque_unit"`
	PressureUnit     string `json:"pressure_unit"`
	DisplacementUnit string `json:"displacement_unit"`
}

func toFormatsJSON(f auth.Formats) formatsJSON {
	return formatsJSON{
		DateFormat: f.DateFormat, NumberFormat: f.NumberFormat, TimeFormat: f.TimeFormat, CurrencyCode: f.CurrencyCode,
		PowerUnit: f.PowerUnit, DistanceUnit: f.DistanceUnit, VolumeUnit: f.VolumeUnit, TorqueUnit: f.TorqueUnit,
		PressureUnit: f.PressureUnit, DisplacementUnit: f.DisplacementUnit,
	}
}

func (f formatsJSON) formats() auth.Formats {
	return auth.Formats{
		DateFormat: f.DateFormat, NumberFormat: f.NumberFormat, TimeFormat: f.TimeFormat, CurrencyCode: f.CurrencyCode,
		PowerUnit: f.PowerUnit, DistanceUnit: f.DistanceUnit, VolumeUnit: f.VolumeUnit, TorqueUnit: f.TorqueUnit,
		PressureUnit: f.PressureUnit, DisplacementUnit: f.DisplacementUnit,
	}
}

type userJSON struct {
	ID             int64   `json:"id"`
	Username       string  `json:"username"`
	DisplayName    string  `json:"display_name"`
	Role           string  `json:"role"`
	Locale         string  `json:"locale"`
	CurrencySymbol *string `json:"currency_symbol"`
	Disabled       bool    `json:"disabled"`
	formatsJSON
}

func toUserJSON(u auth.User) userJSON {
	out := userJSON{
		ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, Role: string(u.Role),
		Locale: u.Prefs.Locale, Disabled: u.Disabled, formatsJSON: toFormatsJSON(u.Prefs.Formats),
	}
	if u.Prefs.CurrencySymbol != "" {
		sym := u.Prefs.CurrencySymbol
		out.CurrencySymbol = &sym
	}
	return out
}

// meResponse adds instance facts every signed-in user needs (upload limit,
// version). Token is set only for a sign-in that asked for a bearer token.
type meResponse struct {
	User     userJSON     `json:"user"`
	Instance instanceInfo `json:"instance"`
	Token    string       `json:"token,omitempty"`
}

// signIn holds the optional fields of login and setup for app clients
// (research M1): Token asks for a bearer token instead of the session cookie.
type signIn struct {
	Token      bool   `json:"token"`
	DeviceName string `json:"device_name"`
}

// instanceInfo describes the server to clients, in GET /instance and in every
// sign-in response, so both always agree (FR-017).
type instanceInfo struct {
	APIVersion  int    `json:"api_version"`
	MaxUploadMB int    `json:"max_upload_mb"`
	Version     string `json:"version"`
	DemoMode    bool   `json:"demo_mode"`
	TokenAuth   bool   `json:"token_auth"`
	// SpecAutofill tells the UI whether the web spec lookup is configured
	// (GEMINI_API_KEY and TAVILY_API_KEY).
	SpecAutofill bool `json:"spec_autofill"`
}

func (s *Server) instanceInfo() instanceInfo {
	return instanceInfo{
		APIVersion: APIVersion, MaxUploadMB: s.Config.MaxUploadMB, Version: s.Version,
		DemoMode: s.Config.DemoMode, TokenAuth: true, SpecAutofill: s.specfill != nil,
	}
}

// instance lets a client check a server before signing in (research M6).
func (s *Server) instance(w http.ResponseWriter, r *http.Request) error {
	required, err := s.users.SetupRequired(r.Context())
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, struct {
		instanceInfo
		SetupRequired bool `json:"setup_required"`
	}{s.instanceInfo(), required})
	return nil
}

func (s *Server) setupStatus(w http.ResponseWriter, r *http.Request) error {
	required, err := s.users.SetupRequired(r.Context())
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]bool{"setup_required": required})
	return nil
}

func (s *Server) setup(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
		Locale      string `json:"locale"`
		signIn
		formatsJSON
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	user, err := s.users.Setup(r.Context(), auth.NewUser{
		Username: in.Username, DisplayName: in.DisplayName, Password: in.Password, Locale: in.Locale,
	}, in.formats())
	if err != nil {
		return err
	}
	token, err := s.startSession(w, r, user.ID, in.signIn)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, meResponse{User: toUserJSON(user), Instance: s.instanceInfo(), Token: token})
	return nil
}

// startSession creates a session for userID. It sets the session cookie, or,
// when the client asked for a token, returns the token instead.
func (s *Server) startSession(w http.ResponseWriter, r *http.Request, userID int64, in signIn) (string, error) {
	token, err := s.sessions.Create(r.Context(), userID, r.UserAgent(), auth.ClientIP(r, s.Config.TrustedProxies), in.DeviceName)
	if err != nil {
		return "", err
	}
	if in.Token {
		return token, nil
	}
	s.setSessionCookie(w, r, token)
	return "", nil
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
		signIn
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	username := strings.TrimSpace(in.Username)
	ip := "ip:" + auth.ClientIP(r, s.Config.TrustedProxies)
	ipKeys, userKeys := []string{ip}, []string(nil)
	// Over-long credentials can't match an account. They count against the IP
	// only, so they never reach argon2id or create big throttle entries.
	tooLong := len(username) > auth.MaxUsernameLength || len(in.Password) > local.MaxPasswordBytes
	if !tooLong {
		user := "user:" + strings.ToLower(username)
		ipKeys = append(ipKeys, user+"@"+ip)
		userKeys = []string{user}
	}
	if wait := max(s.throttle.RetryAfter(ipKeys...), s.userThrottle.RetryAfter(userKeys...)); wait > 0 {
		return apperr.Throttled(int(math.Ceil(wait.Seconds())))
	}
	userID, err := int64(0), local.ErrInvalidCredentials
	if !tooLong {
		userID, err = s.password.AuthenticatePassword(r.Context(), username, in.Password)
	}
	if errors.Is(err, local.ErrInvalidCredentials) {
		s.throttle.Fail(ipKeys...)
		s.userThrottle.Fail(userKeys...)
		return &apperr.Error{Status: http.StatusUnauthorized, Code: "auth.invalid_credentials"}
	}
	if err != nil {
		return err
	}
	// The IP key stays: logging into one account must not wipe the failures
	// collected against others.
	s.throttle.Succeed(ipKeys[1:]...)
	s.userThrottle.Succeed(userKeys...)
	user, err := s.users.Get(r.Context(), userID)
	if err != nil {
		return err
	}
	token, err := s.startSession(w, r, userID, in.signIn)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, meResponse{User: toUserJSON(user), Instance: s.instanceInfo(), Token: token})
	return nil
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) error {
	if id, ok := auth.SessionIDFrom(r.Context()); ok {
		if err := s.sessions.Revoke(r.Context(), id); err != nil {
			return err
		}
	}
	if !viaBearer(r) {
		s.clearSessionCookie(w, r)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) error {
	writeJSON(w, http.StatusOK, meResponse{User: toUserJSON(currentUser(r)), Instance: s.instanceInfo()})
	return nil
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	sessionID, _ := auth.SessionIDFrom(r.Context())
	if err := s.users.ChangeOwnPassword(r.Context(), currentUser(r).ID, sessionID, in.CurrentPassword, in.NewPassword); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) updatePreferences(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		DisplayName    *string `json:"display_name"`
		Locale         *string `json:"locale"`
		CurrencySymbol *string `json:"currency_symbol"`
		formatsJSON
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	user, err := s.users.UpdatePreferences(r.Context(), currentUser(r).ID, auth.PreferencesPatch{
		DisplayName: in.DisplayName, Locale: in.Locale, CurrencySymbol: in.CurrencySymbol, Formats: in.formats(),
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]userJSON{"user": toUserJSON(user)})
	return nil
}
